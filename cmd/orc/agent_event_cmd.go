package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	orcnotify "github.com/cengebretson/orc/internal/notify"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/spf13/cobra"
)

var (
	applyAgentEvent           = tmux.ApplyAgentEvent
	agentEventInput io.Reader = os.Stdin
)

var (
	agentEventAgentID   string
	agentEventInstance  string
	agentEventEngine    string
	agentEventProvider  string
	agentEventLifecycle string
	agentEventID        string
	agentEventHookInput bool
)

var agentEventCmd = &cobra.Command{
	Use:    "agent-event",
	Short:  "Record a lifecycle event from an agent hook",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE:   runAgentEvent,
}

func runAgentEvent(_ *cobra.Command, _ []string) error {
	agentID := strings.TrimSpace(agentEventAgentID)
	if agentID == "" {
		agentID = strings.TrimSpace(os.Getenv("ORC_AGENT_ID"))
	}
	instance := strings.TrimSpace(agentEventInstance)
	if instance == "" {
		instance = strings.TrimSpace(os.Getenv("ORC_AGENT_INSTANCE"))
	}
	engine := strings.ToLower(strings.TrimSpace(agentEventEngine))
	lifecycle := strings.ToLower(strings.TrimSpace(agentEventLifecycle))
	providerSession := strings.TrimSpace(agentEventProvider)
	eventID := strings.TrimSpace(agentEventID)
	if agentEventHookInput {
		payload, ignore, err := readAgentHookPayload(agentEventInput)
		if err != nil {
			return err
		}
		if ignore {
			return nil
		}
		providerSession = hookString(payload["session_id"])
		eventID, err = agentHookEventID(engine, lifecycle, payload)
		if err != nil {
			return err
		}
	}

	result, err := applyAgentEvent(os.Getenv("TMUX_PANE"), mux.AgentEvent{
		AgentID:           agentID,
		AgentInstance:     instance,
		Engine:            engine,
		ProviderSessionID: providerSession,
		Lifecycle:         lifecycle,
		EventID:           eventID,
	})
	if err != nil {
		return err
	}
	if result.Notify && result.FeatureDir != "" {
		notifyAuthoritativeAgentEvent(result.FeatureDir, result.Target, result.Lifecycle)
	}
	return nil
}

func readAgentHookPayload(r io.Reader) (map[string]any, bool, error) {
	const maxPayloadBytes = 1 << 20
	data, err := io.ReadAll(io.LimitReader(r, maxPayloadBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("read agent hook payload: %w", err)
	}
	if len(data) > maxPayloadBytes {
		return nil, false, fmt.Errorf("agent hook payload exceeds %d bytes", maxPayloadBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	payload := map[string]any{}
	if err = decoder.Decode(&payload); err != nil {
		return nil, false, fmt.Errorf("parse agent hook payload: %w", err)
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple top-level values")
		}
		return nil, false, fmt.Errorf("parse agent hook payload: %w", err)
	}
	return payload, hookValueTruthy(payload["agent_id"]), nil
}

func agentHookEventID(engine, lifecycle string, payload map[string]any) (string, error) {
	identity := map[string]any{
		"engine":            engine,
		"hook_event_name":   payload["hook_event_name"],
		"notification_type": payload["notification_type"],
		"session_id":        hookString(payload["session_id"]),
		"source":            payload["source"],
		"state":             lifecycle,
		"tool_use_id":       payload["tool_use_id"],
		"turn_id":           payload["turn_id"],
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("encode agent hook identity: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("evt_%x", sum[:16]), nil
}

func hookString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func hookValueTruthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return typed != ""
	case json.Number:
		return typed.String() != "0" && typed.String() != "0.0"
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func notifyAuthoritativeAgentEvent(featureDir string, target mux.Target, lifecycle string) {
	eventName := ""
	switch lifecycle {
	case mux.LifecycleBlocked:
		eventName = "blocked"
	case mux.LifecycleDone:
		eventName = "complete"
	default:
		return
	}
	s, err := state.Load(featureDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load agent notification context: %v\n", err)
		return
	}
	recordedTarget, configured := s.Runtime.MuxTarget(s.Stage.Name)
	if !configured || recordedTarget.Backend != "tmux" || recordedTarget.Workspace != target.Workspace || recordedTarget.Tab != target.Tab || recordedTarget.Pane != target.Pane ||
		s.Runtime.Agent == nil || s.Runtime.Agent.ID != target.AgentID || s.Runtime.Agent.Instance != target.AgentInstance {
		fmt.Fprintln(os.Stderr, "warning: ignored agent notification from a stale or replaced runtime target")
		return
	}
	root := filepath.Dir(filepath.Dir(filepath.Clean(featureDir)))
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load agent notification settings: %v\n", err)
		return
	}
	event := orcnotify.Event{
		Ticket: s.Ticket, Slug: s.Slug, Name: eventName, Stage: s.Stage.Name,
		Workflow: s.Workflow, WorkDir: root,
	}
	if err := sendTransitionNotification(cfg.Settings.Notify, event); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
}
