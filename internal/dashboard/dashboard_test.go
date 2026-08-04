package dashboard

import (
	"strings"
	"testing"

	terminalui "github.com/cengebretson/orc/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func dashboardModel(t *testing.T, value tea.Model) Model {
	t.Helper()
	m, ok := value.(Model)
	if !ok {
		t.Fatalf("model = %T, want dashboard.Model", value)
	}
	return m
}

func TestDashboardCyclesUnifiedSections(t *testing.T) {
	m, err := New(t.TempDir(), Options{
		Start: SectionFeatures,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = dashboardModel(t, updated)
	if !strings.Contains(m.View(), "👹 ORC") || !strings.Contains(m.View(), "[ LIVE ]") {
		t.Fatalf("features dashboard header missing brand or selection:\n%s", m.View())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = dashboardModel(t, updated)
	if m.section != SectionWorkflows || !strings.Contains(m.View(), "[ WORKFLOWS ]") {
		t.Fatalf("] should switch to Workflows, section=%v:\n%s", m.section, m.View())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = dashboardModel(t, updated)
	if m.section != SectionFeatures {
		t.Fatalf("[ should switch to Features, section=%v", m.section)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = dashboardModel(t, updated)
	if m.section != SectionWorkflows {
		t.Fatalf("tab should switch to Workflows, section=%v", m.section)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = dashboardModel(t, updated)
	if m.section != SectionFeatures {
		t.Fatalf("shift+tab should switch to Features, section=%v", m.section)
	}
}

func TestAdaptiveWatchKeepsNarrowLayoutUnwrapped(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionWatch, Adaptive: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 32, Height: 30})
	m = dashboardModel(t, updated)
	if m.shellVisible() || strings.Contains(m.View(), "ORC DASHBOARD") {
		t.Fatalf("narrow adaptive watch should not render dashboard shell:\n%s", m.View())
	}
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = dashboardModel(t, updated)
	if m.shellVisible() || strings.Contains(m.View(), "[ LIVE ]") {
		t.Fatalf("wide adaptive watch should remain a dedicated Live view:\n%s", m.View())
	}
}

func TestResponsiveDashboardUsesLiveNarrowAndRestoresWideSection(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionHealth, Adaptive: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = dashboardModel(t, updated)
	if m.section != SectionHealth || !m.shellVisible() {
		t.Fatalf("wide dashboard should start in Health: section=%v shell=%v", m.section, m.shellVisible())
	}

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 30})
	m = dashboardModel(t, updated)
	if m.section != SectionWatch || m.shellVisible() || !m.live.IsActive() || m.workspace.IsActive() {
		t.Fatalf("narrow dashboard should use Live: section=%v shell=%v live=%v workspace=%v",
			m.section, m.shellVisible(), m.live.IsActive(), m.workspace.IsActive())
	}

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = dashboardModel(t, updated)
	if m.section != SectionHealth || !m.shellVisible() || m.live.IsActive() || !m.workspace.IsActive() {
		t.Fatalf("wide dashboard should restore Health: section=%v shell=%v live=%v workspace=%v",
			m.section, m.shellVisible(), m.live.IsActive(), m.workspace.IsActive())
	}
}

func TestHealthNavigationLabelShowsWarningBadge(t *testing.T) {
	badge := renderNavigationItem(SectionHealth, 2, false, false)
	if !strings.Contains(badge, "HEALTH") {
		t.Fatalf("Health nav item = %q, want HEALTH label", badge)
	}
	if !strings.Contains(badge, warningStyle.Render("⚠ 2")) {
		t.Fatalf("Health warning badge is not yellow: %q", badge)
	}
	healthy := renderNavigationItem(SectionHealth, 0, false, false)
	if strings.Contains(healthy, "⚠") {
		t.Fatalf("healthy Health nav item = %q, want no warning badge", healthy)
	}
}

func TestHealthBadgePulsesOnChangeButNotOnFirstObservation(t *testing.T) {
	var m Model

	// First observation just records a baseline -- nothing to have "changed"
	// from yet, so it must not pulse.
	if cmd := m.notePulseIfHealthChanged(2); cmd != nil {
		t.Fatalf("first observation should not pulse")
	}
	if m.healthPulseStep != 0 {
		t.Fatalf("healthPulseStep = %d after first observation, want 0", m.healthPulseStep)
	}

	// Same count again: still no pulse.
	if cmd := m.notePulseIfHealthChanged(2); cmd != nil {
		t.Fatalf("unchanged count should not pulse")
	}

	// Count changes: pulse starts.
	if cmd := m.notePulseIfHealthChanged(3); cmd == nil {
		t.Fatalf("changed count should return a pulse tick command")
	}
	if m.healthPulseStep != pulseSteps {
		t.Fatalf("healthPulseStep = %d, want %d", m.healthPulseStep, pulseSteps)
	}

	badge := renderNavigationItem(SectionHealth, 3, false, true)
	if !strings.Contains(badge, pulseWarningStyle.Render("⚠ 3")) {
		t.Fatalf("pulsing Health badge missing pulse style: %q", badge)
	}

	// The pulse tick decays it back to steady state.
	for m.healthPulseStep > 0 {
		updated, _ := m.Update(pulseTickMsg{})
		m = dashboardModel(t, updated)
	}
	badge = renderNavigationItem(SectionHealth, 3, false, false)
	if !strings.Contains(badge, warningStyle.Render("⚠ 3")) {
		t.Fatalf("settled Health badge missing steady style: %q", badge)
	}
}

func TestSwitchSectionUpdatesSelectedTab(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = dashboardModel(t, updated)

	m.switchSection(SectionWorkers)
	if m.section != SectionWorkers {
		t.Fatalf("section = %v, want SectionWorkers", m.section)
	}
	header := ansi.Strip(m.renderHeader())
	if !strings.Contains(header, "[ WORKERS ]") {
		t.Fatalf("header missing selected tab: %q", header)
	}
}

func TestZeroOpensHiddenOrcViewAndOneReturnsToLive(t *testing.T) {
	m, err := New(t.TempDir(), Options{
		Start:     SectionFeatures,
		Version:   "1.2.3",
		BuildDate: "2026-07-21",
		Revision:  "abcdef123456",
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = dashboardModel(t, updated)
	updated, animationCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	m = dashboardModel(t, updated)
	view := m.View()
	for _, want := range []string{
		"[ 👹 ORC ]", "⣤⣤", defaultLegacyQuote,
		"orc 1.2.3", "built 2026-07-21", "abcdef12", "workspace",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("hidden Orc view missing %q:\n%s", want, view)
		}
	}
	if m.section != SectionOrc || m.live.IsActive() || m.workspace.IsActive() {
		t.Fatalf("Orc lifecycle: section=%v live=%v workspace=%v",
			m.section, m.live.IsActive(), m.workspace.IsActive())
	}
	if animationCmd == nil || m.orcAnimationStep != terminalui.RainbowSteps {
		t.Fatalf("Orc animation did not start: cmd=%v step=%d", animationCmd != nil, m.orcAnimationStep)
	}
	updated, _ = m.Update(orcTickMsg{})
	m = dashboardModel(t, updated)
	if m.orcAnimationStep != terminalui.RainbowSteps-1 {
		t.Fatalf("Orc animation step = %d, want %d", m.orcAnimationStep, terminalui.RainbowSteps-1)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = dashboardModel(t, updated)
	if m.section != SectionFeatures || m.live.IsActive() || !m.workspace.IsActive() {
		t.Fatalf("1 should return to Live: section=%v live=%v workspace=%v",
			m.section, m.live.IsActive(), m.workspace.IsActive())
	}
	if m.orcAnimationStep != 0 {
		t.Fatalf("leaving Orc should stop animation, step=%d", m.orcAnimationStep)
	}
}

func TestOrcViewRotatesConfiguredQuotesOnEachVisit(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	m.quotes = []string{"first quote", "second quote"}
	m.quote = "first quote"

	m.switchSection(SectionOrc)
	if m.quote != "second quote" {
		t.Fatalf("first Orc visit quote = %q, want second quote", m.quote)
	}
	m.switchSection(SectionFeatures)
	m.switchSection(SectionOrc)
	if m.quote != "first quote" {
		t.Fatalf("second Orc visit quote = %q, want first quote", m.quote)
	}
}

func TestDashboardHelpOwnsSharedNavigation(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = dashboardModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = dashboardModel(t, updated)
	for _, want := range []string{"ORC DASHBOARD · HELP", "[ / shift+tab", "1–5", "Live / Workflows"} {
		if !strings.Contains(m.View(), want) {
			t.Fatalf("dashboard help missing %q:\n%s", want, m.View())
		}
	}
}

func TestDashboardNumberKeysOpenEveryDestination(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = dashboardModel(t, updated)

	for index, want := range sections {
		key := rune('1' + index)
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = dashboardModel(t, updated)
		if m.section != want {
			t.Fatalf("%c selected %v, want %v", key, m.section, want)
		}
	}
}

func TestClickingATabSwitchesSection(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = dashboardModel(t, updated)

	for _, want := range sections {
		header := ansi.Strip(strings.Split(m.View(), "\n")[0])
		col := strings.Index(header, strings.ToUpper(sectionLabel(want)))
		if col < 0 {
			t.Fatalf("tab label for %v not found in header: %q", want, header)
		}
		updated, _ = m.Update(tea.MouseMsg{X: col, Y: 0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		m = dashboardModel(t, updated)
		if m.section != want {
			t.Fatalf("clicking %v tab selected %v", want, m.section)
		}
	}
}

func TestClickingOutsideTheHeaderRowDoesNotSwitchSection(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = dashboardModel(t, updated)

	updated, _ = m.Update(tea.MouseMsg{X: 40, Y: 5, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	m = dashboardModel(t, updated)
	if m.section != SectionFeatures {
		t.Fatalf("click below the header row switched section to %v", m.section)
	}
}

func TestDashboardHeaderCollapsesWhenAllTabsDoNotFit(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionRepositories})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 56, Height: 30})
	m = dashboardModel(t, updated)
	header := strings.Split(m.View(), "\n")[0]
	if !strings.Contains(header, "[ REPOSITORIES ]") || !strings.Contains(header, "‹") || !strings.Contains(header, "›") {
		t.Fatalf("compact header missing active-tab controls: %q", header)
	}
	if strings.Contains(header, "WORKFLOWS") {
		t.Fatalf("compact header should hide inactive tabs: %q", header)
	}
}

func TestDashboardHelpFitsNarrowTerminal(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 24, Height: 30})
	m = dashboardModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = dashboardModel(t, updated)
	if got := lipgloss.Width(m.View()); got > 24 {
		t.Fatalf("dashboard help width = %d, want <= 24:\n%s", got, m.View())
	}
}

func TestDashboardDoesNotStealSectionKeyFromLiveSearch(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionWatch, Adaptive: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = dashboardModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = dashboardModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = dashboardModel(t, updated)
	if m.section != SectionWatch {
		t.Fatalf("search input ] switched section to %v", m.section)
	}
}

func TestDashboardOnlyRunsTheActiveSection(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionFeatures})
	if err != nil {
		t.Fatal(err)
	}
	if m.live.IsActive() || !m.workspace.IsActive() || m.live.Init() != nil {
		t.Fatalf("initial lifecycle: live=%v workspace=%v", m.live.IsActive(), m.workspace.IsActive())
	}
	if cmd := m.switchSection(SectionWatch); cmd == nil {
		t.Fatal("switching to Live should start its refresh lifecycle")
	}
	if !m.live.IsActive() || m.workspace.IsActive() || m.workspace.Init() != nil {
		t.Fatalf("switched lifecycle: live=%v workspace=%v", m.live.IsActive(), m.workspace.IsActive())
	}
}

func TestDashboardTerminalSizeMatrix(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		start         Section
	}{
		{name: "narrow", width: 24, height: 18, start: SectionWatch},
		{name: "adaptive boundary", width: 56, height: 18, start: SectionWatch},
		{name: "narrow responsive dashboard", width: 40, height: 30, start: SectionFeatures},
		{name: "short features", width: 80, height: 10, start: SectionFeatures},
		{name: "wide live", width: 120, height: 40, start: SectionWatch},
		{name: "wide workspace", width: 120, height: 40, start: SectionWorkflows},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := New(t.TempDir(), Options{Start: tt.start, Adaptive: true})
			if err != nil {
				t.Fatal(err)
			}
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			m = dashboardModel(t, updated)
			view := m.View()
			if view == "" {
				t.Fatal("View() returned empty output after resize")
			}
			for lineNumber, line := range strings.Split(view, "\n") {
				if width := lipgloss.Width(line); width > tt.width {
					t.Fatalf("line %d width = %d, terminal width = %d:\n%s", lineNumber, width, tt.width, view)
				}
			}
		})
	}
}
