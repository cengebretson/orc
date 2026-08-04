package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/cengebretson/orc/internal/agenthooks"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/cengebretson/orc/internal/validate"
	"github.com/spf13/cobra"
)

var doctorLookPath = exec.LookPath
var buildAgentHookPlan = agenthooks.BuildPlan
var applyAgentHookPlan = agenthooks.Apply

func runDoctor(cmd *cobra.Command, args []string) error {
	if doctorDryRun && !doctorInstallAgentHooks {
		return fmt.Errorf("doctor --dry-run requires --install-agent-hooks")
	}
	if doctorInstallAgentHooks {
		if len(args) > 0 {
			return fmt.Errorf("doctor --install-agent-hooks does not accept a ticket")
		}
		if doctorFix || doctorSystem {
			return fmt.Errorf("doctor --install-agent-hooks cannot be combined with --fix or --system")
		}
		plan := buildAgentHookPlan(agenthooks.Options{})
		printAgentHookPlan(plan, doctorDryRun)
		if err := plan.Err(); err != nil {
			return err
		}
		if doctorDryRun {
			return nil
		}
		if err := applyAgentHookPlan(plan); err != nil {
			return err
		}
		fmt.Println()
		fmt.Println("Codex: review and trust the installed hook with /hooks.")
		fmt.Println("Claude: restart active sessions to load the installed hook.")
		return nil
	}
	if doctorSystem {
		if len(args) > 0 {
			return fmt.Errorf("doctor --system does not accept a ticket")
		}
		report := doctor.RunSystemWithOptions(doctor.Options{Fix: doctorFix, Version: version, LookPath: doctorLookPath})
		doctor.AppendAgentHookChecks(report, buildAgentHookPlan(agenthooks.Options{}), doctorLookPath)
		doctor.Print(report)
		if !report.OK() {
			return fmt.Errorf("doctor found problems")
		}
		return nil
	}

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
	doctor.AppendAgentHookChecks(report, buildAgentHookPlan(agenthooks.Options{}), doctorLookPath)
	doctor.Print(report)
	if !report.OK() {
		return fmt.Errorf("doctor found problems")
	}
	return nil
}

func printAgentHookPlan(plan *agenthooks.Plan, dryRun bool) {
	title := "Agent hooks"
	if dryRun {
		title += " (dry run)"
	}
	fmt.Println(title)
	fmt.Println()
	for _, integration := range plan.Integrations {
		fmt.Printf("  %s\n", integration.Engine)
		if integration.Err != nil {
			fmt.Printf("    ✗  %s\n", integration.Err)
			continue
		}
		for _, change := range integration.Changes {
			fmt.Printf("    %-9s %s\n", change.Kind, change.Path)
		}
		fmt.Printf("    states    %s\n", strings.Join(integration.SupportedStates, ", "))
	}
}
