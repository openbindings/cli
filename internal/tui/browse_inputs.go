package tui

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/openbindings/cli/internal/app"
	"github.com/openbindings/cli/internal/execref"
	openbindings "github.com/openbindings/openbindings-go"
)

// inputFile represents a saved input file for an operation
type inputFile struct {
	Name       string
	Path       string           // The ref (file path) from workspace association
	ModTime    time.Time
	Selected   bool
	Validation ValidationResult // Schema validation result
	Status     string           // "ok" | "missing" - whether the referenced file exists
}

// inputFilesLoadedMsg is sent when input files are loaded
type inputFilesLoadedMsg struct {
	tabID int
	opKey string
	files []inputFile
}

// inputCreatedMsg is sent after creating a new input file
type inputCreatedMsg struct {
	tabID int
	opKey string
	file  inputFile
}

type inputErrorMsg struct {
	tabID int
	opKey string
	err   error
}

// inputDeletedMsg is sent after deleting an input file
type inputDeletedMsg struct {
	tabID int
	opKey string
	path  string
}

// loadInputsFromWorkspace loads inputs from workspace associations for a target/operation.
// Returns inputFile entries with Status set to "ok" or "missing" based on file existence.
func loadInputsFromWorkspace(wsInputs map[string]map[string]map[string]string, targetID, opKey string, op *openbindings.Operation, iface *openbindings.Interface) []inputFile {
	if wsInputs == nil {
		return nil
	}

	opInputs := wsInputs[targetID]
	if opInputs == nil {
		return nil
	}

	inputRefs := opInputs[opKey]
	if inputRefs == nil {
		return nil
	}

	// Collect and sort input names for stable ordering
	var names []string
	for name := range inputRefs {
		names = append(names, name)
	}
	sort.Strings(names)

	var files []inputFile
	for _, name := range names {
		ref := inputRefs[name]

		f := inputFile{
			Name:   name,
			Path:   ref,
			Status: app.InputStatusOK,
		}

		// Check if file exists
		info, err := os.Stat(ref)
		if err != nil {
			f.Status = app.InputStatusMissing
		} else {
			f.ModTime = info.ModTime()
			// Validate if we have a schema and file exists
			if op != nil {
				f.Validation = ValidateInputFile(ref, *op, iface)
			}
		}

		files = append(files, f)
	}

	return files
}

// targetSlug creates a filesystem-safe slug from a URL
func targetSlug(rawURL string) string {
	if execref.IsExec(rawURL) {
		// exec:ob -> cmd-ob
		// exec:ob openbindings -> cmd-ob-openbindings
		s := strings.TrimPrefix(rawURL, "exec:")
		s = strings.TrimSpace(s)
		return "cmd-" + slugify(s)
	}

	// http://localhost:8080/api -> localhost-8080-api
	s := rawURL
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimSuffix(s, "/")
	return slugify(s)
}

var slugifyRe = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func slugify(s string) string {
	s = slugifyRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	return strings.ToLower(s)
}

// inputsDir returns the directory for input files
func inputsDir(targetURL, opKey string) string {
	slug := targetSlug(targetURL)
	opSlug := slugify(opKey)
	return filepath.Join(".openbindings", "inputs", slug, opSlug)
}

// readInputFile reads and parses an input file
func readInputFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// findEditor returns the user's preferred editor from environment or common defaults.
func findEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}
	for _, e := range []string{"code", "vim", "nano", "vi"} {
		if _, err := exec.LookPath(e); err == nil {
			return e
		}
	}
	return ""
}

// editInputCmd creates a command to open an input file in the user's editor.
// This uses tea.ExecProcess to properly suspend the TUI.
func editInputCmd(path string) tea.Cmd {
	editor := findEditor()
	if editor == "" {
		return nil
	}

	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		// After editor closes, reload input files would be nice
		// For now, just return nil (user can manually refresh)
		return nil
	})
}

// deleteInputCmd creates a command to delete an input file
func deleteInputCmd(tabID int, opKey string, path string) tea.Cmd {
	return func() tea.Msg {
		if err := os.Remove(path); err != nil {
			return inputErrorMsg{tabID: tabID, opKey: opKey, err: err}
		}
		return inputDeletedMsg{tabID: tabID, opKey: opKey, path: path}
	}
}
