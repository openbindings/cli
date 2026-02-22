package delegates

import "testing"

func TestSupportsFormat_SemverExactAndCaret(t *testing.T) {
	tests := []struct {
		delegate string
		req      string
		want     bool
	}{
		{"usage@2.0.0", "usage@2.0.0", true},
		{"usage@2.0.0", "usage@2.0.1", false},
		{"usage@^2.0.0", "usage@2.1.0", true},
		{"usage@^2.0.0", "usage@3.0.0", false},
		{"openapi@^3.0.0", "usage@2.1.0", false},
		{"usage", "usage@999.0.0", true}, // name-only = permissive
	}
	for _, tt := range tests {
		if got := SupportsFormat(tt.delegate, tt.req); got != tt.want {
			t.Errorf("SupportsFormat(%q, %q) = %v, want %v", tt.delegate, tt.req, got, tt.want)
		}
	}
}

func TestPreferredDelegate_DeterministicAndSpecific(t *testing.T) {
	prefs := map[string]string{
		"usage@^2.0.0":   "exec:delegate-a",
		"usage@2.1.0":    "exec:delegate-b",
		"usage@>=2.0.0":  "exec:delegate-c",
		"openapi@^3.0.0": "exec:delegate-d",
	}

	// Exact match should win over range matches.
	if got, ok := PreferredDelegate(prefs, "usage@2.1.0"); !ok || got != "exec:delegate-b" {
		t.Fatalf("PreferredDelegate exact = (%v, %v), want (%q, true)", got, ok, "exec:delegate-b")
	}

	// For a non-exact key, caret should beat >= for the same name (by score).
	if got, ok := PreferredDelegate(prefs, "usage@2.5.0"); !ok || got != "exec:delegate-a" {
		t.Fatalf("PreferredDelegate caret = (%v, %v), want (%q, true)", got, ok, "exec:delegate-a")
	}

	// Deterministic: repeated calls should return the same answer.
	for i := 0; i < 50; i++ {
		got, ok := PreferredDelegate(prefs, "usage@2.5.0")
		if !ok || got != "exec:delegate-a" {
			t.Fatalf("PreferredDelegate deterministic iteration %d = (%v, %v), want (%q, true)", i, got, ok, "exec:delegate-a")
		}
	}
}
