package featurelist

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
)

type Feature struct {
	State             *state.State
	FeatureDir        string
	Archived          bool
	Workflow          string
	WorkflowLabel     string
	Stage             string
	StageLabel        string
	StageLoopLabel    string
	RequiredArtifacts []string
	WorkerID          string
	WorkerName        string
	Engine            string
	TmuxLive          bool
	Attention         string
	HasIssues         bool
	LoadError         error
}

type Options struct {
	IncludeArchived bool
	// Config and Workers let callers reuse one immutable workspace snapshot
	// instead of loading the same workspace context for each projection.
	Config  *config.Config
	Workers []*workers.Worker
	// Mux supplies live session and attention state. Defaults to tmux.
	Mux mux.Backend
}

func Collect(root string, opts Options) ([]*Feature, error) {
	if opts.Mux == nil {
		opts.Mux = tmux.New()
	}

	cfg := opts.Config
	allWorkers := opts.Workers
	if cfg == nil {
		ctx, err := workspacectx.Load(root)
		if err != nil {
			return nil, err
		}
		cfg = ctx.Config
		allWorkers = ctx.Workers
	}
	activeSessions := map[string]bool{}
	if opts.Mux.Available() {
		for _, name := range opts.Mux.ListSessions() {
			activeSessions[name] = true
		}
	}

	featuresDir := filepath.Join(root, "features")
	var out []*Feature
	if err := collectDir(root, featuresDir, false, cfg, allWorkers, activeSessions, opts.Mux.Attention, &out); err != nil {
		return nil, err
	}
	if opts.IncludeArchived {
		if err := collectDir(root, filepath.Join(featuresDir, "_archive"), true, cfg, allWorkers, activeSessions, opts.Mux.Attention, &out); err != nil {
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
				HasIssues:  true,
				LoadError:  err,
			})
			continue
		}

		workflow := resolveWorkflow(cfg, s)
		workerID := resolveWorkerID(cfg, workflow, s)
		target, configured := s.Runtime.MuxTarget(s.Stage.Name)
		tmuxLive := configured && activeSessions[target.Workspace]
		attention := ""
		if tmuxLive && windowAttention != nil {
			attention = windowAttention(target.Workspace, target.Tab)
		}
		worker := workers.FindByID(allWorkers, workerID)
		engine := ""
		if worker != nil {
			engine = worker.Engine
		}
		*out = append(*out, &Feature{
			State:             s,
			FeatureDir:        featureDir,
			Archived:          archived,
			Workflow:          workflow,
			WorkflowLabel:     workflowDisplayName(cfg, workflow),
			Stage:             resolveStage(cfg, s.Stage.Name),
			StageLabel:        stageDisplayName(cfg, s.Stage.Name),
			StageLoopLabel:    loopCountSuffix(cfg, workflow, s.Stage.Name, s),
			RequiredArtifacts: requiredArtifacts(cfg, workflow, s.Stage.Name),
			WorkerID:          workerID,
			WorkerName:        resolveWorkerName(allWorkers, workerID),
			Engine:            engine,
			TmuxLive:          tmuxLive,
			Attention:         attention,
			HasIssues:         workerID == "",
		})
	}
	return nil
}

func workflowDisplayName(cfg *config.Config, workflow string) string {
	if cfg == nil {
		return workflow
	}
	return cfg.WorkflowDisplayName(workflow)
}

func resolveStage(cfg *config.Config, stage string) string {
	if cfg == nil {
		return stage
	}
	return cfg.ResolveStage(stage)
}

func stageDisplayName(cfg *config.Config, stage string) string {
	if cfg == nil {
		return stage
	}
	return cfg.StageDisplayName(cfg.ResolveStage(stage))
}

func requiredArtifacts(cfg *config.Config, workflow, stage string) []string {
	if cfg == nil {
		return nil
	}
	stageConfig, ok := cfg.StageConfig(workflow, stage)
	if !ok {
		return nil
	}
	return append([]string(nil), stageConfig.RequiredArtifacts...)
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
