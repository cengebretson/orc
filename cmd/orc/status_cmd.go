package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/cengebretson/orc/internal/ticketview"

	"github.com/cengebretson/orc/internal/validate"
	"github.com/spf13/cobra"
)

type statusRow struct {
	ticket   string
	status   string
	workflow string
	worker   string
	next     string
	session  string
}

type statusJSONView struct {
	*state.State
	Live *telemetry.Live `json:"live,omitempty"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		t, err := ticket.Load(root, args[0])
		if err != nil {
			return err
		}
		if statusJSON {
			view := statusJSONView{State: t.State}
			if sessions, liveErr := sessionlist.Collect(root, sessionlist.Options{}); liveErr == nil {
				for _, session := range sessions {
					if session.Ticket == t.State.Ticket && session.Live != nil {
						view.Live = session.Live
						break
					}
				}
			}
			return printJSON(view)
		}
		if err := printShow(root, t.FeatureDir, t.State); err != nil {
			return err
		}
		if !validate.Run(root, t.FeatureDir).OK() {
			fmt.Printf("\n⚠  state has problems — run `orc doctor %s` for details\n", t.State.Ticket)
		}
		return nil
	}

	// Collect does its own availability and session lookup; this one is for
	// deciding whether the printed table carries a tmux column.
	showTmux := muxBackend.Available()

	features, err := featurelist.Collect(root, featurelist.Options{
		IncludeArchived: true,
	})
	if err != nil {
		return err
	}

	if statusJSON {
		return printJSON(map[string]any{
			"active":   statusStates(features, false),
			"archived": statusStates(features, true),
		})
	}

	active := statusRows(features, false)
	archived := statusRows(features, true)

	if len(active) == 0 && len(archived) == 0 {
		fmt.Println("No features found. Start one with `orc work <ticket>`.")
		return nil
	}

	if len(active) > 0 {
		fmt.Printf("Active (%d)\n\n", len(active))
		printStatusTable(active, showTmux)
	}

	if len(archived) > 0 {
		if len(active) > 0 {
			fmt.Println()
		}
		fmt.Printf("Archived (%d)\n\n", len(archived))
		printStatusTable(archived, showTmux)
	}

	return nil
}

func statusRows(features []*featurelist.Feature, archived bool) []statusRow {
	var rows []statusRow
	for _, f := range features {
		if f.Archived != archived {
			continue
		}
		rows = append(rows, statusRowForFeature(f))
	}
	return rows
}

func statusRowForFeature(f *featurelist.Feature) statusRow {
	if f.LoadError != nil {
		return statusRow{
			ticket: filepath.Base(f.FeatureDir),
			status: "error",
			next:   f.LoadError.Error(),
		}
	}

	s := f.State
	next := s.NextAction.Prompt
	if len(next) > 40 {
		next = next[:40] + "…"
	}
	session := "-"
	if s.Runtime.Tmux != nil {
		if f.TmuxLive {
			session = "✓"
		} else {
			session = "✗" // configured but not running
		}
	}
	stageLabel := f.Workflow + " · " + s.Stage.Name + f.StageLoopLabel
	if s.Runtime.JIT != nil {
		stageLabel += " + jit"
	}
	return statusRow{
		ticket:   s.Ticket,
		status:   s.Status,
		workflow: stageLabel,
		worker:   f.WorkerID,
		next:     next,
		session:  session,
	}
}

func statusStates(features []*featurelist.Feature, archived bool) []*state.State {
	var out []*state.State
	for _, f := range features {
		if f.Archived != archived || f.LoadError != nil {
			continue
		}
		out = append(out, f.State)
	}
	return out
}

func printStatusTable(rows []statusRow, showTmux bool) {
	if showTmux {
		fmt.Printf("%-16s  %-16s  %-28s  %-20s  %-6s  %s\n", "Ticket", "Status", "Workflow", "Worker", "Tmux", "Next")
		fmt.Printf("%-16s  %-16s  %-28s  %-20s  %-6s  %s\n", "------", "------", "--------", "-----", "----", "----")
		for _, r := range rows {
			fmt.Printf("%-16s  %-16s  %-28s  %-20s  %-6s  %s\n", r.ticket, r.status, r.workflow, r.worker, r.session, r.next)
		}
		return
	}

	fmt.Printf("%-16s  %-16s  %-28s  %-20s  %s\n", "Ticket", "Status", "Workflow", "Worker", "Next")
	fmt.Printf("%-16s  %-16s  %-28s  %-20s  %s\n", "------", "------", "--------", "-----", "----")
	for _, r := range rows {
		fmt.Printf("%-16s  %-16s  %-28s  %-20s  %s\n", r.ticket, r.status, r.workflow, r.worker, r.next)
	}
}

func printShow(root, featureDir string, s *state.State) error {
	summary := ticketview.Build(root, featureDir, s, ticketview.Options{})

	fmt.Printf("Ticket:   %s\n", s.Ticket)
	fmt.Printf("Slug:     %s\n", s.Slug)
	fmt.Printf("Status:   %s\n", s.Status)
	if summary.TmuxConfigured {
		if summary.TmuxLive {
			fmt.Printf("Session:  %s\n", summary.TmuxAttachHint)
		} else {
			fmt.Printf("Session:  %s  (not running — %s)\n", summary.TmuxSession, summary.TmuxRestart)
		}
	}
	if summary.JIT != nil {
		fmt.Println()
		fmt.Println("JIT")
		fmt.Printf("  Worker:   %s\n", summary.JIT.Worker)
		fmt.Printf("  Task:     %s\n", summary.JIT.Task)
		fmt.Printf("  Started:  %s\n", summary.JIT.StartedAt)
	}

	fmt.Println()
	fmt.Println("Stage")
	fmt.Printf("  Stage:     %s · %s%s\n", summary.Workflow, summary.Stage, summary.StageLoopLabel)
	fmt.Printf("  Worker:    %s\n", summary.WorkerID)
	if summary.NextStage != "" {
		fmt.Printf("  Next:      %s  (%s)\n", summary.NextStage, summary.NextAdvance)
	}

	if len(s.Repos) > 0 {
		fmt.Println()
		fmt.Println("Repos")
		for name, r := range s.Repos {
			fmt.Printf("  %s\n", name)
			if r.Main != "" {
				fmt.Printf("    main:     %s\n", r.Main)
			}
			if r.Worktree != "" {
				fmt.Printf("    worktree: %s\n", r.Worktree)
				fmt.Printf("    branch:   %s\n", r.Branch)
			}
		}
	}

	if len(s.Inputs.Ready)+len(s.Inputs.Required)+len(s.Inputs.Completed) > 0 {
		fmt.Println()
		fmt.Println("Inputs")
		for _, f := range s.Inputs.Ready {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Inputs.Required {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Inputs.Completed {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
	}

	if len(s.Outputs.Ready)+len(s.Outputs.Required)+len(s.Outputs.Completed) > 0 {
		fmt.Println()
		fmt.Println("Outputs")
		for _, f := range s.Outputs.Ready {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Outputs.Required {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Outputs.Completed {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
	}

	fmt.Println()
	fmt.Println("Next")
	switch s.Status {
	case "paused":
		fmt.Printf("  Paused:  %s\n", summary.PausedReason)
		fmt.Println("  Run `orc next` after resolving to continue.")
	default:
		if summary.WorkerID != "" {
			if summary.WorkerFound {
				fmt.Printf("  Worker:  %s (%s)\n", summary.WorkerName, summary.WorkerEngine)
				if summary.WorkerModel != "" {
					fmt.Printf("  Model:   %s\n", summary.WorkerModel)
				}
			} else {
				fmt.Printf("  Worker:  %s (not found in workers/)\n", summary.WorkerID)
			}
		} else {
			fmt.Println("  Worker:  none assigned — set worker: in orc.yaml")
		}
		fmt.Println("  Run `orc next` to launch.")
	}

	if len(s.History) > 0 {
		fmt.Println()
		fmt.Println("History")
		for _, h := range s.History {
			ts := h.At
			if t, err := time.Parse(time.RFC3339, h.At); err == nil {
				ts = t.Format("2006-01-02 15:04")
			}
			fmt.Printf("  %-16s  %-20s  %-20s  %s\n", ts, h.Stage, h.Worker, h.Result)
		}
	}

	return nil
}

func fileCheck(featureDir, relPath string) string {
	if _, err := os.Stat(filepath.Join(featureDir, relPath)); err == nil {
		return "✓"
	}
	return "✗"
}
