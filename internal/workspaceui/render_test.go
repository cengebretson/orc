package workspaceui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/state"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

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
	m := Model{data: workspaceData{healthItems: []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "features/", Status: doctor.Warning},
		{Group: "tools", Name: "tmux", Status: doctor.OK},
		{Group: "tools", Name: "codex", Status: doctor.Fail},
	}}}

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
	m := Model{data: workspaceData{healthItems: []doctor.Check{
		{Name: "a", Status: doctor.OK},
		{Name: "b", Status: doctor.Warning},
		{Name: "c", Status: doctor.Fail},
		{Name: "d", Status: doctor.Warning},
	}}}
	got := ansi.Strip(m.healthSummaryExtra())
	if !strings.Contains(got, "1 ✗") || !strings.Contains(got, "2 ⚠") {
		t.Errorf("summary extra = %q, want \"1 ✗\" and \"2 ⚠\"", got)
	}

	clean := Model{data: workspaceData{healthItems: []doctor.Check{{Name: "a", Status: doctor.OK}}}}
	if got := clean.healthSummaryExtra(); got != "" {
		t.Errorf("all-OK summary extra = %q, want empty", got)
	}
}

func TestHealthIssueCount(t *testing.T) {
	m := Model{data: workspaceData{healthItems: []doctor.Check{
		{Name: "ok", Status: doctor.OK},
		{Name: "warning", Status: doctor.Warning},
		{Name: "failure", Status: doctor.Fail},
	}}}
	if got := m.HealthIssueCount(); got != 2 {
		t.Fatalf("HealthIssueCount() = %d, want 2", got)
	}
}

func TestDashboardShowsArtifactPolicyAndRepoCapabilityBadges(t *testing.T) {
	m := testModel(t)
	m.navigation.expanded[sectionRepositories] = true
	m.data.artifactPolicy = "block"
	m.data.repos = []config.Repo{{
		Name:          "app",
		Path:          "projects/app",
		Purpose:       "primary app",
		WorktreeSetup: "scripts/setup.sh",
		AgentHints:    []string{"make test"},
	}}
	m.data.routes = []config.RepoRoute{{Labels: []string{"application"}, Components: []string{"web"}, Repos: []string{"app"}}}

	out := ansi.Strip(m.viewDashboard())
	for _, want := range []string{"artifacts block", "Repositories", "app", "projects/app", "setup", "hints", "label:application", "component:web", "→"} {
		if !strings.Contains(out, want) {
			t.Errorf("dashboard missing %q:\n%s", want, out)
		}
	}
}

func TestEmbeddedWorkspaceRendersOnlySelectedDestination(t *testing.T) {
	m := testModel(t)
	m.embedded = true
	m = m.SetDestination(DestinationWorkflows)
	out := ansi.Strip(m.viewDashboard())
	if !strings.Contains(out, "Workflows") {
		t.Fatalf("workflow destination missing its content:\n%s", out)
	}
	for _, hidden := range []string{"Workers", "Repositories", "Features  [a]"} {
		if strings.Contains(out, hidden) {
			t.Fatalf("workflow destination should hide %q:\n%s", hidden, out)
		}
	}
}

func TestOperationalBannerAppearsOnEveryWorkspaceDestination(t *testing.T) {
	m := testModel(t)
	m.embedded = true
	m.data.features[0].s.Runtime.Tmux = &state.TmuxRuntime{Session: "story-1"}
	m.data.features[0].tmuxLive = true

	for _, destination := range []Destination{
		DestinationFeatures,
		DestinationWorkflows,
		DestinationWorkers,
	} {
		m = m.SetDestination(destination)
		out := ansi.Strip(m.View())
		for _, want := range []string{"3 FEATURES", "● 1 RUNNING", "◐ 1 PAUSED", "! 1 NEEDS YOU"} {
			if !strings.Contains(out, want) {
				t.Fatalf("destination %v banner missing %q:\n%s", destination, want, out)
			}
		}
	}
}

func TestRepositoriesSectionIsCollapsedByDefault(t *testing.T) {
	m := New(t.TempDir())
	if m.navigation.expanded[sectionRepositories] {
		t.Fatal("Repositories section should start collapsed")
	}
}

