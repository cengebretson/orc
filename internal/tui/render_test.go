package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ── primitives ───────────────────────────────────────────────────

func TestPadRight(t *testing.T) {
	if got := padRight("ab", 5); got != "ab   " {
		t.Errorf("padRight plain = %q", got)
	}
	// ANSI escapes must not count toward visible width.
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render("ab")
	if w := lipgloss.Width(padRight(styled, 5)); w != 5 {
		t.Errorf("padRight styled width = %d, want 5", w)
	}
	if got := padRight("abcdef", 3); got != "abcdef" {
		t.Errorf("padRight should not truncate: %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("hello world", 6); got != "hello…" {
		t.Errorf("truncate long = %q", got)
	}
}

func TestWrapText(t *testing.T) {
	got := wrapText("one two three four", 9)
	want := "one two\nthree\nfour"
	if got != want {
		t.Errorf("wrapText = %q, want %q", got, want)
	}
	if wrapText("", 10) != "" {
		t.Error("wrapText empty should be empty")
	}
}

func TestDrawBoxLabeledWidthInvariant(t *testing.T) {
	const outerW = 40
	box := drawBoxLabeled("Title", []string{"line one", "a much longer second line"}, outerW)
	for i, line := range strings.Split(box, "\n") {
		if w := lipgloss.Width(line); w != outerW {
			t.Errorf("line %d width = %d, want %d: %q", i, w, outerW, line)
		}
	}
}

// ── health section ───────────────────────────────────────────────

func TestRenderHealthLinesGroupsAndIcons(t *testing.T) {
	m := Model{healthItems: []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "features/", Status: doctor.Warning},
		{Group: "tools", Name: "tmux", Status: doctor.OK},
		{Group: "tools", Name: "codex", Status: doctor.Fail},
	}}

	plain := ansi.Strip(strings.Join(m.renderHealthLines(80), "\n"))

	for _, want := range []string{
		"workspace", "tools",
		"✓ AGENTS.md", "⚠ features/", "✓ tmux", "✗ codex",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("health lines missing %q:\n%s", want, plain)
		}
	}
}

func TestHealthSummaryExtra(t *testing.T) {
	m := Model{healthItems: []doctor.Check{
		{Name: "a", Status: doctor.OK},
		{Name: "b", Status: doctor.Warning},
		{Name: "c", Status: doctor.Fail},
		{Name: "d", Status: doctor.Warning},
	}}
	got := ansi.Strip(m.healthSummaryExtra())
	if !strings.Contains(got, "1 ✗") || !strings.Contains(got, "2 ⚠") {
		t.Errorf("summary extra = %q, want \"1 ✗\" and \"2 ⚠\"", got)
	}

	clean := Model{healthItems: []doctor.Check{{Name: "a", Status: doctor.OK}}}
	if got := clean.healthSummaryExtra(); got != "" {
		t.Errorf("all-OK summary extra = %q, want empty", got)
	}
}

// Health is collapsed by default, so the issue badge must show in the summary
// line without expanding the section.
func TestDashboardCollapsedHealthShowsIssueCount(t *testing.T) {
	m := testModel(t)
	m.healthItems = []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
	}
	out := ansi.Strip(m.viewDashboard())
	if !strings.Contains(out, "1 ⚠") {
		t.Errorf("collapsed health summary should show the issue badge:\n%s", out)
	}
}

func TestRenderHealthReport(t *testing.T) {
	checks := []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
		{Group: "config", Name: "orc.yaml", Status: doctor.OK, Detail: "valid"},
		{Group: "tools", Name: "codex", Status: doctor.Fail, Detail: "not found on PATH"},
	}
	plain := ansi.Strip(renderHealthReport(checks, 80))

	for _, want := range []string{
		"workspace", "config", "tools",
		"✓", "⚠", "✗",
		"AGENTS.md", "worktrees/", "not created yet",
		"orc.yaml", "valid", "codex", "not found on PATH",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("health report missing %q:\n%s", want, plain)
		}
	}
}

func TestRenderHealthReportEmpty(t *testing.T) {
	if got := ansi.Strip(renderHealthReport(nil, 80)); !strings.Contains(got, "No health checks") {
		t.Errorf("empty report = %q", got)
	}
}

