package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/shlex"
	openbindings "github.com/openbindings/openbindings-go"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// InputRef is a reference to input data (initially a file path).
type InputRef string

// InputEntry is a named input association.
type InputEntry struct {
	Name string   `json:"name"`
	Ref  InputRef `json:"ref"`
}

// InputsListOutput lists inputs for a target+operation.
type InputsListOutput struct {
	Workspace string       `json:"workspace"`
	TargetID  string       `json:"targetId"`
	OpKey     string       `json:"opKey"`
	Inputs    []InputEntry `json:"inputs"`
}

func (o InputsListOutput) Render() string {
	if len(o.Inputs) == 0 {
		return Styles.Dim.Render("No inputs")
	}
	var sb strings.Builder
	sb.WriteString(Styles.Header.Render("Inputs:"))
	for _, in := range o.Inputs {
		sb.WriteString("\n  ")
		sb.WriteString(Styles.Bullet.Render("•"))
		sb.WriteString(" ")
		sb.WriteString(Styles.Key.Render(in.Name))
		sb.WriteString(Styles.Dim.Render(" → "))
		sb.WriteString(string(in.Ref))
	}
	return sb.String()
}

// InputsShowOutput shows one input association.
type InputsShowOutput struct {
	Workspace string   `json:"workspace"`
	TargetID  string   `json:"targetId"`
	OpKey     string   `json:"opKey"`
	Name      string   `json:"name"`
	Ref       InputRef `json:"ref"`
}

func (o InputsShowOutput) Render() string {
	return fmt.Sprintf("%s → %s", o.Name, o.Ref)
}

// InputsMutateOutput is returned by add/remove/create operations.
type InputsMutateOutput struct {
	Workspace string   `json:"workspace"`
	TargetID  string   `json:"targetId"`
	OpKey     string   `json:"opKey"`
	Name      string   `json:"name"`
	Ref       InputRef `json:"ref,omitempty"`
	Action    string   `json:"action"` // added|removed|created|deleted
}

func (o InputsMutateOutput) Render() string {
	s := Styles
	switch o.Action {
	case "removed":
		return s.Success.Render("ok") + " removed input " + s.Key.Render(o.Name)
	case "deleted":
		if o.Ref != "" {
			return s.Success.Render("ok") + " deleted file and removed input " + s.Key.Render(o.Name) + s.Dim.Render(" → ") + string(o.Ref)
		}
		return s.Success.Render("ok") + " deleted file and removed input " + s.Key.Render(o.Name)
	case "added", "created":
		if o.Ref != "" {
			return s.Success.Render("ok") + " " + o.Action + " input " + s.Key.Render(o.Name) + s.Dim.Render(" → ") + string(o.Ref)
		}
		return s.Success.Render("ok") + " " + o.Action + " input " + s.Key.Render(o.Name)
	default:
		if o.Ref != "" {
			return s.Success.Render("ok") + " " + o.Action + " input " + s.Key.Render(o.Name) + s.Dim.Render(" → ") + string(o.Ref)
		}
		return s.Success.Render("ok") + " " + o.Action + " input " + s.Key.Render(o.Name)
	}
}

// InputsValidateOutput validates a named input against an operation schema.
type InputsValidateOutput struct {
	Workspace string `json:"workspace"`
	TargetID  string `json:"targetId"`
	OpKey     string `json:"opKey"`
	Name      string `json:"name"`
	Ref       string `json:"ref"`

	Status  string `json:"status"`            // unknown|valid|invalid|error
	Message string `json:"message,omitempty"` // details for invalid/error
}

func (o InputsValidateOutput) Render() string {
	switch o.Status {
	case "valid":
		return Styles.Success.Render("valid")
	case "invalid":
		return Styles.Error.Render("invalid") + ": " + o.Message
	case "error":
		return Styles.Error.Render("error") + ": " + o.Message
	default:
		return Styles.Dim.Render("unknown") + ": " + o.Message
	}
}

