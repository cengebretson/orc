package main

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/agenthooks"
	"github.com/spf13/cobra"
)

var applyAgentHookPlan = agenthooks.Apply

var hooksInstallDryRun bool

// Hook installation lives here rather than on `orc doctor` because it is not a
// repair of anything doctor diagnoses: it writes into the user's Codex and
// Claude configuration, outside the workspace entirely. Doctor still reports
// whether hooks are installed — diagnosis there, action here.
var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage Orc's Codex and Claude lifecycle hooks",
	Args:  cobra.NoArgs,
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Orc lifecycle hooks into Codex and Claude configuration",
	Args:  cobra.NoArgs,
	RunE:  runHooksInstall,
}

func runHooksInstall(_ *cobra.Command, _ []string) error {
	plan := buildAgentHookPlan(agenthooks.Options{})
	printAgentHookPlan(plan, hooksInstallDryRun)
	if err := plan.Err(); err != nil {
		return err
	}
	if hooksInstallDryRun {
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