// Non-OK checks get an explanatory line (name + detail) on the expanded
// dashboard; OK checks do not clutter it.
func TestHealthIssueLines(t *testing.T) {
	m := Model{healthItems: []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
		{Group: "tools", Name: "codex", Status: doctor.Fail, Detail: "not found on PATH"},
	}}
	lines := m.healthIssueLines(80)
	if len(lines) != 2 {
		t.Fatalf("issue lines = %d, want 2 (non-OK only)", len(lines))
	}
	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"worktrees/", "not created yet", "codex", "not found on PATH"} {
		if !strings.Contains(plain, want) {
			t.Errorf("issue lines missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "AGENTS.md") {
		t.Errorf("OK check should not appear in issue lines:\n%s", plain)
	}
}

// ── route chain ──────────────────────────────────────────────────

func TestRenderRouteChain(t *testing.T) {
	chain := []routeStep{
		{name: "intake", advance: "auto"},
		{name: "develop", advance: "manual"},
		{name: "pr-open", advance: "auto"},
	}
	loops := []repairLoop{{name: "pr-repair", target: "develop"}}

	rows := renderRouteChain(chain, loops, 100)
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (chain + loop annotation)", len(rows))
	}
	for _, name := range []string{"intake", "develop", "pr-open"} {
		if !strings.Contains(rows[0], name) {
			t.Errorf("chain row missing stage %q", name)
		}
	}
	if !strings.Contains(rows[1], "↺") || !strings.Contains(rows[1], "pr-repair") {
		t.Errorf("loop annotation missing: %q", rows[1])
	}
}

func TestRenderRouteChainWraps(t *testing.T) {
	chain := []routeStep{
		{name: "stage-one", advance: "auto"},
		{name: "stage-two", advance: "auto"},
		{name: "stage-three", advance: "auto"},
		{name: "stage-four", advance: "auto"},
	}
	const maxW = 30
	rows := renderRouteChain(chain, nil, maxW)
	if len(rows) < 2 {
		t.Fatalf("expected wrapping into multiple rows, got %d", len(rows))
	}
	for i, r := range rows {
		if w := lipgloss.Width(r); w > maxW {
			t.Errorf("row %d width = %d, exceeds maxW %d", i, w, maxW)
		}
	}
}

func TestRenderRouteChainEmpty(t *testing.T) {
	if rows := renderRouteChain(nil, nil, 80); rows != nil {
		t.Errorf("empty chain should render nil, got %v", rows)
	}
}

// ── feature table ────────────────────────────────────────────────

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

	var m Model
	out := m.renderTable([]*featureRow{live, dead, plain}, 140, 0)

	for _, want := range []string{"Ticket", "Status", "Worker", "Tmux"} {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q", want)
		}
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
	if !strings.Contains(out, "!") {
		t.Error("row with issues should render '!' health marker")
	}
	if !strings.Contains(out, "default/develop") {
		t.Error("stage cell should render workflow/stage")
	}
}

func TestRenderTableUsesAliasLabels(t *testing.T) {
	row := testRow("STORY-1", "active", "default:develop")
	row.workflow = "default:standard"
	row.workflowLabel = "default"
	row.stageLabel = "develop"

	var m Model
	out := ansi.Strip(m.renderTable([]*featureRow{row}, 140, 0))

	if !strings.Contains(out, "default/develop") {
		t.Fatalf("stage cell missing alias labels:\n%s", out)
	}
	if strings.Contains(out, "default:standard/default:develop") {
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

// Every canonical status (state validStatuses) needs a distinct icon — none may
// fall through to the generic "·" default, which is reserved for unknown values.
func TestStatusIconCoversCanonicalStatuses(t *testing.T) {
	for _, status := range []string{"pending", "ready", "active", "paused", "done", "archived"} {
		if got := statusIcon(status); got == "·" {
			t.Errorf("statusIcon(%q) fell through to the default dot", status)
		}
	}
}

func TestTruncateNonPositiveMax(t *testing.T) {
	if got := truncate("hello", 0); got != "" {
		t.Errorf("truncate(_, 0) = %q, want empty", got)
	}
	if got := truncate("hello", -5); got != "" {
		t.Errorf("truncate(_, negative) = %q, want empty", got)
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
			{name: "develop", advance: "auto", workerID: "bob"},
			{name: "code-review", advance: "manual"},
		},
		loops:       []repairLoop{{name: "pr-repair", target: "develop"}},
		repairSteps: []repairStep{{name: "pr-repair", workerID: "bob", advance: "auto", repairs: "develop", maxRetries: 3}},
	}}
}

func TestRenderWorkflowDetail(t *testing.T) {
	stagesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stagesDir, "develop.md"), []byte("# develop"), 0644); err != nil {
		t.Fatal(err)
	}
	allWorkers := []*workers.Worker{{ID: "bob", Name: "Bob", Engine: "claude"}}
	features := []*featureRow{testRow("STORY-1", "active", "develop")}

	out := renderWorkflowDetail("default", testChains(), allWorkers, stagesDir, features, 0, 100)

	for _, want := range []string{"Route", "Stages", "develop", "code-review", "Bob", "claude", "manual", "auto"} {
		if !strings.Contains(out, want) {
			t.Errorf("workflow detail missing %q", want)
		}
	}
	if !strings.Contains(out, "Loop Stages") {
		t.Error("missing Loop Stages box")
	}
	if !strings.Contains(out, "repairs develop · max 3") {
		t.Error("missing repair annotation with max retries")
	}
	// develop.md exists, code-review.md does not
	if !strings.Contains(out, "✓") || !strings.Contains(out, "✗") {
		t.Error("stage file existence markers missing")
	}
}

