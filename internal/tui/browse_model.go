package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	openbindings "github.com/openbindings/openbindings-go"

	"github.com/openbindings/cli/internal/app"
)

type model struct {
	width  int
	height int

	tabs      []tab
	active    int
	nextTabID int

	urlInput textinput.Model
	focusURL bool

	viewport viewport.Model

	// Status message (e.g., "Copied!")
	statusMsg string

	// New input modal state
	newInputModal *newInputModalState

	// Pending action confirmation (double-key pattern)
	pendingConfirm *pendingConfirmState

	// Workspace state
	workspace      *app.Workspace // In-memory workspace (nil = no workspace loaded)
	workspacePath  string         // Path to workspace file (for save)
	workspaceMtime int64          // Mtime when workspace was loaded (for conflict detection)
	dirty          bool           // True if in-memory workspace differs from saved

	// Save confirmation modal
	saveConfirm *saveConfirmState

	// Conflict confirmation modal (external changes detected)
	conflictConfirm *conflictConfirmState

	// Remove input confirmation modal
	removeInputConfirm *removeInputConfirmState

	// Spinner state - prevents multiple tick chains
	spinnerActive bool

	// Per-frame render cache — cleared at the top of Update().
	// Avoids recomputing the operations view 3-4 times per frame.
	renderCache *frameCache
}

// frameCache holds cached render output for the current frame.
type frameCache struct {
	opsView     string // cached viewOperations() output
	opsViewSet  bool
	selLine     int  // cached selectedOperationLine() output
	selLineSet  bool
}

// pendingConfirmState holds a pending action awaiting key confirmation
type pendingConfirmState struct {
	key    string // Key to press again to confirm
	action string // Action identifier (e.g., "delete-input")
	data   any    // Context data for the action
}

// saveConfirmState holds state for save confirmation modal
type saveConfirmState struct {
	action string // "save", "save-quit", "quit-discard"
}

// conflictConfirmState holds state for external modification conflict
type conflictConfirmState struct {
	postAction string // "save" or "save-quit" - what to do after resolution
}

// removeInputConfirmState holds state for remove input confirmation modal
type removeInputConfirmState struct {
	targetID   string // Target ID containing the input
	opKey      string // Operation key
	inputName  string // Name of the input association
	inputPath  string // Path to the file
	fileMissing bool  // True if file doesn't exist
	refCount   int    // Number of other references to this file in the environment
}

// templateOption represents a template choice in the new input modal
type templateOption struct {
	ID      string // "blank", "schema", or "example:name"
	Label   string
	Preview string // JSON preview
}

// newInputModalState holds state for the new input modal
type newInputModalState struct {
	opKey            string           // Operation this is for
	interfaceName    string           // Interface name for context
	nameInput        textinput.Model  // Filename input
	nameTaken        bool             // Whether name is already used
	templates        []templateOption // Available templates
	selectedTemplate int              // Index of selected template
	focusField       int              // 0 = name, 1 = template list, 2 = path input (for existing file)
	pathInput        textinput.Model  // Path input (for "existing" template)
	pathError        string           // Error message for invalid path
}

func (m *model) Init() tea.Cmd {
	if len(m.tabs) == 0 {
		return nil
	}

	// Probe all tabs on startup
	var cmds []tea.Cmd
	for i := range m.tabs {
		t := &m.tabs[i]
		if t.url != "" {
			t.probe = probeResult{status: app.ProbeStatusProbing}
			cmds = append(cmds, probeCmd(t.id, t.url))
		}
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// defaultBrowseURL is the default target URL for new tabs.
var defaultBrowseURL = app.DefaultTargetURL

type tab struct {
	id    int
	url   string
	probe probeResult

	// Workspace target ID (if this tab came from workspace)
	targetID string

	// Parsed OBI (nil if not yet parsed or parse failed)
	obi *openbindings.Interface

	// OBI base directory for resolving relative artifact paths.
	obiDir string

	// Tree-based navigation (new)
	tree   *TreeState
	opKeys []string // sorted operation keys for stable ordering

	// Operation run state (per operation)
	runState map[string]*opRunState

	// Input files per operation
	inputFiles     map[string][]inputFile
	selectedInputs map[string]int // selected input index per operation
}

// opRunState tracks the execution state of an operation
type opRunState struct {
	status     string // "idle" | "running" | "streaming" | "success" | "error"
	output     string // stdout/response body
	error      string // error message if failed
	expanded   bool   // whether the output is expanded
	inputName  string // which input was used for this run
	durationMs int64  // execution duration in milliseconds
	cancel     func() // cancel function for running operations
	frame      int    // spinner frame counter for animation
	streaming  bool   // true when this is a streaming/event subscription
	eventCount int    // number of events received so far
	streamCh   <-chan app.StreamEvent // active stream channel (for re-issuing listenCmd)
}

// findTab returns a pointer to the tab with the given ID, or nil if not found.
func (m *model) findTab(tabID int) *tab {
	for i := range m.tabs {
		if m.tabs[i].id == tabID {
			return &m.tabs[i]
		}
	}
	return nil
}

func (t *tab) setURL(url string) {
	t.url = app.NormalizeURL(url)
	t.probe = probeResult{status: app.ProbeStatusProbing}
}

func (m *model) newTab(url string) tea.Cmd {
	m.nextTabID++
	t := tab{
		id:       m.nextTabID,
		url:      app.NormalizeURL(url),
		targetID: app.GenerateTargetID(),
		probe:    probeResult{status: app.ProbeStatusIdle},
	}
	m.tabs = append(m.tabs, t)
	m.active = len(m.tabs) - 1

	// Mark workspace dirty and sync
	m.markDirty()
	m.syncWorkspaceTargets()

	if t.url != "" {
		m.tabs[m.active].probe = probeResult{status: app.ProbeStatusProbing}
		return probeCmd(t.id, t.url)
	}
	return nil
}

func (m *model) closeActive() {
	if len(m.tabs) <= 1 {
		// Always keep at least one tab.
		m.tabs[0] = tab{id: m.tabs[0].id, url: "", targetID: "", probe: probeResult{status: app.ProbeStatusIdle}}
		m.active = 0
		m.markDirty()
		m.syncWorkspaceTargets()
		return
	}
	i := m.active
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}

	// Mark workspace dirty and sync
	m.markDirty()
	m.syncWorkspaceTargets()
}

func (m *model) focusURLBar() tea.Cmd {
	m.focusURL = true
	m.urlInput.SetValue(m.tabs[m.active].url)
	m.urlInput.CursorEnd()
	m.urlInput.Focus()
	return textinput.Blink
}

func (m *model) blurURLBar() {
	m.focusURL = false
	m.urlInput.Blur()
}
