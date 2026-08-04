package watch

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/parking"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspacesnapshot"
	tea "github.com/charmbracelet/bubbletea"
)

func loadDataWithMux(root, ticket string, demo bool, backend mux.Backend) tea.Cmd {
	return func() tea.Msg {
		if demo {
			return dataMsg{rows: demoRows(ticket)}
		}
		rows, err := collectRowsWithMux(root, ticket, backend)
		if err != nil && rows != nil {
			return dataMsg{rows: rows, warning: err}
		}
		return dataMsg{rows: rows, err: err}
	}
}

func collectRowsWithMux(root, ticket string, backend mux.Backend) ([]row, error) {
	snapshot, err := workspacesnapshot.LoadWithMux(root, backend)
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		f := item.Feature
		if f.Archived {
			continue
		}
		r := rowFromFeature(f, snapshot.Config)
		r.stuckAfter = configuredStuckAfter(snapshot.Config)
		r.context = item.Context
		if item.Attention != "" {
			r.attention = item.Attention
		}
		if item.Lifecycle != "" {
			r.liveState = item.Lifecycle
		}
		r.lifecycleSince = item.LifecycleSince
		r.stateChangeSeq = item.StateChangeSeq
		if item.HasTelemetry {
			live := item.Live
			r.providerID = live.ProviderSessionID
			if item.Lifecycle == "" {
				r.liveState = live.State
			}
			r.model = live.Model
			r.lastActive = live.LastActive
			if live.Engine != "" {
				r.engine = live.Engine
			}
		}
		rows = append(rows, r)
	}
	if settings := snapshot.Config.Settings.Parking; settings != nil && len(settings.AutoPark) > 0 {
		path, pathErr := parking.PolicyPath(root, "")
		if pathErr != nil {
			return filterRowsByTicket(rows, ticket), pathErr
		}
		observations := make([]parking.Observation, 0, len(rows))
		for _, r := range rows {
			observations = append(observations, parking.Observation{Ticket: r.ticket, Status: r.status, Stage: r.stageName, Attention: r.attention})
		}
		policyErr := applyParkingToRows(rows, path, root, parking.Policy{AutoPark: settings.AutoPark, WakeOn: settings.WakeOn}, observations, time.Now().UTC())
		if policyErr != nil {
			sortRows(rows)
			return filterRowsByTicket(rows, ticket), policyErr
		}
	}
	sortRows(rows)
	return filterRowsByTicket(rows, ticket), nil
}

func configuredStuckAfter(cfg *config.Config) time.Duration {
	if cfg != nil && cfg.Settings.Rail != nil && cfg.Settings.Rail.StuckAfter != "" {
		if duration, err := time.ParseDuration(cfg.Settings.Rail.StuckAfter); err == nil && duration > 0 {
			return duration
		}
	}
	return stuckLifecycleAge
}

func applyParkingToRows(rows []row, path, root string, policy parking.Policy, observations []parking.Observation, now time.Time) error {
	decisions, err := parking.ApplyPolicy(path, root, policy, observations, now)
	for i := range rows {
		decision := decisions[rows[i].ticket]
		rows[i].parked = decision.Parked
		rows[i].woken = decision.Woken
		rows[i].wakeReason = decision.WakeReason
	}
	return err
}

func filterRowsByTicket(rows []row, ticket string) []row {
	if ticket == "" {
		return rows
	}
	filtered := make([]row, 0, 1)
	for _, r := range rows {
		if strings.EqualFold(r.ticket, ticket) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func sortRows(rows []row) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].parked != rows[j].parked {
			return !rows[i].parked
		}
		left, right := rowPriority(rows[i]), rowPriority(rows[j])
		if left != right {
			return left < right
		}
		return rows[i].ticket < rows[j].ticket
	})
}

func mergeLiveVisuals(previous, next []row, now time.Time) []row {
	byTicket := make(map[string]row, len(previous))
	for _, r := range previous {
		byTicket[r.ticket] = r
	}
	for i := range next {
		current := &next[i]
		old, ok := byTicket[current.ticket]
		if ok {
			current.contextTrend = append([]uint64(nil), old.contextTrend...)
			current.flashUntil = old.flashUntil
			current.celebrateUntil = old.celebrateUntil
			_, oldState := displayState(old)
			_, newState := displayState(*current)
			if oldState != newState {
				current.flashUntil = now.Add(2 * time.Second)
				if newState == "done" {
					current.celebrateUntil = now.Add(4 * time.Second)
				}
			}
		}
		if current.context.Observed && current.context.Available {
			current.contextTrend = appendContextSample(current.contextTrend, current.context.Percent, 10)
		}
	}
	return next
}