func TestRenderWorkflowDetailUsesAliasLabels(t *testing.T) {
	stagesDir := t.TempDir()
	stagePath := filepath.Join(stagesDir, "default", "develop.md")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagePath, []byte("# develop"), 0644); err != nil {
		t.Fatal(err)
	}
	chains := []workflowChain{{
		name:  "default:standard",
		label: "default",
		steps: []routeStep{
			{name: "default:develop", label: "develop", advance: "manual", workerID: "default:bob"},
		},
		loops: []repairLoop{{name: "default:code-review", label: "code-review", target: "default:develop"}},
		repairSteps: []repairStep{{
			name:         "default:code-review",
			label:        "code-review",
			workerID:     "default:zach",
			advance:      "auto",
			repairs:      "default:develop",
			repairsLabel: "develop",
			maxRetries:   3,
		}},
	}}
	features := []*featureRow{{
		s:        &state.State{Ticket: "STORY-1", Stage: state.Stage{Name: "default:develop"}},
		workflow: "default:standard",
		stage:    "default:develop",
	}}

	out := ansi.Strip(renderWorkflowDetail("default:standard", chains, nil, stagesDir, features, 0, 100))

	for _, want := range []string{"develop", "code-review", "repairs develop · max 3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("workflow detail missing alias label %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "default:develop") || strings.Contains(out, "default:code-review") {
		t.Fatalf("workflow detail leaked canonical stage IDs:\n%s", out)
	}
}

func TestRenderWorkflowDetailNotFound(t *testing.T) {
	out := renderWorkflowDetail("nope", testChains(), nil, t.TempDir(), nil, 0, 100)
	if !strings.Contains(out, "not found") {
		t.Errorf("unknown workflow should report not found: %q", out)
	}
}

// ── worker file ──────────────────────────────────────────────────

func TestRenderWorkerFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bob-developer.md")
	content := `---
id: bob-developer
name: Bob
engine: claude
model: opus
---

# Role

Build features end to end.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	story := testRow("STORY-7", "active", "develop")
	story.s.Stage.Worker = "bob-developer"

	styled, err := renderWorkerFile(path, []*featureRow{story}, 80)
	if err != nil {
		t.Fatalf("renderWorkerFile: %v", err)
	}
	// glamour styles individual word spans, so assert on ANSI-stripped text
	out := ansi.Strip(styled)
	for _, want := range []string{"Bob", "bob-developer", "claude", "opus"} {
		if !strings.Contains(out, want) {
			t.Errorf("worker info missing %q", want)
		}
	}
	if !strings.Contains(out, "Active Stories (1)") {
		t.Error("missing active stories count")
	}
	if !strings.Contains(out, "STORY-7") {
		t.Error("missing active story ticket")
	}
	if !strings.Contains(out, "Build features end to end.") {
		t.Error("missing rendered markdown body")
	}
}

func TestRenderWorkerFileNoFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("just a plain body"), 0644); err != nil {
		t.Fatal(err)
	}
	styled, err := renderWorkerFile(path, nil, 80)
	if err != nil {
		t.Fatalf("renderWorkerFile: %v", err)
	}
	out := ansi.Strip(styled)
	if !strings.Contains(out, "just a plain body") {
		t.Error("body not rendered")
	}
	if strings.Contains(out, "Active Stories") {
		t.Error("file without frontmatter should not render the info boxes")
	}
}

// ── model filtering ──────────────────────────────────────────────

func TestVisibleFeatures(t *testing.T) {
	m := New("")
	m.features = []*featureRow{
		testRow("STORY-1", "active", "develop"),
		testRow("STORY-2", "archived", "done"),
		testRow("AUTH-9", "pending", "intake"),
	}

	if got := len(m.visibleFeatures()); got != 2 {
		t.Errorf("archived hidden by default: got %d rows, want 2", got)
	}

	m.showArchived = true
	if got := len(m.visibleFeatures()); got != 3 {
		t.Errorf("with showArchived: got %d rows, want 3", got)
	}

	m.search.SetValue("auth")
	vis := m.visibleFeatures()
	if len(vis) != 1 || vis[0].s.Ticket != "AUTH-9" {
		t.Errorf("search filter: got %d rows, want only AUTH-9", len(vis))
	}
}
