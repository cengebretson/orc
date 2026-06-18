package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cengebretson/orc/internal/workspace"
	"github.com/spf13/cobra"
)

func runInit(cmd *cobra.Command, args []string) error {
	if initListPacks {
		return printPacks()
	}

	fmt.Print(banner)

	interactive := isTTY()

	// Workspace path — prompt if not explicitly set and running interactively.
	if !cmd.Root().PersistentFlags().Changed("workspace") && interactive {
		cwd, _ := os.Getwd()
		ans := promptLine(fmt.Sprintf("Workspace path [%s]: ", cwd))
		if ans == "" {
			globalWorkspace = cwd
		} else {
			globalWorkspace = ans
		}
	}

	// Pack — prompt if not explicitly set and running interactively.
	if !cmd.Flags().Changed("pack") && interactive {
		ans := strings.TrimSpace(promptLine("Which pack? [default] (or 'none' for a base-only workspace): "))
		if ans != "" {
			initPacks = []string{ans}
		}
	}

	opts := workspace.InitOptions{
		Root:   globalWorkspace,
		Packs:  initPacks,
		DryRun: initDryRun,
		Force:  initForce,
	}

	return workspace.Init(opts)
}

// printPacks lists the available packs for `orc init --list-packs`.

func printPacks() error {
	packs, err := workspace.ListPacks()
	if err != nil {
		return err
	}
	fmt.Println("Available packs:")
	fmt.Println()
	for _, p := range packs {
		fmt.Printf("  %-12s %s\n", p.Name, p.Description)
		fmt.Printf("  %-12s engines: %s\n", "", strings.Join(p.Engines, ", "))
	}
	fmt.Println()
	fmt.Println("Install with: orc init --pack <name>   (repeatable; omit for 'default', 'none' for base only)")
	return nil
}

// isTTY returns true when stdin is an interactive terminal.
