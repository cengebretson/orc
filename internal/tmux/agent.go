package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

const maxAgentEventField = 1024

// PrepareAgentLaunch resolves an exact tmux pane, stamps its identity before
// the provider starts, and injects the identity into the provider environment.
func (Backend) PrepareAgentLaunch(target mux.Target, tab, dir string, meta mux.Metadata, argv []string) (mux.Target, []string, error) {
	if len(argv) == 0 {
		return mux.Target{}, nil, fmt.Errorf("launch argv is empty")
	}
	if meta.AgentID == "" || meta.AgentInstance == "" {
		return mux.Target{}, nil, fmt.Errorf("tmux agent launch requires agent and instance ids")
	}
	if tab == "" {
		tab = target.Tab
	}
	if target.Workspace == "" || tab == "" {
		return mux.Target{}, nil, fmt.Errorf("tmux agent launch requires exact session and window ids")
	}
	if !WindowExists(target.Workspace, tab) {
		if err := newCommand("tmux", "new-window", "-t", target.Workspace, "-n", tab, "-c", dir).Run(); err != nil {
			return mux.Target{}, nil, fmt.Errorf("create window %s: %w", tab, err)
		}
	}
	pane, err := ValidatePaneTarget(target.Workspace, tab, target.Pane)
	if err != nil {
		return mux.Target{}, nil, err
	}
	if pane == "" {
		pane, err = ResolvePaneTarget(target.Workspace, tab)
		if err != nil {
			return mux.Target{}, nil, err
		}
	}
	preparedTarget := mux.Target{Backend: "tmux", Workspace: target.Workspace, Tab: tab, Pane: pane}
	if err := (Backend{}).SetTargetMetadata(preparedTarget, meta); err != nil {
		return mux.Target{}, nil, err
	}
	if err := initializeAgentLifecycle(pane, time.Now()); err != nil {
		return mux.Target{}, nil, err
	}
	preparedArgv := make([]string, 0, len(argv)+4)
	preparedArgv = append(preparedArgv,
		"env",
		mux.EnvAgentID+"="+meta.AgentID,
		mux.EnvAgentInstance+"="+meta.AgentInstance,
		mux.EnvFeatureDir+"="+meta.FeatureDir,
	)
	preparedArgv = append(preparedArgv, argv...)
	return preparedTarget, preparedArgv, nil
}

// initializeAgentLifecycle prevents a new instance from inheriting the prior
// occupant's pane-bound state before its first provider hook arrives.
func initializeAgentLifecycle(pane string, at time.Time) error {
	updates := [][2]string{
		{"@orc_agent_state", mux.LifecycleUnknown},
		{"@orc_agent_state_seq", "0"},
		{"@orc_agent_state_since", strconv.FormatInt(at.Unix(), 10)},
		{"@orc_agent_state_source", "launch"},
		{"@orc_agent_event_id", ""},
		{"@orc_agent_event_seq", "0"},
		{"@orc_agent_seen_seq", "0"},
		{"@agent_attention", ""},
		{"@agent_attention_since", ""},
		{"@agent_attention_source", "launch"},
	}
	for _, update := range updates {
		if err := setPaneOption(pane, update[0], update[1]); err != nil {
			return fmt.Errorf("initialize tmux agent lifecycle: %w", err)
		}
	}
	return nil
}

// AcknowledgeAgent records the committed lifecycle sequence observed by an
// explicit human action. A later hook sequence remains unseen even if it races
// this write.
func (Backend) AcknowledgeAgent(target mux.Target) error {
	snapshot, err := readAgentState(target)
	if err != nil {
		return err
	}
	return setPaneOption(target.Pane, "@orc_agent_seen_seq", strconv.FormatUint(snapshot.result.StateChangeSeq, 10))
}

// ApplyAgentEvent validates and commits one provider lifecycle event from the
// pane identified by TMUX_PANE. The sequence option is written last and acts as
// the commit marker for readers polling the other pane options.
func ApplyAgentEvent(pane string, event mux.AgentEvent) (mux.AgentEventResult, error) {
	return applyAgentEventAt(pane, event, time.Now())
}

