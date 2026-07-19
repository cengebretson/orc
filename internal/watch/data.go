package watch

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/workspacesnapshot"
	tea "github.com/charmbracelet/bubbletea"
)

func loadData(root, ticket string, demo bool) tea.Cmd {
	return func() tea.Msg {
		if demo {
			return dataMsg{rows: demoRows(ticket)}
		}
		rows, err := collectRows(root, ticket)
		return dataMsg{rows: rows, err: err}
	}
}

func collectRows(root, ticket string) ([]row, error) {
	snapshot, err := workspacesnapshot.Load(root)
	if err != nil {
		return nil, err
	}
	thresholds := snapshot.Config.ContextPressureThresholds()
	rows := make([]row, 0, len(snapshot.Features))
	for _, f := range snapshot.Features {
		if f.Archived {
			continue
		}
		r := rowFromFeature(f, snapshot.Config)
		if live, ok := snapshot.Telemetry[filepath.Clean(f.FeatureDir)]; ok {
			r.context = contextpressure.Evaluate(live.ContextUsed, live.ContextLimit, thresholds)
			r.providerID = live.ProviderSessionID
			r.liveState = live.State
			r.model = live.Model
			r.lastActive = live.LastActive
			if live.Engine != "" {
				r.engine = live.Engine
			}
		}
		if ticket != "" && !strings.EqualFold(r.ticket, ticket) {
			continue
		}
		rows = append(rows, r)
	}
	sortRows(rows)
	return rows, nil
}

func sortRows(rows []row) {
	sort.SliceStable(rows, func(i, j int) bool {
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
	workflowID := s.Workflow
	if workflowID == "" {
		workflowID = f.Workflow
	}
	workflowLabel := workflowID
	stageLabel := s.Stage.Name
	var workflowSteps []workflowStep
	if cfg != nil {
		workflowID = cfg.ResolveWorkflow(workflowID)
		workflowLabel = cfg.WorkflowDisplayName(workflowID)
		stageLabel = stageDisplayLabel(cfg, s.Stage.Name)
		for _, stage := range cfg.Stages(workflowID) {
			workflowSteps = append(workflowSteps, workflowStep{name: stage.Name, label: cfg.StageDisplayName(stage.Name), advance: stage.Advance})
			if stage.Loop != nil && stage.Loop.Via == s.Stage.Name {
				workflowSteps = append(workflowSteps, workflowStep{name: stage.Loop.Via, label: "↺ " + cfg.StageDisplayName(stage.Loop.Via), advance: "loop"})
			}
		}
	}
	room, branch := featureRoom(s)
	worker := f.WorkerName
	if worker == "" {
		worker = f.WorkerID
	}
	if worker == "" {
		worker = s.Stage.Worker
	}
	searchFields := []string{
		s.Ticket, s.Slug, s.Status, s.Workflow, s.Stage.Name, stageLabel, s.Stage.Worker,
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
		window:        s.Stage.Name,
		pane:          tmuxPane(s),
		tmuxState:     tmuxState(s, f.TmuxLive),
		attention:     f.Attention,
		room:          room,
		branch:        branch,
		engine:        f.Engine,
		history:       historyRows(s.History),
		search:        searchFields,
	}
}

func stageDisplayLabel(cfg *config.Config, stage string) string {
	if cfg == nil {
		return stage
	}
	return cfg.StageDisplayName(cfg.ResolveStage(stage))
}

func tmuxSession(s *state.State) string {
	if s.Runtime.Tmux == nil {
		return ""
	}
	return s.Runtime.Tmux.Session
}

func tmuxPane(s *state.State) string {
	if s.Runtime.Tmux == nil {
		return ""
	}
	return s.Runtime.Tmux.Pane
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
	if s.Runtime.Tmux == nil {
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
		case tmux.AttentionInput:
			return "!", "input"
		case tmux.AttentionBlocked:
			return "!", "blocked"
		case tmux.AttentionReview:
			return "◆", "review"
		case tmux.AttentionDone:
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
