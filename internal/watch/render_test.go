package watch

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/config"
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

func TestStageDisplayLabelUsesConfiguredAlias(t *testing.T) {
	cfg := &config.Config{Aliases: config.Aliases{Stages: map[string]string{"build": "default:develop"}}}
	if got := stageDisplayLabel(cfg, "default:develop"); got != "build" {
		t.Fatalf("stageDisplayLabel() = %q, want build", got)
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

func TestRenderWideUsesSelectionIndicator(t *testing.T) {
	m := Model{
		width:  80,
		height: 20,
		cursor: 1,
		rows: []row{
			{ticket: "PROJ-123", stage: "develop", worker: "bob", status: "paused", tmuxState: "live"},
			{
				ticket:    "PROJ-124",
				stage:     "review",
				worker:    "ada",
				status:    "active",
				tmuxState: "live",
				history: []historyRow{
					{at: "2026-06-20T10:00:00Z", stage: "develop", worker: "bob", result: "implemented watch rail"},
					{at: "2026-06-21T11:00:00Z", stage: "review", worker: "ada", result: "requested changes"},
				},
			},
		},
	}

	view := m.renderWide()
	if !strings.Contains(view, "▌ PROJ-124") {
		t.Fatalf("renderWide() should mark selected work row with ▌:\n%s", view)
	}
	if !strings.Contains(view, "Worker") || !strings.Contains(view, "ada") {
		t.Fatalf("renderWide() should include worker in the session list:\n%s", view)
	}
	for _, want := range []string{"orc", "workspace orchestrator", "● 2 RUNNING", "◐ 1 PAUSED", "╭", "● ACTIVE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderWide() missing redesigned watch element %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "WORKERS") {
		t.Fatalf("renderWide() should not render a separate worker list:\n%s", view)
	}
	for _, notWant := range []string{"HISTORY", "implemented watch rail", "requested changes"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("renderWide() should keep history on the expanded details page, found %q in:\n%s", notWant, view)
		}
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
		{name: "green boundary", used: 40, limit: 100, want: contextGreenStyle.Render("40%")},
		{name: "yellow boundary", used: 70, limit: 100, want: contextYellowStyle.Render("70%")},
		{name: "red boundary", used: 90, limit: 100, want: contextRedStyle.Render("90%")},
		{name: "unknown limit", used: 42, limit: 0, want: mutedStyle.Render("n/a")},
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

func TestRenderWideShowsContextColumnAndUnavailableLimit(t *testing.T) {
	m := Model{width: 100, rows: []row{{
		ticket: "PROJ-123", status: "active", tmuxState: "live",
		context: contextpressure.Evaluate(42, 0, contextpressure.DefaultThresholds()),
	}}}
	view := m.renderWide()
	for _, want := range []string{"Context", "n/a"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderWide() missing %q in:\n%s", want, view)
		}
	}
}

func TestRenderWideExpandsColumnsWithTerminal(t *testing.T) {
	m := Model{width: 140, rows: []row{{
		ticket: "PROJECT-1234567890-LONG", stage: "quality-assurance-automation", worker: "Alexandra Documentation Specialist",
		status: "active", tmuxState: "attached-session-window",
	}}}
	view := m.renderWide()
	for _, want := range []string{"PROJECT-1234567890-LONG", "quality-assurance-automation", "Alexandra Documentation Specialist", "attached-session"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expanded wide table missing %q:\n%s", want, view)
		}
	}
}

func TestRenderWideUnicodeRowsStayWithinTerminal(t *testing.T) {
	m := Model{width: 80, rows: []row{{
		ticket: "界界-123", stage: "révision-界", worker: "🧙-worker", status: "active", tmuxState: "live",
	}}}
	for lineNo, line := range strings.Split(m.renderWide(), "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("line %d occupied %d cells, want <= %d:\n%s", lineNo+1, got, m.width, m.renderWide())
		}
	}
}

