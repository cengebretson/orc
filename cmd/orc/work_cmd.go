package main

import (
	"fmt"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspace"
	"github.com/spf13/cobra"
)

func runWork(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	result, err := workspace.Work(workspace.WorkOptions{
		Root:     root,
		Ticket:   args[0],
		Slug:     workSlug,
		Workflow: workWorkflow,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Created: features/%s/\n\n", result.Slug)

	cfg, _ := config.Load(root)

	useTmux := workTmux
	if !useTmux && cfg != nil {
		useTmux = cfg.Settings.AutoTmux
	}
	if useTmux {
		if err := state.SetRuntime(result.FeatureDir, result.Slug); err != nil {
			fmt.Printf("warning: could not write tmux runtime to STATE.yaml: %v\n", err)
		}
	}

	plan, err := runner.Compute(root, result.FeatureDir, "")
	if err != nil {
		return err
	}

	useNext := workNext || (cfg != nil && cfg.Settings.AutoNext)
	if useNext {
		s, err := state.Load(result.FeatureDir)
		if err != nil {
			return err
		}
		return launchPlan(root, result.FeatureDir, s, plan)
	}

	printDryRun(plan, result.Slug)
	return nil
}