func appendContextSample(samples []uint64, value uint64, limit int) []uint64 {
	samples = append(samples, value)
	if len(samples) > limit {
		samples = append([]uint64(nil), samples[len(samples)-limit:]...)
	}
	return samples
}

func demoRows(ticket string) []row {
	now := time.Now().UTC()
	steps := []workflowStep{
		{name: "intake", label: "intake", advance: "auto"},
		{name: "develop", label: "develop", advance: "auto"},
		{name: "review", label: "review", advance: "manual"},
		{name: "ship", label: "ship", advance: "manual"},
	}
	thresholds := contextpressure.DefaultThresholds()
	rows := []row{
		{
			ticket: "ORC-DEMO-1", name: "live-dashboard", stage: "develop", stageName: "develop", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "builder", status: "active", next: "Finish the responsive watch layout and hand off for review.", session: "orc-demo-1", window: "develop", pane: "%21", tmuxState: "live",
			context: contextpressure.Evaluate(44, 100, thresholds), contextTrend: []uint64{12, 18, 27, 35, 44}, room: "api/feature-orc-demo-1", branch: "feature/orc-demo-1", engine: "codex", model: "gpt-5", liveState: "working", lastActive: now.Add(-35 * time.Second),
			history: []historyRow{{at: now.Add(-48 * time.Minute).Format(time.RFC3339), stage: "intake", worker: "planner", result: "Implementation plan approved."}, {at: now.Add(-31 * time.Minute).Format(time.RFC3339), stage: "develop", worker: "builder", result: "Started implementation."}},
		},
		{
			ticket: "ORC-DEMO-2", name: "review-input", stage: "review", stageName: "review", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "reviewer", status: "paused", next: "Choose whether the new watch demo mode should be documented as a user feature.", session: "orc-demo-2", window: "review", pane: "%22", tmuxState: "live", attention: "input",
			context: contextpressure.Evaluate(76, 100, thresholds), contextTrend: []uint64{42, 51, 61, 69, 76}, room: "web/feature-orc-demo-2", branch: "feature/orc-demo-2", engine: "claude", model: "opus", liveState: "waiting", lastActive: now.Add(-4 * time.Minute),
			history: []historyRow{{at: now.Add(-2 * time.Hour).Format(time.RFC3339), stage: "develop", worker: "builder", result: "Completed the first implementation pass."}, {at: now.Add(-18 * time.Minute).Format(time.RFC3339), stage: "review", worker: "reviewer", result: "Requested a product decision."}},
		},
		{
			ticket: "ORC-DEMO-3", name: "stopped-session", stage: "review", stageName: "review", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "qa", status: "active", next: "Resume the stopped tmux session before continuing QA.", tmuxState: "stopped",
			context: contextpressure.Evaluate(94, 100, thresholds), contextTrend: []uint64{63, 72, 81, 88, 94}, room: "cli/feature-orc-demo-3", branch: "feature/orc-demo-3", engine: "codex", model: "gpt-5", liveState: "stopped", lastActive: now.Add(-27 * time.Minute),
		},
		{
			ticket: "ORC-DEMO-4", name: "completed-release", stage: "ship", stageName: "ship", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "release", status: "done", next: "", tmuxState: "stopped", room: "docs/feature-orc-demo-4", branch: "feature/orc-demo-4", lastActive: now.Add(-2 * time.Minute), demoCelebration: true,
			history: []historyRow{{at: now.Add(-2 * time.Minute).Format(time.RFC3339), stage: "ship", worker: "release", result: "Released successfully."}},
		},
	}
	filtered := make([]row, 0, len(rows))
	for i := range rows {
		rows[i].search = []string{rows[i].ticket, rows[i].name, rows[i].stage, rows[i].worker, rows[i].status, rows[i].room, rows[i].engine}
		if ticket == "" || strings.EqualFold(ticket, rows[i].ticket) {
			filtered = append(filtered, rows[i])
		}
	}
	sortRows(filtered)
	return filtered
}

