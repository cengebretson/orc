package tmux

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

var agentWaitPollInterval = 100 * time.Millisecond

// StateAgent reads hook-owned lifecycle metadata from the exact recorded pane.
// Terminal contents are deliberately excluded: only provider events can move
// this state away from unknown.
func (Backend) StateAgent(target mux.Target) (mux.AgentControlResult, error) {
	if target.Backend != "" && target.Backend != "tmux" {
		return mux.AgentControlResult{}, tmuxAgentError("invalid_target", "tmux cannot control %s target", target.Backend)
	}
	if target.Workspace == "" || target.Tab == "" || target.Pane == "" {
		return mux.AgentControlResult{}, tmuxAgentError("invalid_target", "exact session, window, and pane ids are required")
	}
	if target.AgentID == "" || target.AgentInstance == "" {
		return mux.AgentControlResult{}, tmuxAgentError("agent_identity_missing", "recorded agent and instance ids are required")
	}

	out, err := newCommand(
		"tmux", "display-message", "-p", "-t", target.Pane,
		"#{session_name}\t#{window_name}\t#{@orc_agent_id}\t#{@orc_agent_instance}\t#{@orc_agent_state}\t#{@orc_agent_state_seq}\t#{@orc_agent_event_seq}\t#{pane_dead}",
	).Output()
	if err != nil {
		return mux.AgentControlResult{}, tmuxAgentError("agent_offline", "agent pane %s is unavailable", target.Pane)
	}
	fields := strings.Split(strings.TrimRight(string(out), "\r\n"), "\t")
	if len(fields) != 8 {
		return mux.AgentControlResult{}, tmuxAgentError("invalid_state", "agent pane %s returned invalid lifecycle metadata", target.Pane)
	}
	if fields[0] != target.Workspace || fields[1] != target.Tab || fields[2] != target.AgentID || fields[3] != target.AgentInstance {
		return mux.AgentControlResult{}, tmuxAgentError("agent_replaced", "agent pane %s no longer hosts the recorded instance", target.Pane)
	}
	if fields[7] == "1" {
		return mux.AgentControlResult{}, tmuxAgentError("agent_offline", "agent pane %s is dead", target.Pane)
	}

	lifecycle := fields[4]
	if lifecycle == "" {
		lifecycle = mux.LifecycleUnknown
	}
	if !validAgentLifecycle(lifecycle) {
		return mux.AgentControlResult{}, tmuxAgentError("invalid_state", "agent pane %s has invalid lifecycle %q", target.Pane, lifecycle)
	}
	var sequence uint64
	if fields[5] != "" {
		sequence, err = strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			return mux.AgentControlResult{}, tmuxAgentError("invalid_state", "agent pane %s has invalid lifecycle sequence", target.Pane)
		}
	}
	eventSequence := sequence
	if fields[6] != "" {
		eventSequence, err = strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return mux.AgentControlResult{}, tmuxAgentError("invalid_state", "agent pane %s has invalid event sequence", target.Pane)
		}
	}
	if eventSequence != sequence {
		// Hooks commit state_seq last. A mismatch means the reader observed an
		// interrupted or in-flight event, so expose uncertainty rather than the
		// uncommitted lifecycle value.
		lifecycle = mux.LifecycleUnknown
	}
	return mux.AgentControlResult{
		Backend: "tmux", Target: target, Agent: target.AgentID,
		Lifecycle: lifecycle, StateChangeSeq: sequence,
	}, nil
}

// WaitAgent polls authoritative pane metadata until one requested lifecycle is
// reached. Every read revalidates the durable agent instance, so replacement
// and pane loss terminate the wait instead of silently following a new agent.
func (b Backend) WaitAgent(target mux.Target, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
	until, err := tmuxWaitStates(options.Until)
	if err != nil {
		return mux.AgentControlResult{}, err
	}
	if options.Timeout < 0 {
		return mux.AgentControlResult{}, tmuxAgentError("invalid_argument", "timeout must not be negative")
	}
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	var timeout <-chan time.Time
	var timer *time.Timer
	if options.Timeout > 0 {
		timer = time.NewTimer(options.Timeout)
		defer timer.Stop()
		timeout = timer.C
	}
	ticker := time.NewTicker(agentWaitPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return mux.AgentControlResult{}, tmuxAgentError("cancelled", "agent wait was cancelled")
		default:
		}
		result, stateErr := b.StateAgent(target)
		if stateErr != nil {
			return mux.AgentControlResult{}, stateErr
		}
		if until[result.Lifecycle] {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return mux.AgentControlResult{}, tmuxAgentError("cancelled", "agent wait was cancelled")
		case <-timeout:
			return mux.AgentControlResult{}, tmuxAgentError("timeout", "agent did not reach a requested lifecycle before the timeout")
		case <-ticker.C:
		}
	}
}

func tmuxWaitStates(values []string) (map[string]bool, error) {
	if len(values) == 0 {
		return map[string]bool{
			mux.LifecycleIdle: true, mux.LifecycleBlocked: true, mux.LifecycleDone: true,
		}, nil
	}
	states := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !validAgentLifecycle(value) {
			return nil, tmuxAgentError("invalid_argument", "invalid lifecycle state %q", value)
		}
		states[value] = true
	}
	return states, nil
}

func validAgentLifecycle(value string) bool {
	switch value {
	case mux.LifecycleIdle, mux.LifecycleWorking, mux.LifecycleBlocked, mux.LifecycleDone, mux.LifecycleUnknown:
		return true
	default:
		return false
	}
}

func tmuxAgentError(code, format string, args ...any) error {
	return &mux.AgentControlError{Backend: "tmux", Code: code, Message: fmt.Sprintf(format, args...)}
}
