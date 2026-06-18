package main

import (
	"fmt"

	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/cengebretson/orc/internal/validate"
	"github.com/spf13/cobra"
)

func runDoctor(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		featureDir, err := ticket.Resolve(root, args[0])
		if err != nil {
			return err
		}
		if doctorFix {
			removed, err := state.ClearStaleLock(featureDir)
			if err != nil {
				return err
			}
			if removed {
				fmt.Printf("✓ removed stale %s.lock\n\n", state.Filename)
			}
		}
		report := validate.Run(root, featureDir)
		validate.Print(report)
		if !report.OK() {
			return fmt.Errorf("validation failed")
		}
		return nil
	}

	report := doctor.RunWithOptions(root, doctor.Options{Fix: doctorFix})
	doctor.Print(report)
	if !report.OK() {
		return fmt.Errorf("doctor found problems")
	}
	return nil
}
