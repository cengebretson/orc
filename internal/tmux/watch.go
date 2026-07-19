package tmux

import (
	"fmt"
	"os"
	"strings"
)

// AttachHint returns the command string a user should run to attach.
func AttachHint(session, window string) string {
	return fmt.Sprintf("tmux attach -t %s:%s", session, window)
}

type WatchToggleOptions struct {
	Root      string
	Ticket    string
	Interval  string
	Wide      bool
	View      string
	PetLayout string
	Demo      bool
	Layout    string
	Size      string
	ExecPath  string
}

// ToggleWatchPane closes an existing watch pane in the current tmux window, or
// opens a narrow right split running orc watch and marks it for future toggles.
func ToggleWatchPane(opts WatchToggleOptions) error {
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("--tmux-toggle must be run inside tmux")
	}
	if !Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}

	pane, err := findWatchPane()
	if err != nil {
		return err
	}
	if pane != "" {
		if err := newCommand("tmux", "kill-pane", "-t", pane).Run(); err != nil {
			return fmt.Errorf("close watch pane %s: %w", pane, err)
		}
		return nil
	}

	size := opts.Size
	if size == "" {
		size = "32"
	}
	layoutFlag, err := watchSplitFlag(opts.Layout)
	if err != nil {
		return err
	}
	cmd := buildWatchCommand(opts)
	out, err := newCommand("tmux", "split-window", layoutFlag, "-l", size, "-P", "-F", "#{pane_id}", cmd).Output()
	if err != nil {
		return fmt.Errorf("open watch pane: %w", err)
	}
	newPane := strings.TrimSpace(string(out))
	if newPane == "" {
		return fmt.Errorf("open watch pane: tmux did not return a pane id")
	}
	if err := newCommand("tmux", "set-option", "-p", "-t", newPane, "@orc_watch", "1").Run(); err != nil {
		return fmt.Errorf("mark watch pane %s: %w", newPane, err)
	}
	return nil
}

func watchSplitFlag(layout string) (string, error) {
	switch layout {
	case "", "right":
		return "-h", nil
	case "bottom":
		return "-v", nil
	default:
		return "", fmt.Errorf("unsupported tmux watch layout %q (use right or bottom)", layout)
	}
}

func findWatchPane() (string, error) {
	out, err := newCommand("tmux", "list-panes", "-F", "#{pane_id}\t#{@orc_watch}").Output()
	if err != nil {
		return "", fmt.Errorf("list panes: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) >= 2 && fields[1] == "1" {
			return fields[0], nil
		}
	}
	return "", nil
}

func buildWatchCommand(opts WatchToggleOptions) string {
	execPath := opts.ExecPath
	if execPath == "" {
		if p, err := os.Executable(); err == nil {
			execPath = p
		}
	}
	if execPath == "" {
		execPath = "orc"
	}

	argv := []string{execPath}
	if opts.Root != "" {
		argv = append(argv, "--workspace", opts.Root)
	}
	argv = append(argv, "watch")
	if opts.Ticket != "" {
		argv = append(argv, opts.Ticket)
	}
	if opts.Interval != "" {
		argv = append(argv, "--interval", opts.Interval)
	}
	if opts.Wide {
		argv = append(argv, "--wide")
	}
	if opts.View != "" && opts.View != "rail" {
		argv = append(argv, "--view", opts.View)
	}
	if opts.PetLayout != "" && opts.PetLayout != "responsive" {
		argv = append(argv, "--pet-layout", opts.PetLayout)
	}
	if opts.Demo {
		argv = append(argv, "--demo")
	}
	return shellJoin(argv)
}
