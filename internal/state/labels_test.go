package state_test

import (
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
)

func labelFeature(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := state.Create(dir, &state.State{
		Ticket: "ORC-1", Slug: "orc-1", Status: "active",
		Stage: state.Stage{Name: "develop"},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSetAndRemoveLabel(t *testing.T) {
	dir := labelFeature(t)
	if err := state.SetLabel(dir, "area", "api"); err != nil {
		t.Fatal(err)
	}
	if err := state.SetLabel(dir, "priority", "high"); err != nil {
		t.Fatal(err)
	}
	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.LabelPairs(); len(got) != 2 || got[0] != "area=api" || got[1] != "priority=high" {
		t.Fatalf("LabelPairs = %v, want sorted area then priority", got)
	}

	// Setting an existing key replaces rather than appends.
	if err := state.SetLabel(dir, "area", "web"); err != nil {
		t.Fatal(err)
	}
	s, _ = state.Load(dir)
	if s.Labels["area"] != "web" {
		t.Fatalf("labels = %v, want area=web", s.Labels)
	}

	if err := state.RemoveLabel(dir, "priority"); err != nil {
		t.Fatal(err)
	}
	s, _ = state.Load(dir)
	if _, ok := s.Labels["priority"]; ok {
		t.Fatalf("labels = %v, want priority removed", s.Labels)
	}
}

// Removing a key that was never set is an error: succeeding quietly would hide
// a typo in the key.
func TestRemoveUnknownLabelIsAnError(t *testing.T) {
	dir := labelFeature(t)
	if err := state.RemoveLabel(dir, "nope"); err == nil || !strings.Contains(err.Error(), `no label "nope"`) {
		t.Fatalf("RemoveLabel error = %v", err)
	}
}

func TestLabelKeyValidation(t *testing.T) {
	dir := labelFeature(t)
	for _, test := range []struct{ key, value, wantErr string }{
		{"", "x", "label key is required"},
		{"bad key", "x", "cannot contain"},
		{"bad=key", "x", "cannot contain"},
		{"area", "", "needs a value"},
	} {
		if err := state.SetLabel(dir, test.key, test.value); err == nil || !strings.Contains(err.Error(), test.wantErr) {
			t.Errorf("SetLabel(%q,%q) error = %v, want %q", test.key, test.value, err, test.wantErr)
		}
	}
}

func TestParseSelectorAndMatching(t *testing.T) {
	labels := map[string]string{"area": "api", "priority": "high"}

	for _, test := range []struct {
		raw   string
		match bool
	}{
		{"area=api", true},
		{"area=API", true},  // matching is case-insensitive
		{"AREA=api", true},  // ... on the key too
		{"area", true},      // key-only matches any value
		{"area=web", false}, // wrong value
		{"owner", false},    // absent key
	} {
		selector, err := state.ParseSelector(test.raw)
		if err != nil {
			t.Fatalf("ParseSelector(%q): %v", test.raw, err)
		}
		if got := selector.Matches(labels); got != test.match {
			t.Errorf("%q matched = %v, want %v", test.raw, got, test.match)
		}
	}

	if _, err := state.ParseSelector("area="); err == nil {
		t.Fatal("an empty value should be rejected rather than treated as key-only")
	}
}

// Several selectors intersect. Any other reading would make a second --label
// widen the result, which is not what a filter is for.
func TestMatchesAllIntersects(t *testing.T) {
	labels := map[string]string{"area": "api", "priority": "high"}
	both := []state.LabelSelector{{Key: "area", Value: "api", HasValue: true}, {Key: "priority", Value: "high", HasValue: true}}
	if !state.MatchesAll(labels, both) {
		t.Fatal("both selectors present should match")
	}
	missing := append(both, state.LabelSelector{Key: "owner", Value: "me", HasValue: true})
	if state.MatchesAll(labels, missing) {
		t.Fatal("an unmatched selector must exclude the feature")
	}
	if !state.MatchesAll(labels, nil) {
		t.Fatal("no selectors means no filtering")
	}
}

// Labels are workflow-neutral: a transition must neither read nor disturb them.
func TestLabelsSurviveTransitions(t *testing.T) {
	dir := labelFeature(t)
	if err := state.SetLabel(dir, "area", "api"); err != nil {
		t.Fatal(err)
	}
	if err := state.Pause(dir, "waiting", nil); err != nil {
		t.Fatal(err)
	}
	if err := state.Resume(dir); err != nil {
		t.Fatal(err)
	}
	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Labels["area"] != "api" {
		t.Fatalf("labels = %v, want area preserved across pause/resume", s.Labels)
	}
}
