package workspaceui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// asModel asserts a tea.Model back to the concrete Model.
func asModel(t *testing.T, tm tea.Model) Model {
	t.Helper()
	m, ok := tm.(Model)
	if !ok {
		t.Fatalf("model type = %T, want Model", tm)
	}
	return m
}

// press feeds a sequence of keys through handleKey and returns the final model.
func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	var tm tea.Model = m
	for _, k := range keys {
		tm, cmd = asModel(t, tm).handleKey(keyMsg(k))
	}
	return asModel(t, tm), cmd
}

// testModel builds a dashboard model with three features and worker/workflow
// sections, sized so enter handlers can construct viewports.
func testModel(t *testing.T) Model {
	t.Helper()
	m := New("")
	m.width = 100
	m.height = 40
	m.features = []*featureRow{
		testRow("STORY-1", "active", "develop"),
		testRow("STORY-2", "paused", "code-review"),
		testRow("AUTH-9", "pending", "intake"),
	}
	m.workflows = testChains()
	m.workflowGroups = []workflowGroup{{name: "default", items: []sectionItem{{label: "default", id: "default", path: ""}}}}
	m.workerGroups = []workerGroup{{name: "local", items: []sectionItem{{label: "Bob", id: "bob", path: "bob.md"}}}}
	m.sectionItems = map[sectionID][]sectionItem{
		sectionWorkflows: {{label: "default", id: "default", path: ""}},
		sectionWorkers:   {{label: "Bob", id: "bob", path: "bob.md"}},
	}
	return m
}

func TestHandleKeyQuit(t *testing.T) {
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := press(t, testModel(t), k)
		if cmd == nil {
			t.Fatalf("%s should return a quit command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s returned %T, want tea.QuitMsg", k, cmd())
		}
	}
}

func TestHandleKeyCursorBounds(t *testing.T) {
	m, _ := press(t, testModel(t), "j", "j", "j", "j", "j")
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want clamped at 2", m.cursor)
	}
	m, _ = press(t, m, "k", "k", "k", "k")
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want clamped at 0", m.cursor)
	}
}

func TestHandleKeyArchiveToggle(t *testing.T) {
	m, _ := press(t, testModel(t), "j", "a")
	if !m.showArchived {
		t.Error("a should toggle showArchived on")
	}
	if m.cursor != 0 {
		t.Error("a should reset cursor")
	}
	m, _ = press(t, m, "a")
	if m.showArchived {
		t.Error("a should toggle showArchived off")
	}
}

func TestHandleKeySearch(t *testing.T) {
	m, _ := press(t, testModel(t), "/")
	if !m.searching {
		t.Fatal("/ should enter search mode")
	}

	m, _ = press(t, m, "a", "u", "t", "h")
	if m.search.Value() != "auth" {
		t.Errorf("search value = %q, want auth", m.search.Value())
	}
	if got := len(m.visibleFeatures()); got != 1 {
		t.Errorf("filtered rows = %d, want 1", got)
	}

	// enter keeps the filter, esc clears it
	m, _ = press(t, m, "enter")
	if m.searching || m.search.Value() != "auth" {
		t.Errorf("enter should exit search mode keeping value, got searching=%v value=%q", m.searching, m.search.Value())
	}
	m, _ = press(t, m, "esc")
	if m.search.Value() != "" {
		t.Errorf("esc should clear the filter, got %q", m.search.Value())
	}
}

