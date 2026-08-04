package main

import (
	"fmt"

	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/spf13/cobra"
)

var labelRemove []string

var labelCmd = &cobra.Command{
	Use:   "label <ticket> [key=value ...]",
	Short: "Set, remove, or list a ticket's labels",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runLabel,
}

func runLabel(_ *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	t, err := ticket.LoadWithArchive(root, args[0])
	if err != nil {
		return err
	}

	// Validate every argument before writing any of them, so a typo in the
	// third label does not leave the first two applied.
	type pair struct{ key, value string }
	pairs := make([]pair, 0, len(args)-1)
	for _, raw := range args[1:] {
		key, value, err := state.ParseLabel(raw)
		if err != nil {
			return err
		}
		pairs = append(pairs, pair{key, value})
	}

	for _, key := range labelRemove {
		if err := state.RemoveLabel(t.FeatureDir, key); err != nil {
			return err
		}
	}
	for _, p := range pairs {
		if err := state.SetLabel(t.FeatureDir, p.key, p.value); err != nil {
			return err
		}
	}

	updated, err := state.Load(t.FeatureDir)
	if err != nil {
		return err
	}
	fmt.Printf("Ticket:  %s\n", updated.Ticket)
	labels := updated.LabelPairs()
	if len(labels) == 0 {
		fmt.Println("Labels:  (none)")
		return nil
	}
	fmt.Println("Labels:")
	for _, label := range labels {
		fmt.Printf("  %s\n", label)
	}
	return nil
}
