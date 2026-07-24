package main

import (
	"fmt"

	"github.com/cengebretson/orc/internal/dashboard"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/cengebretson/orc/internal/tmux"
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
	})
}

func runAttach(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	if !tmux.Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}

	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}
	s := t.State

	session := s.Slug
	if s.Runtime.Tmux != nil && s.Runtime.Tmux.Session != "" {
		session = s.Runtime.Tmux.Session
	}

	if !tmux.SessionExists(session) {
		return fmt.Errorf("no tmux session for %s — run `orc next %s` to start one", s.Ticket, s.Ticket)
	}

	pane := ""
	if s.Runtime.Tmux != nil {
		pane = s.Runtime.Tmux.Pane
	}
	return tmux.AttachTarget(session, s.Stage.Name, pane)
}

func runFocus(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	if !tmux.Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}
	return watch.Focus(root)
}

// resolveWorkflow returns the ticket's workflow name for display purposes.
