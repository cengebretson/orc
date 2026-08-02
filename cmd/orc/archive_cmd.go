package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/orchestrator"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/spf13/cobra"
)

func runArchive(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}

	return archiveFeature(root, t.FeatureDir, t.State)
}

func archiveFeature(root, featureDir string, s *state.State) error {
	if err := selectMuxForState(s); err != nil {
		return err
	}
	archiver := orchestrator.NewArchiver()
	archiver.Mux = muxBackend
	result, err := archiver.Archive(orchestrator.ArchiveOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
	})
	if err != nil {
		return err
	}

	printArchiveResult(result)
	return nil
}

func printArchiveResult(result *orchestrator.ArchiveResult) {
	for _, wt := range result.Worktrees {
		fmt.Printf("Removing worktree: %s\n", wt.WorktreeRel)
		if wt.Warning != "" {
			fmt.Printf("  warning: %v\n", wt.Warning)
			fmt.Printf("  you may need to run: git -C %q worktree remove %q --force\n", wt.Main, wt.WorktreePath)
		} else {
			fmt.Printf("  removed %s (%s)\n", wt.Name, wt.Branch)
		}
	}

	if result.TmuxKillWarn != "" {
		fmt.Printf("warning: %s\n", result.TmuxKillWarn)
	} else if result.KilledTmux {
		fmt.Printf("Stopped multiplexer workspace: %s\n", result.TmuxSession)
	}
	if result.RuntimeClearWarn != "" {
		fmt.Printf("warning: %s\n", result.RuntimeClearWarn)
	}

	fmt.Printf("Archived: features/_archive/%s/\n", result.Slug)
}

func runDelete(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	t, err := ticket.LoadWithArchive(root, args[0])
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State

	if s.Status != "done" && s.Status != "archived" {
		return fmt.Errorf("cannot delete %q: status is %q (must be done or archived)", s.Slug, s.Status)
	}

	rel, _ := filepath.Rel(root, featureDir)
	if isTTY() {
		ans := promptLine(fmt.Sprintf("Permanently delete %s? [y/N]: ", rel))
		ans = strings.ToLower(strings.TrimSpace(ans))
		if ans != "y" && ans != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := os.RemoveAll(featureDir); err != nil {
		return fmt.Errorf("deleting feature folder: %w", err)
	}

	fmt.Printf("Deleted: %s/\n", rel)
	return nil
}
