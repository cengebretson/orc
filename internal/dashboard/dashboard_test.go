package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/watch"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func dashboardModel(t *testing.T, value tea.Model) Model {
	t.Helper()
	m, ok := value.(Model)
	if !ok {
		t.Fatalf("model = %T, want dashboard.Model", value)
	}
	return m
}

func TestDashboardSwitchesLiveAndWorkspaceSections(t *testing.T) {
	m, err := New(t.TempDir(), Options{
		Start:    SectionLive,
		Adaptive: true,
		Watch:    watch.Options{Interval: time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = dashboardModel(t, updated)
	if !strings.Contains(m.View(), "[ LIVE ]") {
		t.Fatalf("live dashboard header missing selection:\n%s", m.View())
	}
	if strings.Contains(m.View(), "ORC WATCH") {
		t.Fatalf("embedded Live view should rely on dashboard navigation:\n%s", m.View())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = dashboardModel(t, updated)
	if m.section != SectionWorkspace || !strings.Contains(m.View(), "[ WORKSPACE ]") {
		t.Fatalf("] should switch to Workspace, section=%v:\n%s", m.section, m.View())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	m = dashboardModel(t, updated)
	if m.section != SectionLive {
		t.Fatalf("[ should switch to Live, section=%v", m.section)
	}
}

func TestAdaptiveWatchKeepsNarrowLayoutUnwrapped(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionLive, Adaptive: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 32, Height: 30})
	m = dashboardModel(t, updated)
	if m.shellVisible() || strings.Contains(m.View(), "ORC DASHBOARD") {
		t.Fatalf("narrow adaptive watch should not render dashboard shell:\n%s", m.View())
	}
}

func TestDashboardHelpOwnsSharedNavigation(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionLive})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = dashboardModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = dashboardModel(t, updated)
	for _, want := range []string{"ORC DASHBOARD · HELP", "[ / ]", "LIVE", "WORKSPACE"} {
		if !strings.Contains(m.View(), want) {
			t.Fatalf("dashboard help missing %q:\n%s", want, m.View())
		}
	}
}

func TestDashboardHelpFitsNarrowTerminal(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionWorkspace})
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
	m, err := New(t.TempDir(), Options{Start: SectionLive})
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	m = dashboardModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = dashboardModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	m = dashboardModel(t, updated)
	if m.section != SectionLive {
		t.Fatalf("search input ] switched section to %v", m.section)
	}
}

func TestDashboardOnlyRunsTheActiveSection(t *testing.T) {
	m, err := New(t.TempDir(), Options{Start: SectionWorkspace})
	if err != nil {
		t.Fatal(err)
	}
	if m.live.IsActive() || !m.workspace.IsActive() || m.live.Init() != nil {
		t.Fatalf("initial lifecycle: live=%v workspace=%v", m.live.IsActive(), m.workspace.IsActive())
	}
	if cmd := m.switchSection(SectionLive); cmd == nil {
		t.Fatal("switching to Live should start its refresh lifecycle")
	}
	if !m.live.IsActive() || m.workspace.IsActive() || m.workspace.Init() != nil {
		t.Fatalf("switched lifecycle: live=%v workspace=%v", m.live.IsActive(), m.workspace.IsActive())
	}
}
