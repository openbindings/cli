package grpc

import (
	"fmt"

	"github.com/jhump/protoreflect/desc"
	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
	"google.golang.org/protobuf/types/descriptorpb"
)

// DefaultSourceName is the default source key for gRPC sources.
const DefaultSourceName = "grpcServer"

// ConvertToInterface converts gRPC discovery results to an OpenBindings interface.
func ConvertToInterface(disc *Discovery, sourceLocation string) (openbindings.Interface, error) {
	if disc == nil {
		return openbindings.Interface{}, fmt.Errorf("nil discovery result")
	}

	iface := openbindings.Interface{
		OpenBindings: openbindings.MaxTestedVersion,
		Operations:   map[string]openbindings.Operation{},
		Bindings:     map[string]openbindings.BindingEntry{},
		Sources: map[string]openbindings.Source{
			DefaultSourceName: {
				Format:   FormatToken,
				Location: sourceLocation,
			},
		},
	}

	usedKeys := map[string]string{}

	for _, svc := range disc.Services {
		for _, method := range svc.GetMethods() {
			if method.IsClientStreaming() {
				continue
			}

			fqn := svc.GetFullyQualifiedName() + "/" + method.GetName()
			opKey := delegates.SanitizeKey(method.GetName())
			opKey = resolveKeyCollision(opKey, svc.GetName(), usedKeys)
			usedKeys[opKey] = fqn

			kind := openbindings.OperationKindMethod
			if method.IsServerStreaming() {
				kind = openbindings.OperationKindEvent
			}

			op := openbindings.Operation{
				Kind:        kind,
				Description: commentToDescription(method),
			}

			inputType := method.GetInputType()
			if inputType != nil {
				op.Input = messageToJSONSchema(inputType)
			}

			outputType := method.GetOutputType()
			if outputType != nil {
				if method.IsServerStreaming() {
					op.Payload = messageToJSONSchema(outputType)
				} else {
					op.Output = messageToJSONSchema(outputType)
				}
			}

			iface.Operations[opKey] = op

			bindingKey := opKey + "." + DefaultSourceName
			iface.Bindings[bindingKey] = openbindings.BindingEntry{
				Operation: opKey,
				Source:    DefaultSourceName,
				Ref:       fqn,
			}
		}
	}

	if len(disc.Services) > 0 {
		svc := disc.Services[0]
		iface.Name = svc.GetName()
		if len(disc.Services) > 1 {
			iface.Name = packageName(svc)
		}
	}

	return iface, nil
}

func packageName(svc *desc.ServiceDescriptor) string {
	pkg := svc.GetFile().GetPackage()
	if pkg != "" {
		return pkg
	}
	return svc.GetName()
}

func commentToDescription(method *desc.MethodDescriptor) string {
	info := method.GetSourceInfo()
	if info != nil && info.GetLeadingComments() != "" {
		return trimComment(info.GetLeadingComments())
	}
	return ""
}

func trimComment(s string) string {
	if len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s
}

// messageToJSONSchema converts a protobuf message descriptor to JSON Schema.
func messageToJSONSchema(msg *desc.MessageDescriptor) map[string]any {
	schema := map[string]any{
		"type": "object",
	}

	fields := msg.GetFields()
	if len(fields) == 0 {
		return schema
	}

	properties := map[string]any{}
	for _, field := range fields {
		properties[field.GetJSONName()] = fieldToSchema(field)
	}
	schema["properties"] = properties

	return schema
}

func fieldToSchema(field *desc.FieldDescriptor) map[string]any {
	if field.IsMap() {
		valField := field.GetMapValueType()
		return map[string]any{
			"type":                 "object",
			"additionalProperties": scalarOrMessageSchema(valField),
		}
	}

	s := scalarOrMessageSchema(field)

	if field.IsRepeated() && !field.IsMap() {
		return map[string]any{
			"type":  "array",
			"items": s,
		}
	}

	return s
}

func scalarOrMessageSchema(field *desc.FieldDescriptor) map[string]any {
	t := field.GetType()
	switch t {
	case descriptorpb.FieldDescriptorProto_TYPE_BOOL:
		return map[string]any{"type": "boolean"}

	case descriptorpb.FieldDescriptorProto_TYPE_INT32,
		descriptorpb.FieldDescriptorProto_TYPE_SINT32,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED32,
		descriptorpb.FieldDescriptorProto_TYPE_UINT32,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED32:
		return map[string]any{"type": "integer"}

	case descriptorpb.FieldDescriptorProto_TYPE_INT64,
		descriptorpb.FieldDescriptorProto_TYPE_SINT64,
		descriptorpb.FieldDescriptorProto_TYPE_SFIXED64,
		descriptorpb.FieldDescriptorProto_TYPE_UINT64,
		descriptorpb.FieldDescriptorProto_TYPE_FIXED64:
		return map[string]any{"type": "string"}

	case descriptorpb.FieldDescriptorProto_TYPE_FLOAT,
		descriptorpb.FieldDescriptorProto_TYPE_DOUBLE:
		return map[string]any{"type": "number"}

	case descriptorpb.FieldDescriptorProto_TYPE_STRING:
		return map[string]any{"type": "string"}

	case descriptorpb.FieldDescriptorProto_TYPE_BYTES:
		return map[string]any{"type": "string", "contentEncoding": "base64"}

	case descriptorpb.FieldDescriptorProto_TYPE_ENUM:
		enumDesc := field.GetEnumType()
		if enumDesc != nil {
			var values []any
			for _, v := range enumDesc.GetValues() {
				values = append(values, v.GetName())
			}
			return map[string]any{"type": "string", "enum": values}
		}
		return map[string]any{"type": "string"}

	case descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		descriptorpb.FieldDescriptorProto_TYPE_GROUP:
		msgDesc := field.GetMessageType()
		if msgDesc != nil {
			return messageToJSONSchema(msgDesc)
		}
		return map[string]any{"type": "object"}

	default:
		return map[string]any{"type": "string"}
	}
}

func resolveKeyCollision(key string, prefix string, used map[string]string) string {
	if _, taken := used[key]; !taken {
		return key
	}
	candidate := delegates.SanitizeKey(prefix) + "_" + key
	if _, taken := used[candidate]; !taken {
		return candidate
	}
	for i := 2; ; i++ {
		numbered := fmt.Sprintf("%s_%d", candidate, i)
		if _, taken := used[numbered]; !taken {
			return numbered
		}
	}
}
