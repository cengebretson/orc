// Package report derives time-in-stage statistics from a ticket's STATE.yaml
// history. Every state mutation appends a timestamped HistoryEntry, so each
// ticket already carries a complete event log of its life — this package reads
// that log and turns it into durations. It is purely read-side: no state is
// mutated and it works retroactively on any existing or archived ticket.
package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/state"
)

const timeLayout = time.RFC3339

// StageStat is the time a single ticket spent in one stage.
type StageStat struct {
	Stage  string
	Active time.Duration // wall-clock minus paused intervals
	Wall   time.Duration // total wall-clock
	Visits int           // number of times the ticket entered this stage
}

// WorkerStat is time attributed to one worker on a ticket.
//
// Attribution comes from the same history intervals the stage stats use: each
// entry records the worker that was active up to its timestamp. Runs counts how
// many separate times that worker picked the ticket up, which is what makes a
// worker that loops repeatedly visible next to one that finishes in a pass.
type WorkerStat struct {
	Worker string
	Active time.Duration
	Wall   time.Duration
	Runs   int
}

// Report is the per-ticket timing breakdown, stages in first-visit order.
type Report struct {
	Ticket  string
	Stages  []StageStat
	Workers []WorkerStat
	Active  time.Duration
	Wall    time.Duration
	Open    bool // ticket still in progress (current stage measured to now)
}

// Compute derives per-stage timing for one ticket from its history.
//
// Each HistoryEntry records the stage that was active up to its timestamp, so
// the interval between two consecutive entries is attributed to the stage on
// the *closing* entry. A paused→resumed gap is identified by the *opening*
// entry's result and excluded from active time. For an unfinished ticket, the
// current stage is measured from the last entry to now.
//
// Unparseable or out-of-order timestamps are tolerated: an entry without a
// usable timestamp is skipped, and a negative interval is clamped to zero, so
// an old or hand-edited STATE.yaml never breaks the report.
func Compute(s *state.State, now time.Time) Report {
	r := Report{Ticket: s.Ticket}

	type point struct {
		t      time.Time
		stage  string
		worker string
		result string
	}
	var pts []point
	for _, h := range s.History {
		t, err := time.Parse(timeLayout, h.At)
		if err != nil {
			continue
		}
		pts = append(pts, point{t: t, stage: h.Stage, worker: h.Worker, result: h.Result})
	}

	idx := map[string]int{}
	add := func(stage string, dur time.Duration, paused, newVisit bool) {
		if dur < 0 {
			dur = 0
		}
		i, ok := idx[stage]
		if !ok {
			i = len(r.Stages)
			idx[stage] = i
			r.Stages = append(r.Stages, StageStat{Stage: stage})
		}
		r.Stages[i].Wall += dur
		r.Wall += dur
		if !paused {
			r.Stages[i].Active += dur
			r.Active += dur
		}
		if newVisit {
			r.Stages[i].Visits++
		}
	}

	workerIdx := map[string]int{}
	addWorker := func(worker string, dur time.Duration, paused, newRun bool) {
		// Entries written by a human action (a pause, an answer) carry no agent
		// worker; attributing that time to an agent would overstate its cost.
		if worker == "" || worker == "human" {
			return
		}
		if dur < 0 {
			dur = 0
		}
		i, ok := workerIdx[worker]
		if !ok {
			i = len(r.Workers)
			workerIdx[worker] = i
			r.Workers = append(r.Workers, WorkerStat{Worker: worker})
		}
		r.Workers[i].Wall += dur
		if !paused {
			r.Workers[i].Active += dur
		}
		if newRun {
			r.Workers[i].Runs++
		}
	}

	prevStage := ""
	prevWorker := ""
	for i := 0; i+1 < len(pts); i++ {
		stage := pts[i+1].stage
		worker := pts[i+1].worker
		dur := pts[i+1].t.Sub(pts[i].t)
		paused := isPause(pts[i].result)
		add(stage, dur, paused, stage != prevStage)
		addWorker(worker, dur, paused, worker != prevWorker)
		prevStage = stage
		if worker != "" && worker != "human" {
			prevWorker = worker
		}
	}

	// A non-terminal ticket is still accruing time in its current stage; measure
	// that open interval to now. A paused ticket's open interval does not count
	// as active — trust the status field here rather than the result string,
	// since the latter carries a free-form reason ("blocked — …") that may not
	// match the canonical "paused — …" prefix.
	if !terminal(s.Status) && len(pts) > 0 {
		last := pts[len(pts)-1]
		stage := s.Stage.Name
		if stage == "" {
			stage = last.stage
		}
		paused := s.Status == "paused" || isPause(last.result)
		worker := s.Stage.Worker
		if worker == "" {
			worker = last.worker
		}
		add(stage, now.Sub(last.t), paused, stage != prevStage)
		addWorker(worker, now.Sub(last.t), paused, worker != prevWorker)
		r.Open = true
	}

	return r
}

