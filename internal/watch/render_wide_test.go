package watch

import (
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

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
