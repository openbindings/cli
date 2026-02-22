package tui

// TreeNode represents a node in a navigable tree.
// This is a generic, reusable tree structure that can represent
// any hierarchical data (operations, schemas, file trees, etc.).
type TreeNode struct {
	ID         string      // Unique identifier within the tree
	Label      string      // Primary display text
	Badge      string      // Optional badge/tag (e.g., "[method]", "[required]")
	Type       string      // Node type for custom rendering (e.g., "operation", "schema-prop", "binding")
	Data       any         // Arbitrary data for rendering (type-specific)
	Children   []*TreeNode // Child nodes (nil or empty = leaf)
	Expanded   bool        // Whether children are visible
	Depth      int         // Nesting depth (set by tree builder)
	Actionable bool        // Whether this node can be selected (has action or children)
	Icon       string      // Icon override: empty = default (bullet/arrow), " " = no icon, or custom icon
}

// IsLeaf returns true if this node has no children.
func (n *TreeNode) IsLeaf() bool {
	return len(n.Children) == 0
}

// IsSelectable returns true if the cursor should be able to stop on this node.
// A node is selectable if it's actionable (explicitly marked) or has children.
func (n *TreeNode) IsSelectable() bool {
	return n.Actionable || !n.IsLeaf()
}

// TreeCursor tracks the current selection in a tree.
type TreeCursor struct {
	Path []int // Index path from root to selected node
}

// TreeState holds the complete state for a tree view.
type TreeState struct {
	Root   *TreeNode
	Cursor TreeCursor
}

// NewTreeState creates a new tree state with the given root.
// Automatically selects the first selectable node.
func NewTreeState(root *TreeNode) *TreeState {
	ts := &TreeState{
		Root:   root,
		Cursor: TreeCursor{Path: []int{0}},
	}
	// Select first selectable node
	ts.MoveToFirst()
	return ts
}

// SelectedNode returns the currently selected node, or nil if none.
func (ts *TreeState) SelectedNode() *TreeNode {
	if ts.Root == nil {
		return nil
	}
	return ts.nodeAtPath(ts.Cursor.Path)
}

// nodeAtPath returns the node at the given index path.
func (ts *TreeState) nodeAtPath(path []int) *TreeNode {
	if len(path) == 0 || ts.Root == nil {
		return nil
	}

	// First index selects from root's children
	nodes := ts.Root.Children
	var current *TreeNode

	for i, idx := range path {
		if idx < 0 || idx >= len(nodes) {
			return nil
		}
		current = nodes[idx]
		if i < len(path)-1 {
			nodes = current.Children
		}
	}

	return current
}

// VisibleNodes returns a flat list of all currently visible nodes
// (respecting expand/collapse state) with their depths.
func (ts *TreeState) VisibleNodes() []*TreeNode {
	if ts.Root == nil {
		return nil
	}
	var result []*TreeNode
	ts.collectVisible(ts.Root.Children, 0, &result)
	return result
}

func (ts *TreeState) collectVisible(nodes []*TreeNode, depth int, result *[]*TreeNode) {
	for _, n := range nodes {
		n.Depth = depth
		*result = append(*result, n)
		if n.Expanded && len(n.Children) > 0 {
			ts.collectVisible(n.Children, depth+1, result)
		}
	}
}

// SelectedIndex returns the index of the selected node in the visible list.
func (ts *TreeState) SelectedIndex() int {
	selected := ts.SelectedNode()
	if selected == nil {
		return -1
	}
	for i, n := range ts.VisibleNodes() {
		if n == selected {
			return i
		}
	}
	return -1
}

// MoveDown moves selection to the next selectable visible node.
func (ts *TreeState) MoveDown() bool {
	visible := ts.VisibleNodes()
	idx := ts.SelectedIndex()
	if idx < 0 {
		return false
	}
	for i := idx + 1; i < len(visible); i++ {
		if visible[i].IsSelectable() {
			ts.selectNode(visible[i])
			return true
		}
	}
	return false
}

// MoveUp moves selection to the previous selectable visible node.
func (ts *TreeState) MoveUp() bool {
	visible := ts.VisibleNodes()
	idx := ts.SelectedIndex()
	if idx <= 0 {
		return false
	}
	for i := idx - 1; i >= 0; i-- {
		if visible[i].IsSelectable() {
			ts.selectNode(visible[i])
			return true
		}
	}
	return false
}

// MoveToFirst moves selection to the first selectable visible node.
func (ts *TreeState) MoveToFirst() bool {
	visible := ts.VisibleNodes()
	for _, n := range visible {
		if n.IsSelectable() {
			ts.selectNode(n)
			return true
		}
	}
	return false
}

// MoveToLast moves selection to the last selectable visible node.
func (ts *TreeState) MoveToLast() bool {
	visible := ts.VisibleNodes()
	for i := len(visible) - 1; i >= 0; i-- {
		if visible[i].IsSelectable() {
			ts.selectNode(visible[i])
			return true
		}
	}
	return false
}

