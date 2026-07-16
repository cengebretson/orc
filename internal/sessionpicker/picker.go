// Package sessionpicker provides the interactive provider-session chooser used
// by `orc sessions resume` when no deterministic session ID is supplied.
package sessionpicker

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/searchmatch"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

var ErrCancelled = errors.New("session selection cancelled")

type Candidate struct {
	Live   telemetry.Live
	Branch string
}

type Model struct {
	all          []Candidate
	visible      []Candidate
	search       textinput.Model
	cursor       int
	width        int
	height       int
	now          time.Time
	selected     Candidate
	hasSelection bool
	cancelled    bool
}

func New(candidates []Candidate) Model {
	search := textinput.New()
	search.Prompt = "/ "
	search.Placeholder = "filter sessions..."
	search.CharLimit = 128
	search.Focus()
	items := append([]Candidate(nil), candidates...)
	return Model{all: items, visible: items, search: search, now: time.Now()}
}

func Select(candidates []Candidate) (Candidate, error) {
	if len(candidates) == 0 {
		return Candidate{}, fmt.Errorf("no resumable Claude or Codex sessions were discovered")
	}
	result, err := tea.NewProgram(New(candidates), tea.WithAltScreen()).Run()
	if err != nil {
		return Candidate{}, err
	}
	model, ok := result.(Model)
	if !ok || model.cancelled || !model.hasSelection {
		return Candidate{}, ErrCancelled
	}
	return model.selected, nil
}

func (m Model) Init() tea.Cmd { return textinput.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.search.Width = max(12, msg.Width-4)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n":
			if m.cursor < len(m.visible)-1 {
				m.cursor++
			}
			return m, nil
		case "enter":
			if len(m.visible) == 0 {
				return m, nil
			}
			m.selected = m.visible[m.cursor]
			m.hasSelection = true
			return m, tea.Quit
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.applyFilter()
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) View() string {
	width := max(40, m.width)
	height := max(8, m.height)
	var b strings.Builder
	b.WriteString("Resume provider session\n\n")
	b.WriteString(m.search.View())
	_, _ = fmt.Fprintf(&b, "  %d/%d\n\n", len(m.visible), len(m.all))

	if len(m.visible) == 0 {
		b.WriteString("  No matching sessions.\n")
	} else {
		limit := max(1, (height-7)/2)
		start := 0
		if m.cursor >= limit {
			start = m.cursor - limit + 1
		}
		end := min(len(m.visible), start+limit)
		for i := start; i < end; i++ {
			candidate := m.visible[i]
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			b.WriteString(truncate(prefix+candidate.summary(m.now), width))
			b.WriteString("\n")
			b.WriteString(truncate("    "+cwdLabel(candidate.Live.CWD), width))
			b.WriteString("\n")
		}
	}
	b.WriteString("\nup/down select  enter resume  esc cancel")
	return b.String()
}

func (m *Model) applyFilter() {
	query := m.search.Value()
	visible := make([]Candidate, 0, len(m.all))
	for _, candidate := range m.all {
		live := candidate.Live
		if searchmatch.Match(query, live.Engine, live.ProviderSessionID, live.Model,
			live.Effort, live.State, live.CWD, live.Ticket, candidate.Branch) {
			visible = append(visible, candidate)
		}
	}
	m.visible = visible
	m.cursor = 0
}

func (c Candidate) summary(now time.Time) string {
	live := c.Live
	return fmt.Sprintf("%s  %s  %s  ctx %s  %s  %s",
		emptyDash(live.Engine), emptyDash(live.Model), shortID(live.ProviderSessionID),
		contextValue(live), emptyDash(c.Branch), relativeTime(now, live.LastActive))
}

func contextValue(live telemetry.Live) string {
	if live.ContextLimit > 0 {
		return fmt.Sprintf("%d%%", live.ContextUsed*100/live.ContextLimit)
	}
	if live.ContextUsed > 0 {
		return fmt.Sprintf("%d", live.ContextUsed)
	}
	return "-"
}

func relativeTime(now, then time.Time) string {
	if then.IsZero() {
		return "-"
	}
	d := now.Sub(then)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func shortID(id string) string {
	if len(id) <= 12 {
		return emptyDash(id)
	}
	return id[:12]
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(value) <= width {
		return value
	}
	if width == 1 {
		return value[:1]
	}
	return value[:width-1] + "…"
}

func cwdLabel(cwd string) string {
	if cwd == "" {
		return "-"
	}
	return filepath.Clean(cwd)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
