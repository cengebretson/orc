package tmux

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
)

// paneFields lists the tmux format fields ListPanesDetailed and windowPanes
// read back, in wire order. This slice is the single source of truth: the
// format string is built from it and the parser looks fields up by name
// through it, so the two can no longer drift.
//
// Order still matters on the wire, but it is no longer load-bearing in the
// parser. Fields may be inserted anywhere; previously an insertion silently
// shifted every later index and a pane's stage would surface in its worker
// column, with nothing failing.
//
// The @-prefixed entries are user options, and tmux resolves those through the
// pane → window → session → global hierarchy. That inheritance is what keeps
// older setups working: a tool that only ever set @agent_attention on the
// window still shows up here on each of that window's panes.
var paneFields = []string{
	"pane_id", "session_name", "window_name",
	"pane_current_path", "pane_current_command", "pane_pid",
	"@orc_agent", "@orc_agent_id", "@orc_agent_instance",
	"@orc_ticket", "@orc_stage",
	"@orc_worker", "@orc_engine", "@orc_provider_engine",
	"@orc_provider_session", "@orc_feature_dir",
	"@orc_agent_state", "@orc_agent_state_seq",
	"@orc_agent_state_since", "@orc_agent_state_source",
	"@agent_attention", "@agent_attention_since",
	"@orc_agent_seen_seq",
	"pane_title", "@orc_agent_observed_state",
	"@orc_agent_observed_source", "@orc_agent_observed_since",
	"@orc_agent_observed_rule", "@agent_attention_source",
	// tmux-attention's authoritative pane schema.
	"@agent_pane_attention", "@agent_pane_attention_updated_at",
	// Its active-turn signal, set between turn-start and turn-done.
	"@agent_pane_context_active",
}

var paneFormat = buildPaneFormat(paneFields)

func buildPaneFormat(fields []string) string {
	wrapped := make([]string, len(fields))
	for i, field := range fields {
		wrapped[i] = "#{" + field + "}"
	}
	return strings.Join(wrapped, "\t")
}

// paneRecord is one parsed row, addressed by field name rather than position.
type paneRecord struct {
	values []string
	index  map[string]int
}

var paneFieldIndex = func() map[string]int {
	index := make(map[string]int, len(paneFields))
	for i, field := range paneFields {
		index[field] = i
	}
	return index
}()

// get returns a field's value, or "" when tmux reported a short row. Older tmux
// versions and panes that predate a newly added field both produce short rows,
// so a missing trailing field is normal rather than an error.
func (r paneRecord) get(field string) string {
	i, ok := r.index[field]
	if !ok || i >= len(r.values) {
		return ""
	}
	return r.values[i]
}

func (r paneRecord) int64(field string) int64 {
	value, _ := strconv.ParseInt(r.get(field), 10, 64)
	return value
}

func (r paneRecord) uint64(field string) uint64 {
	value, _ := strconv.ParseUint(r.get(field), 10, 64)
	return value
}

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
		record := paneRecord{values: strings.Split(line, "\t"), index: paneFieldIndex}
		// A row must carry at least through the legacy attention pair to be
		// usable; anything past that is optional and reads as empty.
		if len(record.values) < 22 || record.get("pane_id") == "" {
			continue
		}
		pid, _ := strconv.Atoi(record.get("pane_pid"))
		sequence := record.uint64("@orc_agent_state_seq")
		seenSequence := record.uint64("@orc_agent_seen_seq")

		// Prefer tmux-attention's own pane fields when present. A marker set by
		// the plugin records its timestamp as @agent_pane_attention_updated_at
		// and leaves @agent_attention_since empty, so reading only the legacy
		// pair saw the state with no age at all.
		attention := normalizeAttention(record.get("@agent_attention"))
		attentionSince := record.int64("@agent_attention_since")
		if pluginAttention := normalizeAttention(record.get("@agent_pane_attention")); pluginAttention != "" {
			attention = pluginAttention
			if pluginSince := record.int64("@agent_pane_attention_updated_at"); pluginSince > 0 {
				attentionSince = pluginSince
			}
		}
		if seenSequence >= sequence && sequence > 0 {
			attention = ""
			attentionSince = 0
		}
		lifecycle := normalizeLifecycle(record.get("@orc_agent_state"))
		if lifecycle == mux.LifecycleDone && seenSequence >= sequence && sequence > 0 {
			lifecycle = mux.LifecycleIdle
		}
		panes = append(panes, mux.Pane{
			Backend: "tmux",
			ID:      record.get("pane_id"), Session: record.get("session_name"), Window: record.get("window_name"),
			CWD: record.get("pane_current_path"), Command: record.get("pane_current_command"), PID: pid,
			Agent:   record.get("@orc_agent") == "1",
			AgentID: record.get("@orc_agent_id"), AgentInstance: record.get("@orc_agent_instance"),
			Ticket: record.get("@orc_ticket"), Stage: record.get("@orc_stage"),
			Worker: record.get("@orc_worker"), Engine: record.get("@orc_engine"),
			ProviderEngine: record.get("@orc_provider_engine"), ProviderSessionID: record.get("@orc_provider_session"),
			FeatureDir: record.get("@orc_feature_dir"), Lifecycle: lifecycle,
			StateChangeSeq: sequence, LifecycleSince: record.int64("@orc_agent_state_since"),
			LifecycleSource: record.get("@orc_agent_state_source"),
			Attention:       attention, AttentionSource: record.get("@agent_attention_source"),
			AttentionSince: attentionSince, SeenSeq: seenSequence,
			Title:             record.get("pane_title"),
			ObservedLifecycle: normalizeLifecycle(record.get("@orc_agent_observed_state")),
			ObservationSource: record.get("@orc_agent_observed_source"),
			ObservationSince:  record.int64("@orc_agent_observed_since"),
			ObservationRule:   record.get("@orc_agent_observed_rule"),
			// Only an explicit "1" means a turn is running. An empty value is
			// the plugin not installed, and a cleared one is not proof of idle
			// -- both must fall through to the other evidence rather than
			// settle the pane.
			ContextActive: record.get("@agent_pane_context_active") == "1",
		})
	}
	return panes
}

func normalizeLifecycle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case mux.LifecycleIdle, mux.LifecycleWorking, mux.LifecycleBlocked, mux.LifecycleDone, mux.LifecycleUnknown:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}
