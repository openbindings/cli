package tui

import (
	"testing"
)

// Helper to create a simple tree for testing
func makeTestTree() *TreeNode {
	return &TreeNode{
		ID:    "root",
		Label: "Root",
		Children: []*TreeNode{
			{
				ID:         "a",
				Label:      "Node A",
				Actionable: true,
				Children: []*TreeNode{
					{ID: "a1", Label: "A1", Actionable: true},
					{ID: "a2", Label: "A2", Actionable: true},
				},
			},
			{
				ID:         "b",
				Label:      "Node B",
				Actionable: true,
				Children: []*TreeNode{
					{ID: "b1", Label: "B1", Actionable: true},
				},
			},
			{
				ID:         "c",
				Label:      "Leaf C",
				Actionable: true,
			},
		},
	}
}

func TestTreeNode_IsLeaf(t *testing.T) {
	root := makeTestTree()
	if root.Children[0].IsLeaf() {
		t.Error("Node A should not be a leaf")
	}
	if !root.Children[2].IsLeaf() {
		t.Error("Leaf C should be a leaf")
	}
}

func TestTreeNode_IsSelectable(t *testing.T) {
	node := &TreeNode{ID: "x", Actionable: false}
	if node.IsSelectable() {
		t.Error("Non-actionable leaf should not be selectable")
	}

	node.Actionable = true
	if !node.IsSelectable() {
		t.Error("Actionable node should be selectable")
	}

	node.Actionable = false
	node.Children = []*TreeNode{{ID: "child"}}
	if !node.IsSelectable() {
		t.Error("Node with children should be selectable")
	}
}

func TestNewTreeState_SelectsFirstNode(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	selected := ts.SelectedNode()
	if selected == nil {
		t.Fatal("Should have selected a node")
	}
	if selected.ID != "a" {
		t.Errorf("Expected 'a', got %q", selected.ID)
	}
}

func TestTreeState_MoveDown(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	// Start at "a", move down to "b"
	if !ts.MoveDown() {
		t.Error("MoveDown should succeed")
	}
	if ts.SelectedNode().ID != "b" {
		t.Errorf("Expected 'b', got %q", ts.SelectedNode().ID)
	}

	// Move down to "c"
	ts.MoveDown()
	if ts.SelectedNode().ID != "c" {
		t.Errorf("Expected 'c', got %q", ts.SelectedNode().ID)
	}

	// At end, can't move further
	if ts.MoveDown() {
		t.Error("MoveDown at end should return false")
	}
}

func TestTreeState_MoveUp(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)
	ts.MoveToLast()

	if ts.SelectedNode().ID != "c" {
		t.Errorf("Expected 'c', got %q", ts.SelectedNode().ID)
	}

	ts.MoveUp()
	if ts.SelectedNode().ID != "b" {
		t.Errorf("Expected 'b', got %q", ts.SelectedNode().ID)
	}

	ts.MoveUp()
	if ts.SelectedNode().ID != "a" {
		t.Errorf("Expected 'a', got %q", ts.SelectedNode().ID)
	}

	// At start, can't move further
	if ts.MoveUp() {
		t.Error("MoveUp at start should return false")
	}
}

func TestTreeState_ExpandCollapse(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	// Node A starts collapsed
	nodeA := ts.SelectedNode()
	if nodeA.Expanded {
		t.Error("Node A should start collapsed")
	}

	// Expand it
	if !ts.Expand() {
		t.Error("Expand should succeed")
	}
	if !nodeA.Expanded {
		t.Error("Node A should be expanded")
	}

	// Expand again should fail (already expanded)
	if ts.Expand() {
		t.Error("Expand on already expanded should return false")
	}

	// Collapse it
	if !ts.Collapse() {
		t.Error("Collapse should succeed")
	}
	if nodeA.Expanded {
		t.Error("Node A should be collapsed")
	}
}

func TestTreeState_Toggle(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)
	nodeA := ts.SelectedNode()

	ts.Toggle()
	if !nodeA.Expanded {
		t.Error("Toggle should expand")
	}

	ts.Toggle()
	if nodeA.Expanded {
		t.Error("Toggle should collapse")
	}
}

func TestTreeState_VisibleNodes_Collapsed(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	// All collapsed: should see a, b, c
	visible := ts.VisibleNodes()
	if len(visible) != 3 {
		t.Errorf("Expected 3 visible nodes, got %d", len(visible))
	}
	if visible[0].ID != "a" || visible[1].ID != "b" || visible[2].ID != "c" {
		t.Error("Wrong visible nodes when collapsed")
	}
}

func TestTreeState_VisibleNodes_Expanded(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	// Expand A
	ts.Expand()

	visible := ts.VisibleNodes()
	// Should see: a, a1, a2, b, c
	if len(visible) != 5 {
		t.Errorf("Expected 5 visible nodes, got %d", len(visible))
	}
	if visible[1].ID != "a1" || visible[2].ID != "a2" {
		t.Error("Children of A should be visible")
	}
}

