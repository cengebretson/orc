package main

import (
	"fmt"
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

var applyAgentEvent = tmux.ApplyAgentEvent

var (
	agentEventAgentID   string
	agentEventInstance  string
	agentEventEngine    string
	agentEventProvider  string
	agentEventLifecycle string
	agentEventID        string
)

var agentEventCmd = &cobra.Command{
	Use:    "agent-event",
	Short:  "Record a lifecycle event from an agent hook",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE:   runAgentEvent,
}

func runAgentEvent(_ *cobra.Command, _ []string) error {
	result, err := applyAgentEvent(os.Getenv("TMUX_PANE"), mux.AgentEvent{
		AgentID:           strings.TrimSpace(agentEventAgentID),
		AgentInstance:     strings.TrimSpace(agentEventInstance),
		Engine:            strings.ToLower(strings.TrimSpace(agentEventEngine)),
		ProviderSessionID: strings.TrimSpace(agentEventProvider),
		Lifecycle:         strings.ToLower(strings.TrimSpace(agentEventLifecycle)),
		EventID:           strings.TrimSpace(agentEventID),
	})
	if err != nil {
		return err
	}
	if result.Notify && result.FeatureDir != "" {
		notifyAuthoritativeAgentEvent(result.FeatureDir, result.Target, result.Lifecycle)
	}
	return nil
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
