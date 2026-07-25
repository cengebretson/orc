package workspaceui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	m.data.features = []*featureRow{
		testRow("STORY-1", "active", "develop"),
		testRow("STORY-2", "paused", "code-review"),
		testRow("AUTH-9", "pending", "intake"),
	}
	m.data.workflows = testChains()
	m.data.workflowGroups = []workflowGroup{{name: "default", items: []sectionItem{{label: "default", id: "default", path: ""}}}}
	m.data.workerGroups = []workerGroup{{name: "local", items: []sectionItem{{label: "Bob", id: "bob", path: "bob.md"}}}}
	m.navigation.items = map[sectionID][]sectionItem{
		sectionWorkflows: {{label: "default", id: "default", path: ""}},
		sectionWorkers:   {{label: "Bob", id: "bob", path: "bob.md"}},
	}
	return m
}

func TestRecordContextHistoryTracksObservedAvailableFeaturesOnly(t *testing.T) {
	tracked := testRow("STORY-1", "active", "develop")
	tracked.context = contextpressure.Pressure{Observed: true, Available: true, Percent: 10, Level: contextpressure.LevelGreen}

	unavailable := testRow("STORY-2", "active", "develop")
	unavailable.context = contextpressure.Pressure{Observed: true, Available: false}

	notObserved := testRow("STORY-3", "active", "develop")

	m := testModel(t)
	m.data.features = []*featureRow{tracked, unavailable, notObserved}

	m.recordContextHistory()
	if got := m.effects.contextHistory["STORY-1"]; len(got) != 1 || got[0] != 10 {
		t.Fatalf("STORY-1 history = %v, want [10]", got)
	}
	if _, ok := m.effects.contextHistory["STORY-2"]; ok {
		t.Fatal("unavailable telemetry should not be recorded")
	}
	if _, ok := m.effects.contextHistory["STORY-3"]; ok {
		t.Fatal("unobserved telemetry should not be recorded")
	}

	// Accumulates across refreshes and caps at contextHistoryLimit, keeping
	// the most recent samples.
	for i := range contextHistoryLimit + 3 {
		tracked.context.Percent = uint64(20 + i)
		m.recordContextHistory()
	}
	got := m.effects.contextHistory["STORY-1"]
	if len(got) != contextHistoryLimit {
		t.Fatalf("history length = %d, want capped at %d", len(got), contextHistoryLimit)
	}
	if want := uint64(20 + contextHistoryLimit + 2); got[len(got)-1] != want {
		t.Fatalf("most recent sample = %d, want %d", got[len(got)-1], want)
	}
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

func TestWorkspaceDestinationsPreserveSectionCursors(t *testing.T) {
	m := testModel(t).SetDestination(DestinationWorkers)
	m.navigation.sectionCursor = 2
	m = m.SetDestination(DestinationWorkflows)
	m.navigation.sectionCursor = 1

	m = m.SetDestination(DestinationWorkers)
	if got := m.navigation.sectionCursor; got != 2 {
		t.Fatalf("worker cursor = %d, want 2", got)
	}
	m = m.SetDestination(DestinationWorkflows)
	if got := m.navigation.sectionCursor; got != 1 {
		t.Fatalf("workflow cursor = %d, want 1", got)
	}
}

func TestHandleKeyCursorBounds(t *testing.T) {
	m, _ := press(t, testModel(t), "j", "j", "j", "j", "j")
	if m.navigation.featureCursor != 2 {
		t.Errorf("cursor = %d, want clamped at 2", m.navigation.featureCursor)
	}
	m, _ = press(t, m, "k", "k", "k", "k")
	if m.navigation.featureCursor != 0 {
		t.Errorf("cursor = %d, want clamped at 0", m.navigation.featureCursor)
	}
}

func TestHandleKeyArchiveToggle(t *testing.T) {
	m, _ := press(t, testModel(t), "j", "a")
	if !m.navigation.showArchived {
		t.Error("a should toggle showArchived on")
	}
	if m.navigation.featureCursor != 0 {
		t.Error("a should reset cursor")
	}
	m, _ = press(t, m, "a")
	if m.navigation.showArchived {
		t.Error("a should toggle showArchived off")
	}
}

func TestHandleKeySearch(t *testing.T) {
	m, _ := press(t, testModel(t), "/")
	if !m.filter.active {
		t.Fatal("/ should enter search mode")
	}

	m, _ = press(t, m, "a", "u", "t", "h")
	if m.filter.input.Value() != "auth" {
		t.Errorf("search value = %q, want auth", m.filter.input.Value())
	}
	if got := len(m.visibleFeatures()); got != 1 {
		t.Errorf("filtered rows = %d, want 1", got)
	}

	// enter keeps the filter, esc clears it
	m, _ = press(t, m, "enter")
	if m.filter.active || m.filter.input.Value() != "auth" {
		t.Errorf("enter should exit search mode keeping value, got searching=%v value=%q", m.filter.active, m.filter.input.Value())
	}
	m, _ = press(t, m, "esc")
	if m.filter.input.Value() != "" {
		t.Errorf("esc should clear the filter, got %q", m.filter.input.Value())
	}
}

func TestHandleKeyTabCyclesSections(t *testing.T) {
	m := testModel(t)
	// navigable: health (always), workflows, workers
	m, _ = press(t, m, "tab")
	if m.navigation.pane != paneSection || m.navigation.section != sectionHealth {
		t.Fatalf("tab: pane=%v focus=%v, want section/health", m.navigation.pane, m.navigation.section)
	}
	if !m.navigation.expanded[sectionHealth] {
		t.Error("tab should expand the focused section")
	}
	m, _ = press(t, m, "tab")
	if m.navigation.section != sectionWorkflows {
		t.Errorf("second tab: focus=%v, want workflows", m.navigation.section)
	}
	if m.navigation.expanded[sectionHealth] {
		t.Error("tabbing out of an auto-expanded section should collapse it")
	}
	if !m.navigation.expanded[sectionWorkflows] {
		t.Error("tab should expand the newly focused section")
	}
	m, _ = press(t, m, "tab", "tab")
	if m.navigation.pane != paneFeatures {
		t.Errorf("tab past last section should return to features, got %v", m.navigation.pane)
	}
	if m.navigation.expanded[sectionWorkers] {
		t.Error("tabbing out to features should collapse the last auto-expanded section")
	}

	m, _ = press(t, m, "shift+tab")
	if m.navigation.section != sectionWorkers {
		t.Errorf("shift+tab from features: focus=%v, want last section workers", m.navigation.section)
	}

	m, _ = press(t, m, "esc")
	if m.navigation.pane != paneFeatures {
		t.Errorf("esc should return focus to features, got %v", m.navigation.pane)
	}
	if m.navigation.expanded[sectionWorkers] {
		t.Error("esc should collapse an auto-expanded focused section")
	}
}

func TestHandleKeySectionToggleCollapseReturnsFocus(t *testing.T) {
	m := testModel(t)
	m, _ = press(t, m, "tab", "tab") // focus workflows (expands it)
	if m.navigation.section != sectionWorkflows {
		t.Fatalf("setup: focus=%v", m.navigation.section)
	}
	m, _ = press(t, m, "2") // collapse focused section
	if m.navigation.expanded[sectionWorkflows] {
		t.Error("2 should collapse workflows")
	}
	if m.navigation.pane != paneFeatures {
		t.Errorf("collapsing the focused section should return focus to features, got %v", m.navigation.pane)
	}
}

func TestHandleKeyFeaturesPageAndJump(t *testing.T) {
	m := testModel(t) // 3 features, height 40 → page size 32

	m, _ = press(t, m, "G")
	if m.navigation.featureCursor != 2 {
		t.Errorf("G: cursor = %d, want last row 2", m.navigation.featureCursor)
	}
	m, _ = press(t, m, "g")
	if m.navigation.featureCursor != 0 {
		t.Errorf("g: cursor = %d, want 0", m.navigation.featureCursor)
	}
	m, _ = press(t, m, "pgdown")
	if m.navigation.featureCursor != 2 {
		t.Errorf("pgdown: cursor = %d, want clamped to 2", m.navigation.featureCursor)
	}
	m, _ = press(t, m, "pgup")
	if m.navigation.featureCursor != 0 {
		t.Errorf("pgup: cursor = %d, want clamped to 0", m.navigation.featureCursor)
	}

	// jump keys are inert while a section is focused
	m, _ = press(t, m, "tab", "G")
	if m.navigation.pane != paneSection {
		t.Fatalf("tab should focus a section")
	}
	if m.navigation.featureCursor != 0 {
		t.Errorf("G in section pane should not move the feature cursor, got %d", m.navigation.featureCursor)
	}
}

func TestHandleKeyEnterOpensDetail(t *testing.T) {
	m, _ := press(t, testModel(t), "j", "enter")
	if m.view != viewDetail {
		t.Fatalf("view = %v, want viewDetail", m.view)
	}
	if m.detail.feature == nil || m.detail.feature.s.Ticket != "STORY-2" {
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
	m.data.allWorkers = []*workers.Worker{{ID: "bob", Name: "Bob", Engine: "claude", FilePath: path}}
	m.navigation.items[sectionWorkers] = []sectionItem{{label: "Bob", id: "bob", path: path}}

	// shift+tab focuses last section (workers), enter opens the file viewer
	m, _ = press(t, m, "shift+tab", "enter")
	if m.view != viewFile {
		t.Fatalf("view = %v, want viewFile", m.view)
	}
	if m.viewer.returnView != viewDashboard {
		t.Errorf("viewerReturn = %v, want viewDashboard", m.viewer.returnView)
	}
	if m.effects.charSheetWorker == nil || m.effects.charSheetWorker.ID != "bob" {
		t.Fatalf("charSheetWorker not resolved: %+v", m.effects.charSheetWorker)
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

// TestOperationalBannerAppearsOnTicketDetailAndFileDrillIn covers the two
// scrollable views that don't go through a destination's own direct header:
// the ticket detail page (opened from a feature row) and a plain file
// viewer drilled into from a section list. Both should still pin the shared
// banner above their scrollable content, matching every other embedded view.
func TestOperationalBannerAppearsOnTicketDetailAndFileDrillIn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bob.md")
	if err := os.WriteFile(path, []byte("---\nid: bob\nname: Bob\nengine: claude\n---\n\nbody"), 0644); err != nil {
		t.Fatal(err)
	}
	m := testModel(t)
	m.embedded = true
	m.data.allWorkers = []*workers.Worker{{ID: "bob", Name: "Bob", Engine: "claude", FilePath: path}}
	m.navigation.items[sectionWorkers] = []sectionItem{{label: "Bob", id: "bob", path: path}}

	m, _ = press(t, m, "enter")
	if m.view != viewDetail {
		t.Fatalf("enter on a feature row should open viewDetail, got %v", m.view)
	}
	if out := ansi.Strip(m.View()); !strings.Contains(out, "3 FEATURES") || !strings.Contains(out, "STORY-1-some-feature") {
		t.Fatalf("ticket detail missing pinned banner or title:\n%s", out)
	}
	m, _ = press(t, m, "esc")

	m, _ = press(t, m, "shift+tab", "enter")
	if m.view != viewFile {
		t.Fatalf("view = %v, want viewFile", m.view)
	}
	if out := ansi.Strip(m.View()); !strings.Contains(out, "3 FEATURES") {
		t.Fatalf("file drill-in missing pinned banner:\n%s", out)
	}
}

func TestWorkersDestinationSelectsInlineDetailsAndKeepsCharacterSheet(t *testing.T) {
	dir := t.TempDir()
	bobPath := filepath.Join(dir, "bob.md")
	janePath := filepath.Join(dir, "jane.md")
	if err := os.WriteFile(bobPath, []byte("---\nid: bob\nname: Bob\nengine: claude\n---\n\nBob builds things."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(janePath, []byte("---\nid: jane\nname: Jane\nengine: codex\n---\n\nJane reviews things."), 0644); err != nil {
		t.Fatal(err)
	}

	m := testModel(t)
	m.embedded = true
	m.data.workerNames = []string{"Bob", "Jane"}
	m.data.workerGroups = []workerGroup{{name: "local", items: []sectionItem{
		{label: "Bob", id: "bob", path: bobPath},
		{label: "Jane", id: "jane", path: janePath},
	}}}
	m.data.allWorkers = []*workers.Worker{
		{ID: "bob", Name: "Bob", Engine: "claude", FilePath: bobPath},
		{ID: "jane", Name: "Jane", Engine: "codex", FilePath: janePath},
	}
	m.navigation.items[sectionWorkers] = m.data.workerGroups[0].items
	m = m.SetDestination(DestinationWorkers)

	if m.view != viewFile || !m.isDirectWorkers() {
		t.Fatalf("Workers destination = view %v kind %v, want direct worker inspector", m.view, m.viewer.kind)
	}
	body := ansi.Strip(m.View())
	for _, want := range []string{"Workers", "▶  Bob", "Bob builds things"} {
		if !strings.Contains(body, want) {
			t.Fatalf("direct worker inspector missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "enter to view") {
		t.Fatalf("Workers destination should not require drill-in:\n%s", body)
	}
	if strings.Contains(body, "configured") || strings.Contains(body, "scroll details") {
		t.Fatalf("Workers selector should contain only worker choices:\n%s", body)
	}
	if got := lipgloss.Height(m.View()); got != m.height {
		t.Fatalf("Workers view height = %d, want terminal height %d", got, m.height)
	}

	m, _ = press(t, m, "j")
	body = ansi.Strip(m.View())
	if m.navigation.sectionCursor != 1 || !strings.Contains(body, "▶  Jane") || !strings.Contains(body, "Jane reviews things") {
		t.Fatalf("selecting Jane should replace inline details, cursor=%d:\n%s", m.navigation.sectionCursor, body)
	}
	if m.viewer.viewport.YOffset != 0 {
		t.Fatalf("changing workers should reset detail scroll, offset=%d", m.viewer.viewport.YOffset)
	}

	m, _ = press(t, m, "!")
	if m.view != viewCharacterSheet || m.effects.charSheetWorker == nil || m.effects.charSheetWorker.ID != "jane" {
		t.Fatalf("Bard's Tale should open for selected worker: view=%v worker=%+v", m.view, m.effects.charSheetWorker)
	}
	m, _ = press(t, m, "esc")
	if m.view != viewFile || !m.isDirectWorkers() {
		t.Fatalf("character sheet should return to inline Workers view, view=%v", m.view)
	}
}

func TestHandleKeyHealthDrillInOpensReport(t *testing.T) {
	m := testModel(t)
	m.data.healthItems = []doctor.Check{
		{Group: "workspace", Name: "AGENTS.md", Status: doctor.OK},
		{Group: "workspace", Name: "worktrees/", Status: doctor.Warning, Detail: "not created yet"},
	}

	// tab focuses the first navigable section (always health); enter drills in
	m, _ = press(t, m, "tab")
	if m.navigation.section != sectionHealth {
		t.Fatalf("tab should focus health, got %v", m.navigation.section)
	}
	m, _ = press(t, m, "enter")
	if m.view != viewFile {
		t.Fatalf("view = %v, want viewFile", m.view)
	}
	if m.viewer.title != "doctor report" {
		t.Errorf("viewerTitle = %q, want \"doctor report\"", m.viewer.title)
	}
	if m.viewer.returnView != viewDashboard {
		t.Errorf("viewerReturn = %v, want viewDashboard", m.viewer.returnView)
	}
	if body := ansi.Strip(m.viewer.viewport.View()); !strings.Contains(body, "not created yet") {
		t.Errorf("report viewport missing check detail:\n%s", body)
	}

	m, _ = press(t, m, "esc")
	if m.view != viewDashboard {
		t.Errorf("esc should return to dashboard, got %v", m.view)
	}
}

func TestHealthDestinationOpensScrollableReportDirectly(t *testing.T) {
	m := testModel(t)
	m.embedded = true
	for i := 0; i < 20; i++ {
		m.data.healthItems = append(m.data.healthItems, doctor.Check{
			Group:  fmt.Sprintf("group-%d", i),
			Name:   "check",
			Status: doctor.OK,
			Detail: "healthy",
		})
	}
	m = m.SetDestination(DestinationHealth)
	if m.view != viewFile || m.viewer.kind != viewerHealth {
		t.Fatalf("Health destination = view %v kind %v, want direct report", m.view, m.viewer.kind)
	}
	body := ansi.Strip(m.View())
	summary := strings.Index(body, "Health summary")
	firstGroup := strings.Index(body, "group-0")
	if summary < 0 || firstGroup < 0 || summary > firstGroup {
		t.Fatalf("Health report should lead with its summary:\n%s", body)
	}
	if strings.Contains(body, "doctor report") || strings.Contains(body, "enter to view full report") {
		t.Fatalf("Health destination should use the summary as its only header:\n%s", body)
	}
	if got := lipgloss.Height(m.View()); got != m.height {
		t.Fatalf("Health view height = %d, want terminal height %d", got, m.height)
	}

	m, _ = press(t, m, "j")
	if m.viewer.viewport.YOffset != 1 {
		t.Fatalf("j should scroll direct Health report, offset=%d", m.viewer.viewport.YOffset)
	}
	if !strings.Contains(ansi.Strip(m.View()), "Health summary") {
		t.Fatal("Health summary should remain pinned while details scroll")
	}
	m, _ = press(t, m, "esc")
	if m.view != viewFile {
		t.Fatalf("esc should not reveal the retired Health overview, view=%v", m.view)
	}
}

func TestRepositoriesDestinationOpensMapDirectlyAndFitsTypicalWorkspace(t *testing.T) {
	m := testModel(t)
	m.embedded = true
	m.data.repos = []config.Repo{
		{Name: "app", Path: "projects/app", Purpose: "Primary application", AgentHints: []string{"run tests"}},
		{Name: "api", Path: "projects/api", Purpose: "Service API", WorktreeSetup: "setup {{worktree_path}}"},
	}
	m.data.routes = []config.RepoRoute{
		{Labels: []string{"application"}, Repos: []string{"app"}},
		{Components: []string{"api"}, Repos: []string{"api"}},
	}
	m = m.SetDestination(DestinationRepositories)
	if m.view != viewFile || m.viewer.kind != viewerRepositories {
		t.Fatalf("Repositories destination = view %v kind %v, want direct map", m.view, m.viewer.kind)
	}
	body := ansi.Strip(m.View())
	for _, want := range []string{
		"Repository map", "2 repositories", "app", "projects/app", "Primary application",
		"api", "projects/api", "Optional route 1", "Optional route 2", "selects",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("direct repository map missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "enter to inspect repositories") || strings.Contains(body, "· map ·") {
		t.Fatalf("Repositories should not render drill-in chrome:\n%s", body)
	}
	if lines, height := m.viewer.viewport.TotalLineCount(), m.viewer.viewport.Height; lines > height {
		t.Fatalf("typical repository map requires scrolling: %d lines, viewport height %d", lines, height)
	}

	m, _ = press(t, m, "enter")
	if m.view != viewFile {
		t.Fatalf("enter should not open a second repository page, view=%v", m.view)
	}
}

func TestOpenViewerReplacesPreviousViewerState(t *testing.T) {
	m := testModel(t)
	m.viewer.kind = viewerWorker
	m.viewer.path = "/stale/worker.md"
	m.openViewer(func(int) string { return "fresh content" }, "fresh", "test", viewDashboard)

	if m.viewer.kind != viewerNone || m.viewer.path != "" {
		t.Fatalf("viewer retained stale structured state: kind=%v path=%q", m.viewer.kind, m.viewer.path)
	}
	if m.viewer.title != "fresh" || m.viewer.context != "test" || m.viewer.returnView != viewDashboard {
		t.Fatalf("viewer metadata = title %q context %q return %v", m.viewer.title, m.viewer.context, m.viewer.returnView)
	}
	if got := m.viewer.viewport.View(); !strings.Contains(got, "fresh content") {
		t.Fatalf("viewer content = %q, want fresh content", got)
	}
}

func TestHealthViewerSupportsExplicitScrollKeys(t *testing.T) {
	checks := make([]doctor.Check, 0, 12)
	for i := 0; i < 12; i++ {
		checks = append(checks, doctor.Check{Group: fmt.Sprintf("group-%d", i), Name: "check", Status: doctor.OK, Detail: "healthy"})
	}
	m := testModel(t)
	m.view = viewFile
	m.viewer.context = "Health"
	m.viewer.title = "doctor report"
	m.viewer.viewport = viewport.New(60, 6)
	m.viewer.viewport.SetContent(renderHealthReport(checks, 60))

	m, _ = press(t, m, "j")
	if m.viewer.viewport.YOffset != 1 {
		t.Fatalf("j health scroll offset = %d, want 1", m.viewer.viewport.YOffset)
	}
	m, _ = press(t, m, "pgdown")
	if m.viewer.viewport.YOffset <= 1 {
		t.Fatalf("pgdown should advance health report, offset=%d", m.viewer.viewport.YOffset)
	}
	m, _ = press(t, m, "G")
	if !m.viewer.viewport.AtBottom() {
		t.Fatalf("G should reach health report bottom, offset=%d", m.viewer.viewport.YOffset)
	}
	m, _ = press(t, m, "g")
	if m.viewer.viewport.YOffset != 0 {
		t.Fatalf("g should return to health report top, offset=%d", m.viewer.viewport.YOffset)
	}
}

func TestRoutesDrillInOpensStructuredRepositoryInspector(t *testing.T) {
	m := testModel(t)
	m.navigation.section = sectionRepositories
	m.navigation.items[sectionRepositories] = []sectionItem{{label: "repository configuration"}}
	m.data.repos = []config.Repo{{Name: "app", Path: "projects/app", Purpose: "Primary application"}}
	m.data.routes = []config.RepoRoute{{Labels: []string{"application"}, Repos: []string{"app"}}}
	m.openSectionItem()

	if m.view != viewFile || m.viewer.title != "map" {
		t.Fatalf("repository inspector = view %v title %q", m.view, m.viewer.title)
	}
	body := ansi.Strip(m.viewer.viewport.View())
	for _, want := range []string{"Repository map", "projects/app", "label:application", "selects", "app"} {
		if !strings.Contains(body, want) {
			t.Fatalf("repository inspector missing %q:\n%s", want, body)
		}
	}
}

// The detail body lives in a scrollable viewport so long tickets stay usable on
// short terminals. Scrolling, opening a file, and returning must all behave.