func applyAgentEventAt(pane string, event mux.AgentEvent, at time.Time) (mux.AgentEventResult, error) {
	if pane == "" {
		return mux.AgentEventResult{}, fmt.Errorf("agent event requires TMUX_PANE")
	}
	if err := validateAgentEvent(event); err != nil {
		return mux.AgentEventResult{}, err
	}
	out, err := newCommand(
		"tmux", "display-message", "-p", "-t", pane,
		"#{session_name}\t#{window_name}\t#{@orc_agent_id}\t#{@orc_agent_instance}\t#{@orc_agent_state}\t#{@orc_agent_state_seq}\t#{@orc_agent_event_id}\t#{@orc_agent_event_seq}\t#{@orc_feature_dir}\t#{@orc_agent_seen_seq}",
	).Output()
	if err != nil {
		return mux.AgentEventResult{}, fmt.Errorf("read tmux agent pane %s: %w", pane, err)
	}
	fields := strings.Split(strings.TrimRight(string(out), "\r\n"), "\t")
	if len(fields) < 8 || fields[0] == "" || fields[1] == "" {
		return mux.AgentEventResult{}, fmt.Errorf("tmux pane %s returned invalid agent metadata", pane)
	}
	target := mux.Target{
		Backend: "tmux", Workspace: fields[0], Tab: fields[1], Pane: pane,
		AgentID: event.AgentID, AgentInstance: event.AgentInstance,
	}
	featureDir := ""
	var seenSequence uint64
	if len(fields) >= 10 {
		featureDir = fields[8]
		seenSequence, err = parseAgentSequence(fields[9])
		if err != nil {
			return mux.AgentEventResult{}, fmt.Errorf("tmux pane %s: invalid seen sequence: %w", pane, err)
		}
	}
	if fields[2] != event.AgentID || fields[3] != event.AgentInstance {
		return mux.AgentEventResult{}, fmt.Errorf("tmux pane %s hosts a different agent instance", pane)
	}
	sequence, err := parseAgentSequence(fields[5])
	if err != nil {
		return mux.AgentEventResult{}, fmt.Errorf("tmux pane %s: %w", pane, err)
	}
	if fields[6] == event.EventID {
		eventSequence, eventSeqErr := parseAgentSequence(fields[7])
		if eventSeqErr != nil {
			return mux.AgentEventResult{}, fmt.Errorf("tmux pane %s: %w", pane, eventSeqErr)
		}
		if eventSequence > sequence {
			// Complete an interrupted prior commit. All lifecycle fields are
			// written before event_seq and event_id; state_seq is the final
			// reader-visible commit marker.
			if err := setPaneOption(pane, "@orc_agent_state_seq", strconv.FormatUint(eventSequence, 10)); err != nil {
				return mux.AgentEventResult{}, err
			}
			sequence = eventSequence
		}
		return mux.AgentEventResult{
			Target: target, Lifecycle: fields[4], StateChangeSeq: sequence, Duplicate: true, FeatureDir: featureDir,
		}, nil
	}

	nextSequence := sequence + 1
	since := strconv.FormatInt(at.Unix(), 10)
	updates := [][2]string{
		{"@orc_agent_state", event.Lifecycle},
		{"@orc_agent_state_since", since},
		{"@orc_agent_state_source", "hook"},
		{"@orc_provider_engine", event.Engine},
	}
	if event.ProviderSessionID != "" {
		updates = append(updates, [2]string{"@orc_provider_session", event.ProviderSessionID})
	}
	attention := lifecycleAttentionState(event.Lifecycle)
	updates = append(updates,
		[2]string{"@agent_attention", attention},
		[2]string{"@agent_attention_since", attentionTimestamp(attention, since)},
		[2]string{"@agent_attention_source", attentionSource(attention)},
	)
	for _, update := range updates {
		if err := setPaneOption(pane, update[0], update[1]); err != nil {
			return mux.AgentEventResult{}, err
		}
	}
	if err := setPaneOption(pane, "@orc_agent_event_seq", strconv.FormatUint(nextSequence, 10)); err != nil {
		return mux.AgentEventResult{}, err
	}
	if err := setPaneOption(pane, "@orc_agent_event_id", event.EventID); err != nil {
		return mux.AgentEventResult{}, err
	}
	if err := setPaneOption(pane, "@orc_agent_state_seq", strconv.FormatUint(nextSequence, 10)); err != nil {
		return mux.AgentEventResult{}, err
	}
	shouldNotify := (event.Lifecycle == mux.LifecycleBlocked || event.Lifecycle == mux.LifecycleDone) && nextSequence > seenSequence
	return mux.AgentEventResult{
		Target: target, Lifecycle: event.Lifecycle, StateChangeSeq: nextSequence,
		FeatureDir: featureDir, Notify: shouldNotify,
	}, nil
}

func validateAgentEvent(event mux.AgentEvent) error {
	fields := []struct {
		name  string
		value string
	}{
		{"agent id", event.AgentID},
		{"agent instance", event.AgentInstance},
		{"engine", event.Engine},
		{"lifecycle", event.Lifecycle},
		{"event id", event.EventID},
	}
	for _, field := range fields {
		if field.value == "" {
			return fmt.Errorf("agent event requires %s", field.name)
		}
		if len(field.value) > maxAgentEventField || strings.ContainsAny(field.value, "\x00\r\n\t") {
			return fmt.Errorf("agent event %s is invalid", field.name)
		}
	}
	if len(event.ProviderSessionID) > maxAgentEventField || strings.ContainsAny(event.ProviderSessionID, "\x00\r\n\t") {
		return fmt.Errorf("agent event provider session is invalid")
	}
	switch event.Lifecycle {
	case mux.LifecycleIdle, mux.LifecycleWorking, mux.LifecycleBlocked, mux.LifecycleDone, mux.LifecycleUnknown:
		return nil
	default:
		return fmt.Errorf("unsupported agent lifecycle %q", event.Lifecycle)
	}
}

func parseAgentSequence(value string) (uint64, error) {
	if value == "" {
		return 0, nil
	}
	sequence, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid agent state sequence %q", value)
	}
	return sequence, nil
}

func setPaneOption(pane, name, value string) error {
	if err := newCommand("tmux", "set-option", "-p", "-t", pane, name, value).Run(); err != nil {
		return fmt.Errorf("set %s on pane %s: %w", name, pane, err)
	}
	return nil
}

func lifecycleAttentionState(lifecycle string) string {
	switch lifecycle {
	case mux.LifecycleBlocked:
		return mux.AttentionBlocked
	case mux.LifecycleDone:
		return mux.AttentionDone
	default:
		return ""
	}
}

func attentionTimestamp(attention, since string) string {
	if attention == "" {
		return ""
	}
	return since
}

func attentionSource(attention string) string {
	if attention == "" {
		return ""
	}
	return "hook"
}
