package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cengebretson/orc/internal/orchestrator"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
	"github.com/spf13/cobra"
)

func runJIT(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	ticketArg := args[0]
	instruction := args[1]

	t, err := ticket.LoadWithArchive(root, ticketArg)
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State

	if s.Runtime.JIT != nil && !jitDry {
		return fmt.Errorf("jit task already running for %s (worker: %s, started: %s)\nRun `orc mark %s jit \"<summary>\"` to close it first",
			s.Ticket, s.Runtime.JIT.Worker, s.Runtime.JIT.StartedAt, s.Ticket)
	}

	ctx, err := workspacectx.Load(root)
	if err != nil {
		return err
	}
	w := workers.FindByID(ctx.Workers, jitWorker)
	if w == nil {
		return fmt.Errorf("worker %q not found in workers/", jitWorker)
	}

	timestamp := time.Now().Format("20060102-150405")
	outputDir := filepath.Join(featureDir, "jit", timestamp)
	prompt := buildJITPrompt(s, instruction, outputDir)
	launchArgv := workers.LaunchArgs(w, root, featureDir, prompt)
	launchCommand := workers.LaunchCommand(w, root, featureDir, prompt)

	if jitDry {
		fmt.Printf("Worker:  %s (%s)\n", w.Name, w.Engine)
		if w.Model != "" {
			fmt.Printf("Model:   %s\n", w.Model)
		}
		fmt.Printf("Output:  jit/%s/\n", timestamp)
		fmt.Println()
		fmt.Println("Would run:")
		fmt.Printf("  %s\n", launchCommand)
		fmt.Println()
		fmt.Println("Prompt:")
		fmt.Println(prompt)
		return nil
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating jit output dir: %w", err)
	}
	if err := state.SetJIT(featureDir, jitWorker, instruction); err != nil {
		return fmt.Errorf("writing runtime.jit: %w", err)
	}

	fmt.Printf("Ticket:  %s\n", s.Ticket)
	fmt.Printf("Worker:  %s (%s)\n", w.Name, w.Engine)
	fmt.Printf("Output:  jit/%s/\n", timestamp)
	fmt.Println()

	plan := &runner.Plan{
		Ticket:        s.Ticket,
		Stage:         "jit",
		Worker:        w,
		Prompt:        prompt,
		LaunchCommand: launchCommand,
		LaunchArgv:    launchArgv,
		CWD:           featureDir,
	}
	launcher := orchestrator.NewLauncher()
	result, err := launcher.Launch(orchestrator.LaunchOptions{
		Root:                root,
		FeatureDir:          featureDir,
		State:               s,
		Plan:                plan,
		Window:              "jit",
		In:                  os.Stdin,
		Out:                 os.Stdout,
		Err:                 os.Stderr,
		DisableTmux:         !jitTmux,
		RequireExistingTmux: true,
		OnFallback: func(message string) {
			fmt.Printf("%s — running in foreground\n", message)
		},
		OnHistoryWarning: func(message string) {
			fmt.Println(message)
		},
		OnForeground: func() {
			fmt.Printf("Launching %s (%s)...\n", w.Name, w.Engine)
		},
	})
	if err != nil {
		return err
	}
	if result.Mode == orchestrator.LaunchModeTmux {
		fmt.Printf("Agent launched in tmux session %s:%s\n", result.Session, result.Window)
		fmt.Printf("Attach:  %s\n", result.AttachHint)
	}
	return nil
}

func buildJITPrompt(s *state.State, instruction, outputDir string) string {
	return fmt.Sprintf(`Before starting: read AGENTS.md and ORC.md.

## JIT task: %s

%s

## Context

Start in features/%s/ and orient yourself by reading:
- STATE.yaml — current state and history
- TICKET.md — original ticket
- SPEC.md — scope and requirements (if present)
- DECISIONS.md — decisions made so far (if present)

Current pipeline stage: %s (do not advance — this is a one-off task outside the pipeline)

Write any output or notes to %s

When you are done, run:
  orc mark %s jit "<summary of what you did>"`,
		s.Ticket, instruction, s.Slug, s.Stage.Name, outputDir, s.Ticket)
}
