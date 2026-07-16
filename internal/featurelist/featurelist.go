package featurelist

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
)

type Feature struct {
	State          *state.State
	FeatureDir     string
	Archived       bool
	Workflow       string
	StageLoopLabel string
	WorkerID       string
	WorkerName     string
	Engine         string
	TmuxLive       bool
	Attention      string
	HasIssues      bool
	LoadError      error
}

type Options struct {
	IncludeArchived bool
	TmuxAvailable   func() bool
	ListSessions    func() []string
	WindowAttention func(session, window string) string
}

func Collect(root string, opts Options) ([]*Feature, error) {
	if opts.TmuxAvailable == nil {
		opts.TmuxAvailable = tmux.Available
	}
	if opts.ListSessions == nil {
		opts.ListSessions = tmux.ListSessions
	}
	if opts.WindowAttention == nil {
		opts.WindowAttention = tmux.WindowAttention
	}

	ctx, _ := workspacectx.Load(root)
	var cfg *config.Config
	var allWorkers []*workers.Worker
	if ctx != nil {
		cfg = ctx.Config
		allWorkers = ctx.Workers
	}
	activeSessions := map[string]bool{}
	if opts.TmuxAvailable() {
		for _, name := range opts.ListSessions() {
			activeSessions[name] = true
		}
	}

	featuresDir := filepath.Join(root, "features")
	var out []*Feature
	if err := collectDir(root, featuresDir, false, cfg, allWorkers, activeSessions, opts.WindowAttention, &out); err != nil {
		return nil, err
	}
	if opts.IncludeArchived {
		if err := collectDir(root, filepath.Join(featuresDir, "_archive"), true, cfg, allWorkers, activeSessions, opts.WindowAttention, &out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func collectDir(root, dir string, archived bool, cfg *config.Config, allWorkers []*workers.Worker, activeSessions map[string]bool, windowAttention func(session, window string) string, out *[]*Feature) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_template" || e.Name() == "_archive" {
			continue
		}
		featureDir := filepath.Join(dir, e.Name())
		s, err := state.Load(featureDir)
		if err != nil {
			*out = append(*out, &Feature{
				FeatureDir: featureDir,
				Archived:   archived,
				LoadError:  err,
			})
			continue
		}

		workflow := resolveWorkflow(cfg, s)
		workerID := resolveWorkerID(cfg, workflow, s)
		tmuxLive := s.Runtime.Tmux != nil && activeSessions[s.Runtime.Tmux.Session]
		attention := ""
		if tmuxLive && windowAttention != nil {
			attention = windowAttention(s.Runtime.Tmux.Session, s.Stage.Name)
		}
		worker := workers.FindByID(allWorkers, workerID)
		engine := ""
		if worker != nil {
			engine = worker.Engine
		}
		*out = append(*out, &Feature{
			State:          s,
			FeatureDir:     featureDir,
			Archived:       archived,
			Workflow:       workflow,
			StageLoopLabel: loopCountSuffix(cfg, workflow, s.Stage.Name, s),
			WorkerID:       workerID,
			WorkerName:     resolveWorkerName(allWorkers, workerID),
			Engine:         engine,
			TmuxLive:       tmuxLive,
			Attention:      attention,
			HasIssues:      workerID == "",
		})
	}
	return nil
}

func resolveWorkflow(cfg *config.Config, s *state.State) string {
	if s.Workflow != "" {
		if cfg != nil {
			return cfg.ResolveWorkflow(s.Workflow)
		}
		return s.Workflow
	}
	if cfg != nil && cfg.DefaultWorkflow() != "" {
		return cfg.ResolveWorkflow(cfg.DefaultWorkflow())
	}
	return "default"
}

func resolveWorkerID(cfg *config.Config, workflow string, s *state.State) string {
	if s.Stage.Worker != "" {
		return s.Stage.Worker
	}
	if cfg == nil {
		return ""
	}
	if sc, ok := cfg.StageConfig(workflow, s.Stage.Name); ok {
		return sc.Worker
	}
	return ""
}

func resolveWorkerName(allWorkers []*workers.Worker, workerID string) string {
	if workerID == "" {
		return ""
	}
	if w := workers.FindByID(allWorkers, workerID); w != nil {
		return w.Name
	}
	return workerID
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
