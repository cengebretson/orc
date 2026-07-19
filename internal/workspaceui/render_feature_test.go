package workspaceui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/state"
	"github.com/charmbracelet/x/ansi"
)

func testRow(ticket, status, stage string) *featureRow {
	return &featureRow{
		s: &state.State{
			Ticket: ticket,
			Slug:   ticket + "-some-feature",
			Status: status,
			Stage:  state.Stage{Name: stage},
		},
		workflow:   "default",
		workerName: "Bob",
	}
}

func TestRenderTable(t *testing.T) {
	live := testRow("STORY-1", "active", "develop")
	live.s.Runtime.Tmux = &state.TmuxRuntime{Session: "story-1"}
	live.tmuxLive = true

	dead := testRow("STORY-2", "paused", "code-review")
	dead.s.Runtime.Tmux = &state.TmuxRuntime{Session: "story-2"}

	plain := testRow("STORY-3", "pending", "intake")
	plain.hasIssues = true
	plain.s.Runtime.JIT = &state.JITRuntime{Worker: "bob", Task: "spot check"}

	longName := testRow("STORY-4", "active", "develop")
	longName.s.Slug = "STORY-4-this-is-a-very-long-feature-name-that-should-truncate"

	var m Model
	out := m.renderTable([]*featureRow{live, dead, plain, longName}, 140, 0)

	for _, want := range []string{"Ticket", "Status", "Worker", "Context", "Tmux"} {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q", want)
		}
	}
	if strings.Contains(out, "Health") {
		t.Error("health should render as a ticket marker, not as a separate column")
	}
	for _, want := range []string{"STORY-1", "STORY-2", "STORY-3", "some-feature"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q", want)
		}
	}
	if !strings.Contains(out, "✓") {
		t.Error("live tmux session should render ✓")
	}
	if !strings.Contains(out, "✗") {
		t.Error("dead tmux session should render ✗")
	}
	if !strings.Contains(out, "+ jit") {
		t.Error("running jit task should render '+ jit' in stage cell")
	}
	if !strings.Contains(out, "! STORY-3") {
		t.Error("row with issues should render '!' before the ticket")
	}
	if !strings.Contains(out, "default › develop") {
		t.Error("stage cell should render workflow and stage with a separator")
	}
	if !strings.Contains(out, "this-is-a-very-long-feature-name-that-should-tru…") {
		t.Errorf("long feature name should truncate with an ellipsis:\n%s", ansi.Strip(out))
	}
	if strings.Contains(out, "this-is-a-very-long-feature-name-that-should-truncate  ") {
		t.Errorf("long feature name should not bleed across columns:\n%s", ansi.Strip(out))
	}
	wideOut := m.renderTable([]*featureRow{longName}, 180, 0)
	if !strings.Contains(wideOut, "this-is-a-very-long-feature-name-that-should-truncate") {
		t.Errorf("name column should expand in wider tables:\n%s", ansi.Strip(wideOut))
	}
}

