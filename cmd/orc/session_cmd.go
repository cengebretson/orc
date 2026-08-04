package main

import (
	"fmt"

	"github.com/cengebretson/orc/internal/dashboard"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/ticket"

	"github.com/cengebretson/orc/internal/watch"
	"github.com/spf13/cobra"
)

func runDashboard(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	return dashboard.Run(root, dashboard.Options{
		Start:     dashboard.SectionFeatures,
		Adaptive:  true,
		Version:   version,
		BuildDate: buildDate,
		Mux:       muxBackend,
	})
}

func runAttach(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}
	s := t.State
	if err := selectMuxForState(s); err != nil {
		return err
	}
	if !muxBackend.Available() {
		return fmt.Errorf("%s is not installed or its server is unavailable", muxBackend.Name())
	}

	target, configured := runtimeTarget(s)
	if !configured {
		target = mux.Target{Backend: muxBackend.Name(), Workspace: s.Slug, Tab: s.Stage.Name}
	}

	if !muxBackend.SessionExists(target.Workspace) {
		return fmt.Errorf("no %s workspace for %s — run `orc next %s` to start one", muxBackend.Name(), s.Ticket, s.Ticket)
	}
	acknowledgeTarget(muxBackend, target)
	if backend, ok := muxBackend.(mux.TargetBackend); ok {
		return backend.AttachTarget(target)
	}
	return muxBackend.AttachPane(target.Workspace, target.Tab, target.Pane)
}

func acknowledgeTarget(backend mux.Backend, target mux.Target) {
	acknowledger, ok := backend.(mux.AgentAcknowledgeBackend)
	if !ok || target.Pane == "" || target.AgentID == "" || target.AgentInstance == "" {
		return
	}
	_ = acknowledger.AcknowledgeAgent(target)
}

func runFocus(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	if !muxBackend.Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}
	return watch.FocusWithMux(root, muxBackend)
}

// resolveWorkflow returns the ticket's workflow name for display purposes.
