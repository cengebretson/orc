package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspace"
	"github.com/cengebretson/orc/internal/workspacectx"
	"github.com/spf13/cobra"
)

func runLocal(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("reading current directory: %w", err)
	}
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	ctx, validationErrs, err := workspacectx.LoadValidated(root)
	if err != nil {
		return err
	}
	if len(validationErrs) > 0 {
		return fmt.Errorf("invalid workspace config: %w", validationErrs)
	}
	selectedWorker, err := selectRunWorker(runWorker, ctx.Workers)
	if errors.Is(err, errRunSelectionCancelled) {
		fmt.Println("Cancelled.")
		return nil
	}
	if err != nil {
		return err
	}
	repo, err := selectRunRepositoryForCommand(root, cwd, runRepo, ctx.Config.Repos)
	if errors.Is(err, errRunSelectionCancelled) {
		fmt.Println("Cancelled.")
		return nil
	}
	if err != nil {
		return err
	}
	if runAutoAttach && !muxBackend.Available() {
		return fmt.Errorf("cannot attach: %s is not installed or unavailable", muxBackend.Name())
	}
	migrated, err := workspace.EnsureLocalWorkflow(root, selectedWorker)
	if err != nil {
		return err
	}
	ctx, validationErrs, err = workspacectx.LoadValidated(root)
	if err != nil {
		return err
	}
	if len(validationErrs) > 0 {
		return fmt.Errorf("invalid workspace config after adding the local workflow: %w", validationErrs)
	}

	localOpts := workspace.LocalRunOptions{
		Root:        root,
		Instruction: args[0],
		Slug:        runSlug,
		Worker:      selectedWorker,
	}
	if repo != nil {
		localOpts.RepoName = repo.Name
		localOpts.RepoPath = repo.Path
	}
	result, err := workspace.LocalRun(localOpts)
	if err != nil {
		return err
	}

	if migrated {
		fmt.Println("Added local-run support to this workspace.")
	}
	fmt.Printf("Created:  features/%s/\n", result.Slug)
	fmt.Printf("Local ID: %s\n", result.Ticket)
	fmt.Printf("Workflow: %s\n", workspace.LocalWorkflow)
	fmt.Printf("Worker:   %s\n", selectedWorker)
	if repo != nil {
		suffix := ""
		if repo.Inferred {
			suffix = " (inferred)"
		}
		fmt.Printf("Repository: %s%s\n", repo.Name, suffix)
	}
	fmt.Println()

	if err := state.Start(result.FeatureDir); err != nil {
		return err
	}
	plan, err := runner.Compute(root, result.FeatureDir, selectedWorker)
	if err != nil {
		return err
	}
	s, err := state.Load(result.FeatureDir)
	if err != nil {
		return err
	}
	useMux := runTmux || runAutoAttach || ctx.Config.Settings.AutoTmux
	if err := launchPlanWithMux(root, result.FeatureDir, s, plan, useMux); err != nil {
		return err
	}
	if runAutoAttach {
		return runAttach(nil, []string{result.Ticket})
	}
	return nil
}
