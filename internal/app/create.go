package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openbindings/cli/internal/delegates"
	"github.com/openbindings/openbindings-go"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// Default interface values when not provided
const (
	DefaultInterfaceName = "My Interface"
)

// CreateInterfaceSource represents a source for interface creation.
type CreateInterfaceSource struct {
	Format         string
	Location       string
	Name           string // key in sources
	OutputLocation string
	Description    string
	Embed          bool
}

// CreateInterfaceInput represents input for the createInterface operation.
type CreateInterfaceInput struct {
	OpenBindingsVersion string
	Sources             []CreateInterfaceSource
	Name                string
	Version             string
	Description         string
}

// RenderInterface returns a human-friendly summary of a created interface.
func RenderInterface(iface *openbindings.Interface) string {
	s := Styles
	var sb strings.Builder

	if iface == nil {
		return s.Dim.Render("No interface created")
	}

	sb.WriteString(s.Header.Render("Created OpenBindings Interface"))
	sb.WriteString("\n\n")

	if iface.Name != "" {
		sb.WriteString(s.Dim.Render("Name: "))
		sb.WriteString(iface.Name)
		sb.WriteString("\n")
	}

	if iface.Version != "" {
		sb.WriteString(s.Dim.Render("Version: "))
		sb.WriteString(iface.Version)
		sb.WriteString("\n")
	}

	sb.WriteString(s.Dim.Render("Operations: "))
	sb.WriteString(fmt.Sprintf("%d", len(iface.Operations)))
	sb.WriteString("\n")

	sb.WriteString(s.Dim.Render("Sources: "))
	sb.WriteString(fmt.Sprintf("%d", len(iface.Sources)))
	sb.WriteString("\n")

	sb.WriteString(s.Dim.Render("Bindings: "))
	sb.WriteString(fmt.Sprintf("%d", len(iface.Bindings)))

	return sb.String()
}

// ParseSource parses a source string in format:
// format:path[?option&option...]
//
// Options are specified after a '?' delimiter (like URL query params):
//   - name=X             Key name in sources
//   - outputLocation=Y   Location to use in output (instead of input path)
//   - description=Z      Description for this binding source
//   - embed              Embed content inline
//
// Example: usage@2.13.1:./cli.kdl?name=cli&embed
func ParseSource(s string) (CreateInterfaceSource, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return CreateInterfaceSource{}, fmt.Errorf("empty source")
	}

	// Split format:path from options (delimited by ?)
	mainPart := s
	optionsPart := ""
	if idx := strings.Index(s, "?"); idx >= 0 {
		mainPart = s[:idx]
		optionsPart = s[idx+1:]
	}

	// Parse format:path
	colonIdx := strings.Index(mainPart, ":")
	if colonIdx < 0 {
		return CreateInterfaceSource{}, fmt.Errorf("source must be format:path, got %q", s)
	}

	src := CreateInterfaceSource{
		Format:   mainPart[:colonIdx],
		Location: mainPart[colonIdx+1:],
	}

	if src.Format == "" {
		return CreateInterfaceSource{}, fmt.Errorf("source format cannot be empty")
	}
	if src.Location == "" {
		return CreateInterfaceSource{}, fmt.Errorf("source path cannot be empty")
	}

	// Parse options (& delimited)
	if optionsPart != "" {
		opts := strings.Split(optionsPart, "&")
		for _, opt := range opts {
			opt = strings.TrimSpace(opt)
			if opt == "" {
				continue
			}
			if opt == "embed" {
				src.Embed = true
				continue
			}

			// Handle key=value options
			if idx := strings.Index(opt, "="); idx > 0 {
				key := opt[:idx]
				value := opt[idx+1:]
				switch key {
				case "name":
					src.Name = value
				case "outputLocation":
					src.OutputLocation = value
				case "description":
					src.Description = value
				default:
					return CreateInterfaceSource{}, fmt.Errorf("unknown source option %q", key)
				}
			} else {
				return CreateInterfaceSource{}, fmt.Errorf("invalid source option %q (expected key=value or 'embed')", opt)
			}
		}
	}

	return src, nil
}

// deriveSourceKey generates a sensible default key for a binding source
// based on the format and file path.
func deriveSourceKey(src CreateInterfaceSource, index int) string {
	if src.Name != "" {
		return src.Name
	}

	// Try to derive from format name
	formatName := strings.Split(src.Format, "@")[0]

	// Try to get a base name from the path
	baseName := filepath.Base(src.Location)
	// Remove extension
	if ext := filepath.Ext(baseName); ext != "" {
		baseName = baseName[:len(baseName)-len(ext)]
	}
	// Remove common suffixes
	for _, suffix := range []string{".usage", ".openapi", ".asyncapi", ".spec"} {
		baseName = strings.TrimSuffix(baseName, suffix)
	}

	// Clean up the base name
	baseName = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1 // drop invalid chars
	}, baseName)

	if baseName != "" && len(baseName) <= 20 {
		return baseName + cases.Title(language.English).String(formatName) + "Spec"
	}

	// Fall back to format-based naming
	return fmt.Sprintf("%sSpec%d", formatName, index)
}