func TestStaleLifecycleTicksDoNotRestartAfterReactivation(t *testing.T) {
	m, err := New(t.TempDir(), Options{Interval: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	staleEpoch := m.epoch
	m = m.SetActive(false)
	m = m.SetActive(true)
	for _, msg := range []tea.Msg{
		tickMsg{at: time.Now(), epoch: staleEpoch},
		watchAnimationMsg{at: time.Now(), epoch: staleEpoch},
		petTickMsg{at: time.Now(), epoch: staleEpoch},
	} {
		updated, cmd := m.Update(msg)
		m = watchModel(t, updated)
		if cmd != nil {
			t.Fatalf("stale lifecycle message %T restarted a timer", msg)
		}
	}
}

func TestRenderWideSelectedPanelAlignsWithTableWidth(t *testing.T) {
	m := Model{width: 100, rows: []row{{ticket: "PROJ-123", stage: "develop", status: "active", next: "continue"}}}
	lines := strings.Split(ansi.Strip(m.renderWide()), "\n")
	panelTop := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "╭") {
			panelTop = line
		}
	}
	if got := lipgloss.Width(panelTop); got != 100 {
		t.Fatalf("selected panel width = %d, want 100:\n%s", got, m.renderWide())
	}
}

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
	view := renderRailDetail(row{
		stage:     "develop",
		worker:    "bob",
		status:    "active",
		tmuxState: "live",
		next:      "fix the failing password reset tests and hand off to review",
	}, 18)

	for _, want := range []string{"Next", "fix the failing", "password reset", "hand off", "review"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderRailDetail() missing wrapped text %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "fix the failing password reset tests and hand off to review") {
		t.Fatalf("renderRailDetail() should wrap, not leave next action on one line:\n%s", view)
	}
}

func TestRailDetailUsesIndentedMetaAndPromptHeader(t *testing.T) {
	view := renderRailDetail(row{
		stage:     "default:pr-repair",
		worker:    "Bob (Developer)",
		status:    "paused",
		tmuxState: "live",
		next:      "Need product decision on refresh token TTL",
	}, 32)
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
		t.Fatalf("renderRailDetail() has too few lines:\n%s", view)
	}
	for i, want := range wantOrder {
		if lines[i] != want {
			t.Fatalf("line %d = %q, want %q in:\n%s", i, lines[i], want, view)
		}
	}
}

func TestPromptLabelUsesBlockerForPaused(t *testing.T) {
	blocked := renderRailDetail(row{status: "paused", next: "need product decision"}, 24)
	if !strings.Contains(blocked, "Blocker") || strings.Contains(blocked, "Next") {
		t.Fatalf("paused rail detail should label prompt as Blocker:\n%s", blocked)
	}

	active := renderRailDetail(row{status: "active", next: "continue implementation"}, 24)
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

	view := renderHistory(rows, 72, 2)
	if strings.Contains(view, "oldest") {
		t.Fatalf("renderHistory() should omit entries past the limit:\n%s", view)
	}
	for _, want := range []string{"2026-06-18", "older", "2026-06-19", "newest"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderHistory() missing %q in:\n%s", want, view)
		}
	}
}

func TestDisplayStateUsesDurableStatusFirst(t *testing.T) {
	tests := []struct {
		name      string
		row       row
		wantIcon  string
		wantLabel string
	}{
		{
			name:      "paused is blocked even with live tmux",
			row:       row{status: "paused", tmuxState: "live"},
			wantIcon:  "!",
			wantLabel: "blocked",
		},
		{
			name:      "done is done even with stopped tmux",
			row:       row{status: "done", tmuxState: "stopped"},
			wantIcon:  "✓",
			wantLabel: "done",
		},
		{
			name:      "pending is pending",
			row:       row{status: "pending", tmuxState: "live"},
			wantIcon:  "○",
			wantLabel: "pending",
		},
		{
			name:      "ready is ready",
			row:       row{status: "ready", tmuxState: "live"},
			wantIcon:  "▶",
			wantLabel: "ready",
		},
		{
			name:      "active without tmux is stopped",
			row:       row{status: "active", tmuxState: "stopped"},
			wantIcon:  "x",
			wantLabel: "stopped",
		},
		{
			name:      "active with live tmux is active",
			row:       row{status: "active", tmuxState: "live"},
			wantIcon:  "●",
			wantLabel: "active",
		},
		{
			name:      "active input attention needs input",
			row:       row{status: "active", tmuxState: "live", attention: "input"},
			wantIcon:  "!",
			wantLabel: "input",
		},
		{
			name:      "active review attention needs review",
			row:       row{status: "active", tmuxState: "live", attention: "review"},
			wantIcon:  "◆",
			wantLabel: "review",
		},
		{
			name:      "durable paused overrides input attention",
			row:       row{status: "paused", tmuxState: "live", attention: "input"},
			wantIcon:  "!",
			wantLabel: "blocked",
		},
		{
			name:      "stopped overrides attention",
			row:       row{status: "active", tmuxState: "stopped", attention: "input"},
			wantIcon:  "x",
			wantLabel: "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon, label := displayState(tt.row)
			if icon != tt.wantIcon || label != tt.wantLabel {
				t.Fatalf("displayState() = %q/%q, want %q/%q", icon, label, tt.wantIcon, tt.wantLabel)
			}
		})
	}
}

