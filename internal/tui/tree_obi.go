package tui

import (
	"fmt"
	"math"
	"sort"

	"github.com/openbindings/cli/internal/app"
	openbindings "github.com/openbindings/openbindings-go"
)

// InputSource represents a source for operation input data.
type InputSource struct {
	Name       string           // display name (the association name in workspace)
	SourceType string           // "file" | "example" | "new"
	Path       string           // file path/ref (for file type)
	ExampleKey string           // example key (for example type)
	Selected   bool             // whether this is the currently selected input
	Validation ValidationResult // schema validation result (for file type)
	Status     string           // "ok" | "missing" - whether the referenced file exists
}

// BuildOBITree creates a tree representation of an OpenBindings interface.
func BuildOBITree(iface *openbindings.Interface, opKeys []string) *TreeNode {
	if iface == nil {
		return &TreeNode{ID: "root", Label: "root"}
	}

	root := &TreeNode{
		ID:    "root",
		Label: "root",
	}

	// Build operation nodes
	for _, opKey := range opKeys {
		op, ok := iface.Operations[opKey]
		if !ok {
			continue
		}
		opNode := buildOperationNode(opKey, op, iface, nil) // nil inputFiles initially
		root.Children = append(root.Children, opNode)
	}

	return root
}

// buildOperationNode creates a tree node for an operation.
func buildOperationNode(opKey string, op openbindings.Operation, iface *openbindings.Interface, inputs []InputSource) *TreeNode {
	node := &TreeNode{
		ID:         opKey,
		Label:      opKey,
		Type:       NodeTypeOperation,
		Data:       &op,
		Actionable: true, // Operations can be run
	}

	// Build badge (kind + idempotent + deprecated)
	badge := fmt.Sprintf("[%s]", op.Kind)
	if op.Idempotent != nil && *op.Idempotent {
		badge += " [idempotent]"
	}
	if op.Deprecated {
		badge += " [deprecated]"
	}
	node.Badge = badge

	// Add Bindings section first (choose how to call)
	bindings := findBindingsForOp(opKey, iface)
	if len(bindings) > 0 {
		bindingsNode := buildBindingsNode(opKey, bindings, iface)
		node.Children = append(node.Children, bindingsNode)
	}

	// Add Inputs section (choose what to call with)
	inputsNode := buildInputsNode(opKey, op, inputs)
	node.Children = append(node.Children, inputsNode)

	// Add Aliases if present
	if len(op.Aliases) > 0 {
		aliasesNode := &TreeNode{
			ID:    opKey + ".aliases",
			Label: "Aliases",
			Type:  NodeTypeAliases,
			Badge: fmt.Sprintf("(%d)", len(op.Aliases)),
		}
		for i, alias := range op.Aliases {
			aliasesNode.Children = append(aliasesNode.Children, &TreeNode{
				ID:    fmt.Sprintf("%s.aliases.%d", opKey, i),
				Label: alias,
				Type:  NodeTypeAlias,
			})
		}
		node.Children = append(node.Children, aliasesNode)
	}

	// Add Satisfies if present
	if len(op.Satisfies) > 0 {
		satisfiesNode := &TreeNode{
			ID:    opKey + ".satisfies",
			Label: "Satisfies",
			Type:  NodeTypeSatisfies,
			Badge: fmt.Sprintf("(%d)", len(op.Satisfies)),
		}
		for i, s := range op.Satisfies {
			ref := s.Interface
			if s.Operation != "" {
				ref += "." + s.Operation
			}
			satisfiesNode.Children = append(satisfiesNode.Children, &TreeNode{
				ID:    fmt.Sprintf("%s.satisfies.%d", opKey, i),
				Label: ref,
				Type:  NodeTypeSatisfiesRef,
			})
		}
		node.Children = append(node.Children, satisfiesNode)
	}

	return node
}

// buildInputsNode creates the Inputs section for an operation.
func buildInputsNode(opKey string, op openbindings.Operation, inputs []InputSource) *TreeNode {
	// Count actual inputs (files + examples), not the "New..." action
	node := &TreeNode{
		ID:    opKey + ".inputs",
		Label: "Inputs",
		Type:  NodeTypeInputs,
		Badge: fmt.Sprintf("(%d)", len(inputs)),
	}

	// Add input files first
	for i, src := range inputs {
		src := src // Capture for closure
		label := src.Name
		badge := ""
		if src.SourceType == InputSourceExample {
			label = "example: " + src.Name
		}

		// Build badge: show [missing] for missing files, [invalid] for invalid, nothing for valid/unknown
		if src.SourceType == InputSourceFile {
			if src.Status == app.InputStatusMissing {
				badge = "[missing]"
			} else if src.Validation.Status == ValidationInvalid {
				badge = "[invalid]"
			}
		}
		if src.Selected && badge != "" {
			badge = badge + " ◀"
		} else if src.Selected {
			badge = "◀"
		}

		nodeType := NodeTypeInputFile
		if src.SourceType == InputSourceExample {
			nodeType = NodeTypeInputExample
		}

		node.Children = append(node.Children, &TreeNode{
			ID:         fmt.Sprintf("%s.inputs.%d", opKey, i),
			Label:      label,
			Type:       nodeType,
			Badge:      badge,
			Data:       &src,
			Actionable: true, // Can select/edit this input
		})
	}

	// Add "New..." action
	node.Children = append(node.Children, &TreeNode{
		ID:         opKey + ".inputs.new",
		Label:      "New...",
		Type:       NodeTypeInputNew,
		Actionable: true, // Can create new input
		Icon:       "+",  // Plus icon instead of bullet
	})

	return node
}

// bindingKeyEntry pairs a binding key with its entry for sorting.
type bindingKeyEntry struct {
	Key   string
	Entry openbindings.BindingEntry
}

// buildBindingsNode creates a tree node for operation bindings.
func buildBindingsNode(opKey string, bindings map[string]openbindings.BindingEntry, iface *openbindings.Interface) *TreeNode {
	node := &TreeNode{
		ID:    opKey + ".bindings",
		Label: "Bindings",
		Type:  NodeTypeBindings,
		Badge: fmt.Sprintf("(%d)", len(bindings)),
	}

	// Collect and sort by priority (lower is better; nil = +Inf)
	sorted := make([]bindingKeyEntry, 0, len(bindings))
	for k, b := range bindings {
		sorted = append(sorted, bindingKeyEntry{Key: k, Entry: b})
	}
	sort.Slice(sorted, func(i, j int) bool {
		pi, pj := math.MaxFloat64, math.MaxFloat64
		if sorted[i].Entry.Priority != nil {
			pi = *sorted[i].Entry.Priority
		}
		if sorted[j].Entry.Priority != nil {
			pj = *sorted[j].Entry.Priority
		}
		if pi != pj {
			return pi < pj
		}
		return sorted[i].Key < sorted[j].Key
	})

	for _, bke := range sorted {
		b := bke.Entry
		sourceFmt := b.Source
		if src, ok := iface.Sources[b.Source]; ok && src.Format != "" {
			sourceFmt = src.Format
		}

		label := bke.Key + " (" + sourceFmt
		if b.Ref != "" {
			label += " → " + b.Ref
		}
		label += ")"

		bindingNode := &TreeNode{
			ID:    opKey + ".bindings." + bke.Key,
			Label: label,
			Type:  NodeTypeBinding,
			Data:  &b,
		}

		if b.Deprecated {
			bindingNode.Badge = "[deprecated]"
		}

		node.Children = append(node.Children, bindingNode)
	}

	return node
}