// InputsAdd adds an association (name -> ref) to the active workspace.
func InputsAdd(targetID, opKey, name, ref string, force bool) (InputsMutateOutput, error) {
	ws, _, envPath, err := RequireActiveWorkspace()
	if err != nil {
		return InputsMutateOutput{}, err
	}
	if strings.TrimSpace(targetID) == "" || strings.TrimSpace(opKey) == "" || strings.TrimSpace(name) == "" || strings.TrimSpace(ref) == "" {
		return InputsMutateOutput{}, ExitResult{Code: 2, Message: "target, op, name, and ref are required", ToStderr: true}
	}

	// Ensure target exists
	if FindTargetByID(ws, targetID) == nil {
		return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("target %q not found in workspace", targetID), ToStderr: true}
	}

	ws.Inputs = ensureInputs(ws.Inputs)
	ws.Inputs[targetID] = ensureOpInputs(ws.Inputs[targetID])
	ws.Inputs[targetID][opKey] = ensureNameInputs(ws.Inputs[targetID][opKey])

	if !force {
		if _, exists := ws.Inputs[targetID][opKey][name]; exists {
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("input %q already exists (use --force to overwrite)", name), ToStderr: true}
		}
	}

	ws.Inputs[targetID][opKey][name] = ref
	if err := SaveWorkspace(envPath, ws); err != nil {
		return InputsMutateOutput{}, err
	}
	return InputsMutateOutput{Workspace: ws.Name, TargetID: targetID, OpKey: opKey, Name: name, Ref: InputRef(ref), Action: "added"}, nil
}

// InputsRemove removes an association from the active workspace.
// If deleteFile is true, it also deletes the referenced local file path, with safety checks.
func InputsRemove(targetID, opKey, name string, deleteFile bool, force bool) (InputsMutateOutput, error) {
	ws, _, envPath, err := RequireActiveWorkspace()
	if err != nil {
		return InputsMutateOutput{}, err
	}
	if strings.TrimSpace(targetID) == "" || strings.TrimSpace(opKey) == "" || strings.TrimSpace(name) == "" {
		return InputsMutateOutput{}, ExitResult{Code: 2, Message: "target, op, and name are required", ToStderr: true}
	}

	ops, ok := ws.Inputs[targetID]
	if !ok {
		return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("no inputs for target %q", targetID), ToStderr: true}
	}
	names, ok := ops[opKey]
	if !ok {
		return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("no inputs for operation %q (target %q)", opKey, targetID), ToStderr: true}
	}
	ref, ok := names[name]
	if !ok {
		return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("input %q not found for operation %q (target %q)", name, opKey, targetID), ToStderr: true}
	}

	if deleteFile {
		// Only allow deleting local files
		if strings.HasPrefix(ref, "exec:") || strings.Contains(ref, "://") {
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: "ref is not a local file path; refusing to delete", ToStderr: true}
		}

		// Ensure file exists
		if _, err := os.Stat(ref); err != nil {
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("file not found: %s", ref), ToStderr: true}
		}

		// Check if referenced elsewhere in this environment (across workspaces)
		if !force {
			n, err := countInputRefOccurrencesInEnv(envPath, ref)
			if err != nil {
				return InputsMutateOutput{}, ExitResult{Code: 1, Message: "cannot verify whether file is referenced elsewhere; treating as in use (use -f to force): " + err.Error(), ToStderr: true}
			}
			if n > 1 {
				return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("file is referenced by %d inputs in this environment; refusing to delete (use -f to force)", n), ToStderr: true}
			}
		}

		// Stage delete by renaming (so we can rollback if workspace save fails).
		tmpPath, err := uniqueDeleteTempPath(ref)
		if err != nil {
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: "cannot create temp path for delete: " + err.Error(), ToStderr: true}
		}
		if err := os.Rename(ref, tmpPath); err != nil {
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("failed to prepare file for deletion: %v", err), ToStderr: true}
		}

		// Remove association and save workspace.
		delete(names, name)
		if len(names) == 0 {
			delete(ops, opKey)
		}
		if len(ops) == 0 {
			delete(ws.Inputs, targetID)
		}
		if err := SaveWorkspace(envPath, ws); err != nil {
			// Roll back file rename.
			if restoreErr := os.Rename(tmpPath, ref); restoreErr != nil {
				return InputsMutateOutput{}, ExitResult{
					Code:     1,
					Message:  fmt.Sprintf("failed to save workspace after preparing file delete: %v; additionally failed to restore original file: %v", err, restoreErr),
					ToStderr: true,
				}
			}
			return InputsMutateOutput{}, err
		}

		// Finalize file delete.
		if err := os.Remove(tmpPath); err != nil {
			// Rollback: restore file and re-add association.
			restoreErr := os.Rename(tmpPath, ref)
			ws.Inputs = ensureInputs(ws.Inputs)
			ws.Inputs[targetID] = ensureOpInputs(ws.Inputs[targetID])
			ws.Inputs[targetID][opKey] = ensureNameInputs(ws.Inputs[targetID][opKey])
			ws.Inputs[targetID][opKey][name] = ref
			restoreWorkspaceErr := SaveWorkspace(envPath, ws)
			if restoreErr != nil || restoreWorkspaceErr != nil {
				msg := fmt.Sprintf("failed to delete file and rollback may be incomplete: %v", err)
				if restoreErr != nil {
					msg += fmt.Sprintf("; failed to restore file: %v", restoreErr)
				}
				if restoreWorkspaceErr != nil {
					msg += fmt.Sprintf("; failed to restore workspace: %v", restoreWorkspaceErr)
				}
				return InputsMutateOutput{}, ExitResult{Code: 1, Message: msg, ToStderr: true}
			}
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("failed to delete file; no changes applied: %v", err), ToStderr: true}
		}

		return InputsMutateOutput{Workspace: ws.Name, TargetID: targetID, OpKey: opKey, Name: name, Ref: InputRef(ref), Action: "deleted"}, nil
	}

	// unlink only
	delete(names, name)
	if len(names) == 0 {
		delete(ops, opKey)
	}
	if len(ops) == 0 {
		delete(ws.Inputs, targetID)
	}
	if err := SaveWorkspace(envPath, ws); err != nil {
		return InputsMutateOutput{}, err
	}
	return InputsMutateOutput{Workspace: ws.Name, TargetID: targetID, OpKey: opKey, Name: name, Ref: InputRef(ref), Action: "removed"}, nil
}

