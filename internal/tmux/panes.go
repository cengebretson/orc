package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

// Pane describes the process and Orc metadata attached to a tmux pane.
type Pane struct {
	ID                string `json:"id"`
	Session           string `json:"session"`
	Window            string `json:"window"`
	CWD               string `json:"cwd,omitempty"`
	Command           string `json:"command,omitempty"`
	PID               int    `json:"pid,omitempty"`
	Agent             bool   `json:"agent"`
	Ticket            string `json:"ticket,omitempty"`
	Stage             string `json:"stage,omitempty"`
	Worker            string `json:"worker,omitempty"`
	Engine            string `json:"engine,omitempty"`
	ProviderEngine    string `json:"provider_engine,omitempty"`
	ProviderSessionID string `json:"provider_session_id,omitempty"`
	FeatureDir        string `json:"feature_dir,omitempty"`
	Attention         string `json:"attention,omitempty"`
}

// ListPanesDetailed returns all tmux panes with the small metadata surface Orc
// owns. A missing tmux server is the empty inventory, not an error.
func ListPanesDetailed() ([]Pane, error) {
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

func parseDetailedPanes(out []byte) []Pane {
	var panes []Pane
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
		panes = append(panes, Pane{
			ID: fields[0], Session: fields[1], Window: fields[2],
			CWD: fields[3], Command: fields[4], PID: pid, Agent: fields[6] == "1",
			Ticket: fields[7], Stage: fields[8], Worker: fields[9], Engine: fields[10],
			ProviderEngine: fields[11], ProviderSessionID: fields[12],
			FeatureDir: fields[13], Attention: fields[14],
		})
	}
	return panes
}
