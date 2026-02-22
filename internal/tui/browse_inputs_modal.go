package tui

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	openbindings "github.com/openbindings/openbindings-go"
)

// showNewInputModal opens the modal for creating a new input.
func (m *model) showNewInputModal(opKey string) tea.Cmd {
	t := &m.tabs[m.active]

	// Create name input
	ti := textinput.New()
	ti.Placeholder = "input-name"
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 30

	// Generate a default name using workspace inputs
	baseName := "input"
	existingNames := m.getExistingInputNames(t.targetID, opKey)
	name := baseName
	for i := 1; existingNames[name]; i++ {
		name = baseName + "-" + strconv.Itoa(i)
	}
	ti.SetValue(name)
	ti.CursorEnd()

	// Create path input for existing file reference
	pathInput := textinput.New()
	pathInput.Placeholder = "/absolute/path/to/file.json"
	pathInput.CharLimit = 1024
	pathInput.Width = 50

	// Build template options
	var templates []templateOption

	// Blank option
	templates = append(templates, templateOption{
		ID:      "blank",
		Label:   "Blank",
		Preview: "{}",
	})

	// Schema-based template
	if op, ok := t.obi.Operations[opKey]; ok {
		schemaPreview := generateSchemaPreview(op, t.obi)
		templates = append(templates, templateOption{
			ID:      "schema",
			Label:   "From schema",
			Preview: schemaPreview,
		})

		// Examples from spec
		for exName, ex := range op.Examples {
			preview := "{}"
			if ex.Input != nil {
				if bytes, err := json.MarshalIndent(ex.Input, "", "  "); err == nil {
					preview = string(bytes)
				}
			}
			templates = append(templates, templateOption{
				ID:      "example:" + exName,
				Label:   "example: " + exName,
				Preview: preview,
			})
		}
	}

	// Reference existing file option (always last)
	templates = append(templates, templateOption{
		ID:      "existing",
		Label:   "Reference existing file",
		Preview: "", // No preview for this option
	})

	// Get interface name for context
	interfaceName := ""
	if t.obi != nil {
		if t.obi.Name != "" {
			interfaceName = t.obi.Name
		}
	}

	m.newInputModal = &newInputModalState{
		opKey:            opKey,
		interfaceName:    interfaceName,
		nameInput:        ti,
		nameTaken:        existingNames[name],
		templates:        templates,
		selectedTemplate: 1, // Default to "From schema" if available
		focusField:       0, // Start with name focused
		pathInput:        pathInput,
	}

	// If only blank and existing are available, select blank
	if len(templates) == 2 {
		m.newInputModal.selectedTemplate = 0
	}

	return textinput.Blink
}

// isExistingTemplate returns true if the selected template is "existing file"
func (m *model) isExistingTemplate() bool {
	modal := m.newInputModal
	if modal == nil || modal.selectedTemplate < 0 || modal.selectedTemplate >= len(modal.templates) {
		return false
	}
	return modal.templates[modal.selectedTemplate].ID == "existing"
}