func TestHandleKeyTabCyclesSections(t *testing.T) {
	m := testModel(t)
	// navigable: health (always), workflows, workers
	m, _ = press(t, m, "tab")
	if m.focusedPane != paneSection || m.sectionFocus != sectionHealth {
		t.Fatalf("tab: pane=%v focus=%v, want section/health", m.focusedPane, m.sectionFocus)
	}
	if !m.expanded[sectionHealth] {
		t.Error("tab should expand the focused section")
	}
	m, _ = press(t, m, "tab")
	if m.sectionFocus != sectionWorkflows {
		t.Errorf("second tab: focus=%v, want workflows", m.sectionFocus)
	}
	if m.expanded[sectionHealth] {
		t.Error("tabbing out of an auto-expanded section should collapse it")
	}
	if !m.expanded[sectionWorkflows] {
		t.Error("tab should expand the newly focused section")
	}
	m, _ = press(t, m, "tab", "tab")
	if m.focusedPane != paneFeatures {
		t.Errorf("tab past last section should return to features, got %v", m.focusedPane)
	}
	if m.expanded[sectionWorkers] {
		t.Error("tabbing out to features should collapse the last auto-expanded section")
	}

	m, _ = press(t, m, "shift+tab")
	if m.sectionFocus != sectionWorkers {
		t.Errorf("shift+tab from features: focus=%v, want last section workers", m.sectionFocus)
	}

	m, _ = press(t, m, "esc")
	if m.focusedPane != paneFeatures {
		t.Errorf("esc should return focus to features, got %v", m.focusedPane)
	}
	if m.expanded[sectionWorkers] {
		t.Error("esc should collapse an auto-expanded focused section")
	}
}

func TestHandleKeySectionToggleCollapseReturnsFocus(t *testing.T) {
	m := testModel(t)
	m, _ = press(t, m, "tab", "tab") // focus workflows (expands it)
	if m.sectionFocus != sectionWorkflows {
		t.Fatalf("setup: focus=%v", m.sectionFocus)
	}
	m, _ = press(t, m, "2") // collapse focused section
	if m.expanded[sectionWorkflows] {
		t.Error("2 should collapse workflows")
	}
	if m.focusedPane != paneFeatures {
		t.Errorf("collapsing the focused section should return focus to features, got %v", m.focusedPane)
	}
}

func TestHandleKeyFeaturesPageAndJump(t *testing.T) {
	m := testModel(t) // 3 features, height 40 → page size 32

	m, _ = press(t, m, "G")
	if m.cursor != 2 {
		t.Errorf("G: cursor = %d, want last row 2", m.cursor)
	}
	m, _ = press(t, m, "g")
	if m.cursor != 0 {
		t.Errorf("g: cursor = %d, want 0", m.cursor)
	}
	m, _ = press(t, m, "pgdown")
	if m.cursor != 2 {
		t.Errorf("pgdown: cursor = %d, want clamped to 2", m.cursor)
	}
	m, _ = press(t, m, "pgup")
	if m.cursor != 0 {
		t.Errorf("pgup: cursor = %d, want clamped to 0", m.cursor)
	}

	// jump keys are inert while a section is focused
	m, _ = press(t, m, "tab", "G")
	if m.focusedPane != paneSection {
		t.Fatalf("tab should focus a section")
	}
	if m.cursor != 0 {
		t.Errorf("G in section pane should not move the feature cursor, got %d", m.cursor)
	}
}

func TestHandleKeyEnterOpensDetail(t *testing.T) {
	m, _ := press(t, testModel(t), "j", "enter")
	if m.view != viewDetail {
		t.Fatalf("view = %v, want viewDetail", m.view)
	}
	if m.detail == nil || m.detail.s.Ticket != "STORY-2" {
		t.Errorf("detail should hold the row under the cursor")
	}

	m, _ = press(t, m, "esc")
	if m.view != viewDashboard {
		t.Errorf("esc should return to dashboard, got %v", m.view)
	}
}

func TestHandleKeyWorkerFileViewer(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bob.md")
	if err := os.WriteFile(path, []byte("---\nid: bob\nname: Bob\nengine: claude\n---\n\nbody"), 0644); err != nil {
		t.Fatal(err)
	}
	m := testModel(t)
	m.allWorkers = []*workers.Worker{{ID: "bob", Name: "Bob", Engine: "claude", FilePath: path}}
	m.sectionItems[sectionWorkers] = []sectionItem{{label: "Bob", id: "bob", path: path}}

	// shift+tab focuses last section (workers), enter opens the file viewer
	m, _ = press(t, m, "shift+tab", "enter")
	if m.view != viewFile {
		t.Fatalf("view = %v, want viewFile", m.view)
	}
	if m.viewerReturn != viewDashboard {
		t.Errorf("viewerReturn = %v, want viewDashboard", m.viewerReturn)
	}
	if m.charSheetWorker == nil || m.charSheetWorker.ID != "bob" {
		t.Fatalf("charSheetWorker not resolved: %+v", m.charSheetWorker)
	}

	// ! opens the character sheet, esc returns to the file viewer, esc again to dashboard
	m, _ = press(t, m, "!")
	if m.view != viewCharacterSheet {
		t.Fatalf("! should open character sheet, got %v", m.view)
	}
	m, _ = press(t, m, "esc")
	if m.view != viewFile {
		t.Fatalf("esc should return to file viewer, got %v", m.view)
	}
	m, _ = press(t, m, "esc")
	if m.view != viewDashboard {
		t.Errorf("esc should return to dashboard, got %v", m.view)
	}
}