func rowFromFeature(f *featurelist.Feature, cfg *config.Config) row {
	if f.LoadError != nil || f.State == nil {
		ticket := filepath.Base(f.FeatureDir)
		return row{
			ticket:  ticket,
			name:    ticket,
			status:  "error",
			loadErr: f.LoadError,
			search:  []string{ticket, f.FeatureDir, "error"},
		}
	}
	s := f.State
	workflowID := f.Workflow
	workflowLabel := f.WorkflowLabel
	stageLabel := f.StageLabel
	var workflowSteps []workflowStep
	if cfg != nil {
		for _, stage := range cfg.Stages(workflowID) {
			workflowSteps = append(workflowSteps, workflowStep{name: stage.Name, label: cfg.StageDisplayName(stage.Name), advance: stage.Advance})
			if stage.Loop != nil && stage.Loop.Via == s.Stage.Name {
				workflowSteps = append(workflowSteps, workflowStep{name: stage.Loop.Via, label: "↺ " + cfg.StageDisplayName(stage.Loop.Via), advance: "loop"})
			}
		}
	}
	room, branch := featureRoom(s)
	target, _ := s.Runtime.MuxTarget(s.Stage.Name)
	agentID, agentInstance := "", ""
	if s.Runtime.Agent != nil {
		agentID, agentInstance = s.Runtime.Agent.ID, s.Runtime.Agent.Instance
	}
	worker := f.WorkerName
	if worker == "" {
		worker = f.WorkerID
	}
	if worker == "" {
		worker = s.Stage.Worker
	}
	searchFields := []string{
		s.Ticket, s.Slug, s.Status, s.Workflow, f.Stage, s.Stage.Name, stageLabel, s.Stage.Worker,
		f.Workflow, f.WorkerID, f.WorkerName, f.Engine, f.Attention,
	}
	for name, repo := range s.Repos {
		searchFields = append(searchFields, name, repo.Main, repo.Worktree, repo.Branch)
	}
	return row{
		ticket:        s.Ticket,
		name:          s.Slug,
		stage:         stageLabel + f.StageLoopLabel,
		stageName:     s.Stage.Name,
		workflow:      workflowID,
		workflowLabel: workflowLabel,
		workflowSteps: workflowSteps,
		worker:        shortWorker(worker),
		status:        s.Status,
		next:          s.NextAction.Prompt,
		session:       tmuxSession(s),
		window:        muxTab(s),
		pane:          tmuxPane(s),
		backend:       target.Backend,
		agentID:       agentID,
		agentInstance: agentInstance,
		tmuxState:     tmuxState(s, f.TmuxLive),
		attention:     f.Attention,
		room:          room,
		branch:        branch,
		engine:        f.Engine,
		history:       historyRows(s.History),
		search:        searchFields,
	}
}

func muxTab(s *state.State) string {
	if target, ok := s.Runtime.MuxTarget(s.Stage.Name); ok {
		return target.Tab
	}
	return s.Stage.Name
}

func tmuxSession(s *state.State) string {
	target, ok := s.Runtime.MuxTarget(s.Stage.Name)
	if !ok {
		return ""
	}
	return target.Workspace
}

func tmuxPane(s *state.State) string {
	target, ok := s.Runtime.MuxTarget(s.Stage.Name)
	if !ok {
		return ""
	}
	return target.Pane
}

func historyRows(entries []state.HistoryEntry) []historyRow {
	rows := make([]historyRow, 0, len(entries))
	for _, h := range entries {
		rows = append(rows, historyRow{
			at:     h.At,
			stage:  h.Stage,
			worker: shortWorker(h.Worker),
			result: h.Result,
		})
	}
	return rows
}

func tmuxState(s *state.State, live bool) string {
	if _, ok := s.Runtime.MuxTarget(s.Stage.Name); !ok {
		return "-"
	}
	if live {
		return "live"
	}
	return "stopped"
}

func displayState(r row) (string, string) {
	if r.loadErr != nil {
		return "!", "error"
	}
	switch r.status {
	case "paused":
		return "!", "blocked"
	case "done":
		return "✓", "done"
	case "active":
		if r.tmuxState == "stopped" {
			return "x", "stopped"
		}
		switch r.attention {
		case mux.AttentionInput:
			return "!", "input"
		case mux.AttentionBlocked:
			return "!", "blocked"
		case mux.AttentionReview:
			return "◆", "review"
		case mux.AttentionDone:
			return "✓", "done"
		}
		return "●", "active"
	case "ready":
		return "▶", "ready"
	case "pending":
		return "○", "pending"
	default:
		return "?", r.status
	}
}

func rowPriority(r row) int {
	_, label := displayState(r)
	switch label {
	case "error":
		return 0
	case "blocked":
		return 1
	case "input":
		return 2
	case "review":
		return 3
	case "stopped":
		return 4
	case "ready":
		return 5
	case "pending":
		return 6
	case "active":
		return 7
	case "done":
		return 8
	default:
		return 9
	}
}

func attentionNeeded(r row) bool {
	_, label := displayState(r)
	return label == "blocked" || label == "input" || label == "review"
}

func shortWorker(worker string) string {
	worker = strings.TrimSpace(worker)
	worker = strings.TrimSuffix(worker, "-developer")
	worker = strings.TrimSuffix(worker, "-reviewer")
	worker = strings.TrimSuffix(worker, "-qa")
	if worker == "" {
		return "-"
	}
	return worker
}
