package watch

import (
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func watchModel(t *testing.T, value tea.Model) Model {
	t.Helper()
	model, ok := value.(Model)
	if !ok {
		t.Fatalf("model = %T, want watch.Model", value)
	}
	return model
}

func TestWatchSearchFiltersSharedMetadataAndClears(t *testing.T) {
	searchBox := textinput.New()
	searchBox.Prompt = "/ "
	m := Model{
		searchBox: searchBox,
		rows: []row{
			{ticket: "FLYWL-123", search: []string{"FLYWL-123", "develop", "bob", "los-app", "feature/flywl-123", "active"}},
			{ticket: "FLYWL-456", search: []string{"FLYWL-456", "review", "ada", "los-qa", "feature/flywl-456", "review"}},
		},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = watchModel(t, updated)
	for _, char := range "ada review" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
		m = watchModel(t, updated)
	}
	if len(m.rows) != 1 || m.rows[0].ticket != "FLYWL-456" {
		t.Fatalf("filtered rows = %#v, want FLYWL-456", m.rows)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = watchModel(t, updated)
	if m.searching || m.searchBox.Value() != "ada review" {
		t.Fatalf("enter should keep filter, searching=%v value=%q", m.searching, m.searchBox.Value())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = watchModel(t, updated)
	if m.searchBox.Value() != "" || len(m.rows) != 2 {
		t.Fatalf("esc should clear filter, value=%q rows=%d", m.searchBox.Value(), len(m.rows))
	}
}

func TestWatchRefreshPreservesSelectedTicket(t *testing.T) {
	searchBox := textinput.New()
	m := Model{
		searchBox: searchBox,
		cursor:    1,
		rows:      []row{{ticket: "A"}, {ticket: "B"}},
	}
	updated, _ := m.Update(dataMsg{rows: []row{{ticket: "B"}, {ticket: "A"}}})
	m = watchModel(t, updated)
	if m.cursor != 0 || m.rows[m.cursor].ticket != "B" {
		t.Fatalf("selection after refresh = cursor %d ticket %q, want B", m.cursor, m.rows[m.cursor].ticket)
	}
}

func TestRenderRailIsCompact(t *testing.T) {
	m := Model{
		width:  28,
		height: 20,
		rows: []row{
			{ticket: "PROJ-123", stage: "develop", worker: "bob", status: "paused", tmuxState: "live", next: "fix tests"},
			{ticket: "PROJ-124", stage: "review", worker: "ada", status: "active", tmuxState: "live"},
		},
	}

	view := m.renderRail()
	for _, want := range []string{"orc", "workspace", "● 2 RUNNING", "◐ 1 PAUSED", "! 1 NEEDS YOU", "↺ loading", "▌", "PROJ-123", "PROJ-124", "BLOCKED", "develop", "bob", "Blocker", "fix tests", "╭", "╯"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderRail() missing %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "ORC WATCH") {
		t.Fatalf("renderRail() should not repeat the watch title:\n%s", view)
	}
	for _, tooWordy := range []string{"WORKERS", "Bob (Developer)", "Ticket", "Next"} {
		if strings.Contains(view, tooWordy) {
			t.Fatalf("renderRail() should not render %q in compact view:\n%s", tooWordy, view)
		}
	}
	if got := lipgloss.Width(view); got > m.width {
		t.Fatalf("renderRail() width = %d, terminal width = %d:\n%s", got, m.width, view)
	}
}

func TestParkingGroupIsVisibleExpandableAndWokenRowsAreFlagged(t *testing.T) {
	searchBox := textinput.New()
	allRows := []row{
		{ticket: "ACTIVE-1", status: "active", tmuxState: "live", search: []string{"ACTIVE-1"}},
		{ticket: "PARKED-1", status: "paused", tmuxState: "live", parked: true, search: []string{"PARKED-1"}},
		{ticket: "WOKEN-1", status: "paused", tmuxState: "live", woken: true, wakeReason: "attention", search: []string{"WOKEN-1"}},
	}
	m := Model{width: 48, height: 24, searchBox: searchBox, allRows: allRows}
	m.applyFilter(true)
	view := m.renderRail()
	if !strings.Contains(view, "Parked (1)") || strings.Contains(view, "PARKED-1") || !strings.Contains(view, "↟ WOKEN-1") {
		t.Fatalf("collapsed parking view:\n%s", view)
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = watchModel(t, updated)
	view = m.renderRail()
	if !m.parkedExpanded || !strings.Contains(view, "PARKED-1") || !strings.Contains(view, "▾ Parked (1)") {
		t.Fatalf("expanded parking view:\n%s", view)
	}
}

func TestWatchSummaryDistinguishesRuntimePausedAndAttention(t *testing.T) {
	rows := []row{
		{status: "active", tmuxState: "live"},
		{status: "paused", tmuxState: "stopped"},
	}
	if got, want := watchSummary(rows), "1 running · 1 paused · 1 needs you"; got != want {
		t.Fatalf("watchSummary() = %q, want %q", got, want)
	}
	rows[1].status = "active"
	if got, want := watchSummary(rows), "1 running · 0 paused"; got != want {
		t.Fatalf("watchSummary() = %q, want %q", got, want)
	}
}

func TestRenderWatchStatusCompactsAtNarrowWidths(t *testing.T) {
	rows := []row{{status: "active", tmuxState: "live"}, {status: "paused"}}
	for _, line := range renderWatchStatusLines(liveOverviewFor(rows, time.Time{}, time.Time{}), 12) {
		if got := lipgloss.Width(line); got > 12 {
			t.Fatalf("compact status width = %d, want <= 12", got)
		}
	}
}

func TestRenderWatchOverviewIncludesRefreshAge(t *testing.T) {
	lastLoad := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	view := renderWatchOverview(
		[]row{{status: "paused", tmuxState: "live"}},
		28,
		lastLoad,
		lastLoad.Add(3*time.Second),
	)
	for _, want := range []string{"orc", "workspace", "● 1 RUNNING", "◐ 1 PAUSED", "! 1 NEEDS YOU", "↺ 3s ago", "╭", "╯"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderWatchOverview() missing %q:\n%s", want, view)
		}
	}
	if got := lipgloss.Width(view); got > 28 {
		t.Fatalf("renderWatchOverview() width = %d, want <= 28:\n%s", got, view)
	}
}

func TestRailDetailShowsLocationContextMeterAndActivityAge(t *testing.T) {
	view := renderRailDetailAt(row{
		room:    "api/feature-proj-123",
		stage:   "develop",
		status:  "active",
		context: contextpressure.Evaluate(75, 100, contextpressure.DefaultThresholds()),
		history: []historyRow{{at: "2026-07-18T11:30:00Z"}},
	}, 32, time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC))

	for _, want := range []string{"develop", "api/feature-proj-123", "ctx ", "███", "75%", "updated 30m ago"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderRailDetailAt() missing %q in:\n%s", want, view)
		}
	}
}

func TestDemoRowsExerciseWorkflowAndContextStates(t *testing.T) {
	rows := demoRows("")
	if len(rows) != 4 {
		t.Fatalf("demoRows() returned %d rows, want 4", len(rows))
	}
	levels := map[contextpressure.Level]bool{}
	foundCelebration := false
	for _, r := range rows {
		levels[r.context.Level] = true
		if len(r.workflowSteps) != 4 {
			t.Fatalf("demo row %s has %d workflow steps, want 4", r.ticket, len(r.workflowSteps))
		}
		foundCelebration = foundCelebration || r.demoCelebration
	}
	for _, level := range []contextpressure.Level{contextpressure.LevelGreen, contextpressure.LevelYellow, contextpressure.LevelRed} {
		if !levels[level] {
			t.Fatalf("demoRows() did not include context level %v", level)
		}
	}
	if !foundCelebration {
		t.Fatal("demoRows() should include the completion celebration")
	}
}

func TestMergeLiveVisualsTracksContextAndStateChanges(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	previous := []row{{
		ticket: "PROJ-123", status: "active", tmuxState: "live",
		context:      contextpressure.Evaluate(35, 100, contextpressure.DefaultThresholds()),
		contextTrend: []uint64{20, 35},
	}}
	next := []row{{
		ticket: "PROJ-123", status: "done", tmuxState: "stopped",
		context: contextpressure.Evaluate(52, 100, contextpressure.DefaultThresholds()),
	}}
	merged := mergeLiveVisuals(previous, next, now)
	if got := merged[0].contextTrend; len(got) != 3 || got[2] != 52 {
		t.Fatalf("context trend = %v, want [20 35 52]", got)
	}
	if !merged[0].flashUntil.After(now) || !merged[0].celebrateUntil.After(now) {
		t.Fatalf("done transition did not set flash windows: %#v", merged[0])
	}
}

func TestRenderWorkflowFlowMarksCompletedCurrentAndFutureStages(t *testing.T) {
	r := row{
		stageName: "develop",
		status:    "active",
		workflowSteps: []workflowStep{
			{name: "intake", label: "intake"},
			{name: "develop", label: "develop"},
			{name: "review", label: "review"},
		},
	}
	view := renderWorkflowFlow(r, 80)
	for _, want := range []string{"✓ intake", "● develop", "○ review", "→"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderWorkflowFlow() missing %q in:\n%s", want, view)
		}
	}
}

