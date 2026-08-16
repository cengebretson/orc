package mux_test

import (
	"testing"

	"github.com/cengebretson/orc/internal/mux"
)

func pane(id, attention string, since int64) mux.Pane {
	return mux.Pane{ID: id, Attention: attention, AttentionSince: since}
}

func TestRollUpAttentionPrefersTheMostUrgentState(t *testing.T) {
	tests := []struct {
		name  string
		panes []mux.Pane
		want  string
	}{
		{
			name:  "no panes",
			panes: nil,
			want:  "",
		},
		{
			name:  "no pane reports",
			panes: []mux.Pane{pane("%1", "", 0), pane("%2", "", 0)},
			want:  "",
		},
		{
			name:  "single reporter wins by default",
			panes: []mux.Pane{pane("%1", "", 0), pane("%2", mux.AttentionReview, 100)},
			want:  mux.AttentionReview,
		},
		{
			// The case this rollup exists for: a jit task finished in one pane
			// while the stage agent is blocked in another. Reporting done here
			// would hide work that has actually stopped.
			name:  "blocked beats done",
			panes: []mux.Pane{pane("%1", mux.AttentionDone, 100), pane("%2", mux.AttentionBlocked, 200)},
			want:  mux.AttentionBlocked,
		},
		{
			name:  "blocked beats input",
			panes: []mux.Pane{pane("%1", mux.AttentionInput, 100), pane("%2", mux.AttentionBlocked, 200)},
			want:  mux.AttentionBlocked,
		},
		{
			name:  "input beats review",
			panes: []mux.Pane{pane("%1", mux.AttentionReview, 100), pane("%2", mux.AttentionInput, 200)},
			want:  mux.AttentionInput,
		},
		{
			name:  "review beats done",
			panes: []mux.Pane{pane("%1", mux.AttentionDone, 100), pane("%2", mux.AttentionReview, 200)},
			want:  mux.AttentionReview,
		},
		{
			name:  "unrecognized state is no signal",
			panes: []mux.Pane{pane("%1", "nonsense", 100), pane("%2", mux.AttentionDone, 200)},
			want:  mux.AttentionDone,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got, _ := mux.RollUpAttention(test.panes); got != test.want {
				t.Fatalf("RollUpAttention() = %q, want %q", got, test.want)
			}
		})
	}
}

// On a tie the elapsed timer should track whichever agent has been waiting
// longest, not whichever most recently changed — otherwise a second agent
// entering the same state resets a timer the user is watching.
func TestRollUpAttentionTakesTheEarliestTimeOnATie(t *testing.T) {
	panes := []mux.Pane{
		pane("%1", mux.AttentionBlocked, 500),
		pane("%2", mux.AttentionBlocked, 100),
		pane("%3", mux.AttentionBlocked, 300),
	}

	state, since := mux.RollUpAttention(panes)
	if state != mux.AttentionBlocked {
		t.Fatalf("state = %q, want blocked", state)
	}
	if since != 100 {
		t.Fatalf("since = %d, want 100 (earliest)", since)
	}
}

// A reporter that gives a state but no timestamp must not win a tie by looking
// like the distant past.
func TestRollUpAttentionIgnoresUnreportedTimesInTies(t *testing.T) {
	state, since := mux.RollUpAttention([]mux.Pane{
		pane("%1", mux.AttentionInput, 0),
		pane("%2", mux.AttentionInput, 400),
	})
	if state != mux.AttentionInput {
		t.Fatalf("state = %q, want input", state)
	}
	if since != 400 {
		t.Fatalf("since = %d, want 400 — an unreported time is unknown, not the epoch", since)
	}
}

// The winning state carries its own timestamp, even when a less urgent pane
// entered its state earlier.
func TestRollUpAttentionTimeFollowsTheWinningState(t *testing.T) {
	state, since := mux.RollUpAttention([]mux.Pane{
		pane("%1", mux.AttentionDone, 100),
		pane("%2", mux.AttentionBlocked, 900),
	})
	if state != mux.AttentionBlocked {
		t.Fatalf("state = %q, want blocked", state)
	}
	if since != 900 {
		t.Fatalf("since = %d, want 900 — the blocked pane's own time", since)
	}
}

// The predicate is a positive list on purpose: an unrecognized source must not
// inherit authority just because Orc has not heard of it yet.
func TestIsRegisteredSourceDefaultsToUnauthoritative(t *testing.T) {
	for _, source := range []string{mux.SourceHook, mux.SourceNative} {
		if !mux.IsRegisteredSource(source) {
			t.Errorf("IsRegisteredSource(%q) = false, want true", source)
		}
	}
	for _, source := range []string{
		mux.SourceLaunch, mux.SourceTitle, mux.SourceScreen,
		"claude", "codex", "turn", "event", "orc", "", "HOOK",
	} {
		if mux.IsRegisteredSource(source) {
			t.Errorf("IsRegisteredSource(%q) = true, want false", source)
		}
	}
}
