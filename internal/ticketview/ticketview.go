package ticketview

import (
	"fmt"
	"path/filepath"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/workers"
)

type Options struct {
	// Mux reports whether the ticket's session is live and how to attach to
	// it. Defaults to tmux.
	Mux mux.Backend
}

type Summary struct {
	FeatureDir string
	State      *state.State

	Workflow       string
	Stage          string
	StageLoopLabel string
	WorkerID       string
	WorkerName     string
	WorkerEngine   string
	WorkerModel    string
	WorkerFound    bool

	NextStage   string
	NextAdvance string

	TmuxConfigured bool
	TmuxAvailable  bool
	TmuxLive       bool
	TmuxSession    string
	TmuxAttachHint string
	TmuxRestart    string

	JIT          *state.JITRuntime
	PausedReason string
}

func Build(root, featureDir string, s *state.State, opts Options) Summary {
	if opts.Mux == nil {
		opts.Mux = tmux.New()
	}

	cfg, _ := config.Load(root)
	allWorkers, _ := workers.Load(filepath.Join(root, "workers"))

	workflow := s.Workflow
	if workflow == "" && cfg != nil {
		workflow = cfg.DefaultWorkflow()
	}
	if workflow == "" {
		workflow = "default"
	}
	if cfg != nil {
		workflow = cfg.ResolveWorkflow(workflow)
	}

	workerID := s.Stage.Worker
	if workerID == "" && cfg != nil {
		if sc, ok := cfg.StageConfig(workflow, s.Stage.Name); ok {
			workerID = sc.Worker
		}
	}

	summary := Summary{
		FeatureDir:     featureDir,
		State:          s,
		Workflow:       workflow,
		Stage:          s.Stage.Name,
		StageLoopLabel: loopCountSuffix(cfg, workflow, s.Stage.Name, s),
		WorkerID:       workerID,
		NextAdvance:    "auto",
		JIT:            s.Runtime.JIT,
		PausedReason:   pausedReason(s),
	}

	if workerID != "" {
		if w := workers.FindByID(allWorkers, workerID); w != nil {
			summary.WorkerFound = true
			summary.WorkerName = w.Name
			summary.WorkerEngine = w.Engine
			summary.WorkerModel = w.Model
		} else {
			summary.WorkerName = workerID
		}
	}

	if cfg != nil {
		if next := cfg.NextStage(workflow, s.Stage.Name); next != "" {
			summary.NextStage = next
			sc, _ := cfg.StageConfig(workflow, next)
			if sc.Advance != "" {
				summary.NextAdvance = sc.Advance
			}
		}
	}

	if target, configured := s.Runtime.MuxTarget(s.Stage.Name); configured {
		session := target.Workspace
		summary.TmuxConfigured = true
		summary.TmuxSession = session
		summary.TmuxAvailable = opts.Mux.Available()
		summary.TmuxRestart = fmt.Sprintf("run `orc next %s` to restart", s.Ticket)
		if summary.TmuxAvailable && opts.Mux.SessionExists(session) {
			summary.TmuxLive = true
			if backend, ok := opts.Mux.(mux.TargetBackend); ok {
				summary.TmuxAttachHint = backend.AttachTargetHint(mux.Target{Backend: target.Backend, Workspace: target.Workspace, Tab: target.Tab, Pane: target.Pane})
			} else {
				summary.TmuxAttachHint = opts.Mux.AttachHint(session, target.Tab)
			}
		}
	}

	return summary
}

func loopCountSuffix(cfg *config.Config, workflow, stageName string, s *state.State) string {
	if cfg == nil || !cfg.IsLoopStage(workflow, stageName) {
		return ""
	}
	owner, ok := cfg.OwnerStage(workflow, stageName)
	if !ok {
		return ""
	}
	loopDef, ok := cfg.LoopConfig(workflow, owner)
	if !ok || loopDef.Max <= 0 {
		return ""
	}
	count := s.StageCounts[stageName]
	if count == 0 {
		count = s.StageCounts[cfg.ResolveStage(stageName)]
	}
	if count == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d/%d)", count, loopDef.Max)
}

func pausedReason(s *state.State) string {
	if s.Status != "paused" {
		return ""
	}
	if len(s.History) > 0 {
		if result := s.History[len(s.History)-1].Result; result != "" {
			return result
		}
	}
	return s.NextAction.Prompt
}