// InputsList lists associations for a target+operation.
func InputsList(targetID, opKey string) (InputsListOutput, error) {
	ws, _, _, err := RequireActiveWorkspace()
	if err != nil {
		return InputsListOutput{}, err
	}
	if strings.TrimSpace(targetID) == "" || strings.TrimSpace(opKey) == "" {
		return InputsListOutput{}, ExitResult{Code: 2, Message: "target and op are required", ToStderr: true}
	}
	var inputs []InputEntry
	if ws.Inputs != nil {
		if ops, ok := ws.Inputs[targetID]; ok {
			if names, ok := ops[opKey]; ok {
				for n, r := range names {
					inputs = append(inputs, InputEntry{Name: n, Ref: InputRef(r)})
				}
			}
		}
	}
	// Stable output: sort by name
	sortInputs(inputs)
	return InputsListOutput{Workspace: ws.Name, TargetID: targetID, OpKey: opKey, Inputs: inputs}, nil
}

// InputsShow shows one association.
func InputsShow(targetID, opKey, name string) (InputsShowOutput, error) {
	ws, _, _, err := RequireActiveWorkspace()
	if err != nil {
		return InputsShowOutput{}, err
	}
	ref, ok := lookupInput(ws, targetID, opKey, name)
	if !ok {
		return InputsShowOutput{}, ExitResult{Code: 1, Message: "input not found", ToStderr: true}
	}
	return InputsShowOutput{Workspace: ws.Name, TargetID: targetID, OpKey: opKey, Name: name, Ref: InputRef(ref)}, nil
}

