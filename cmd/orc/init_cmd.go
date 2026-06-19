package main

import (
	"fmt"
	"os"

	"github.com/cengebretson/orc/internal/workspace"
	"github.com/spf13/cobra"
)

func runInit(cmd *cobra.Command, args []string) error {
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

	opts := workspace.InitOptions{
		Root:            globalWorkspace,
		SkipDefaultPack: initSkipDefaultPack,
		DryRun:          initDryRun,
		Force:           initForce,
	}

	return workspace.Init(opts)
}

// isTTY returns true when stdin is an interactive terminal.