func TestRenderRoutingReportUsesStructuredRepositoryAndRulePanels(t *testing.T) {
	repos := []config.Repo{{Name: "app", Path: "projects/app", Purpose: "Primary application", AgentHints: []string{"make test"}}}
	routes := []config.RepoRoute{{Labels: []string{"application"}, Components: []string{"web"}, Repos: []string{"app"}}}
	features := []*featureRow{
		{s: &state.State{Ticket: "STORY-2", Status: "paused", Repos: map[string]state.Repo{"app": {}}}},
		{s: &state.State{Ticket: "STORY-1", Status: "active", Repos: map[string]state.Repo{"app": {}}}},
		{s: &state.State{Ticket: "STORY-3", Status: "done", Repos: map[string]state.Repo{"app": {}}}},
	}
	out := ansi.Strip(renderRoutingReport(repos, routes, features, 72))
	for _, want := range []string{"Repository map", "app", "projects/app", "Primary application", "2 features", "1 active", "1 paused", "STORY-1, STORY-2", "Optional route 1", "exact metadata", "label:application", "component:web", "selects", "app"} {
		if !strings.Contains(out, want) {
			t.Errorf("routing report missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "STORY-3") {
		t.Errorf("routing report should omit completed work:\n%s", out)
	}
	for _, want := range []string{"│  1 repositories", "│  path", "│  label:application"} {
		if !strings.Contains(out, want) {
			t.Errorf("routing report should use consistent inner padding %q:\n%s", want, out)
		}
	}
}

func TestRoutingReportFitsNarrowAndWideTerminals(t *testing.T) {
	repos := []config.Repo{{
		Name:          "application-with-a-long-name",
		Path:          "projects/application-with-a-very-long-directory-name",
		Purpose:       "Primary multilingual application 界界 with a deliberately long description",
		WorktreeSetup: "scripts/setup-worktree.sh",
		AgentHints:    []string{"make test"},
	}}
	routes := []config.RepoRoute{{Labels: []string{"full-stack-application"}, Repos: []string{"application-with-a-long-name"}}}
	for _, width := range []int{20, 24, 32, 72, 120} {
		out := renderRoutingReport(repos, routes, nil, width)
		for lineNo, line := range strings.Split(out, "\n") {
			if got := lipgloss.Width(line); got > width {
				t.Fatalf("width %d line %d occupied %d cells:\n%s", width, lineNo+1, got, out)
			}
		}
	}
}

func TestRepositoryDisplayPathResolvesConfiguredPath(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "opt", "workspace", "orc")
	if got := repositoryDisplayPath(root, "."); got != "workspace root" {
		t.Fatalf("workspace-root path = %q, want workspace root", got)
	}
	want := filepath.Join(string(filepath.Separator), "opt", "project")
	if got := repositoryDisplayPath(root, "../../project"); got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

// Health is collapsed by default, so the issue badge must show in the summary
// line without expanding the section.
func TestDashboardCollapsedHealthShowsIssueCount(t *testing.T) {
	m := testModel(t)
	m.data.healthItems = []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
	}
	out := ansi.Strip(m.viewDashboard())
	if !strings.Contains(out, "1 ⚠") {
		t.Errorf("collapsed health summary should show the issue badge:\n%s", out)
	}
}

func TestDashboardExpandedHealthStaysCompact(t *testing.T) {
	m := testModel(t)
	m.navigation.expanded[sectionHealth] = true
	m.navigation.pane = paneSection
	m.navigation.section = sectionHealth
	m.data.healthItems = []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
		{Group: "tools", Name: "codex", Status: doctor.Fail, Detail: "not found on PATH"},
	}

	out := ansi.Strip(m.viewDashboard())
	for _, want := range []string{"worktrees/", "not created yet", "codex", "not found on PATH", "enter to view full report"} {
		if !strings.Contains(out, want) {
			t.Errorf("expanded health output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "AGENTS.md") {
		t.Errorf("expanded health dashboard should omit OK checks and group headers:\n%s", out)
	}
}

func TestRenderHealthReport(t *testing.T) {
	checks := []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
		{Group: "orc.yaml", Name: "workflow refs", Status: doctor.OK, Detail: "all workers and stages exist"},
		{Group: "tools", Name: "codex", Status: doctor.Fail, Detail: "not found on PATH"},
	}
	plain := ansi.Strip(renderHealthReport(checks, 80))

	for _, want := range []string{
		"Health summary", "workspace", "orc.yaml", "tools", "passing", "warning", "failing",
		"✓", "⚠", "✗",
		"AGENTS.md", "worktrees/", "not created yet",
		"workflow refs", "all workers and stages exist", "codex", "not found on PATH",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("health report missing %q:\n%s", want, plain)
		}
	}
	if boxes := strings.Count(plain, "╭─"); boxes != 4 {
		t.Errorf("health report panels = %d, want summary + 3 groups:\n%s", boxes, plain)
	}
	if got := lipgloss.Width(renderHealthReport(checks, 80)); got > 80 {
		t.Errorf("health report width = %d, want <= 80", got)
	}
	if !strings.Contains(plain, "│  ✓ 2 passing") {
		t.Errorf("health summary should use consistent inner padding:\n%s", plain)
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
	m := Model{data: workspaceData{healthItems: []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
		{Group: "tools", Name: "codex", Status: doctor.Fail, Detail: "not found on PATH"},
	}}}
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

func TestRenderRouteChainUsesAliasLabelsWithCanonicalFallback(t *testing.T) {
	chain := []routeStep{
		{name: "default:intake", label: "intake", advance: "auto"},
		{name: "custom:security-review", advance: "manual"},
	}
	loops := []repairLoop{{name: "default:code-review", label: "code-review", target: "custom:security-review"}}

	out := ansi.Strip(strings.Join(renderRouteChain(chain, loops, 100), "\n"))
	for _, want := range []string{"intake", "custom:security-review", "code-review"} {
		if !strings.Contains(out, want) {
			t.Fatalf("route chain missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "default:intake") || strings.Contains(out, "default:code-review") {
		t.Fatalf("route chain should prefer alias labels when present:\n%s", out)
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
