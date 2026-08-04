package main

import (
	"os"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
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
	_, err := applyAgentEvent(os.Getenv("TMUX_PANE"), mux.AgentEvent{
		AgentID:           strings.TrimSpace(agentEventAgentID),
		AgentInstance:     strings.TrimSpace(agentEventInstance),
		Engine:            strings.ToLower(strings.TrimSpace(agentEventEngine)),
		ProviderSessionID: strings.TrimSpace(agentEventProvider),
		Lifecycle:         strings.ToLower(strings.TrimSpace(agentEventLifecycle)),
		EventID:           strings.TrimSpace(agentEventID),
	})
	return err
}
