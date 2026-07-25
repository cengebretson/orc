package workspaceui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/x/ansi"
)

func TestRenderWorkflowDetail(t *testing.T) {
	stagesDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stagesDir, "develop.md"), []byte("# develop"), 0644); err != nil {
		t.Fatal(err)
	}
	allWorkers := []*workers.Worker{{ID: "bob", Name: "Bob", Engine: "claude"}}
	features := []*featureRow{testRow("STORY-1", "active", "develop")}

	out := renderWorkflowDetail("default", testChains(), allWorkers, stagesDir, features, 0, 100)

	for _, want := range []string{
		"Route", "Stages", "develop", "code-review", "Bob", "claude", "manual", "auto",
		"artifacts: PLAN.md, develop/HANDOFF.md",
		"artifacts: pr-repair/ci-failures.md",
	} {
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
	if !strings.Contains(out, "Active Features (1)") {
		t.Error("missing active features count")
	}
	if !strings.Contains(out, "STORY-7") {
		t.Error("missing active feature ticket")
	}
	if line := firstLineContaining(out, "STORY-7"); !strings.Contains(line, "│  STORY-7") {
		t.Errorf("active feature row should have inspector padding: %q", line)
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
	if strings.Contains(out, "Active Features") {
		t.Error("file without frontmatter should not render the info boxes")
	}
}

func TestRenderWorkerFileStacksDetailsAboveActiveFeaturesAtAnyWidth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bob-developer.md")
	content := `---
id: bob-developer
name: Bob
engine: claude
---

# Bob

## Role

Implementation engineer.
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	story := testRow("STORY-7", "active", "develop")
	story.s.Stage.Worker = "bob-developer"

	styled, err := renderWorkerFile(path, []*featureRow{story}, 120)
	if err != nil {
		t.Fatalf("renderWorkerFile: %v", err)
	}
	out := ansi.Strip(styled)
	// Even at a wide width, the details card and Active Features panel are
	// full-width and stacked, not side by side.
	if line := firstLineContaining(out, "╮  ╭"); line != "" {
		t.Fatalf("worker detail boxes should be stacked, not side by side:\n%s", out)
	}
	detailsIdx := strings.Index(out, "Bob")
	activeIdx := strings.Index(out, "Active Features")
	docsIdx := strings.Index(out, "Documentation")
	if detailsIdx < 0 || activeIdx < 0 || docsIdx < 0 || detailsIdx >= activeIdx || activeIdx >= docsIdx {
		t.Fatalf("expected details, then Active Features, then a Documentation panel, in order:\n%s", out)
	}
	for _, want := range []string{"role", "Implementation engineer.", "active", "1 feature(s)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("details card missing %q pulled up from the body/Active Features:\n%s", want, out)
		}
	}
}

func TestRenderWorkerFileCapsActiveFeatures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bob-developer.md")
	content := `---
id: bob-developer
name: Bob
engine: claude
---

# Role
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	var stories []*featureRow
	for i := 1; i <= 10; i++ {
		story := testRow(fmt.Sprintf("STORY-%d", i), "active", "develop")
		story.s.Stage.Worker = "bob-developer"
		stories = append(stories, story)
	}

	styled, err := renderWorkerFile(path, stories, 120)
	if err != nil {
		t.Fatalf("renderWorkerFile: %v", err)
	}
	out := ansi.Strip(styled)
	for _, want := range []string{"STORY-1", "STORY-8", "+2 more"} {
		if !strings.Contains(out, want) {
			t.Fatalf("capped active feature list missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "STORY-9") || strings.Contains(out, "STORY-10") {
		t.Fatalf("capped active feature list should not render rows past the first eight:\n%s", out)
	}
}

func TestRenderWorkerGroups(t *testing.T) {
	groups := []workerGroup{
		{name: "default", items: []sectionItem{{label: "bob", id: "default:bob"}, {label: "fred", id: "default:fred"}}},
		{name: "hotfix", items: []sectionItem{{label: "patcher", id: "hotfix:patcher"}}},
	}

	collapsed := ansi.Strip(strings.Join(renderWorkerGroups(groups, 80), "\n"))
	for _, want := range []string{"default", "bob", "fred", "hotfix", "patcher"} {
		if !strings.Contains(collapsed, want) {
			t.Fatalf("collapsed worker groups missing %q:\n%s", want, collapsed)
		}
	}

	focused := ansi.Strip(strings.Join(renderGroupedWorkerList(groups, 1), "\n"))
	if !strings.Contains(focused, "fred (default:fred)") {
		t.Fatalf("focused worker groups should include dim canonical id:\n%s", focused)
	}
}

func TestRenderWorkflowGroups(t *testing.T) {
	groups := []workflowGroup{
		{name: "default", items: []sectionItem{{label: "standard", id: "default:standard"}, {label: "release", id: "default:release"}}},
		{name: "hotfix", items: []sectionItem{{label: "standard", id: "hotfix:standard"}}},
	}

	collapsed := ansi.Strip(strings.Join(renderWorkflowGroups(groups, 80), "\n"))
	for _, want := range []string{"default", "standard", "release", "hotfix"} {
		if !strings.Contains(collapsed, want) {
			t.Fatalf("collapsed workflow groups missing %q:\n%s", want, collapsed)
		}
	}

	focused := ansi.Strip(strings.Join(renderGroupedWorkflowList(groups, 2), "\n"))
	if !strings.Contains(focused, "standard (hotfix:standard)") {
		t.Fatalf("focused workflow groups should include dim canonical id:\n%s", focused)
	}
}

func TestRenderWorkflowChainGroupsKeepsStagePreview(t *testing.T) {
	chains := []workflowChain{
		{
			name:  "default:standard",
			label: "standard",
			steps: []routeStep{
				{name: "default:intake", label: "intake", advance: "auto"},
				{name: "default:develop", label: "develop", advance: "manual"},
			},
		},
		{
			name:  "hotfix:standard",
			label: "standard",
			steps: []routeStep{
				{name: "hotfix:patch", label: "patch", advance: "auto"},
				{name: "hotfix:verify", label: "verify", advance: "manual"},
			},
		},
	}

	out := ansi.Strip(strings.Join(renderWorkflowChainGroups(chains, 100), "\n"))
	for _, want := range []string{"default", "standard (default:standard)", "intake", "develop", "hotfix", "standard (hotfix:standard)", "patch", "verify"} {
		if !strings.Contains(out, want) {
			t.Fatalf("workflow chain groups missing %q:\n%s", want, out)
		}
	}
}

func firstLineContaining(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

// ── model filtering ──────────────────────────────────────────────

func TestVisibleFeatures(t *testing.T) {
	m := New("")
	m.data.features = []*featureRow{
		testRow("STORY-1", "active", "develop"),
		testRow("STORY-2", "archived", "done"),
		testRow("AUTH-9", "pending", "intake"),
	}

	if got := len(m.visibleFeatures()); got != 2 {
		t.Errorf("archived hidden by default: got %d rows, want 2", got)
	}

	m.navigation.showArchived = true
	if got := len(m.visibleFeatures()); got != 3 {
		t.Errorf("with showArchived: got %d rows, want 3", got)
	}

	m.filter.input.SetValue("auth")
	vis := m.visibleFeatures()
	if len(vis) != 1 || vis[0].s.Ticket != "AUTH-9" {
		t.Errorf("search filter: got %d rows, want only AUTH-9", len(vis))
	}

	story := m.data.features[0]
	story.workerID = "ada-reviewer"
	story.workerName = "Ada"
	story.engine = "codex"
	story.attention = "review"
	story.s.Repos = map[string]state.Repo{
		"los-app": {Branch: "feature/story-1", Worktree: "/work/los-app"},
	}
	for _, query := range []string{"ada review", "los-app feature/story-1", "codex develop"} {
		m.filter.input.SetValue(query)
		vis = m.visibleFeatures()
		if len(vis) != 1 || vis[0].s.Ticket != "STORY-1" {
			t.Errorf("metadata search %q: got %#v, want only STORY-1", query, vis)
		}
	}
}
