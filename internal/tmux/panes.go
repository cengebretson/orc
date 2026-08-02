package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
)

// paneFormat lists the fields ListPanesDetailed and windowPanes read back.
//
// The @-prefixed entries are user options, and tmux resolves those through the
// pane → window → session → global hierarchy. That inheritance is what keeps
// older setups working: a tool that only ever set @agent_attention on the
// window still shows up here on each of that window's panes.
var paneFormat = strings.Join([]string{
	"#{pane_id}", "#{session_name}", "#{window_name}",
	"#{pane_current_path}", "#{pane_current_command}", "#{pane_pid}",
	"#{@orc_agent}", "#{@orc_ticket}", "#{@orc_stage}",
	"#{@orc_worker}", "#{@orc_engine}", "#{@orc_provider_engine}",
	"#{@orc_provider_session}", "#{@orc_feature_dir}",
	"#{@agent_attention}", "#{@agent_attention_since}",
}, "\t")

// windowPanes returns the panes of one window. A missing server or window is
// the empty inventory, not an error — callers ask this on every refresh.
func windowPanes(session, window string) []mux.Pane {
	if !Available() {
		return nil
	}
	target := session + ":" + window
	out, err := newCommand("tmux", "list-panes", "-t", target, "-F", paneFormat).Output()
	if err != nil {
		return nil
	}
	return parseDetailedPanes(out)
}

// ListPanesDetailed returns all tmux panes with the small metadata surface Orc
// owns. A missing tmux server is the empty inventory, not an error.
func ListPanesDetailed() ([]mux.Pane, error) {
	if !Available() {
		return nil, nil
	}
	out, err := newCommand("tmux", "list-panes", "-a", "-F", paneFormat).CombinedOutput()
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
		if len(fields) < 16 || fields[0] == "" {
			continue
		}
		pid, _ := strconv.Atoi(fields[5])
		since, _ := strconv.ParseInt(fields[15], 10, 64)
		panes = append(panes, mux.Pane{
			Backend: "tmux", ID: fields[0], Session: fields[1], Window: fields[2],
			CWD: fields[3], Command: fields[4], PID: pid, Agent: fields[6] == "1",
			Ticket: fields[7], Stage: fields[8], Worker: fields[9], Engine: fields[10],
			ProviderEngine: fields[11], ProviderSessionID: fields[12],
			FeatureDir: fields[13], Attention: normalizeAttention(fields[14]),
			AttentionSince: since,
		})
	}
	return panes
}
