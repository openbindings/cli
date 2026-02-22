package app

import (
	"encoding/json"
	"testing"
)

func fields(kv ...string) map[string]json.RawMessage {
	m := make(map[string]json.RawMessage)
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = json.RawMessage(kv[i+1])
	}
	return m
}

func TestThreeWayMerge_NoChanges(t *testing.T) {
	base := fields("kind", `"method"`, "description", `"hello"`)
	local := fields("kind", `"method"`, "description", `"hello"`)
	source := fields("kind", `"method"`, "description", `"hello"`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge")
	}
	if mr.HasChanges() {
		t.Error("expected no changes")
	}
	if len(mr.Merged) != 2 {
		t.Errorf("expected 2 merged fields, got %d", len(mr.Merged))
	}
}

func TestThreeWayMerge_SourceUpdated(t *testing.T) {
	base := fields("kind", `"method"`, "description", `"hello"`)
	local := fields("kind", `"method"`, "description", `"hello"`)
	source := fields("kind", `"method"`, "description", `"updated hello"`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge")
	}
	if !mr.HasChanges() {
		t.Error("expected changes from source")
	}
	if len(mr.Updated) != 1 || mr.Updated[0] != "description" {
		t.Errorf("expected description updated, got %v", mr.Updated)
	}
	// Verify source value was accepted.
	if string(mr.Merged["description"]) != `"updated hello"` {
		t.Errorf("expected source description, got %s", mr.Merged["description"])
	}
}

func TestThreeWayMerge_UserChanged(t *testing.T) {
	base := fields("kind", `"method"`, "description", `"hello"`)
	local := fields("kind", `"method"`, "description", `"my custom"`)
	source := fields("kind", `"method"`, "description", `"hello"`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge")
	}
	if mr.HasChanges() {
		t.Error("expected no source changes (user change preserved)")
	}
	if len(mr.Preserved) != 1 || mr.Preserved[0] != "description" {
		t.Errorf("expected description preserved, got %v", mr.Preserved)
	}
	if string(mr.Merged["description"]) != `"my custom"` {
		t.Errorf("expected local description, got %s", mr.Merged["description"])
	}
}

func TestThreeWayMerge_BothChangedSame(t *testing.T) {
	base := fields("description", `"hello"`)
	local := fields("description", `"both agree"`)
	source := fields("description", `"both agree"`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge when both changed to same value")
	}
}

func TestThreeWayMerge_Conflict(t *testing.T) {
	base := fields("kind", `"method"`, "description", `"hello"`)
	local := fields("kind", `"method"`, "description", `"my version"`)
	source := fields("kind", `"method"`, "description", `"source version"`)

	mr := ThreeWayMerge(base, local, source)

	if mr.IsClean() {
		t.Error("expected conflict")
	}
	if len(mr.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(mr.Conflicts))
	}
	c := mr.Conflicts[0]
	if c.Field != "description" {
		t.Errorf("expected conflict on description, got %s", c.Field)
	}
	// Local value should be kept.
	if string(mr.Merged["description"]) != `"my version"` {
		t.Errorf("expected local value kept, got %s", mr.Merged["description"])
	}
}

func TestThreeWayMerge_SourceAddsField(t *testing.T) {
	base := fields("kind", `"method"`)
	local := fields("kind", `"method"`)
	source := fields("kind", `"method"`, "deprecated", `true`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge")
	}
	if len(mr.Added) != 1 || mr.Added[0] != "deprecated" {
		t.Errorf("expected deprecated added, got %v", mr.Added)
	}
	if string(mr.Merged["deprecated"]) != `true` {
		t.Errorf("expected deprecated=true, got %s", mr.Merged["deprecated"])
	}
}

func TestThreeWayMerge_UserAddsField(t *testing.T) {
	base := fields("kind", `"method"`)
	local := fields("kind", `"method"`, "description", `"user added"`)
	source := fields("kind", `"method"`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge")
	}
	if len(mr.Preserved) != 1 || mr.Preserved[0] != "description" {
		t.Errorf("expected description preserved, got %v", mr.Preserved)
	}
}

func TestThreeWayMerge_SourceRemovesField(t *testing.T) {
	base := fields("kind", `"method"`, "deprecated", `true`)
	local := fields("kind", `"method"`, "deprecated", `true`)
	source := fields("kind", `"method"`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge")
	}
	if _, ok := mr.Merged["deprecated"]; ok {
		t.Error("expected deprecated removed by source")
	}
}

func TestThreeWayMerge_UserRemovesField(t *testing.T) {
	base := fields("kind", `"method"`, "deprecated", `true`)
	local := fields("kind", `"method"`)
	source := fields("kind", `"method"`, "deprecated", `true`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge (user removed, source unchanged)")
	}
	if _, ok := mr.Merged["deprecated"]; ok {
		t.Error("expected deprecated to stay removed (user's choice)")
	}
}

func TestThreeWayMerge_ConflictUserRemovedSourceChanged(t *testing.T) {
	base := fields("kind", `"method"`, "description", `"hello"`)
	local := fields("kind", `"method"`)
	source := fields("kind", `"method"`, "description", `"updated"`)

	mr := ThreeWayMerge(base, local, source)

	if mr.IsClean() {
		t.Error("expected conflict (user removed, source changed)")
	}
	if len(mr.Conflicts) != 1 || mr.Conflicts[0].Field != "description" {
		t.Errorf("expected conflict on description, got %v", mr.Conflicts)
	}
}

func TestThreeWayMerge_MixedChanges(t *testing.T) {
	// Source updates input, user changes description. Both should merge cleanly.
	base := fields("kind", `"method"`, "description", `"hello"`, "input", `{"type":"object"}`)
	local := fields("kind", `"method"`, "description", `"my desc"`, "input", `{"type":"object"}`)
	source := fields("kind", `"method"`, "description", `"hello"`, "input", `{"type":"object","required":["name"]}`)

	mr := ThreeWayMerge(base, local, source)

	if !mr.IsClean() {
		t.Error("expected clean merge (non-overlapping changes)")
	}
	// User's description preserved.
	if string(mr.Merged["description"]) != `"my desc"` {
		t.Errorf("expected user description, got %s", mr.Merged["description"])
	}
	// Source's input accepted.
	if string(mr.Merged["input"]) != `{"type":"object","required":["name"]}` {
		t.Errorf("expected source input, got %s", mr.Merged["input"])
	}
}

func TestThreeWayMerge_NilBase(t *testing.T) {
	// First sync — no base. MergeOperation handles this by returning source as-is.
	// But ThreeWayMerge with empty base should treat everything as new from both sides.
	local := fields("kind", `"method"`, "description", `"local"`)
	source := fields("kind", `"method"`, "description", `"source"`)

	mr := ThreeWayMerge(nil, local, source)

	// Both added "kind" with same value: fine.
	// Both added "description" with different values: conflict.
	if mr.IsClean() {
		t.Error("expected conflict on description")
	}
}

func TestJsonEqual_Normalization(t *testing.T) {
	// Same semantic value, different formatting.
	a := json.RawMessage(`{"a":1,"b":2}`)
	b := json.RawMessage(`{"b":2,"a":1}`)
	if !jsonEqual(a, b) {
		t.Error("expected equal after normalization")
	}
}

func TestJsonEqual_Different(t *testing.T) {
	a := json.RawMessage(`"hello"`)
	b := json.RawMessage(`"world"`)
	if jsonEqual(a, b) {
		t.Error("expected not equal")
	}
}
