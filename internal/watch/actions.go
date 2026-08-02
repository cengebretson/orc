package watch

import (
	"fmt"
	"os/exec"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/tmux"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) attachSelected() (tea.Cmd, string) {
	if m.demo {
		return nil, "demo mode: attach is disabled"
	}
	r, ok := m.selectedWork()
	if !ok {
		return nil, "no session selected"
	}
	if r.session == "" || r.window == "" {
		return nil, "no tmux target for " + r.ticket
	}
	if r.tmuxState == "stopped" {
		return nil, "tmux session stopped for " + r.ticket
	}
	target := r.session + ":" + r.window
	if r.pane != "" {
		target = r.pane
	}
	backend := m.mux
	if backend == nil || backend.Name() == "tmux" {
		cmd, err := newAttachCmd(r.session, r.window, r.pane)
		if err != nil {
			return nil, "attach failed: " + err.Error()
		}
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return attachDoneMsg{err: err}
		}), "attaching " + target
	}
	cmd, err := backend.AttachCommand(r.session, r.window, r.pane)
	if err != nil {
		return nil, "attach failed: " + err.Error()
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return attachDoneMsg{err: err}
	}), "attaching " + target
}

func (m *Model) focusNext() (tea.Cmd, string) {
	if m.demo {
		return nil, "demo mode: focus is disabled"
	}
	if len(m.rows) == 0 {
		return nil, "no live session needs attention"
	}
	for offset := 1; offset <= len(m.rows); offset++ {
		idx := (m.cursor + offset) % len(m.rows)
		r := m.rows[idx]
		if !attentionNeeded(r) || r.tmuxState != "live" || r.session == "" || r.window == "" {
			continue
		}
		m.cursor = idx
		m.preview = false
		return m.attachSelected()
	}
	return nil, "no live session needs attention"
}

// Focus attaches to the highest-priority live Orc session that needs human attention.
func Focus(root string) error {
	return FocusWithMux(root, nil)
}

// FocusWithMux attaches to the highest-priority live Orc session using backend.
func FocusWithMux(root string, backend mux.Backend) error {
	if backend == nil {
		backend = tmux.New()
	}
	rows, err := collectRowsWithMux(root, "", backend)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if !attentionNeeded(r) || r.tmuxState != "live" || r.session == "" || r.window == "" {
			continue
		}
		return backend.AttachPane(r.session, r.window, r.pane)
	}
	return fmt.Errorf("no live session needs attention")
}

var newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
	return tmux.New().AttachCommand(session, window, pane)
}
