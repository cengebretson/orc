package watch

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func tickEvery(d time.Duration, epoch uint64) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg{at: t, epoch: epoch}
	})
}

func watchAnimationTick(epoch uint64) tea.Cmd {
	return tea.Tick(watchAnimationInterval, func(t time.Time) tea.Msg {
		return watchAnimationMsg{at: t, epoch: epoch}
	})
}
