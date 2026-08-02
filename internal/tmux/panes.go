package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
)

// ListPanesDetailed returns all tmux panes with the small metadata surface Orc
// owns. A missing tmux server is the empty inventory, not an error.
func ListPanesDetailed() ([]mux.Pane, error) {
	if !Available() {
		return nil, nil
	}
	format := strings.Join([]string{
		"#{pane_id}", "#{session_name}", "#{window_name}",
		"#{pane_current_path}", "#{pane_current_command}", "#{pane_pid}",
		"#{@orc_agent}", "#{@orc_ticket}", "#{@orc_stage}",
		"#{@orc_worker}", "#{@orc_engine}", "#{@orc_provider_engine}",
		"#{@orc_provider_session}", "#{@orc_feature_dir}",
		"#{@agent_attention}",
	}, "\t")
	out, err := newCommand("tmux", "list-panes", "-a", "-F", format).CombinedOutput()
	if err != nil {
		message := strings.ToLower(string(out))
		if strings.Contains(message, "no server running") || strings.Contains(message, "failed to connect") || strings.Contains(message, "error connecting") {
			return nil, nil
		}
		return nil, fmt.Errorf("list tmux panes: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseDetailedPanes(out), nil
}

func parseDetailedPanes(out []byte) []mux.Pane {
	var panes []mux.Pane
	// Remove record terminators only. TrimSpace would also remove the final tab
	// when @agent_attention is empty and make an otherwise valid pane look short.
	text := strings.TrimRight(string(out), "\r\n")
	if text == "" {
		return nil
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		fields := strings.Split(line, "\t")
		if len(fields) < 15 || fields[0] == "" {
			continue
		}
		pid, _ := strconv.Atoi(fields[5])
		panes = append(panes, mux.Pane{
			ID: fields[0], Session: fields[1], Window: fields[2],
			CWD: fields[3], Command: fields[4], PID: pid, Agent: fields[6] == "1",
			Ticket: fields[7], Stage: fields[8], Worker: fields[9], Engine: fields[10],
			ProviderEngine: fields[11], ProviderSessionID: fields[12],
			FeatureDir: fields[13], Attention: fields[14],
		})
	}
	return panes
}