func TestHandleKeyHealthDrillInOpensReport(t *testing.T) {
	m := testModel(t)
	m.healthItems = []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
	}

	// tab focuses the first navigable section (always health); enter drills in
	m, _ = press(t, m, "tab")
	if m.sectionFocus != sectionHealth {
		t.Fatalf("tab should focus health, got %v", m.sectionFocus)
	}
	m, _ = press(t, m, "enter")
	if m.view != viewFile {
		t.Fatalf("view = %v, want viewFile", m.view)
	}
	if m.viewerTitle != "doctor report" {
		t.Errorf("viewerTitle = %q, want \"doctor report\"", m.viewerTitle)
	}
	if m.viewerReturn != viewDashboard {
		t.Errorf("viewerReturn = %v, want viewDashboard", m.viewerReturn)
	}
	if body := ansi.Strip(m.viewport.View()); !strings.Contains(body, "not created yet") {
		t.Errorf("report viewport missing check detail:\n%s", body)
	}

	m, _ = press(t, m, "esc")
	if m.view != viewDashboard {
		t.Errorf("esc should return to dashboard, got %v", m.view)
	}
}

func TestHealthViewerSupportsExplicitScrollKeys(t *testing.T) {
	checks := make([]doctor.Check, 0, 12)
	for i := 0; i < 12; i++ {
		checks = append(checks, doctor.Check{Group: fmt.Sprintf("group-%d", i), Name: "check", Status: doctor.OK, Detail: "healthy"})
	}
	m := testModel(t)
	m.view = viewFile
	m.viewerContext = "Health"
	m.viewerTitle = "doctor report"
	m.viewport = viewport.New(60, 6)
	m.viewport.SetContent(renderHealthReport(checks, 60))

	m, _ = press(t, m, "j")
	if m.viewport.YOffset != 1 {
		t.Fatalf("j health scroll offset = %d, want 1", m.viewport.YOffset)
	}
	m, _ = press(t, m, "pgdown")
	if m.viewport.YOffset <= 1 {
		t.Fatalf("pgdown should advance health report, offset=%d", m.viewport.YOffset)
	}
	m, _ = press(t, m, "G")
	if !m.viewport.AtBottom() {
		t.Fatalf("G should reach health report bottom, offset=%d", m.viewport.YOffset)
	}
	m, _ = press(t, m, "g")
	if m.viewport.YOffset != 0 {
		t.Fatalf("g should return to health report top, offset=%d", m.viewport.YOffset)
	}
}

func TestRoutesDrillInOpensStructuredRepositoryInspector(t *testing.T) {
	m := testModel(t)
	m.sectionFocus = sectionRepositories
	m.sectionItems[sectionRepositories] = []sectionItem{{label: "repository configuration"}}
	m.repos = []config.Repo{{Name: "app", Path: "projects/app", Purpose: "Primary application"}}
	m.routes = []config.RepoRoute{{Labels: []string{"application"}, Repos: []string{"app"}}}
	m.openSectionItem()

	if m.view != viewFile || m.viewerTitle != "map" {
		t.Fatalf("repository inspector = view %v title %q", m.view, m.viewerTitle)
	}
	body := ansi.Strip(m.viewport.View())
	for _, want := range []string{"Repository map", "projects/app", "label:application", "selects", "app"} {
		if !strings.Contains(body, want) {
			t.Fatalf("repository inspector missing %q:\n%s", want, body)
		}
	}
}

// The detail body lives in a scrollable viewport so long tickets stay usable on
// short terminals. Scrolling, opening a file, and returning must all behave.
