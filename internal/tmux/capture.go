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
	return string(out), nil
}