func TestWatchHelpOverlayToggles(t *testing.T) {
	m := Model{width: 32, rows: []row{{ticket: "PROJ-123"}}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = watchModel(t, updated)
	if !m.help || !strings.Contains(m.View(), "ORC WATCH · HELP") {
		t.Fatalf("? should open help overlay, help=%v view=%q", m.help, m.View())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = watchModel(t, updated)
	if m.help {
		t.Fatal("esc should close help overlay")
	}
}

func TestDetailsNavigationScrollsViewport(t *testing.T) {
	vp := viewport.New(24, 5)
	vp.SetContent(strings.Join([]string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}, "\n"))
	m := Model{preview: true, viewport: vp}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = watchModel(t, updated)
	if m.viewport.YOffset != 1 {
		t.Fatalf("j details scroll offset = %d, want 1", m.viewport.YOffset)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = watchModel(t, updated)
	if !m.viewport.AtBottom() {
		t.Fatalf("G should scroll details to bottom, offset=%d", m.viewport.YOffset)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = watchModel(t, updated)
	if m.viewport.YOffset != 0 {
		t.Fatalf("g details scroll offset = %d, want 0", m.viewport.YOffset)
	}
}

func TestDetailsViewDoesNotAddPageHeader(t *testing.T) {
	vp := viewport.New(32, 4)
	vp.SetContent(strings.Join([]string{"one", "two", "three", "four", "five", "six", "seven", "eight"}, "\n"))
	vp.GotoBottom()
	m := Model{width: 32, preview: true, viewport: vp, rows: []row{{ticket: "PROJ-123", name: "password-reset"}}}
	view := m.renderPreview()
	if !strings.Contains(view, "five") || strings.Contains(view, "orc  details") {
		t.Fatalf("renderPreview() should show viewport content without a page header:\n%s", view)
	}
	if got := lipgloss.Width(view); got > 32 {
		t.Fatalf("renderPreview() width = %d, want <= 32:\n%s", got, view)
	}
}

func TestDetailCardEmbedsTitleInBorder(t *testing.T) {
	view := ansi.Strip(renderDetailCard("WORKFLOW", "● develop", 32))
	if !strings.HasPrefix(view, "╭─ WORKFLOW ") || !strings.Contains(view, "│ ● develop") {
		t.Fatalf("renderDetailCard() should embed its title in the border:\n%s", view)
	}
}

func TestFeatureDetailTitleAvoidsRepeatingTicketPrefix(t *testing.T) {
	got := featureDetailTitle(row{ticket: "STORY-789", name: "STORY-789-migrate-database"})
	if got != "migrate-database · STORY-789" {
		t.Fatalf("featureDetailTitle() = %q", got)
	}
}

func TestDetailsWorkflowSummaryAndStageTiming(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	r := row{
		stageName: "develop", status: "active",
		workflowSteps: []workflowStep{
			{name: "intake", label: "intake", advance: "auto"},
			{name: "develop", label: "develop", advance: "manual"},
			{name: "review", label: "review", advance: "manual"},
		},
		history: []historyRow{
			{at: "2026-07-18T10:00:00Z", stage: "develop"},
			{at: "2026-07-18T11:30:00Z", stage: "develop"},
		},
	}
	if current, total := workflowPosition(r); current != 2 || total != 3 {
		t.Fatalf("workflowPosition() = %d/%d, want 2/3", current, total)
	}
	if got, want := nextTransition(r), "next  review · approval required"; got != want {
		t.Fatalf("nextTransition() = %q, want %q", got, want)
	}
	if got, want := currentStageTiming(r, now), "30m in stage · 2 visits"; got != want {
		t.Fatalf("currentStageTiming() = %q, want %q", got, want)
	}
}

func TestTimelineHistoryUsesConnectedEvents(t *testing.T) {
	view := renderTimelineHistory([]historyRow{
		{at: "2026-07-18T10:00:00Z", stage: "develop", worker: "bob", result: "implemented watch"},
		{at: "2026-07-18T11:00:00Z", stage: "review", worker: "ada", result: "approved"},
	}, 48, 12)
	for _, want := range []string{"● Jul 18 10:00 · develop · bob", "│ implemented watch", "● Jul 18 11:00 · review · ada", "│ approved"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderTimelineHistory() missing %q in:\n%s", want, view)
		}
	}
}

func TestDemoRailAndDetailsFitTwentyFourColumns(t *testing.T) {
	rows := demoRows("")
	m := Model{width: 24, height: 40, rows: rows, allRows: rows, now: time.Now()}
	if got := lipgloss.Width(m.renderRail()); got > m.width {
		t.Fatalf("demo rail width = %d, want <= %d", got, m.width)
	}
	m.preview = true
	if got := lipgloss.Width(m.previewContent()); got > m.width {
		t.Fatalf("demo details width = %d, want <= %d", got, m.width)
	}
	m.preview = false
	m.help = true
	if got := lipgloss.Width(m.renderHelp()); got > m.width {
		t.Fatalf("help width = %d, want <= %d", got, m.width)
	}
}
