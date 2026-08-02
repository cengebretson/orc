package tmux

import (
	"fmt"
)

// The attention vocabulary, the resume variable, and the pane and metadata
// shapes all live in internal/mux. They are not re-exported here: while they
// were, a new call site could reach them through this package and stay coupled
// to tmux without meaning to.

// Available returns true if tmux is installed and in PATH.
func Available() bool {
	_, err := findExecutable("tmux")
	return err == nil
}

// SessionExists returns true if a tmux session with the given name exists.
func SessionExists(name string) bool {
	return newCommand("tmux", "has-session", "-t", name).Run() == nil
}

// WindowExists returns true if the named window exists in the session.
func WindowExists(session, window string) bool {
	target := session + ":" + window
	return newCommand("tmux", "select-window", "-t", target).Run() == nil
}

// CreateSession creates a detached tmux session with windows named after each workflow.
func CreateSession(slug, featureDir string, workflows []string) error {
	if len(workflows) == 0 {
		workflows = []string{"shell"}
	}

	// Create detached session with first window
	if err := newCommand("tmux", "new-session", "-d", "-s", slug, "-n", workflows[0], "-c", featureDir).Run(); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Add remaining workflow windows
	for _, w := range workflows[1:] {
		if err := newCommand("tmux", "new-window", "-t", slug, "-n", w, "-c", featureDir).Run(); err != nil {
			return fmt.Errorf("add window %s: %w", w, err)
		}
	}

	// Select the first window — ignore error if focus fails (non-fatal)
	_ = newCommand("tmux", "select-window", "-t", slug+":"+workflows[0]).Run()
	return nil
}

// SendCommand sends a shell command to a window in the session, creating the window if needed.
// runDir is the working directory the command should execute from.
func SendCommand(session, window, featureDir, runDir string, argv []string) error {
	_, err := SendCommandTarget(session, window, "", featureDir, runDir, argv)
	return err
}

// SendCommandTarget sends a command to an exact pane and returns the pane id.
// When pane is empty, a single pane (or a single pane marked @orc_agent) must
// identify the target; ambiguous windows are rejected rather than guessed.
func SendCommandTarget(session, window, pane, featureDir, runDir string, argv []string) (string, error) {

	// Create window if it doesn't exist
	if !WindowExists(session, window) {
		if err := newCommand("tmux", "new-window", "-t", session, "-n", window, "-c", featureDir).Run(); err != nil {
			return "", fmt.Errorf("create window %s: %w", window, err)
		}
	}
	if pane == "" {
		var err error
		pane, err = ResolvePaneTarget(session, window)
		if err != nil {
			return "", err
		}
	} else {
		var err error
		pane, err = ValidatePaneTarget(session, window, pane)
		if err != nil {
			return "", err
		}
	}

	// Write command to a temp script to avoid quoting issues with send-keys.
	// The shell and script both exec so tmux's pane PID becomes the provider PID.
	script, err := writeScript(runDir, argv)
	if err != nil {
		return "", err
	}

	if err := newCommand("tmux", "send-keys", "-t", pane, "exec bash "+shellQuote(script), "Enter").Run(); err != nil {
		return "", fmt.Errorf("send command to pane %s: %w", pane, err)
	}
	return pane, nil
}
