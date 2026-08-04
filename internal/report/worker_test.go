package report_test

import (
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/report"
	"github.com/cengebretson/orc/internal/state"
)

func at(base time.Time, minutes int) string {
	return base.Add(time.Duration(minutes) * time.Minute).Format(time.RFC3339)
}

// Worker time comes from the same intervals the stage stats use, so a ticket
// that changes hands splits cleanly between the two workers.
func TestComputeAttributesTimeToWorkers(t *testing.T) {
	base := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	s := &state.State{
		Ticket: "ORC-1", Status: "done",
		Stage: state.Stage{Name: "review", Worker: "default:rae"},
		History: []state.HistoryEntry{
			{At: at(base, 0), Stage: "develop", Worker: "default:bob", Result: "started"},
			{At: at(base, 30), Stage: "develop", Worker: "default:bob", Result: "implemented"},
			{At: at(base, 40), Stage: "review", Worker: "default:rae", Result: "reviewed"},
		},
	}
	rep := report.Compute(s, base.Add(time.Hour))

	byWorker := map[string]report.WorkerStat{}
	for _, w := range rep.Workers {
		byWorker[w.Worker] = w
	}
	if got := byWorker["default:bob"].Active; got != 30*time.Minute {
		t.Fatalf("bob active = %s, want 30m", got)
	}
	if got := byWorker["default:rae"].Active; got != 10*time.Minute {
		t.Fatalf("rae active = %s, want 10m", got)
	}
}

// A human answering a question must not appear as a worker, and the wait for
// them must not land on the agent's bill.
//
// This is the real shape of a human interaction: an agent works, pauses to ask,
// a human answers, the agent resumes. The pause already excludes the waiting
// interval from active time, so the "human" entry never steals work the agent
// actually did — it only ever closes an interval that was paused anyway.
func TestComputeExcludesHumanFromWorkerAttribution(t *testing.T) {
	base := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	s := &state.State{
		Ticket: "ORC-2", Status: "done",
		Stage: state.Stage{Name: "develop", Worker: "default:bob"},
		History: []state.HistoryEntry{
			{At: at(base, 0), Stage: "develop", Worker: "default:bob", Result: "started"},
			{At: at(base, 10), Stage: "develop", Worker: "default:bob", Result: "paused — which approach?"},
			{At: at(base, 70), Stage: "develop", Worker: "human", Result: "answered — patch in place"},
			{At: at(base, 80), Stage: "develop", Worker: "default:bob", Result: "done"},
		},
	}
	rep := report.Compute(s, base.Add(2*time.Hour))

	for _, w := range rep.Workers {
		if w.Worker == "human" || w.Worker == "" {
			t.Fatalf("non-agent worker %q was attributed time", w.Worker)
		}
	}
	if len(rep.Workers) != 1 {
		t.Fatalf("workers = %#v, want only the agent", rep.Workers)
	}
	// 0->10 is bob working. 10->70 opens on the pause, so it is not active for
	// anyone. 70->80 closes on bob and counts.
	if got := rep.Workers[0].Active; got != 20*time.Minute {
		t.Fatalf("bob active = %s, want 20m — the hour spent waiting on a human is not his", got)
	}
}

// A paused interval is not active time for a worker any more than it is for a
// stage; the agent is not running during it.
func TestComputeExcludesPausedIntervalsFromWorkerActive(t *testing.T) {
	base := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	s := &state.State{
		Ticket: "ORC-3", Status: "done",
		Stage: state.Stage{Name: "develop", Worker: "default:bob"},
		History: []state.HistoryEntry{
			{At: at(base, 0), Stage: "develop", Worker: "default:bob", Result: "started"},
			{At: at(base, 10), Stage: "develop", Worker: "default:bob", Result: "paused — waiting on a human"},
			{At: at(base, 70), Stage: "develop", Worker: "default:bob", Result: "resumed"},
		},
	}
	rep := report.Compute(s, base.Add(90*time.Minute))
	if len(rep.Workers) != 1 {
		t.Fatalf("workers = %#v", rep.Workers)
	}
	w := rep.Workers[0]
	if w.Active != 10*time.Minute {
		t.Fatalf("active = %s, want 10m (the paused hour excluded)", w.Active)
	}
	if w.Wall != 70*time.Minute {
		t.Fatalf("wall = %s, want 70m", w.Wall)
	}
}

// Runs counts pickups, so a worker that loops back is distinguishable from one
// that finishes in a single pass.
func TestComputeCountsWorkerRuns(t *testing.T) {
	base := time.Date(2026, 8, 4, 9, 0, 0, 0, time.UTC)
	s := &state.State{
		Ticket: "ORC-4", Status: "done",
		Stage: state.Stage{Name: "develop", Worker: "default:bob"},
		History: []state.HistoryEntry{
			{At: at(base, 0), Stage: "develop", Worker: "default:bob", Result: "started"},
			{At: at(base, 10), Stage: "review", Worker: "default:rae", Result: "changes requested"},
			{At: at(base, 20), Stage: "develop", Worker: "default:bob", Result: "fixed"},
			{At: at(base, 30), Stage: "review", Worker: "default:rae", Result: "approved"},
		},
	}
	rep := report.Compute(s, base.Add(31*time.Minute))
	runs := map[string]int{}
	for _, w := range rep.Workers {
		runs[w.Worker] = w.Runs
	}
	// Each interval is attributed to the worker on its closing entry, so rae
	// closes two of them (the review and the re-review) and bob one. The ticket
	// is done, so there is no open interval to attribute to bob a second time.
	if runs["default:rae"] != 2 {
		t.Fatalf("rae runs = %d, want 2 — a worker that picks the ticket up twice", runs["default:rae"])
	}
	if runs["default:bob"] != 1 {
		t.Fatalf("bob runs = %d, want 1", runs["default:bob"])
	}
}

// The aggregate leads with the most expensive worker, which is the question the
// report exists to answer.
func TestAggregateWorkersOrdersByTotalActive(t *testing.T) {
	reports := []report.Report{
		{Ticket: "A", Workers: []report.WorkerStat{{Worker: "cheap", Active: time.Minute, Runs: 1}}},
		{Ticket: "B", Workers: []report.WorkerStat{{Worker: "dear", Active: time.Hour, Runs: 1}}},
		{Ticket: "C", Workers: []report.WorkerStat{{Worker: "dear", Active: time.Hour, Runs: 2}}},
	}
	aggs := report.AggregateWorkers(reports)
	if len(aggs) != 2 || aggs[0].Worker != "dear" {
		t.Fatalf("aggs = %#v, want dear first", aggs)
	}
	if aggs[0].TotalActive != 2*time.Hour || aggs[0].Tickets != 2 || aggs[0].Runs != 3 {
		t.Fatalf("dear = %#v", aggs[0])
	}
}
