package watch

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
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

// FocusWithMux attaches to the highest-priority live Orc session that needs
// human attention, using backend.
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

// beginAnswer opens the control matching a pending question's declared shape.
// Unlike prompting, this needs no live agent: the answer is recorded durably,
// so a ticket whose agent has exited can still be answered.
func (m *Model) beginAnswer(r row) string {
	if m.demo {
		return "demo mode: answering is disabled"
	}
	if r.featureDir == "" {
		return "no feature directory for " + r.ticket
	}
	m.answerRow = r
	m.answerCursor = 0
	m.answering = true
	if r.question.Kind == state.QuestionKindText {
		m.promptBox.SetValue("")
		m.promptBox.Focus()
	}
	return ""
}

func (m *Model) cancelAnswer() {
	m.answering = false
	m.answerCursor = 0
	m.answerRow = row{}
	m.promptBox.Blur()
	m.promptBox.SetValue("")
}

// submitAnswer records the answer and, separately and best effort, nudges a
// live agent. The record is what matters: if the nudge fails the answer is
// still durable and the next launch prompt will carry it.
func (m *Model) submitAnswer(raw string) tea.Cmd {
	r := m.answerRow
	answered, err := state.AnswerQuestion(r.featureDir, raw)
	if err != nil {
		// Keep the control open so an invalid answer can be corrected in place.
		m.message = err.Error()
		return nil
	}
	m.cancelAnswer()
	m.message = "answered " + r.ticket + ": " + answered.Label(answered.Answer)

	controller, ok := m.mux.(mux.AgentPromptBackend)
	if !ok || controller.Name() != r.backend || r.agentID == "" || r.agentInstance == "" {
		return nil
	}
	target := mux.Target{
		Backend: r.backend, Workspace: r.session, Tab: r.window, Pane: r.pane,
		AgentID: r.agentID, AgentInstance: r.agentInstance,
	}
	text := "The human answered \"" + answered.Prompt + "\": " + answered.Label(answered.Answer) +
		". Read runtime.question in STATE.yaml and continue."
	return func() tea.Msg {
		_, promptErr := controller.PromptAgent(target, text, false, mux.AgentControlOptions{Context: context.Background()})
		return answerDeliveredMsg{ticket: r.ticket, err: promptErr}
	}
}

// answerChoiceAt returns the choice under the cursor, for the pick-list.
func (m Model) answerChoiceAt(index int) (state.QuestionChoice, bool) {
	q := m.answerRow.question
	if q == nil || index < 0 || index >= len(q.Choices) {
		return state.QuestionChoice{}, false
	}
	return q.Choices[index], true
}

// updateAnswering drives the control for a pending question. Each kind gets the
// interaction it deserves: a single keypress for a binary choice, a list for an
// enumerated one, and a text box only where the answer is genuinely open.
func (m Model) updateAnswering(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	q := m.answerRow.question
	if q == nil {
		m.cancelAnswer()
		return m, nil
	}
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.cancelAnswer()
		m.message = "answer cancelled"
		return m, nil
	}

	switch q.Kind {
	case state.QuestionKindConfirm:
		switch msg.String() {
		case "y", "Y":
			return m, m.submitAnswer("yes")
		case "n", "N":
			return m, m.submitAnswer("no")
		}
		return m, nil

	case state.QuestionKindChoice:
		switch msg.String() {
		case "j", "down":
			if m.answerCursor < len(q.Choices)-1 {
				m.answerCursor++
			}
			return m, nil
		case "k", "up":
			if m.answerCursor > 0 {
				m.answerCursor--
			}
			return m, nil
		case "enter":
			if choice, ok := m.answerChoiceAt(m.answerCursor); ok {
				return m, m.submitAnswer(choice.Key)
			}
			return m, nil
		}
		// A choice key typed directly selects it, so a two-option question does
		// not require arrowing to an obvious answer.
		for _, choice := range q.Choices {
			if strings.EqualFold(msg.String(), choice.Key) {
				return m, m.submitAnswer(choice.Key)
			}
		}
		return m, nil

	case state.QuestionKindText:
		if msg.String() == "enter" {
			if strings.TrimSpace(m.promptBox.Value()) == "" {
				m.message = "answer text is required"
				return m, nil
			}
			return m, m.submitAnswer(m.promptBox.Value())
		}
		var cmd tea.Cmd
		m.promptBox, cmd = m.promptBox.Update(msg)
		return m, cmd
	}
	return m, nil
}
