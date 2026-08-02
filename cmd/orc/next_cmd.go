package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/orchestrator"
	"github.com/cengebretson/orc/internal/resume"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"

	"github.com/spf13/cobra"
)

func runNext(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State
	if err := selectMuxForState(s); err != nil {
		return err
	}

	if nextJSON {
		plan, err := runner.Compute(root, featureDir, nextWorker)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{
			"ticket":       plan.Ticket,
			"status":       s.Status,
			"workflow":     plan.Workflow,
			"stage":        plan.Stage,
			"stage_worker": s.Stage.Worker,
			"cwd":          plan.CWD,
			"prompt":       plan.Prompt,
			"worker":       plan.Worker.ID,
			"product":      plan.Worker.Engine,
			"model":        plan.Worker.Model,
			"launch":       plan.LaunchCommand,
		})
	}

	fmt.Printf("Ticket:   %s\n", s.Ticket)
	fmt.Printf("Status:   %s\n", s.Status)
	fmt.Printf("Workflow: %s\n", resolveWorkflow(root, s.Workflow))
	fmt.Printf("Stage:    %s\n", s.Stage.Name)
	fmt.Printf("Worker:   %s\n", s.Stage.Worker)

	interactive := isTTY()
	useResume := false

	switch s.Status {
	case "pending":
		// --dry must not mutate state; starting the ticket (pending → active +
		// a "started" history entry) is a real write, so skip it when previewing.
		if !nextDry {
			if err := state.Start(featureDir); err != nil {
				return err
			}
		}

	case "active":
		target, configured := runtimeTarget(s)
		sessionActive := configured && target.Backend == muxBackend.Name() && muxBackend.Available() && muxBackend.SessionExists(target.Workspace)
		if sessionActive {
			fmt.Println()
			fmt.Printf("⚠ %s workspace %q is already running.\n", muxBackend.Name(), target.Workspace)
			if interactive {
				ans := promptLine("  Attach to existing session? [Y/n]: ")
				ans = strings.ToLower(strings.TrimSpace(ans))
				if ans == "" || ans == "y" || ans == "yes" {
					if backend, ok := muxBackend.(mux.TargetBackend); ok {
						return backend.AttachTarget(target)
					}
					return muxBackend.AttachSession(target.Workspace)
				}
				fmt.Println("Cancelled.")
				return nil
			}
			if backend, ok := muxBackend.(mux.TargetBackend); ok {
				return backend.AttachTarget(target)
			}
			return muxBackend.AttachSession(target.Workspace)
		} else {
			fmt.Println()
			fmt.Println("⚠ Ticket is active but no session found — likely interrupted.")
			if interactive {
				ans := promptLine("  Launch with recovery context? [Y/n]: ")
				ans = strings.ToLower(strings.TrimSpace(ans))
				if ans == "" || ans == "y" || ans == "yes" {
					useResume = true
				}
			} else {
				useResume = true
			}
		}

	case "paused":
		reason := s.NextAction.Prompt
		if len(s.History) > 0 {
			reason = s.History[len(s.History)-1].Result
		}
		fmt.Println()
		fmt.Printf("⚠ Ticket is paused:\n  %s\n", reason)
		if interactive {
			ans := promptLine("  Launch with recovery context? [Y/n]: ")
			ans = strings.ToLower(strings.TrimSpace(ans))
			if ans != "" && ans != "y" && ans != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		useResume = true
	}
	fmt.Println()

	plan, err := runner.Compute(root, featureDir, nextWorker)
	if err != nil {
		return err
	}

	if useResume {
		ctx, err := resume.Build(root, featureDir)
		if err != nil {
			return fmt.Errorf("building resume prompt: %w", err)
		}
		plan.Prompt = ctx.Prompt
		fmt.Println("Using recovery context.")
		fmt.Println()
	}

	if nextDry {
		printDryRun(plan, s.Ticket)
		return nil
	}

	return launchPlan(root, featureDir, s, plan)
}

func launchPlan(root, featureDir string, s *state.State, plan *runner.Plan) error {
	launcher := orchestrator.NewLauncher()
	launcher.Mux = muxBackend
	result, err := launcher.Launch(orchestrator.LaunchOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
		Plan:       plan,
		In:         os.Stdin,
		Out:        os.Stdout,
		Err:        os.Stderr,
		OnFallback: func(message string) {
			if strings.HasPrefix(message, "warning:") {
				fmt.Println(message)
			} else {
				fmt.Printf("%s — running in foreground\n", message)
			}
		},
		OnHistoryWarning: func(message string) {
			fmt.Println(message)
		},
		OnTmuxSend: func(session, window string) {
			fmt.Printf("Sending to %s target %s:%s...\n", muxBackend.Name(), session, window)
		},
		OnForeground: func() {
			fmt.Printf("Launching %s (%s)...\n", plan.Worker.Name, plan.Worker.Engine)
		},
	})
	if err != nil {
		return err
	}

	if result.Mode == orchestrator.LaunchModeTmux {
		fmt.Printf("Agent launched in background.\n")
		fmt.Printf("Attach:  %s\n", result.AttachHint)
	}
	return nil
}
