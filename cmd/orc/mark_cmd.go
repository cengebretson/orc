package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	orcnotify "github.com/cengebretson/orc/internal/notify"
	"github.com/cengebretson/orc/internal/orchestrator"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/spf13/cobra"
)

var sendTransitionNotification = orcnotify.Send

func runMark(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	ticketArg := args[0]
	action := strings.ToLower(args[1])

	// jit can target any status including archived; all other actions use features/ only.
	var t *ticket.Ticket
	if action == "jit" {
		t, err = ticket.LoadWithArchive(root, ticketArg)
	} else {
		t, err = ticket.Load(root, ticketArg)
	}
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State

	if action == "pause" && len(args) < 3 {
		return fmt.Errorf("orc mark %s pause requires a reason — e.g. orc mark %s pause \"<reason>\"", ticketArg, ticketArg)
	}

	switch action {
	case "start":
		if !oneOf(s.Status, "pending", "ready") {
			return fmt.Errorf("cannot mark %s start from status %q — use `orc mark %s resume` to continue a paused ticket", s.Ticket, s.Status, s.Ticket)
		}
		if err := state.ValidateRepos(s, root); err != nil {
			return err
		}
		if err := state.Start(featureDir); err != nil {
			return err
		}
		printTicketStatus(s.Ticket, "active")
		return nil

	case "resume":
		if s.Status != "paused" {
			return fmt.Errorf("cannot mark %s resume from status %q — resume is only valid from paused", s.Ticket, s.Status)
		}
		if err := state.ValidateRepos(s, root); err != nil {
			return err
		}
		if err := state.Resume(featureDir); err != nil {
			return err
		}
		printTicketStatus(s.Ticket, "active")
		return nil

	case "pause":
		if oneOf(s.Status, "done", "archived") {
			return fmt.Errorf("cannot pause %s from status %q", s.Ticket, s.Status)
		}
		if err := state.ValidateRepos(s, root); err != nil {
			return err
		}
		reason := strings.Join(args[2:], " ")
		if err := state.Pause(featureDir, reason); err != nil {
			return err
		}
		notifyTransition(root, featureDir, "blocked")
		printPauseResult(s.Ticket, reason)
		return nil

	case "next":
		return runMarkNext(root, featureDir)

	case "done":
		if !oneOf(s.Status, "active", "ready", "paused") {
			return fmt.Errorf("cannot mark %s done from status %q", s.Ticket, s.Status)
		}
		result := markResult
		if result == "" {
			result = "done"
		}
		if err := state.Done(featureDir, result); err != nil {
			return err
		}
		notifyTransition(root, featureDir, "complete")
		printTicketStatus(s.Ticket, "done")
		return nil

	case "jit":
		if len(args) < 3 {
			return fmt.Errorf("orc mark %s jit requires a summary", ticketArg)
		}
		summary := strings.Join(args[2:], " ")
		workerID := ""
		if s.Runtime.JIT != nil {
			workerID = s.Runtime.JIT.Worker
		}
		if err := state.AppendHistory(featureDir, "jit", workerID, summary); err != nil {
			return err
		}
		if err := state.ClearJIT(featureDir); err != nil {
			return err
		}
		fmt.Printf("Done: jit task recorded for %s\n", s.Ticket)
		return nil

	default:
		return fmt.Errorf("unknown action %q — use: start | resume | next [--result] [--stage] [--worker] [--force] | pause <reason> | done [--result] | jit <summary>", action)
	}
}

func printTicketStatus(ticket, status string) {
	fmt.Printf("Ticket:  %s\n", ticket)
	fmt.Printf("Status:  %s\n", status)
}

func printPauseResult(ticket, reason string) {
	printTicketStatus(ticket, "paused")
	fmt.Printf("Reason:  %s\n", reason)
	fmt.Printf("\nRun `orc next %s` to resume once resolved.\n", ticket)
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func runMarkNext(root, featureDir string) error {
	result, err := orchestrator.Advance(orchestrator.AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
		Stage:      markStage,
		Worker:     markWorker,
		Result:     markResult,
		Force:      markForce,
	})
	if err != nil {
		return err
	}

	if result.Outcome == orchestrator.AdvanceOutcomePaused {
		fmt.Printf("Loop limit reached %s. Pausing for human review.\n", strings.TrimPrefix(result.Reason, "loop limit reached "))
		return nil
	}
	notifyTransition(root, featureDir, "complete")

	printAdvanceResult(result)

	if result.Outcome == orchestrator.AdvanceOutcomeDone {
		if result.AutoArchive {
			fmt.Println()
			s, err := state.Load(featureDir)
			if err == nil {
				return archiveFeature(root, featureDir, s)
			}
			return err
		}
		return nil
	}

	fmt.Printf("\nRun `orc next %s` to launch the next worker.\n", result.Ticket)
	fmt.Println()

	plan, err := runner.Compute(root, featureDir, "")
	if err != nil {
		return err
	}
	printDryRun(plan, result.Ticket)
	return nil
}

func notifyTransition(root, featureDir, eventName string) {
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load notification settings: %v\n", err)
		return
	}
	s, err := state.Load(featureDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load notification context: %v\n", err)
		return
	}
	if err := sendTransitionNotification(cfg.Settings.Notify, orcnotify.Event{
		Ticket: s.Ticket, Slug: s.Slug, Name: eventName, Stage: s.Stage.Name,
		Workflow: s.Workflow, WorkDir: root,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
}

func printAdvanceResult(result *orchestrator.AdvanceResult) {
	fmt.Printf("Ticket:   %s\n", result.Ticket)
	if result.Next != "" && result.Next != result.Previous {
		fmt.Printf("Stage:    %s → %s\n", result.Previous, result.Next)
	} else if result.Next == "" {
		fmt.Printf("Stage:    %s  (final)\n", result.Previous)
		fmt.Printf("Status:   done\n")
	} else {
		fmt.Printf("Stage:    %s  (unchanged)\n", result.Previous)
	}
	if result.Worker != "" {
		fmt.Printf("Worker:   %s\n", result.Worker)
	}
	if result.Reason != "" {
		fmt.Printf("Note:     %s\n", result.Reason)
	}
}