func TestSortRowsPrioritizesAttention(t *testing.T) {
	rows := []row{
		{ticket: "ACTIVE", status: "active", tmuxState: "live"},
		{ticket: "DONE", status: "done", tmuxState: "stopped"},
		{ticket: "REVIEW", status: "active", tmuxState: "live", attention: "review"},
		{ticket: "STOPPED", status: "active", tmuxState: "stopped"},
		{ticket: "INPUT", status: "active", tmuxState: "live", attention: "input"},
		{ticket: "BLOCKED", status: "paused", tmuxState: "live"},
	}

	sortRows(rows)
	got := make([]string, 0, len(rows))
	for _, r := range rows {
		got = append(got, r.ticket)
	}
	want := []string{"BLOCKED", "INPUT", "REVIEW", "STOPPED", "ACTIVE", "DONE"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sortRows() = %v, want %v", got, want)
	}
}

func TestAttachSelectedValidatesTarget(t *testing.T) {
	tests := []struct {
		name    string
		model   Model
		message string
	}{
		{
			name:    "no selection",
			model:   Model{},
			message: "no session selected",
		},
		{
			name:    "missing target",
			model:   Model{rows: []row{{ticket: "PROJ-123", status: "active"}}},
			message: "no tmux target for PROJ-123",
		},
		{
			name: "stopped",
			model: Model{rows: []row{{
				ticket:    "PROJ-123",
				session:   "PROJ-123",
				window:    "develop",
				status:    "active",
				tmuxState: "stopped",
			}}},
			message: "tmux session stopped for PROJ-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, message := tt.model.attachSelected()
			if cmd != nil {
				t.Fatalf("attachSelected() cmd = %#v, want nil", cmd)
			}
			if message != tt.message {
				t.Fatalf("attachSelected() message = %q, want %q", message, tt.message)
			}
		})
	}
}

func TestAttachSelectedBuildsCommand(t *testing.T) {
	orig := newAttachCmd
	defer func() { newAttachCmd = orig }()
	var gotSession, gotWindow string
	newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
		gotSession = session
		gotWindow = window
		return exec.Command("true"), nil
	}

	m := Model{rows: []row{{
		ticket:    "PROJ-123",
		session:   "PROJ-123",
		window:    "develop",
		status:    "active",
		tmuxState: "live",
	}}}
	cmd, message := m.attachSelected()
	if cmd == nil {
		t.Fatal("attachSelected() cmd = nil, want command")
	}
	if message != "attaching PROJ-123:develop" {
		t.Fatalf("attachSelected() message = %q", message)
	}
	if gotSession != "PROJ-123" || gotWindow != "develop" {
		t.Fatalf("newAttachCmd called with %q/%q", gotSession, gotWindow)
	}
}

func TestWatchUpdateAttachSetsMessage(t *testing.T) {
	orig := newAttachCmd
	defer func() { newAttachCmd = orig }()
	newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
		return exec.Command("true"), nil
	}

	m := Model{rows: []row{{
		ticket:    "PROJ-123",
		session:   "PROJ-123",
		window:    "develop",
		status:    "active",
		tmuxState: "live",
	}}}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if got.message != "attaching PROJ-123:develop" {
		t.Fatalf("message = %q", got.message)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want attach command")
	}
}

func TestWatchUpdateFocusesNextAttentionSession(t *testing.T) {
	orig := newAttachCmd
	defer func() { newAttachCmd = orig }()
	var gotSession, gotWindow string
	newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
		gotSession = session
		gotWindow = window
		return exec.Command("true"), nil
	}

	m := Model{
		cursor: 0,
		rows: []row{
			{ticket: "ACTIVE", session: "ACTIVE", window: "develop", status: "active", tmuxState: "live"},
			{ticket: "REVIEW", session: "REVIEW", window: "code-review", status: "active", tmuxState: "live", attention: "review"},
		},
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}
	if got.message != "attaching REVIEW:code-review" {
		t.Fatalf("message = %q", got.message)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want focus attach command")
	}
	if gotSession != "REVIEW" || gotWindow != "code-review" {
		t.Fatalf("newAttachCmd called with %q/%q", gotSession, gotWindow)
	}
}
