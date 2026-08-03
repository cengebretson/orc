package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
)

// CaptureTarget returns recent scrollback from an exact pane after proving the
// pane still belongs to the recorded session and window.
func CaptureTarget(target mux.Target, lines int) (string, error) {
	if target.Backend != "" && target.Backend != "tmux" {
		return "", fmt.Errorf("tmux cannot capture %s target", target.Backend)
	}
	if target.Workspace == "" || target.Tab == "" || target.Pane == "" {
		return "", fmt.Errorf("tmux capture requires exact session, window, and pane ids")
	}
	if lines <= 0 {
		return "", fmt.Errorf("tmux capture lines must be greater than zero")
	}
	if lines > mux.MaxCaptureLines {
		return "", fmt.Errorf("tmux capture lines must not exceed %d", mux.MaxCaptureLines)
	}
	pane, err := ValidatePaneTarget(target.Workspace, target.Tab, target.Pane)
	if err != nil {
		return "", err
	}
	out, err := newCommand(
		"tmux", "capture-pane", "-p", "-J", "-t", pane,
		"-S", "-"+strconv.Itoa(lines),
	).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("capture tmux pane %s: %s: %w", pane, strings.TrimSpace(string(out)), err)
	}
	return recentLines(string(out), lines), nil
}

func recentLines(text string, limit int) string {
	trailingNewline := strings.HasSuffix(text, "\n")
	trimmed := strings.TrimSuffix(text, "\n")
	lines := strings.Split(trimmed, "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	result := strings.Join(lines, "\n")
	if trailingNewline {
		result += "\n"
	}
	return result
}
