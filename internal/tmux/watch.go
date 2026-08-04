package tmux

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultRailSize      = "64"
	defaultCollapsedSize = "5"
)

var railAvailable = Available

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

// ToggleWatchPane preserves the legacy watch flag while using the managed rail.
func ToggleWatchPane(opts WatchToggleOptions) error {
	if opts.Size == "" {
		opts.Size = "32"
	}
	return ToggleRail(opts)
}

// OpenRail opens one owned watch rail in the current tmux window. Existing
// owned rails are reused and the invoking pane keeps focus.
func OpenRail(opts WatchToggleOptions) error {
	if err := requireRailTmux(); err != nil {
		return err
	}
	pane, err := findRailPane()
	if err != nil {
		return err
	}
	if pane != "" {
		return nil
	}

	size := opts.Size
	if size == "" {
		size = defaultRailSize
	}
	layout := opts.Layout
	if layout == "" {
		layout = "right"
	}
	layoutFlag, err := watchSplitFlag(layout)
	if err != nil {
		return err
	}
	cmd := buildWatchCommand(opts)
	out, err := newCommand("tmux", "split-window", "-d", layoutFlag, "-l", size, "-P", "-F", "#{pane_id}", cmd).Output()
	if err != nil {
		return fmt.Errorf("open watch rail: %w", err)
	}
	newPane := strings.TrimSpace(string(out))
	if newPane == "" {
		return fmt.Errorf("open watch rail: tmux did not return a pane id")
	}
	updates := [][2]string{
		{"@orc_rail", "1"},
		{"@orc_role", "rail"},
		{"@orc_rail_layout", layout},
		{"@orc_rail_expanded_size", size},
		// Retain the old marker so earlier Orc versions can close the pane.
		{"@orc_watch", "1"},
	}
	for _, update := range updates {
		if err := setPaneOption(newPane, update[0], update[1]); err != nil {
			_ = newCommand("tmux", "kill-pane", "-t", newPane).Run()
			return fmt.Errorf("mark watch rail %s: %w", newPane, err)
		}
	}
	if err := setRailCollapsed(newPane, false); err != nil {
		_ = newCommand("tmux", "kill-pane", "-t", newPane).Run()
		return err
	}
	return nil
}

// CloseRail closes only a pane whose Orc ownership marker can be proved.
func CloseRail() error {
	if err := requireRailTmux(); err != nil {
		return err
	}
	pane, err := findRailPane()
	if err != nil {
		return err
	}
	if pane == "" {
		return fmt.Errorf("no Orc-owned rail in the current tmux window")
	}
	if err := newCommand("tmux", "kill-pane", "-t", pane).Run(); err != nil {
		return fmt.Errorf("close watch rail %s: %w", pane, err)
	}
	return nil
}

// ToggleRail closes an owned rail or opens a new one.
func ToggleRail(opts WatchToggleOptions) error {
	if err := requireRailTmux(); err != nil {
		return err
	}
	pane, err := findRailPane()
	if err != nil {
		return err
	}
	if pane != "" {
		if err := newCommand("tmux", "kill-pane", "-t", pane).Run(); err != nil {
			return fmt.Errorf("close watch rail %s: %w", pane, err)
		}
		return nil
	}
	return OpenRail(opts)
}

func CollapseRail() error { return resizeRail(true, "") }

func ExpandRail(size string) error { return resizeRail(false, size) }

func ToggleCollapsedRail(size string) error {
	pane, err := ownedRailPane()
	if err != nil {
		return err
	}
	out, err := newCommand("tmux", "display-message", "-p", "-t", pane, "#{@orc_rail_collapsed}").Output()
	if err != nil {
		return fmt.Errorf("read rail collapsed state: %w", err)
	}
	return resizeOwnedRail(pane, strings.TrimSpace(string(out)) != "1", size)
}

func resizeRail(collapsed bool, size string) error {
	pane, err := ownedRailPane()
	if err != nil {
		return err
	}
	return resizeOwnedRail(pane, collapsed, size)
}

func resizeOwnedRail(pane string, collapsed bool, requestedSize string) error {
	out, err := newCommand("tmux", "display-message", "-p", "-t", pane,
		"#{pane_width}\t#{pane_height}\t#{@orc_rail_layout}\t#{@orc_rail_expanded_size}").Output()
	if err != nil {
		return fmt.Errorf("read watch rail size: %w", err)
	}
	fields := strings.Split(strings.TrimRight(string(out), "\r\n"), "\t")
	if len(fields) != 4 {
		return fmt.Errorf("watch rail %s returned invalid size metadata", pane)
	}
	layout := fields[2]
	if layout == "" {
		layout = "right"
	}
	dimension, flag := fields[0], "-x"
	if layout == "bottom" {
		dimension, flag = fields[1], "-y"
	}
	if collapsed {
		if validRailSize(dimension) && dimension != defaultCollapsedSize {
			if err := setPaneOption(pane, "@orc_rail_expanded_size", dimension); err != nil {
				return err
			}
		}
		if err := newCommand("tmux", "resize-pane", "-t", pane, flag, defaultCollapsedSize).Run(); err != nil {
			return fmt.Errorf("collapse watch rail %s: %w", pane, err)
		}
		return setRailCollapsed(pane, true)
	}

	size := requestedSize
	if size == "" {
		size = fields[3]
	}
	if !validRailSize(size) {
		size = defaultRailSize
	}
	if err := newCommand("tmux", "resize-pane", "-t", pane, flag, size).Run(); err != nil {
		return fmt.Errorf("expand watch rail %s: %w", pane, err)
	}
	if err := setPaneOption(pane, "@orc_rail_expanded_size", size); err != nil {
		return err
	}
	return setRailCollapsed(pane, false)
}

func validRailSize(size string) bool {
	n, err := strconv.Atoi(size)
	return err == nil && n >= 5
}

func setRailCollapsed(pane string, collapsed bool) error {
	value := "0"
	if collapsed {
		value = "1"
	}
	if err := newCommand("tmux", "set-option", "-w", "-t", pane, "@orc_rail_collapsed", value).Run(); err != nil {
		return fmt.Errorf("record watch rail collapsed state: %w", err)
	}
	return nil
}

func ownedRailPane() (string, error) {
	if err := requireRailTmux(); err != nil {
		return "", err
	}
	pane, err := findRailPane()
	if err != nil {
		return "", err
	}
	if pane == "" {
		return "", fmt.Errorf("no Orc-owned rail in the current tmux window")
	}
	return pane, nil
}

func requireRailTmux() error {
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("rail commands must be run inside tmux")
	}
	if !railAvailable() {
		return fmt.Errorf("tmux is not installed or not in PATH")
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

func findRailPane() (string, error) {
	out, err := newCommand("tmux", "list-panes", "-F", "#{pane_id}\t#{@orc_rail}\t#{@orc_role}\t#{@orc_watch}").Output()
	if err != nil {
		return "", fmt.Errorf("list panes: %w", err)
	}
	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			continue
		}
		owned := fields[1] == "1" && fields[2] == "rail"
		legacyOwned := fields[3] == "1"
		if owned || legacyOwned {
			matches = append(matches, fields[0])
		}
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple Orc-owned rails found in the current tmux window")
	}
	if len(matches) == 1 {
		return matches[0], nil
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