// CreateInterface creates an OpenBindings interface from the given input.
func CreateInterface(input CreateInterfaceInput) (*openbindings.Interface, error) {
	targetVersion := input.OpenBindingsVersion
	if targetVersion == "" {
		targetVersion = openbindings.MaxTestedVersion
	}

	ok, err := openbindings.IsSupportedVersion(targetVersion)
	if err != nil || !ok {
		return nil, fmt.Errorf("unsupported openbindings version %q", targetVersion)
	}

	iface := openbindings.Interface{
		OpenBindings: targetVersion,
		Name:         DefaultInterfaceName,
		Operations:   map[string]openbindings.Operation{},
		Sources:      map[string]openbindings.Source{},
		Bindings:     map[string]openbindings.BindingEntry{},
	}

	for i, src := range input.Sources {
		if err := processSource(&iface, src, i); err != nil {
			return nil, fmt.Errorf("source %s (%s): %w", src.Location, src.Format, err)
		}
	}

	if input.Name != "" {
		iface.Name = input.Name
	}
	if input.Version != "" {
		iface.Version = input.Version
	}
	if input.Description != "" {
		iface.Description = input.Description
	}

	return &iface, nil
}

// processSource processes a single source and adds its operations/bindings to the interface.
// It uses the handler registry to dispatch format-specific conversion, then applies
// format-agnostic merge logic.
func processSource(iface *openbindings.Interface, src CreateInterfaceSource, index int) error {
	// Determine binding source key using smart derivation.
	sourceKey := deriveSourceKey(src, index)

	// Look up the handler for this format.
	handler, err := DefaultRegistry().ForFormat(src.Format)
	if err != nil {
		return err
	}

	// Let the handler convert the source to an Interface.
	generated, err := handler.CreateInterface(delegates.Source{
		Format:   src.Format,
		Location: src.Location,
	})
	if err != nil {
		return err
	}

	// Merge generated content into the target interface.
	return mergeGeneratedSource(iface, &generated, src, sourceKey)
}

// mergeGeneratedSource merges a handler-generated Interface into the target,
// applying format-agnostic merge logic for metadata, operations, sources, and bindings.
// It writes x-ob metadata on sources and marks generated operations/bindings as managed.
func mergeGeneratedSource(iface *openbindings.Interface, generated *openbindings.Interface, src CreateInterfaceSource, sourceKey string) error {
	// Merge metadata from first source if not set.
	if iface.Name == DefaultInterfaceName && generated.Name != "" {
		iface.Name = generated.Name
	}
	if iface.Description == "" && generated.Description != "" {
		iface.Description = generated.Description
	}
	if iface.Version == "" && generated.Version != "" {
		iface.Version = generated.Version
	}

	// Add operations, marking each as managed (x-ob: {}).
	for key, op := range generated.Operations {
		if _, exists := iface.Operations[key]; exists {
			return fmt.Errorf("duplicate operation %q", key)
		}
		SetXOB(&op.LosslessFields)
		iface.Operations[key] = op
	}

	// Create source entry.
	bsrc := openbindings.Source{
		Format:      src.Format,
		Description: src.Description,
	}

	// Determine resolve mode and build x-ob metadata.
	var resolveMode string
	if src.Embed {
		resolveMode = ResolveModeContent
	} else {
		resolveMode = ResolveModeLocation
	}

	meta := SourceMeta{
		Ref:     src.Location,
		Resolve: resolveMode,
	}

	// If OutputLocation is set, use it as the published URI.
	if src.OutputLocation != "" && src.OutputLocation != src.Location {
		meta.URI = src.OutputLocation
	}

	if src.Embed {
		// Read and embed content.
		content, err := readEmbedContent(src.Location)
		if err != nil {
			return fmt.Errorf("embed content: %w", err)
		}
		bsrc.Content = content

		// Compute contentHash from the source file.
		data, err := os.ReadFile(src.Location)
		if err == nil {
			meta.ContentHash = HashContent(data)
		}
	} else {
		// Use outputLocation if provided, otherwise input location.
		if src.OutputLocation != "" {
			bsrc.Location = src.OutputLocation
		} else {
			bsrc.Location = src.Location
		}

		// Compute contentHash from the source file.
		data, err := os.ReadFile(src.Location)
		if err == nil {
			meta.ContentHash = HashContent(data)
		}
	}

	// Set sync timestamps.
	meta.LastSynced = NowISO()
	meta.OBVersion = OBVersion

	// Write x-ob metadata onto the source.
	if err := SetSourceMeta(&bsrc, meta); err != nil {
		return fmt.Errorf("set source x-ob: %w", err)
	}

	iface.Sources[sourceKey] = bsrc

	// Add bindings, remapping source key. Mark each as managed (x-ob: {}).
	if iface.Bindings == nil {
		iface.Bindings = map[string]openbindings.BindingEntry{}
	}
	for _, binding := range generated.Bindings {
		bindingKey := binding.Operation + "." + sourceKey
		entry := openbindings.BindingEntry{
			Operation: binding.Operation,
			Source:    sourceKey,
			Ref:       binding.Ref,
		}
		SetXOB(&entry.LosslessFields)
		iface.Bindings[bindingKey] = entry
	}

	return nil
}

// readEmbedContent reads a file and returns its content as map[string]any for embedding.
// Supports JSON and YAML files. For other formats, returns an error with guidance.
func readEmbedContent(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".json":
		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		return result, nil

	case ".yaml", ".yml":
		var result map[string]any
		if err := yaml.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
		return result, nil

	default:
		// Non-JSON/YAML formats (KDL, protobuf, etc.) are embedded as raw string content.
		return string(data), nil
	}
}