func TestTreeState_ExpandAll(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	ts.ExpandAll()

	visible := ts.VisibleNodes()
	// Should see: a, a1, a2, b, b1, c
	if len(visible) != 6 {
		t.Errorf("Expected 6 visible nodes, got %d", len(visible))
	}
}

func TestTreeState_CollapseAll(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	ts.ExpandAll()
	ts.CollapseAll()

	visible := ts.VisibleNodes()
	if len(visible) != 3 {
		t.Errorf("Expected 3 visible nodes after CollapseAll, got %d", len(visible))
	}
}

func TestTreeState_NavigateExpanded(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	// Expand A, then navigate
	ts.Expand()
	ts.MoveDown() // to a1
	if ts.SelectedNode().ID != "a1" {
		t.Errorf("Expected 'a1', got %q", ts.SelectedNode().ID)
	}

	ts.MoveDown() // to a2
	if ts.SelectedNode().ID != "a2" {
		t.Errorf("Expected 'a2', got %q", ts.SelectedNode().ID)
	}

	ts.MoveDown() // to b
	if ts.SelectedNode().ID != "b" {
		t.Errorf("Expected 'b', got %q", ts.SelectedNode().ID)
	}
}

func TestTreeState_SelectByID(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	if !ts.SelectByID("c") {
		t.Error("SelectByID should succeed for 'c'")
	}
	if ts.SelectedNode().ID != "c" {
		t.Errorf("Expected 'c', got %q", ts.SelectedNode().ID)
	}

	// Nested node (need to find in children)
	if !ts.SelectByID("a1") {
		t.Error("SelectByID should succeed for 'a1'")
	}
	if ts.SelectedNode().ID != "a1" {
		t.Errorf("Expected 'a1', got %q", ts.SelectedNode().ID)
	}

	// Non-existent
	if ts.SelectByID("nonexistent") {
		t.Error("SelectByID should fail for non-existent ID")
	}
}

func TestTreeState_ExpandToNode(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	// a1 is nested, expand to it
	ts.ExpandToNode("a1")

	// Node A should now be expanded
	nodeA := root.Children[0]
	if !nodeA.Expanded {
		t.Error("Node A should be expanded after ExpandToNode('a1')")
	}
}

func TestTreeState_Depth(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)
	ts.ExpandAll()

	visible := ts.VisibleNodes()
	// Check depths
	for _, n := range visible {
		switch n.ID {
		case "a", "b", "c":
			if n.Depth != 0 {
				t.Errorf("Node %s should have depth 0, got %d", n.ID, n.Depth)
			}
		case "a1", "a2", "b1":
			if n.Depth != 1 {
				t.Errorf("Node %s should have depth 1, got %d", n.ID, n.Depth)
			}
		}
	}
}

func TestTreeState_MoveToNextSibling(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)
	ts.ExpandAll()

	// Start at a, move to next sibling (b)
	ts.MoveToNextSibling()
	if ts.SelectedNode().ID != "b" {
		t.Errorf("Expected 'b', got %q", ts.SelectedNode().ID)
	}

	// Move from b to c
	ts.MoveToNextSibling()
	if ts.SelectedNode().ID != "c" {
		t.Errorf("Expected 'c', got %q", ts.SelectedNode().ID)
	}

	// c is last, can't move further
	if ts.MoveToNextSibling() {
		t.Error("MoveToNextSibling at end should return false")
	}
}

func TestTreeState_MoveToPrevSibling(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)
	ts.SelectByID("c")

	// Move from c to b
	ts.MoveToPrevSibling()
	if ts.SelectedNode().ID != "b" {
		t.Errorf("Expected 'b', got %q", ts.SelectedNode().ID)
	}

	// Move from b to a
	ts.MoveToPrevSibling()
	if ts.SelectedNode().ID != "a" {
		t.Errorf("Expected 'a', got %q", ts.SelectedNode().ID)
	}

	// a is first, can't move further
	if ts.MoveToPrevSibling() {
		t.Error("MoveToPrevSibling at start should return false")
	}
}

func TestTreeState_CollapseMovesToParent(t *testing.T) {
	root := makeTestTree()
	ts := NewTreeState(root)

	// Expand A, select a1, collapse moves to parent
	ts.Expand()
	ts.MoveDown() // a1
	if ts.SelectedNode().ID != "a1" {
		t.Errorf("Expected 'a1', got %q", ts.SelectedNode().ID)
	}

	// a1 is a leaf, collapse should move to parent
	ts.Collapse()
	if ts.SelectedNode().ID != "a" {
		t.Errorf("Expected to move to parent 'a', got %q", ts.SelectedNode().ID)
	}
}

func TestTreeState_EmptyTree(t *testing.T) {
	root := &TreeNode{ID: "empty", Children: nil}
	ts := NewTreeState(root)

	if ts.SelectedNode() != nil {
		t.Error("Empty tree should have no selected node")
	}
	if ts.MoveDown() {
		t.Error("MoveDown on empty tree should return false")
	}
	if ts.MoveUp() {
		t.Error("MoveUp on empty tree should return false")
	}
}