// StageAgg aggregates one stage across multiple tickets.
type StageAgg struct {
	Stage     string
	Tickets   int           // how many tickets visited this stage
	AvgActive time.Duration // mean active time across those tickets
	MedActive time.Duration // median active time across those tickets
	Visits    int           // total entries into this stage across tickets
}

// Aggregate combines per-ticket reports into per-stage statistics, ordered by
// the stage's first appearance across the reports.
func Aggregate(reports []Report) []StageAgg {
	type acc struct {
		actives []time.Duration
		visits  int
	}
	accs := map[string]*acc{}
	var order []string
	for _, r := range reports {
		for _, st := range r.Stages {
			a, ok := accs[st.Stage]
			if !ok {
				a = &acc{}
				accs[st.Stage] = a
				order = append(order, st.Stage)
			}
			a.actives = append(a.actives, st.Active)
			a.visits += st.Visits
		}
	}
	out := make([]StageAgg, 0, len(order))
	for _, stage := range order {
		a := accs[stage]
		out = append(out, StageAgg{
			Stage:     stage,
			Tickets:   len(a.actives),
			AvgActive: mean(a.actives),
			MedActive: median(a.actives),
			Visits:    a.visits,
		})
	}
	return out
}

// terminal reports whether a status means the ticket is finished — no open
// interval should be measured to now for these.
func terminal(status string) bool {
	return status == "done" || status == "archived"
}

func isPause(result string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(result)), "paused")
}

func mean(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum / time.Duration(len(ds))
}

func median(ds []time.Duration) time.Duration {
	n := len(ds)
	if n == 0 {
		return 0
	}
	s := append([]time.Duration(nil), ds...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// Humanize renders a duration compactly: "45s", "12m", "3h 20m", "2d 4h".
func Humanize(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d / time.Hour)
		m := int((d % time.Hour) / time.Minute)
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		days := int(d / (24 * time.Hour))
		h := int((d % (24 * time.Hour)) / time.Hour)
		if h == 0 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd %dh", days, h)
	}
}

// WorkerAgg aggregates one worker across multiple tickets.
type WorkerAgg struct {
	Worker      string
	Tickets     int           // how many tickets this worker touched
	TotalActive time.Duration // summed active time, the closest honest proxy for cost
	AvgActive   time.Duration // mean active time per ticket
	MedActive   time.Duration // median active time per ticket
	Runs        int           // total pickups across tickets
}

// AggregateWorkers combines per-ticket reports into per-worker statistics,
// ordered by total active time so the most expensive worker leads.
//
// Time is the unit deliberately. Token and currency cost would need cumulative
// usage the providers do not both expose the same way, plus a per-model price
// table Orc would have to keep current — a number that looks precise and is
// quietly wrong is worse than one that is honestly coarse.
func AggregateWorkers(reports []Report) []WorkerAgg {
	type acc struct {
		actives []time.Duration
		runs    int
	}
	accs := map[string]*acc{}
	for _, r := range reports {
		for _, w := range r.Workers {
			a, ok := accs[w.Worker]
			if !ok {
				a = &acc{}
				accs[w.Worker] = a
			}
			a.actives = append(a.actives, w.Active)
			a.runs += w.Runs
		}
	}
	out := make([]WorkerAgg, 0, len(accs))
	for worker, a := range accs {
		var total time.Duration
		for _, d := range a.actives {
			total += d
		}
		out = append(out, WorkerAgg{
			Worker:      worker,
			Tickets:     len(a.actives),
			TotalActive: total,
			AvgActive:   mean(a.actives),
			MedActive:   median(a.actives),
			Runs:        a.runs,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalActive != out[j].TotalActive {
			return out[i].TotalActive > out[j].TotalActive
		}
		return out[i].Worker < out[j].Worker
	})
	return out
}