// InputsCreate creates a JSON file at path (or default) and associates it.
func InputsCreate(targetID, opKey, name, path, template string, force bool, openEditor bool) (InputsMutateOutput, error) {
	ws, _, envPath, err := RequireActiveWorkspace()
	if err != nil {
		return InputsMutateOutput{}, err
	}

	if FindTargetByID(ws, targetID) == nil {
		return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("target %q not found in workspace", targetID), ToStderr: true}
	}
	if strings.TrimSpace(opKey) == "" || strings.TrimSpace(name) == "" {
		return InputsMutateOutput{}, ExitResult{Code: 2, Message: "op and name are required", ToStderr: true}
	}

	if strings.TrimSpace(path) == "" {
		path = defaultInputPath(targetID, opKey, name)
	}

	if err := os.MkdirAll(filepath.Dir(path), DirPerm); err != nil {
		return InputsMutateOutput{}, err
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("file %q already exists (use --force to overwrite)", path), ToStderr: true}
		}
	}

	content := []byte("{}\n")
	if template == "schema" {
		// Best effort: if we can fetch schema, generate template.
		if t, err := generateSchemaTemplateForTargetOp(ws, targetID, opKey); err == nil && t != nil {
			if b, err := json.MarshalIndent(t, "", "  "); err == nil {
				content = append(b, '\n')
			}
		}
	}
	if err := AtomicWriteFile(path, content, FilePerm); err != nil {
		return InputsMutateOutput{}, err
	}

	// Associate (overwriting if force)
	ws.Inputs = ensureInputs(ws.Inputs)
	ws.Inputs[targetID] = ensureOpInputs(ws.Inputs[targetID])
	ws.Inputs[targetID][opKey] = ensureNameInputs(ws.Inputs[targetID][opKey])
	if !force {
		if _, exists := ws.Inputs[targetID][opKey][name]; exists {
			return InputsMutateOutput{}, ExitResult{Code: 1, Message: fmt.Sprintf("input %q already exists (use --force to overwrite)", name), ToStderr: true}
		}
	}
	ws.Inputs[targetID][opKey][name] = path
	if err := SaveWorkspace(envPath, ws); err != nil {
		return InputsMutateOutput{}, err
	}

	if openEditor {
		if err := openInEditor(ws.Settings.Editor, path); err != nil {
			return InputsMutateOutput{}, err
		}
	}
	return InputsMutateOutput{Workspace: ws.Name, TargetID: targetID, OpKey: opKey, Name: name, Ref: InputRef(path), Action: "created"}, nil
}

// InputsEdit opens the referenced file in the user's editor.
func InputsEdit(targetID, opKey, name string) error {
	ws, _, _, err := RequireActiveWorkspace()
	if err != nil {
		return err
	}
	ref, ok := lookupInput(ws, targetID, opKey, name)
	if !ok {
		return ExitResult{Code: 1, Message: "input not found", ToStderr: true}
	}
	return openInEditor(ws.Settings.Editor, ref)
}

// InputsValidate validates the referenced file against the operation schema (best effort).
func InputsValidate(targetID, opKey, name string, timeout time.Duration) (InputsValidateOutput, error) {
	ws, _, _, err := RequireActiveWorkspace()
	if err != nil {
		return InputsValidateOutput{}, err
	}
	ref, ok := lookupInput(ws, targetID, opKey, name)
	if !ok {
		return InputsValidateOutput{}, ExitResult{Code: 1, Message: "input not found", ToStderr: true}
	}
	target := FindTargetByID(ws, targetID)
	if target == nil {
		return InputsValidateOutput{}, ExitResult{Code: 1, Message: "target not found", ToStderr: true}
	}

	res := InputsValidateOutput{
		Workspace: ws.Name,
		TargetID:  targetID,
		OpKey:     opKey,
		Name:      name,
		Ref:       ref,
		Status:    "unknown",
		Message:   "no schema available",
	}

	// Parse JSON input first
	data, err := os.ReadFile(ref)
	if err != nil {
		res.Status = "error"
		res.Message = "read error: " + err.Error()
		return res, nil
	}
	var inputData any
	if err := json.Unmarshal(data, &inputData); err != nil {
		res.Status = "error"
		res.Message = "JSON parse error: " + err.Error()
		return res, nil
	}

	// Fetch OBI to get schema
	probed := ProbeOBI(target.URL, timeout)
	if probed.Status != "ok" || strings.TrimSpace(probed.OBI) == "" {
		res.Status = "unknown"
		res.Message = "could not fetch OBI"
		return res, nil
	}
	var iface openbindings.Interface
	if err := json.Unmarshal([]byte(probed.OBI), &iface); err != nil {
		res.Status = "unknown"
		res.Message = "could not parse OBI"
		return res, nil
	}
	op, ok := iface.Operations[opKey]
	if !ok || op.Input == nil {
		res.Status = "unknown"
		res.Message = "no schema for operation"
		return res, nil
	}

	status, msg := validateInputAgainstOperationSchema(inputData, op, &iface)
	res.Status = status
	res.Message = msg
	if res.Status == "valid" {
		res.Message = ""
	}
	return res, nil
}