// MoveToNextSibling moves selection to the next sibling at the same depth,
// or to the parent's next sibling if no sibling exists.
func (ts *TreeState) MoveToNextSibling() bool {
	visible := ts.VisibleNodes()
	idx := ts.SelectedIndex()
	if idx < 0 {
		return false
	}

	currentDepth := visible[idx].Depth

	// Look for next node at same depth or shallower (sibling or parent's sibling)
	for i := idx + 1; i < len(visible); i++ {
		node := visible[i]
		if node.Depth < currentDepth {
			// Gone up to parent level - this is our fallback
			if node.IsSelectable() {
				ts.selectNode(node)
				return true
			}
		} else if node.Depth == currentDepth && node.IsSelectable() {
			// Found a sibling at same depth
			ts.selectNode(node)
			return true
		}
		// Skip nodes deeper than current (children of current or siblings)
	}
	return false
}

// MoveToPrevSibling moves selection to the previous sibling at the same depth,
// or to the parent if no previous sibling exists.
func (ts *TreeState) MoveToPrevSibling() bool {
	visible := ts.VisibleNodes()
	idx := ts.SelectedIndex()
	if idx <= 0 {
		return false
	}

	currentDepth := visible[idx].Depth

	// Look backwards for node at same depth or shallower
	for i := idx - 1; i >= 0; i-- {
		node := visible[i]
		if node.Depth < currentDepth {
			// Hit a parent - select it as fallback
			if node.IsSelectable() {
				ts.selectNode(node)
				return true
			}
		} else if node.Depth == currentDepth && node.IsSelectable() {
			// Found a sibling at same depth
			ts.selectNode(node)
			return true
		}
		// Skip nodes deeper than current
	}
	return false
}

// Expand expands the selected node if it has children.
// Returns true if the node was expanded.
func (ts *TreeState) Expand() bool {
	node := ts.SelectedNode()
	if node == nil || node.IsLeaf() || node.Expanded {
		return false
	}
	node.Expanded = true
	return true
}

// Collapse collapses the selected node, or moves to parent if already collapsed/leaf.
// Returns true if something changed.
func (ts *TreeState) Collapse() bool {
	node := ts.SelectedNode()
	if node == nil {
		return false
	}
	if node.Expanded && !node.IsLeaf() {
		node.Expanded = false
		return true
	}
	// Move to parent
	if len(ts.Cursor.Path) > 1 {
		ts.Cursor.Path = ts.Cursor.Path[:len(ts.Cursor.Path)-1]
		return true
	}
	return false
}

// Toggle toggles expand/collapse on the selected node.
func (ts *TreeState) Toggle() bool {
	node := ts.SelectedNode()
	if node == nil || node.IsLeaf() {
		return false
	}
	node.Expanded = !node.Expanded
	return true
}

// ExpandAll expands all nodes in the tree.
func (ts *TreeState) ExpandAll() {
	ts.setExpandedRecursive(ts.Root.Children, true)
}

// CollapseAll collapses all nodes in the tree.
func (ts *TreeState) CollapseAll() {
	ts.setExpandedRecursive(ts.Root.Children, false)
}

func (ts *TreeState) setExpandedRecursive(nodes []*TreeNode, expanded bool) {
	for _, n := range nodes {
		if !n.IsLeaf() {
			n.Expanded = expanded
		}
		ts.setExpandedRecursive(n.Children, expanded)
	}
}

// selectNode updates the cursor to point to the given node.
func (ts *TreeState) selectNode(target *TreeNode) {
	path := ts.findPath(ts.Root.Children, target, nil)
	if path != nil {
		ts.Cursor.Path = path
	}
}

// findPath finds the index path to a target node.
func (ts *TreeState) findPath(nodes []*TreeNode, target *TreeNode, prefix []int) []int {
	for i, n := range nodes {
		currentPath := append(append([]int{}, prefix...), i)
		if n == target {
			return currentPath
		}
		if len(n.Children) > 0 {
			if found := ts.findPath(n.Children, target, currentPath); found != nil {
				return found
			}
		}
	}
	return nil
}

// SelectByID selects the node with the given ID.
func (ts *TreeState) SelectByID(id string) bool {
	node := ts.findByID(ts.Root.Children, id)
	if node != nil {
		ts.selectNode(node)
		return true
	}
	return false
}

func (ts *TreeState) findByID(nodes []*TreeNode, id string) *TreeNode {
	for _, n := range nodes {
		if n.ID == id {
			return n
		}
		if found := ts.findByID(n.Children, id); found != nil {
			return found
		}
	}
	return nil
}

// ExpandToNode expands all ancestors of the node with the given ID.
func (ts *TreeState) ExpandToNode(id string) {
	ts.expandToNodeRecursive(ts.Root.Children, id)
}

func (ts *TreeState) expandToNodeRecursive(nodes []*TreeNode, id string) bool {
	for _, n := range nodes {
		if n.ID == id {
			return true
		}
		if ts.expandToNodeRecursive(n.Children, id) {
			n.Expanded = true
			return true
		}
	}
	return false
}
