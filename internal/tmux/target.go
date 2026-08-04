package tmux

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
)

// ValidateAgentTarget proves that an exact pane still hosts the recorded Orc
// agent instance. Unlike ValidatePaneTarget it never falls back to another
// pane: an identity mismatch means the recorded process was replaced.
func ValidateAgentTarget(target mux.Target, agentID, instance string) (string, error) {
	if target.Backend != "" && target.Backend != "tmux" {
		return "", fmt.Errorf("tmux cannot validate %s agent target", target.Backend)
	}
	if target.Workspace == "" || target.Tab == "" || target.Pane == "" {
		return "", fmt.Errorf("tmux agent validation requires exact session, window, and pane ids")
	}
	if agentID == "" || instance == "" {
		return "", fmt.Errorf("tmux agent validation requires agent and instance ids")
	}
	out, err := newCommand(
		"tmux", "display-message", "-p", "-t", target.Pane,
		"#{session_name}\t#{window_name}\t#{@orc_agent_id}\t#{@orc_agent_instance}",
	).Output()
	if err != nil {
		return "", fmt.Errorf("validate tmux agent pane %s: %w", target.Pane, err)
	}
	fields := strings.Split(strings.TrimRight(string(out), "\r\n"), "\t")
	if len(fields) != 4 || fields[0] != target.Workspace || fields[1] != target.Tab {
		return "", fmt.Errorf("tmux agent pane %s no longer belongs to %s:%s", target.Pane, target.Workspace, target.Tab)
	}
	if fields[2] != agentID || fields[3] != instance {
		return "", fmt.Errorf("tmux agent pane %s hosts a different agent instance", target.Pane)
	}
	return target.Pane, nil
}

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