func TestRenderContextPressureThresholdsAndUnknownLimit(t *testing.T) {
	thresholds := contextpressure.Thresholds{Green: 40, Yellow: 70, Red: 90}
	tests := []struct {
		name  string
		used  uint64
		limit uint64
		want  string
	}{
		{name: "green boundary", used: 40, limit: 100, want: styleHealthOK.Render("40%")},
		{name: "yellow boundary", used: 70, limit: 100, want: styleHealthWarn.Render("70%")},
		{name: "red boundary", used: 90, limit: 100, want: styleHealthErr.Render("90%")},
		{name: "unknown limit", used: 42, limit: 0, want: styleDim.Render("n/a")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pressure := contextpressure.Evaluate(tt.used, tt.limit, thresholds)
			if got := renderContextPressure(pressure); got != tt.want {
				t.Fatalf("renderContextPressure() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTableShowsUnavailableContextLimit(t *testing.T) {
	row := testRow("STORY-1", "active", "develop")
	row.context = contextpressure.Evaluate(42, 0, contextpressure.DefaultThresholds())
	plain := ansi.Strip((Model{}).renderTable([]*featureRow{row}, 140, -1))
	if !strings.Contains(plain, "n/a") {
		t.Fatalf("renderTable() missing unavailable context marker:\n%s", plain)
	}
}

func TestRenderTableUsesAliasLabels(t *testing.T) {
	row := testRow("STORY-1", "active", "default:develop")
	row.workflow = "default:standard"
	row.workflowLabel = "default"
	row.stageLabel = "develop"

	var m Model
	out := ansi.Strip(m.renderTable([]*featureRow{row}, 140, 0))

	if !strings.Contains(out, "default › develop") {
		t.Fatalf("stage cell missing alias labels:\n%s", out)
	}
	if strings.Contains(out, "default:standard › default:develop") {
		t.Fatalf("stage cell leaked canonical IDs:\n%s", out)
	}
}

func TestViewDetailShowsTimingSection(t *testing.T) {
	row := testRow("STORY-1", "active", "develop")
	row.featureDir = t.TempDir()
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	at := func(h float64) string {
		return base.Add(time.Duration(h * float64(time.Hour))).Format(time.RFC3339)
	}
	row.s.History = []state.HistoryEntry{
		{At: at(0), Stage: "intake", Worker: "agent", Result: "created"},
		{At: at(2), Stage: "intake", Worker: "agent", Result: "intake done"}, // 2h in intake
	}

	m := New("")
	m.width = 120
	m.height = 50
	m.detail = row
	m.detailFiles = nil

	out := m.renderDetailBody()
	if !strings.Contains(out, "Timing") {
		t.Fatalf("detail view missing Timing section:\n%s", out)
	}
	if !strings.Contains(out, "intake") {
		t.Fatalf("Timing section missing the intake stage:\n%s", out)
	}
	// develop is the open current stage measured to now → a "current" marker.
	if !strings.Contains(out, "current") {
		t.Fatalf("Timing section missing current-stage marker:\n%s", out)
	}
}

func TestViewDetailShowsDimCanonicalIDs(t *testing.T) {
	root := t.TempDir()
	workerPath := filepath.Join(root, "workers", "default", "bob.md")
	if err := os.MkdirAll(filepath.Dir(workerPath), 0o755); err != nil {
		t.Fatal(err)
	}
	workerFile := "---\nid: default:bob\nname: Bob\nengine: codex\n---\n# Bob\n"
	if err := os.WriteFile(workerPath, []byte(workerFile), 0o644); err != nil {
		t.Fatal(err)
	}

	row := testRow("STORY-1", "active", "default:develop")
	row.featureDir = t.TempDir()
	row.s.Workflow = "default:standard"
	row.workflowLabel = "default"
	row.stageLabel = "develop"
	row.s.Stage.Worker = "default:bob"

	m := New(root)
	m.width = 120
	m.height = 40
	m.root = root
	m.detail = row

	out := ansi.Strip(m.renderDetailBody())
	if !strings.Contains(out, "default (default:standard)") {
		t.Fatalf("detail state should show dim canonical workflow id:\n%s", out)
	}
	if !strings.Contains(out, "develop (default:develop)") {
		t.Fatalf("detail state should show dim canonical stage id:\n%s", out)
	}
	if !strings.Contains(out, "Bob (default:bob)") {
		t.Fatalf("detail state should show dim canonical worker id:\n%s", out)
	}
}

func TestViewDetailShowsRequiredArtifactChecklist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PLAN.md"), []byte("plan"), 0644); err != nil {
		t.Fatal(err)
	}
	row := testRow("STORY-1", "active", "develop")
	row.featureDir = dir
	row.requiredArtifacts = []string{"PLAN.md", "develop/HANDOFF.md"}

	m := New("")
	m.width = 120
	m.height = 40
	m.detail = row

	out := ansi.Strip(m.renderDetailBody())
	for _, want := range []string{"Required Artifacts", "PLAN.md", "develop/HANDOFF.md missing"} {
		if !strings.Contains(out, want) {
			t.Fatalf("detail artifact checklist missing %q:\n%s", want, out)
		}
	}
}

func TestViewDetailTimingKeepsLongStageName(t *testing.T) {
	row := testRow("STORY-1", "active", "default:qa-automation")
	row.featureDir = t.TempDir()
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	row.s.History = []state.HistoryEntry{
		{At: base.Format(time.RFC3339), Stage: "default:qa-automation", Worker: "default:brian", Result: "started qa"},
	}

	m := New("")
	m.width = 120
	m.height = 50
	m.detail = row
	m.detailFiles = nil

	out := ansi.Strip(m.renderDetailBody())
	if !strings.Contains(out, "default:qa-automation") {
		t.Fatalf("Timing section should not truncate long stage names when width allows:\n%s", out)
	}
}

// Every canonical status (state validStatuses) needs a distinct icon — none may
// fall through to the generic "·" default, which is reserved for unknown values.
func TestStatusIconCoversCanonicalStatuses(t *testing.T) {
	for _, status := range []string{"pending", "ready", "active", "paused", "done", "archived"} {
		if got := statusIcon(status); got == "·" {
			t.Errorf("statusIcon(%q) fell through to the default dot", status)
		}
	}
}

// At narrow widths innerW-72 (the History result column budget) goes negative.
// viewDetail must not panic slicing the result text.
func TestViewDetailNarrowWidthDoesNotPanic(t *testing.T) {
	row := testRow("STORY-1", "active", "develop")
	row.featureDir = t.TempDir()
	row.s.History = []state.HistoryEntry{
		{At: "2026-06-01T09:00:00Z", Stage: "intake", Worker: "agent", Result: "a fairly long result string that would overflow a narrow column"},
	}

	m := New("")
	m.width = 50 // innerW = 46 → history result budget innerW-72 = -26
	m.height = 40
	m.detail = row

	_ = m.renderDetailBody() // must not panic
}

// ── workflow detail ──────────────────────────────────────────────

func testChains() []workflowChain {
	return []workflowChain{{
		name: "default",
		steps: []routeStep{
			{name: "develop", advance: "auto", workerID: "bob", requiredArtifacts: []string{"PLAN.md", "develop/HANDOFF.md"}},
			{name: "code-review", advance: "manual"},
		},
		loops:       []repairLoop{{name: "pr-repair", target: "develop"}},
		repairSteps: []repairStep{{name: "pr-repair", workerID: "bob", advance: "auto", repairs: "develop", maxRetries: 3, requiredArtifacts: []string{"pr-repair/ci-failures.md"}}},
	}}
}

func TestWorkflowDisplayWithID(t *testing.T) {
	chains := []workflowChain{{name: "default:standard", label: "default"}}
	out := ansi.Strip(workflowDisplayWithID("default:standard", chains))
	if out != "default (default:standard)" {
		t.Fatalf("workflow display = %q, want alias plus canonical id", out)
	}
}
