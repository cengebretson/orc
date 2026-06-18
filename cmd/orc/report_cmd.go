package main

import (
	"fmt"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/report"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/spf13/cobra"
)

func runReport(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	now := time.Now()

	if len(args) == 1 {
		t, err := ticket.LoadWithArchive(root, args[0])
		if err != nil {
			return err
		}
		rep := report.Compute(t.State, now)
		if reportJSON {
			return printJSON(ticketReportJSON(rep, t.State.Stage.Name))
		}
		printTicketReport(rep, t.State.Stage.Name)
		return nil
	}

	features, err := featurelist.Collect(root, featurelist.Options{IncludeArchived: reportArchived})
	if err != nil {
		return err
	}
	var reports []report.Report
	for _, f := range features {
		if f.LoadError != nil {
			continue
		}
		reports = append(reports, report.Compute(f.State, now))
	}
	aggs := report.Aggregate(reports)
	if reportJSON {
		return printJSON(aggregateReportJSON(aggs, len(reports)))
	}
	printAggregateReport(aggs, len(reports))
	return nil
}

func printTicketReport(rep report.Report, currentStage string) {
	status := "complete"
	if rep.Open {
		status = "in progress"
	}
	fmt.Printf("%s · %s\n\n", rep.Ticket, status)
	if len(rep.Stages) == 0 {
		fmt.Println("No timing history yet.")
		return
	}
	fmt.Printf("%-18s  %-10s  %-10s  %-6s\n", "Stage", "Active", "Wall", "Visits")
	fmt.Printf("%-18s  %-10s  %-10s  %-6s\n", "-----", "------", "----", "------")
	for _, st := range rep.Stages {
		marker := ""
		if rep.Open && st.Stage == currentStage {
			marker = "  (current)"
		}
		fmt.Printf("%-18s  %-10s  %-10s  %-6d%s\n",
			st.Stage, report.Humanize(st.Active), report.Humanize(st.Wall), st.Visits, marker)
	}
	fmt.Printf("%-18s  %-10s  %-10s\n", "-----", "------", "----")
	fmt.Printf("%-18s  %-10s  %-10s\n", "Total", report.Humanize(rep.Active), report.Humanize(rep.Wall))
}

func printAggregateReport(aggs []report.StageAgg, tickets int) {
	if tickets == 0 {
		fmt.Println("No features found. Start one with `orc work <ticket>`.")
		return
	}
	fmt.Printf("Stage timing across %d ticket(s)\n\n", tickets)
	fmt.Printf("%-18s  %-8s  %-10s  %-10s  %-6s\n", "Stage", "Tickets", "Avg active", "Med active", "Visits")
	fmt.Printf("%-18s  %-8s  %-10s  %-10s  %-6s\n", "-----", "-------", "----------", "----------", "------")
	for _, a := range aggs {
		fmt.Printf("%-18s  %-8d  %-10s  %-10s  %-6d\n",
			a.Stage, a.Tickets, report.Humanize(a.AvgActive), report.Humanize(a.MedActive), a.Visits)
	}
}

// ── report JSON shapes (durations as whole seconds, plus a human string) ──

func ticketReportJSON(rep report.Report, currentStage string) map[string]any {
	stages := make([]map[string]any, 0, len(rep.Stages))
	for _, st := range rep.Stages {
		stages = append(stages, map[string]any{
			"stage":          st.Stage,
			"active_seconds": int64(st.Active.Seconds()),
			"wall_seconds":   int64(st.Wall.Seconds()),
			"active":         report.Humanize(st.Active),
			"wall":           report.Humanize(st.Wall),
			"visits":         st.Visits,
			"current":        rep.Open && st.Stage == currentStage,
		})
	}
	return map[string]any{
		"ticket":               rep.Ticket,
		"open":                 rep.Open,
		"stages":               stages,
		"total_active_seconds": int64(rep.Active.Seconds()),
		"total_wall_seconds":   int64(rep.Wall.Seconds()),
	}
}

func aggregateReportJSON(aggs []report.StageAgg, tickets int) map[string]any {
	stages := make([]map[string]any, 0, len(aggs))
	for _, a := range aggs {
		stages = append(stages, map[string]any{
			"stage":              a.Stage,
			"tickets":            a.Tickets,
			"avg_active_seconds": int64(a.AvgActive.Seconds()),
			"med_active_seconds": int64(a.MedActive.Seconds()),
			"avg_active":         report.Humanize(a.AvgActive),
			"med_active":         report.Humanize(a.MedActive),
			"visits":             a.Visits,
		})
	}
	return map[string]any{
		"tickets": tickets,
		"stages":  stages,
	}
}

// loopCountSuffix returns " (N/M)" when stageName is an active loop stage with a max defined.
