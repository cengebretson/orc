package workspaceui

import (
	"time"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	tea "github.com/charmbracelet/bubbletea"
)

func attachMux(backend mux.Backend, target state.MuxRuntime) tea.Cmd {
	if backend == nil {
		backend = tmux.New()
	}
	cmd, err := backend.AttachCommand(target.Workspace, target.Tab, target.Pane)
	if err != nil {
		return func() tea.Msg { return err }
	}
	return tea.ExecProcess(cmd, func(error) tea.Msg { return nil })
}

func tickEvery(d time.Duration, epoch uint64) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg{at: t, epoch: epoch}
	})
}

func liveTickEvery(d time.Duration, epoch uint64) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return liveTickMsg{at: t, epoch: epoch}
	})
}

func quoteRotateTick(epoch uint64) tea.Cmd {
	return tea.Tick(quoteRotateInterval, func(time.Time) tea.Msg {
		return quoteRotateTickMsg{epoch: epoch}
	})
}

func breathTick(epoch uint64) tea.Cmd {
	return tea.Tick(breathInterval, func(time.Time) tea.Msg {
		return breathTickMsg{epoch: epoch}
	})
}
