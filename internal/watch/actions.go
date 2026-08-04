package watch

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/tmux"
	tea "github.com/charmbracelet/bubbletea"
)

const watchPromptTimeout = 5 * time.Second

func (m *Model) beginPrompt() string {
	if m.demo {
		return "demo mode: prompting is disabled"
	}
	r, ok := m.selectedWork()
	if !ok {
		return "no session selected"
	}
	if r.tmuxState != "live" {
		return "agent session stopped for " + r.ticket
	}
	if r.backend == "" || r.session == "" || r.window == "" || r.pane == "" || r.agentID == "" || r.agentInstance == "" {
		return "no exact agent target for " + r.ticket
	}
	controller, ok := m.mux.(mux.AgentPromptBackend)
	if !ok || controller.Name() != r.backend {
		return r.backend + " prompting is unavailable"
	}
	acknowledgeRow(m.mux, r)
	m.promptRow = r
	m.promptBox.SetValue("")
	m.promptBox.Focus()
	m.prompting = true
	m.confirming = false
	return ""
}

func (m *Model) cancelPrompt() {
	m.prompting = false
	m.confirming = false
	m.promptBox.Blur()
	m.promptBox.SetValue("")
	m.promptRow = row{}
}

func (m *Model) sendConfirmedPrompt() tea.Cmd {
	r := m.promptRow
	text := m.promptBox.Value()
	controller, ok := m.mux.(mux.AgentPromptBackend)
	if !ok || controller.Name() != r.backend {
		m.cancelPrompt()
		m.message = r.backend + " prompting is unavailable"
		return nil
	}
	target := mux.Target{
		Backend: r.backend, Workspace: r.session, Tab: r.window, Pane: r.pane,
		AgentID: r.agentID, AgentInstance: r.agentInstance,
	}
	m.prompting = false
	m.confirming = false
	m.promptBox.Blur()
	m.promptBox.SetValue("")
	m.promptRow = row{}
	m.message = "sending prompt to " + r.ticket
	return func() tea.Msg {
		result, err := controller.PromptAgent(target, text, true, mux.AgentControlOptions{
			Until: []string{
				mux.LifecycleIdle, mux.LifecycleWorking, mux.LifecycleBlocked, mux.LifecycleDone, mux.LifecycleUnknown,
			},
			Timeout: watchPromptTimeout,
			Context: context.Background(),
		})
		return promptDoneMsg{ticket: r.ticket, result: result, err: err}
	}
}

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
	acknowledgeRow(backend, r)
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
		acknowledgeRow(backend, r)
		return backend.AttachPane(r.session, r.window, r.pane)
	}
	return fmt.Errorf("no live session needs attention")
}

func acknowledgeRow(backend mux.Backend, r row) {
	acknowledger, ok := backend.(mux.AgentAcknowledgeBackend)
	if !ok || r.backend != acknowledger.Name() || r.pane == "" || r.agentID == "" || r.agentInstance == "" {
		return
	}
	_ = acknowledger.AcknowledgeAgent(mux.Target{
		Backend: r.backend, Workspace: r.session, Tab: r.window, Pane: r.pane,
		AgentID: r.agentID, AgentInstance: r.agentInstance,
	})
}

var newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
	return tmux.New().AttachCommand(session, window, pane)
}
