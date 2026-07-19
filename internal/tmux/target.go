package tmux

import (
	"fmt"
	"strings"
)

// ValidatePaneTarget verifies that a durable pane id still belongs to the
// expected session/window. Stale ids are safely re-resolved within that window.
func ValidatePaneTarget(session, window, pane string) (string, error) {
	if pane == "" {
		return "", nil
	}
	out, err := newCommand("tmux", "display-message", "-p", "-t", pane, "#{session_name}\t#{window_name}").Output()
	if err == nil {
		fields := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
		if len(fields) == 2 && fields[0] == session && fields[1] == window {
			return pane, nil
		}
	}
	resolved, resolveErr := ResolvePaneTarget(session, window)
	if resolveErr != nil {
		return "", fmt.Errorf("stored tmux pane %s is stale: %w", pane, resolveErr)
	}
	return resolved, nil
}

// ResolvePaneTarget returns the only safe agent pane for a window.
func ResolvePaneTarget(session, window string) (string, error) {
	target := session + ":" + window
	out, err := newCommand("tmux", "list-panes", "-t", target, "-F", "#{pane_id}\t#{@orc_agent}").Output()
	if err != nil {
		return "", fmt.Errorf("list panes for %s: %w", target, err)
	}
	var panes, marked []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		panes = append(panes, fields[0])
		if len(fields) > 1 && fields[1] == "1" {
			marked = append(marked, fields[0])
		}
	}
	if len(marked) == 1 {
		return marked[0], nil
	}
	if len(panes) == 1 {
		return panes[0], nil
	}
	return "", fmt.Errorf("tmux window %s has %d panes and no unique @orc_agent target", target, len(panes))
}
