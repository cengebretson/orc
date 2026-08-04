package tmux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cengebretson/orc/internal/mux"
)

var (
	agentWaitPollInterval    = 100 * time.Millisecond
	agentPromptStartupGrace  = 2 * time.Second
	newAgentPromptBufferName = randomAgentPromptBufferName
	loadAgentPromptBuffer    = func(name, text string) error {
		cmd := newCommand("tmux", "load-buffer", "-b", name, "-")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
)

type agentStateSnapshot struct {
	result         mux.AgentControlResult
	bracketedPaste bool
}

// StateAgent reads hook-owned lifecycle metadata from the exact recorded pane.
// Terminal contents are deliberately excluded: only provider events can move
// this state away from unknown.
func (Backend) StateAgent(target mux.Target) (mux.AgentControlResult, error) {
	snapshot, err := readAgentState(target)
	return snapshot.result, err
}

func readAgentState(target mux.Target) (agentStateSnapshot, error) {
	if target.Backend != "" && target.Backend != "tmux" {
		return agentStateSnapshot{}, tmuxAgentError("invalid_target", "tmux cannot control %s target", target.Backend)
	}
	if target.Workspace == "" || target.Tab == "" || target.Pane == "" {
		return agentStateSnapshot{}, tmuxAgentError("invalid_target", "exact session, window, and pane ids are required")
	}
	if target.AgentID == "" || target.AgentInstance == "" {
		return agentStateSnapshot{}, tmuxAgentError("agent_identity_missing", "recorded agent and instance ids are required")
	}

	out, err := newCommand(
		"tmux", "display-message", "-p", "-t", target.Pane,
		"#{session_name}\t#{window_name}\t#{@orc_agent_id}\t#{@orc_agent_instance}\t#{@orc_agent_state}\t#{@orc_agent_state_seq}\t#{@orc_agent_event_seq}\t#{bracket_paste_flag}\t#{pane_dead}",
	).Output()
	if err != nil {
		return agentStateSnapshot{}, tmuxAgentError("agent_offline", "agent pane %s is unavailable", target.Pane)
	}
	fields := strings.Split(strings.TrimRight(string(out), "\r\n"), "\t")
	if len(fields) != 9 {
		return agentStateSnapshot{}, tmuxAgentError("invalid_state", "agent pane %s returned invalid lifecycle metadata", target.Pane)
	}
	if fields[0] != target.Workspace || fields[1] != target.Tab || fields[2] != target.AgentID || fields[3] != target.AgentInstance {
		return agentStateSnapshot{}, tmuxAgentError("agent_replaced", "agent pane %s no longer hosts the recorded instance", target.Pane)
	}
	if fields[8] == "1" {
		return agentStateSnapshot{}, tmuxAgentError("agent_offline", "agent pane %s is dead", target.Pane)
	}

	lifecycle := fields[4]
	if lifecycle == "" {
		lifecycle = mux.LifecycleUnknown
	}
	if !validAgentLifecycle(lifecycle) {
		return agentStateSnapshot{}, tmuxAgentError("invalid_state", "agent pane %s has invalid lifecycle %q", target.Pane, lifecycle)
	}
	var sequence uint64
	if fields[5] != "" {
		sequence, err = strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			return agentStateSnapshot{}, tmuxAgentError("invalid_state", "agent pane %s has invalid lifecycle sequence", target.Pane)
		}
	}
	eventSequence := sequence
	if fields[6] != "" {
		eventSequence, err = strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return agentStateSnapshot{}, tmuxAgentError("invalid_state", "agent pane %s has invalid event sequence", target.Pane)
		}
	}
	if eventSequence != sequence {
		// Hooks commit state_seq last. A mismatch means the reader observed an
		// interrupted or in-flight event, so expose uncertainty rather than the
		// uncommitted lifecycle value.
		lifecycle = mux.LifecycleUnknown
	}
	return agentStateSnapshot{
		result: mux.AgentControlResult{
			Backend: "tmux", Target: target, Agent: target.AgentID,
			Lifecycle: lifecycle, StateChangeSeq: sequence,
		},
		bracketedPaste: fields[7] == "1",
	}, nil
}

