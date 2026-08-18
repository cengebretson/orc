package main

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	workspaceevents "github.com/cengebretson/orc/internal/events"
	"github.com/cengebretson/orc/internal/workspacesnapshot"
	"github.com/spf13/cobra"
)

func runEvents(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	interval, err := time.ParseDuration(eventsInterval)
	if err != nil {
		return fmt.Errorf("invalid events interval: %w", err)
	}
	if interval <= 0 {
		return fmt.Errorf("events interval must be greater than zero")
	}

	return workspaceevents.Stream(
		cmd.Context(),
		func() (*workspacesnapshot.Snapshot, error) {
			return workspacesnapshot.LoadWithMux(root, muxBackend)
		},
		workspaceevents.StreamOptions{Follow: eventsFollow, Interval: interval},
		eventEmitter(cmd.OutOrStdout(), eventsJSON),
	)
}

func eventEmitter(writer io.Writer, jsonOutput bool) func(workspaceevents.Event) error {
	if jsonOutput {
		encoder := json.NewEncoder(writer)
		return func(event workspaceevents.Event) error {
			return encoder.Encode(event)
		}
	}
	return func(event workspaceevents.Event) error {
		label := event.Ticket
		if label == "" {
			label = filepath.Base(event.FeatureDir)
		}
		_, err := fmt.Fprintf(writer, "%s  %-18s  %s\n", event.At.Format(time.RFC3339), event.Type, label)
		return err
	}
}
