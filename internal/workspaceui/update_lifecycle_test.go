package workspaceui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/state"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestUpdateDataMsgClampsCursor(t *testing.T) {
	m := testModel(t)
	m.cursor = 2
	tm, _ := m.Update(dataMsg{features: m.features[:1]})
	got := asModel(t, tm)
	if got.cursor != 0 {
		t.Errorf("cursor = %d, want clamped to 0 after data shrank", got.cursor)
	}
}

func TestUpdateWindowSize(t *testing.T) {
	m := testModel(t)
	tm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	got := asModel(t, tm)
	if got.width != 120 || got.height != 50 {
		t.Errorf("size = %dx%d, want 120x50", got.width, got.height)
	}
}

func TestUpdateWindowSizeReflowsFileViewer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "RULES.md")
	long := "word " // a paragraph that must re-wrap when the width changes
	if err := os.WriteFile(path, []byte(strings.Repeat(long, 60)), 0644); err != nil {
		t.Fatal(err)
	}

	m := testModel(t)
	m.view = viewFile
	m.viewerRender = fileRenderer(path)
	m.viewport = viewport.New(m.width-4, m.height-6)
	wide, err := renderFile(path, m.width-4)
	if err != nil {
		t.Fatal(err)
	}
	m.viewport.SetContent(wide)
	wideLines := m.viewport.TotalLineCount()

	tm, _ := m.Update(tea.WindowSizeMsg{Width: 48, Height: 40})
	got := asModel(t, tm)
	if got.viewport.TotalLineCount() <= wideLines {
		t.Errorf("content lines = %d after shrinking from %d-wide render — viewer did not reflow",
			got.viewport.TotalLineCount(), wideLines)
	}
}

// Synthetic viewers (no backing file) must also re-flow on resize: the health
// report wraps long check details to the viewport width.
func TestUpdateWindowSizeReflowsHealthReport(t *testing.T) {
	m := testModel(t)
	m.view = viewFile
	m.healthItems = []doctor.Check{{
		Group:  "config",
		Name:   "orc.yaml",
		Status: doctor.Fail,
		Detail: strings.Repeat("a long validation failure message ", 8),
	}}
	checks := m.healthItems
	m.viewerRender = func(w int) string { return renderHealthReport(checks, w) }
	m.viewport = viewport.New(m.width-4, m.height-6)
	m.viewport.SetContent(m.viewerRender(m.width - 4))
	wideLines := m.viewport.TotalLineCount()

	tm, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 40})
	got := asModel(t, tm)
	if got.viewport.TotalLineCount() <= wideLines {
		t.Errorf("health report lines = %d after shrink from %d — synthetic viewer did not reflow",
			got.viewport.TotalLineCount(), wideLines)
	}
}

func TestUpdateWindowSizeReflowsWorkflowDetail(t *testing.T) {
	m := testModel(t)
	// a route chain long enough that it fits one row at width 100 but must
	// wrap onto more rows at width 60
	var steps []routeStep
	for _, n := range []string{"intake", "develop", "code-review", "qa-automation", "pr-open", "evidence"} {
		steps = append(steps, routeStep{name: n, advance: "auto"})
	}
	m.workflows = []workflowChain{{name: "default", steps: steps}}
	m.view = viewWorkflowDetail
	m.wfDetailName = "default"
	m.root = t.TempDir()
	m.viewport = viewport.New(m.width-4, m.height-6)
	m.viewport.SetContent(renderWorkflowDetail("default", m.workflows, nil, filepath.Join(m.root, "stages"), m.features, 0, m.width-4))
	wideLines := m.viewport.TotalLineCount()

	tm, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	got := asModel(t, tm)
	if got.viewport.TotalLineCount() <= wideLines {
		t.Errorf("content lines = %d after shrinking from %d — workflow detail did not reflow",
			got.viewport.TotalLineCount(), wideLines)
	}
}

func TestRepositoryViewerRefreshesRecordedWorkInPlace(t *testing.T) {
	m := testModel(t)
	m.width, m.height = 72, 30
	m.view = viewFile
	m.viewerKind = viewerRepositories
	m.repos = []config.Repo{{Name: "app", Path: "projects/app"}}
	m.features = []*featureRow{{s: &state.State{
		Ticket: "STORY-1", Status: "active", Repos: map[string]state.Repo{"app": {}},
	}}}
	m.viewport = viewport.New(m.width-4, m.height-6)
	m.refreshStructuredViewer()
	if !strings.Contains(ansi.Strip(m.viewport.View()), "STORY-1") {
		t.Fatalf("initial repository viewer missing STORY-1:\n%s", m.viewport.View())
	}

	updated, _ := m.Update(dataMsg{
		repos: []config.Repo{{Name: "app", Path: "projects/app"}},
		features: []*featureRow{{s: &state.State{
			Ticket: "STORY-2", Status: "paused", Repos: map[string]state.Repo{"app": {}},
		}}},
		sectionItems: map[sectionID][]sectionItem{},
	})
	m = asModel(t, updated)
	out := ansi.Strip(m.viewport.View())
	if !strings.Contains(out, "STORY-2") || strings.Contains(out, "STORY-1") {
		t.Fatalf("repository viewer did not refresh in place:\n%s", out)
	}
}

func TestStaleWorkspaceTickDoesNotRestartAfterReactivation(t *testing.T) {
	m := New(t.TempDir())
	staleEpoch := m.epoch
	m = m.SetActive(false)
	m = m.SetActive(true)
	updated, cmd := m.Update(tickMsg{at: time.Now(), epoch: staleEpoch})
	if cmd != nil {
		t.Fatal("stale workspace tick restarted the refresh timer")
	}
	if got := asModel(t, updated); got.epoch != m.epoch {
		t.Fatalf("epoch changed from %d to %d", m.epoch, got.epoch)
	}
}