// ---- helpers ----

func ensureInputs(in map[string]map[string]map[string]string) map[string]map[string]map[string]string {
	if in == nil {
		return make(map[string]map[string]map[string]string)
	}
	return in
}

func ensureOpInputs(in map[string]map[string]string) map[string]map[string]string {
	if in == nil {
		return make(map[string]map[string]string)
	}
	return in
}

func ensureNameInputs(in map[string]string) map[string]string {
	if in == nil {
		return make(map[string]string)
	}
	return in
}

func lookupInput(ws *Workspace, targetID, opKey, name string) (string, bool) {
	if ws == nil || ws.Inputs == nil {
		return "", false
	}
	ops, ok := ws.Inputs[targetID]
	if !ok {
		return "", false
	}
	names, ok := ops[opKey]
	if !ok {
		return "", false
	}
	ref, ok := names[name]
	return ref, ok
}

func countInputRefOccurrences(ws *Workspace, ref string) int {
	if ws == nil || ws.Inputs == nil {
		return 0
	}
	n := 0
	for _, ops := range ws.Inputs {
		for _, names := range ops {
			for _, r := range names {
				if r == ref {
					n++
				}
			}
		}
	}
	return n
}

func countInputRefOccurrencesInEnv(envPath, ref string) (int, error) {
	names, err := ListWorkspaces(envPath)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, name := range names {
		wsPath := WorkspacePath(envPath, name)
		ws, err := LoadWorkspace(wsPath)
		if err != nil {
			return 0, fmt.Errorf("failed to load workspace %q (%s): %w", name, wsPath, err)
		}
		total += countInputRefOccurrences(ws, ref)
	}
	return total, nil
}

func sortInputs(in []InputEntry) {
	sort.Slice(in, func(i, j int) bool {
		return in[i].Name < in[j].Name
	})
}

func defaultInputPath(targetID, opKey, name string) string {
	// keep out of .openbindings/ so it’s naturally shareable/trackable
	opSlug := slugifyForPath(opKey)
	return filepath.Join("inputs", targetID, opSlug, name+".json")
}

func slugifyForPath(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.ReplaceAll(s, ".", "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "op"
	}
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

func openInEditor(preferredEditor, path string) error {
	editor := strings.TrimSpace(preferredEditor)
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		for _, e := range []string{"code", "vim", "nano", "vi"} {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found (set workspace settings.editor or $EDITOR)")
	}

	parts, err := shlex.Split(editor)
	if err != nil || len(parts) == 0 {
		return fmt.Errorf("invalid editor command %q", editor)
	}
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func uniqueDeleteTempPath(path string) (string, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	for i := 0; i < 10; i++ {
		suffix := randomHex(4)
		cand := filepath.Join(dir, base+".ob-delete."+suffix)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand, nil
		}
	}
	return "", fmt.Errorf("could not find unique temp name")
}

func randomHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		// Best-effort fallback; collisions are extremely unlikely to matter here.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func generateSchemaTemplateForTargetOp(ws *Workspace, targetID, opKey string) (map[string]any, error) {
	target := FindTargetByID(ws, targetID)
	if target == nil {
		return nil, fmt.Errorf("target not found")
	}
	probed := ProbeOBI(target.URL, 2*time.Second)
	if probed.Status != "ok" || strings.TrimSpace(probed.OBI) == "" {
		return nil, fmt.Errorf("failed to fetch OBI")
	}
	var iface openbindings.Interface
	if err := json.Unmarshal([]byte(probed.OBI), &iface); err != nil {
		return nil, err
	}
	op, ok := iface.Operations[opKey]
	if !ok || op.Input == nil {
		return nil, fmt.Errorf("no schema")
	}
	return generateInputTemplate(op.Input), nil
}

