package watch

import (
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
)

func TestPreviewWrapsNextAction(t *testing.T) {
	m := Model{
		width:   24,
		height:  20,
		preview: true,
		rows: []row{
			{
				ticket: "PROJ-123",
				stage:  "develop",
				worker: "bob",
				status: "active",
				next:   "fix the failing password reset tests and hand off to review",
				history: []historyRow{
					{at: "2026-06-20T10:00:00Z", stage: "develop", worker: "bob", result: "implemented watch rail"},
					{at: "2026-06-21T11:00:00Z", stage: "review", worker: "ada", result: "requested changes"},
				},
			},
		},
	}

	view := m.previewContent()
	if !strings.Contains(view, "NEXT") {
		t.Fatalf("previewContent() missing NEXT section:\n%s", view)
	}
	if !strings.Contains(view, "password reset") {
		t.Fatalf("previewContent() missing wrapped next action:\n%s", view)
	}
	for _, want := range []string{"HISTORY", "implemented", "watch rail", "requested", "changes"} {
		if !strings.Contains(view, want) {
			t.Fatalf("previewContent() missing history text %q in:\n%s", want, view)
		}
	}
}

func TestPreviewConsolidatesRuntimeDetailsIntoFeaturePanel(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	m := Model{width: 72, now: now, rows: []row{{
		ticket: "PROJ-123", name: "password-reset", stage: "develop", stageName: "develop", workflowLabel: "standard",
		workflowSteps: []workflowStep{{name: "intake", label: "intake"}, {name: "develop", label: "develop"}, {name: "review", label: "review"}},
		worker:        "bob", status: "active", room: "api/feature-proj-123", branch: "feature/proj-123",
		tmuxState: "live", session: "proj-123", window: "develop", pane: "%7", engine: "codex", model: "gpt-5", liveState: "working",
		context: contextpressure.Evaluate(72, 100, contextpressure.DefaultThresholds()), contextTrend: []uint64{30, 48, 61, 72}, lastActive: now.Add(-3 * time.Minute),
	}}}
	view := m.previewContent()
	for _, want := range []string{"password-reset · PROJ-123", "WORKFLOW · standard", "✓ intake", "● develop", "○ review", "api/feature-proj-123", "feature/proj-123", "codex · gpt-5", "▃▄▅▆", "3m ago"} {
		if !strings.Contains(view, want) {
			t.Fatalf("previewContent() missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "RUNTIME") {
		t.Fatalf("previewContent() should not render a separate Runtime panel:\n%s", view)
	}
}

func TestRailDetailWrapsNextAction(t *testing.T) {
	view := renderRailDetailAt(row{
		stage:     "develop",
		worker:    "bob",
		status:    "active",
		tmuxState: "live",
		next:      "fix the failing password reset tests and hand off to review",
	}, 18, time.Time{})

	for _, want := range []string{"Next", "fix the failing", "password reset", "hand off", "review"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderRailDetailAt() missing wrapped text %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "fix the failing password reset tests and hand off to review") {
		t.Fatalf("renderRailDetailAt() should wrap, not leave next action on one line:\n%s", view)
	}
}

func TestRailDetailUsesIndentedMetaAndPromptHeader(t *testing.T) {
	view := renderRailDetailAt(row{
		stage:     "default:pr-repair",
		worker:    "Bob (Developer)",
		status:    "paused",
		tmuxState: "live",
		next:      "Need product decision on refresh token TTL",
	}, 32, time.Time{})
	lines := strings.Split(view, "\n")

	wantOrder := []string{
		" ! BLOCKED ",
		"  default:pr-repair",
		"  Bob (Developer)",
		"  tmux live",
		"",
		"Blocker",
		"Need product decision on refresh",
	}
	if len(lines) < len(wantOrder) {
		t.Fatalf("renderRailDetailAt() has too few lines:\n%s", view)
	}
	for i, want := range wantOrder {
		if lines[i] != want {
			t.Fatalf("line %d = %q, want %q in:\n%s", i, lines[i], want, view)
		}
	}
}

func TestPromptLabelUsesBlockerForPaused(t *testing.T) {
	blocked := renderRailDetailAt(row{status: "paused", next: "need product decision"}, 24, time.Time{})
	if !strings.Contains(blocked, "Blocker") || strings.Contains(blocked, "Next") {
		t.Fatalf("paused rail detail should label prompt as Blocker:\n%s", blocked)
	}

	active := renderRailDetailAt(row{status: "active", next: "continue implementation"}, 24, time.Time{})
	if !strings.Contains(active, "Next") || strings.Contains(active, "Blocker") {
		t.Fatalf("active rail detail should label prompt as Next:\n%s", active)
	}
}

func TestRenderHistoryShowsMostRecentEntries(t *testing.T) {
	rows := []historyRow{
		{at: "2026-06-17T10:00:00Z", stage: "one", worker: "a", result: "oldest"},
		{at: "2026-06-18T10:00:00Z", stage: "two", worker: "b", result: "older"},
		{at: "2026-06-19T10:00:00Z", stage: "three", worker: "c", result: "newest"},
	}

	view := renderTimelineHistory(rows, 72, 2)
	if strings.Contains(view, "oldest") {
		t.Fatalf("renderTimelineHistory() should omit entries past the limit:\n%s", view)
	}
	for _, want := range []string{"Jun 18", "older", "Jun 19", "newest"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderTimelineHistory() missing %q in:\n%s", want, view)
		}
	}
}
