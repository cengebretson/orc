package workspaceui

import (
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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

func attachTmux(session, window string) tea.Cmd {
	return tea.ExecProcess(
		newTmuxCmd(session, window),
		func(err error) tea.Msg { return nil },
	)
}

func newTmuxCmd(session, window string) *exec.Cmd {
	target := session + ":" + window
	if os.Getenv("TMUX") != "" {
		return exec.Command("tmux", "switch-client", "-t", target)
	}
	return exec.Command("tmux", "attach-session", "-t", target)
}