func generateInputTemplate(schema openbindings.JSONSchema) map[string]any {
	if schema == nil {
		return map[string]any{}
	}
	result := map[string]any{}
	if props, ok := schema["properties"].(map[string]any); ok {
		for name, propRaw := range props {
			prop, ok := propRaw.(map[string]any)
			if !ok {
				continue
			}
			result[name] = generatePropertyTemplate(prop)
		}
	}
	return result
}

func generatePropertyTemplate(prop map[string]any) any {
	propType, _ := prop["type"].(string)
	defaultVal, hasDefault := prop["default"]
	if hasDefault {
		return defaultVal
	}
	switch propType {
	case "string":
		return ""
	case "number", "integer":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		if nested, ok := prop["properties"].(map[string]any); ok {
			result := map[string]any{}
			for name, npRaw := range nested {
				np, ok := npRaw.(map[string]any)
				if !ok {
					continue
				}
				result[name] = generatePropertyTemplate(np)
			}
			return result
		}
		return map[string]any{}
	default:
		return nil
	}
}

func validateInputAgainstOperationSchema(inputData any, op openbindings.Operation, iface *openbindings.Interface) (status string, message string) {
	// No schema defined - can't validate
	if op.Input == nil {
		return "unknown", "no schema defined"
	}

	schema := resolveSchemaFully(op.Input, iface)
	if schema == nil {
		return "unknown", "could not resolve schema"
	}
	schemaDoc := buildSchemaDocument(schema, iface)
	schemaBytes, err := json.Marshal(schemaDoc)
	if err != nil {
		return "error", fmt.Sprintf("schema marshal error: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(string(schemaBytes))); err != nil {
		return "error", fmt.Sprintf("schema compile error: %v", err)
	}
	compiled, err := compiler.Compile("schema.json")
	if err != nil {
		return "error", fmt.Sprintf("schema compile error: %v", err)
	}
	if err := compiled.Validate(inputData); err != nil {
		return "invalid", extractValidationError(err)
	}
	return "valid", ""
}

func buildSchemaDocument(schema map[string]any, iface *openbindings.Interface) map[string]any {
	transformed := transformSchemaRefs(schema)
	doc, ok := transformed.(map[string]any)
	if !ok {
		doc = make(map[string]any)
		for k, v := range schema {
			doc[k] = v
		}
	}
	if iface != nil && len(iface.Schemas) > 0 {
		defs := make(map[string]any)
		for name, s := range iface.Schemas {
			defs[name] = transformSchemaRefs(s)
		}
		doc["$defs"] = defs
	}
	return doc
}

func transformSchemaRefs(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any)
		for k, v := range val {
			if k == "$ref" {
				if refStr, ok := v.(string); ok && strings.HasPrefix(refStr, "#/schemas/") {
					result[k] = "#/$defs/" + strings.TrimPrefix(refStr, "#/schemas/")
					continue
				}
			}
			result[k] = transformSchemaRefs(v)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = transformSchemaRefs(item)
		}
		return result
	default:
		return v
	}
}

func resolveSchemaFully(schema map[string]any, iface *openbindings.Interface) map[string]any {
	if schema == nil {
		return nil
	}
	if ref, ok := schema["$ref"].(string); ok {
		if strings.HasPrefix(ref, "#/schemas/") {
			schemaName := strings.TrimPrefix(ref, "#/schemas/")
			if iface != nil && iface.Schemas != nil {
				if resolved, ok := iface.Schemas[schemaName]; ok {
					return resolved
				}
			}
		}
		return schema
	}
	return schema
}

func extractValidationError(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()
	if strings.Contains(errStr, "missing properties:") {
		idx := strings.Index(errStr, "missing properties:")
		if idx >= 0 {
			return "Missing required: " + strings.TrimSpace(errStr[idx+len("missing properties:"):])
		}
	}
	if strings.Contains(errStr, "expected") {
		parts := strings.Split(errStr, ":")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	lines := strings.Split(errStr, "\n")
	msg := lines[0]
	if len(msg) > 120 {
		msg = msg[:117] + "..."
	}
	return msg
}
