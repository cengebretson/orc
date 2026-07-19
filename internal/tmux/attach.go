package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Attach attaches to a session (or switches if already inside tmux).
// target is "session" or "session:window".
func Attach(target string) error {
	if os.Getenv("TMUX") != "" {
		return newCommand("tmux", "switch-client", "-t", target).Run()
	}
	cmd := newCommand("tmux", "attach-session", "-t", target)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AttachTarget focuses an exact agent pane. Older STATE.yaml files without a
// pane id are resolved only when the stage window has one safe target.
func AttachTarget(session, window, pane string) error {
	cmd, err := AttachCommandTarget(session, window, pane)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AttachCommandTarget validates a stored pane before constructing the attach
// command. This prevents reused or stale pane ids from focusing unrelated work.
func AttachCommandTarget(session, window, pane string) (*exec.Cmd, error) {
	var err error
	if pane == "" {
		pane, err = ResolvePaneTarget(session, window)
	} else {
		pane, err = ValidatePaneTarget(session, window, pane)
	}
	if err != nil {
		return nil, err
	}
	return AttachCommand(session, window, pane), nil
}

// AttachCommand constructs the tmux command used by CLI and Bubble Tea paths.
func AttachCommand(session, window, pane string) *exec.Cmd {
	target := session + ":" + window
	if pane == "" {
		if os.Getenv("TMUX") != "" {
			return newCommand("tmux", "switch-client", "-t", target)
		}
		return newCommand("tmux", "attach-session", "-t", target)
	}
	if os.Getenv("TMUX") != "" {
		return newCommand("tmux", "switch-client", "-t", session, ";", "select-pane", "-t", pane)
	}
	return newCommand("tmux", "select-pane", "-t", pane, ";", "attach-session", "-t", target)
}

// KillSession kills a tmux session by name.
func KillSession(name string) error {
	if err := newCommand("tmux", "kill-session", "-t", name).Run(); err != nil {
		return fmt.Errorf("kill session %s: %w", name, err)
	}
	return nil
}

// ListSessions returns all tmux session names, or nil if none exist.
func ListSessions() []string {
	out, err := newCommand("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}