// handleNewInputModalKeys handles keyboard input in the new input modal.
func (m *model) handleNewInputModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	modal := m.newInputModal
	t := &m.tabs[m.active]

	switch msg.String() {
	case "esc":
		// Cancel modal
		m.newInputModal = nil
		return m, nil

	case "enter":
		// Create the input if name is valid
		name := strings.TrimSpace(modal.nameInput.Value())
		if name == "" || modal.nameTaken {
			return m, nil
		}

		opKey := modal.opKey

		// Handle "existing file" template specially
		if m.isExistingTemplate() {
			path := strings.TrimSpace(modal.pathInput.Value())
			if path == "" {
				modal.pathError = "Path is required"
				return m, nil
			}

			// Expand ~ to home directory
			if strings.HasPrefix(path, "~/") {
				if home, err := os.UserHomeDir(); err == nil {
					path = home + path[1:]
				}
			}

			// Validate file exists
			info, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					modal.pathError = "File not found"
				} else if os.IsPermission(err) {
					modal.pathError = "Permission denied"
				} else {
					modal.pathError = "Cannot access file"
				}
				return m, nil
			}
			if info.IsDir() {
				modal.pathError = "Path is a directory, not a file"
				return m, nil
			}

			m.newInputModal = nil // Close modal

			// Just add association, don't create file
			// Note: addInputAssociation auto-creates global environment if needed
			m.addInputAssociation(t.targetID, opKey, name, path)
			if m.statusMsg == "" || !strings.HasPrefix(m.statusMsg, "Failed") {
				m.statusMsg = "Added \"" + name + "\""
			}
			return m, clearStatusAfter(2 * time.Second)
		}

		// Get the selected template content
		templateContent := "{}"
		if modal.selectedTemplate >= 0 && modal.selectedTemplate < len(modal.templates) {
			templateContent = modal.templates[modal.selectedTemplate].Preview
		}

		m.newInputModal = nil // Close modal

		// Create input file and add association to workspace
		return m, m.createWorkspaceInputCmd(t.targetID, opKey, name, templateContent)

	case "tab":
		// Cycle focus: name -> templates -> path (if existing) -> name
		isExisting := m.isExistingTemplate()
		if isExisting {
			// 3 fields: name (0), templates (1), path (2)
			modal.focusField = (modal.focusField + 1) % 3
		} else {
			// 2 fields: name (0), templates (1)
			modal.focusField = (modal.focusField + 1) % 2
		}

		// Update focus state
		modal.nameInput.Blur()
		modal.pathInput.Blur()
		if modal.focusField == 0 {
			modal.nameInput.Focus()
		} else if modal.focusField == 2 && isExisting {
			modal.pathInput.Focus()
		}
		return m, nil

	case "up", "k":
		// Move up in template list (when focused on templates)
		if modal.focusField == 1 && modal.selectedTemplate > 0 {
			modal.selectedTemplate--
			// Clear path error when changing templates
			modal.pathError = ""
		}
		return m, nil

	case "down", "j":
		// Move down in template list (when focused on templates)
		if modal.focusField == 1 && modal.selectedTemplate < len(modal.templates)-1 {
			modal.selectedTemplate++
			// Clear path error when changing templates
			modal.pathError = ""
		}
		return m, nil

	default:
		// If name field is focused, update the text input
		if modal.focusField == 0 {
			var cmd tea.Cmd
			modal.nameInput, cmd = modal.nameInput.Update(msg)

			// Check if name is taken using workspace inputs
			name := strings.TrimSpace(modal.nameInput.Value())
			existingNames := m.getExistingInputNames(t.targetID, modal.opKey)
			modal.nameTaken = existingNames[name]

			return m, cmd
		}

		// If path field is focused (for existing file), update path input
		if modal.focusField == 2 && m.isExistingTemplate() {
			var cmd tea.Cmd
			modal.pathInput, cmd = modal.pathInput.Update(msg)
			// Clear error when typing
			modal.pathError = ""
			return m, cmd
		}
		return m, nil
	}
}

// getExistingInputNames returns a set of existing input names for a target/operation.
func (m *model) getExistingInputNames(targetID, opKey string) map[string]bool {
	existingNames := make(map[string]bool)

	if m.workspace == nil || m.workspace.Inputs == nil {
		return existingNames
	}

	if targetInputs, ok := m.workspace.Inputs[targetID]; ok {
		if opInputs, ok := targetInputs[opKey]; ok {
			for name := range opInputs {
				existingNames[name] = true
			}
		}
	}

	return existingNames
}

// generateSchemaPreview creates a JSON preview from an operation's input schema.
func generateSchemaPreview(op openbindings.Operation, iface *openbindings.Interface) string {
	if op.Input == nil {
		return "{}"
	}

	// Resolve the schema (handling $ref)
	resolved := resolveSchemaFully(op.Input, iface)

	// Try to generate a template from the schema
	template := generateTemplateFromSchema(resolved, iface)
	bytes, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(bytes)
}

// generateTemplateFromSchema creates a template value from a JSON schema.
func generateTemplateFromSchema(schema map[string]any, iface *openbindings.Interface) any {
	if schema == nil {
		return map[string]any{}
	}

	// Resolve $ref first
	schema = resolveSchemaFully(schema, iface)

	schemaType, _ := schema["type"].(string)

	// Infer type from schema structure if not explicit
	if schemaType == "" {
		if _, hasProps := schema["properties"]; hasProps {
			schemaType = "object"
		} else if _, hasItems := schema["items"]; hasItems {
			schemaType = "array"
		} else if enum, hasEnum := schema["enum"].([]any); hasEnum && len(enum) > 0 {
			// Return first enum value as example
			return enum[0]
		} else if def, hasDef := schema["default"]; hasDef {
			// Use default value if provided
			return def
		}
	}

	switch schemaType {
	case "object":
		result := make(map[string]any)
		if props, ok := schema["properties"].(map[string]any); ok {
			for key, propSchema := range props {
				if propMap, ok := propSchema.(map[string]any); ok {
					result[key] = generateTemplateFromSchema(propMap, iface)
				}
			}
		}
		return result
	case "array":
		return []any{}
	case "string":
		// Check for format hints
		if format, ok := schema["format"].(string); ok {
			switch format {
			case "date":
				return "YYYY-MM-DD"
			case "date-time":
				return "YYYY-MM-DDT00:00:00Z"
			case "email":
				return "user@example.com"
			case "uri", "url":
				return "https://example.com"
			case "uuid":
				return "00000000-0000-0000-0000-000000000000"
			}
		}
		return ""
	case "number":
		return 0.0
	case "integer":
		return 0
	case "boolean":
		return false
	case "null":
		return nil
	default:
		// For truly unknown schemas, return an empty object as a safe default
		return map[string]any{}
	}
}