// PromptAgent delivers literal text to the exact recorded agent instance and,
// when requested, proves that a provider hook observed the prompt before
// waiting for the requested settled lifecycle.
func (b Backend) PromptAgent(target mux.Target, text string, wait bool, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
	if err := validateAgentPrompt(text); err != nil {
		return mux.AgentControlResult{}, err
	}
	if !wait && (len(options.Until) > 0 || options.Timeout > 0) {
		return mux.AgentControlResult{}, tmuxAgentError("invalid_argument", "prompt wait options require wait=true")
	}
	if wait {
		if _, err := tmuxWaitStates(options.Until); err != nil {
			return mux.AgentControlResult{}, err
		}
		if options.Timeout < 0 {
			return mux.AgentControlResult{}, tmuxAgentError("invalid_argument", "timeout must not be negative")
		}
	}
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return mux.AgentControlResult{}, tmuxAgentError("cancelled", "agent prompt was cancelled")
	}

	if _, err := readAgentState(target); err != nil {
		return mux.AgentControlResult{}, err
	}
	bufferName, err := newAgentPromptBufferName()
	if err != nil {
		return mux.AgentControlResult{}, tmuxAgentError("prompt_delivery_failed", "create private prompt buffer name: %v", err)
	}
	bufferLoaded := true
	defer func() {
		if bufferLoaded {
			_ = newCommand("tmux", "delete-buffer", "-b", bufferName).Run()
		}
	}()
	if err := loadAgentPromptBuffer(bufferName, text); err != nil {
		return mux.AgentControlResult{}, tmuxAgentError("backend_unavailable", "load private tmux prompt buffer")
	}

	starting, err := readAgentState(target)
	if err != nil {
		return mux.AgentControlResult{}, err
	}
	if !starting.bracketedPaste {
		return mux.AgentControlResult{}, tmuxAgentError("bracketed_paste_unavailable", "agent pane %s has not enabled bracketed paste", target.Pane)
	}
	if err := newCommand("tmux", "paste-buffer", "-dprS", "-b", bufferName, "-t", target.Pane).Run(); err != nil {
		return mux.AgentControlResult{}, promptDeliveryError(b, target, "paste prompt into agent pane")
	}
	bufferLoaded = false

	// Revalidate after paste and immediately before the encoded submit key. A
	// replacement is never followed with Enter even if the paste itself raced.
	starting, err = readAgentState(target)
	if err != nil {
		return mux.AgentControlResult{}, err
	}
	if err := newCommand("tmux", "send-keys", "-t", target.Pane, "Enter").Run(); err != nil {
		return mux.AgentControlResult{}, promptDeliveryError(b, target, "submit prompt to agent pane")
	}
	if !wait {
		return starting.result, nil
	}

	startedAt := time.Now()
	changed, err := b.waitForPromptStart(ctx, target, starting.result.StateChangeSeq, options.Timeout, startedAt)
	if err != nil {
		return mux.AgentControlResult{}, err
	}
	desired, _ := tmuxWaitStates(options.Until)
	if desired[changed.Lifecycle] {
		return changed, nil
	}
	remaining := options.Timeout
	if remaining > 0 {
		remaining -= time.Since(startedAt)
		if remaining <= 0 {
			return mux.AgentControlResult{}, tmuxAgentError("timeout", "agent did not reach a requested lifecycle before the timeout")
		}
	}
	return b.WaitAgent(target, mux.AgentControlOptions{Until: options.Until, Timeout: remaining, Context: ctx})
}

func (b Backend) waitForPromptStart(ctx context.Context, target mux.Target, sequence uint64, timeout time.Duration, startedAt time.Time) (mux.AgentControlResult, error) {
	graceDeadline := startedAt.Add(agentPromptStartupGrace)
	var timeoutDeadline time.Time
	if timeout > 0 {
		timeoutDeadline = startedAt.Add(timeout)
	}
	ticker := time.NewTicker(agentWaitPollInterval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return mux.AgentControlResult{}, tmuxAgentError("cancelled", "agent prompt was cancelled")
		}
		now := time.Now()
		if !timeoutDeadline.IsZero() && !now.Before(timeoutDeadline) {
			return mux.AgentControlResult{}, tmuxAgentError("timeout", "agent did not acknowledge the prompt before the timeout")
		}
		if !now.Before(graceDeadline) {
			return mux.AgentControlResult{}, tmuxAgentError("agent_prompt_stalled", "no authoritative lifecycle change followed the prompt")
		}
		result, err := b.StateAgent(target)
		if err != nil {
			return mux.AgentControlResult{}, err
		}
		if result.StateChangeSeq > sequence {
			return result, nil
		}
		if result.StateChangeSeq < sequence {
			return mux.AgentControlResult{}, tmuxAgentError("invalid_state", "agent lifecycle sequence moved backwards")
		}
		select {
		case <-ctx.Done():
			return mux.AgentControlResult{}, tmuxAgentError("cancelled", "agent prompt was cancelled")
		case <-ticker.C:
		}
	}
}

func validateAgentPrompt(text string) error {
	if strings.TrimSpace(text) == "" {
		return tmuxAgentError("invalid_argument", "agent prompt requires text")
	}
	if len(text) > mux.MaxAgentPromptBytes {
		return tmuxAgentError("invalid_argument", "agent prompt exceeds %d bytes", mux.MaxAgentPromptBytes)
	}
	if !utf8.ValidString(text) {
		return tmuxAgentError("invalid_argument", "agent prompt must be valid UTF-8")
	}
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return tmuxAgentError("invalid_argument", "agent prompt contains unsafe control data")
		}
	}
	return nil
}

func randomAgentPromptBufferName() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return "orc-prompt-" + hex.EncodeToString(id[:]), nil
}

func promptDeliveryError(b Backend, target mux.Target, message string) error {
	if _, err := b.StateAgent(target); err != nil {
		return err
	}
	return tmuxAgentError("prompt_delivery_failed", "%s", message)
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
