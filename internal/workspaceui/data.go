package workspaceui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacesnapshot"
	tea "github.com/charmbracelet/bubbletea"
)

func loadData(root string) tea.Cmd {
	return func() tea.Msg {
		snapshot, err := workspacesnapshot.Load(root)
		if err != nil {
			return dataMsg{err: err}
		}
		features := collectFeatures(snapshot)

		// build workflow chains from workflows.yaml
		workflowCfg := snapshot.Config
		var chains []workflowChain
		for _, wfName := range workflowCfg.Names() {
			stages := workflowCfg.StageNames(wfName)
			var steps []routeStep
			inThisChain := map[string]bool{}
			for _, stageName := range stages {
				sc, _ := workflowCfg.StageConfig(wfName, stageName)
				steps = append(steps, routeStep{
					name:              stageName,
					label:             workflowCfg.StageDisplayName(stageName),
					advance:           sc.Advance,
					workerID:          sc.Worker,
					requiredArtifacts: sc.RequiredArtifacts,
				})
				inThisChain[stageName] = true
			}
			// loop stages — derived from Loop blocks on pipeline stages
			var loops []repairLoop
			var repairs []repairStep
			for _, sc := range workflowCfg.Stages(wfName) {
				if sc.Loop == nil || !inThisChain[sc.Name] {
					continue
				}
				loops = append(loops, repairLoop{
					name:   sc.Loop.Via,
					label:  workflowCfg.StageDisplayName(sc.Loop.Via),
					target: sc.Name,
				})
				repairs = append(repairs, repairStep{
					name:              sc.Loop.Via,
					label:             workflowCfg.StageDisplayName(sc.Loop.Via),
					workerID:          sc.Loop.Worker,
					repairs:           sc.Name,
					repairsLabel:      workflowCfg.StageDisplayName(sc.Name),
					maxRetries:        sc.Loop.Max,
					requiredArtifacts: sc.Loop.RequiredArtifacts,
				})
			}
			chains = append(chains, workflowChain{
				name:        wfName,
				label:       workflowCfg.WorkflowDisplayName(wfName),
				description: workflowCfg.WorkflowDescription(wfName),
				steps:       steps,
				loops:       loops,
				repairSteps: repairs,
			})
		}
		// fallback: flat list of all stage files
		if len(chains) == 0 {
			stagesDir := filepath.Join(root, "stages")
			entries, _ := os.ReadDir(stagesDir)
			var steps []routeStep
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					steps = append(steps, routeStep{name: strings.TrimSuffix(e.Name(), ".md")})
				}
			}
			chains = []workflowChain{{name: "", steps: steps}}
		}
		// worker names
		allWorkers := snapshot.Workers
		var workerNames []string
		workerGroupsByName := map[string][]sectionItem{}
		for _, w := range allWorkers {
			item := sectionItem{label: workerDisplayName(workflowCfg, w), id: w.ID, path: w.FilePath}
			workerNames = append(workerNames, item.label)
			if w.FilePath != "" {
				workerGroupsByName[workerNamespace(w.ID)] = append(workerGroupsByName[workerNamespace(w.ID)], item)
			}
		}
		var workerGroups []workerGroup
		for _, name := range sortedKeys(workerGroupsByName) {
			workerGroups = append(workerGroups, workerGroup{name: name, items: workerGroupsByName[name]})
		}

		// section items for navigable file viewer
		si := map[sectionID][]sectionItem{}

		// workflows: one entry per workflow chain; path="" signals detail view
		workflowGroupsByName := map[string][]sectionItem{}
		for _, c := range chains {
			item := sectionItem{label: chainLabel(c), id: c.name, path: ""}
			si[sectionWorkflows] = append(si[sectionWorkflows], item)
			workflowGroupsByName[workflowNamespace(c.name)] = append(workflowGroupsByName[workflowNamespace(c.name)], item)
		}
		var workflowGroups []workflowGroup
		for _, name := range sortedKeys(workflowGroupsByName) {
			workflowGroups = append(workflowGroups, workflowGroup{name: name, items: workflowGroupsByName[name]})
		}

		// workers: actual namespaced .md files in workers/<namespace>/
		for _, group := range workerGroups {
			si[sectionWorkers] = append(si[sectionWorkers], group.items...)
		}

		repos := snapshot.Config.Repos
		routes := snapshot.Config.Routing

		// routes: repository routing now lives in orc.yaml
		configPath := filepath.Join(root, config.Filename)
		if _, err := os.Stat(configPath); err == nil {
			si[sectionRepositories] = []sectionItem{{label: "repository configuration", path: configPath}}
		}

		return dataMsg{
			features:        features,
			healthItems:     snapshot.Health,
			workerNames:     workerNames,
			workerGroups:    workerGroups,
			workflowGroups:  workflowGroups,
			allWorkers:      allWorkers,
			workflows:       chains,
			repos:           repos,
			routes:          routes,
			sectionItems:    si,
			refreshInterval: workflowCfg.WorkspaceRefreshInterval(),
			artifactPolicy:  workflowCfg.ArtifactPolicy(),
			quotes:          workflowCfg.Settings.Quotes,
		}
	}
}

func collectFeatures(snapshot *workspacesnapshot.Snapshot) []*featureRow {
	workflowCfg := snapshot.Config
	features := snapshot.Features
	liveByFeature := snapshot.Telemetry
	thresholds := workflowCfg.ContextPressureThresholds()
	rows := make([]*featureRow, 0, len(features))
	for _, f := range features {
		if f.LoadError != nil {
			// Surface broken tickets instead of hiding them — a row with no
			// parsed state renders as a "broken" entry the user can act on.
			rows = append(rows, &featureRow{
				featureDir: f.FeatureDir,
				loadErr:    f.LoadError,
				hasIssues:  true,
			})
			continue
		}
		context := contextpressure.Pressure{}
		if live, ok := liveByFeature[filepath.Clean(f.FeatureDir)]; ok {
			context = contextpressure.Evaluate(live.ContextUsed, live.ContextLimit, thresholds)
		}
		rows = append(rows, &featureRow{
			s:                 f.State,
			featureDir:        f.FeatureDir,
			workflow:          f.Workflow,
			stage:             workflowCfg.ResolveStage(f.State.Stage.Name),
			workflowLabel:     workflowDisplayName(workflowCfg, f.Workflow),
			stageLabel:        stageDisplayName(workflowCfg, f.State.Stage.Name),
			stageLoopLabel:    f.StageLoopLabel,
			workerID:          f.WorkerID,
			workerName:        f.WorkerName,
			engine:            f.Engine,
			attention:         f.Attention,
			context:           context,
			tmuxLive:          f.TmuxLive,
			hasIssues:         f.HasIssues,
			requiredArtifacts: currentStageArtifacts(workflowCfg, f.Workflow, f.State.Stage.Name),
		})
	}
	return rows
}

func currentStageArtifacts(cfg *config.Config, workflowName, stageName string) []string {
	if cfg == nil {
		return nil
	}
	sc, ok := cfg.StageConfig(workflowName, stageName)
	if !ok {
		return nil
	}
	return sc.RequiredArtifacts
}

func workflowDisplayName(cfg *config.Config, id string) string {
	if cfg == nil {
		return id
	}
	return cfg.WorkflowDisplayName(cfg.ResolveWorkflow(id))
}

func stageDisplayName(cfg *config.Config, id string) string {
	if cfg == nil {
		return id
	}
	return cfg.StageDisplayName(cfg.ResolveStage(id))
}

func workerDisplayName(cfg *config.Config, w *workers.Worker) string {
	if w == nil {
		return ""
	}
	if cfg != nil {
		if alias := cfg.WorkerDisplayName(w.ID); alias != "" && alias != w.ID {
			return alias
		}
	}
	if w.Name != "" {
		return w.Name
	}
	return w.ID
}

func workerNamespace(id string) string {
	if before, _, ok := strings.Cut(id, ":"); ok && before != "" {
		return before
	}
	return "local"
}

func workflowNamespace(id string) string {
	if before, _, ok := strings.Cut(id, ":"); ok && before != "" {
		return before
	}
	return "local"
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func chainLabel(c workflowChain) string {
	if c.label != "" {
		return c.label
	}
	return c.name
}

func workflowLabel(name string, chains []workflowChain) string {
	for _, c := range chains {
		if c.name == name {
			return chainLabel(c)
		}
	}
	return name
}

func workflowDisplayWithID(name string, chains []workflowChain) string {
	return labelWithDimID(workflowLabel(name, chains), name)
}

func workflowStageLabel(name string, chains []workflowChain) string {
	for _, c := range chains {
		for _, s := range c.steps {
			if s.name == name {
				return stepLabel(s)
			}
		}
		for _, s := range c.repairSteps {
			if s.name == name {
				return repairStepLabel(s)
			}
		}
	}
	return name
}

func stepLabel(s routeStep) string {
	if s.label != "" {
		return s.label
	}
	return s.name
}

func repairStepLabel(s repairStep) string {
	if s.label != "" {
		return s.label
	}
	return s.name
}

func repairTargetLabel(s repairStep) string {
	if s.repairsLabel != "" {
		return s.repairsLabel
	}
	return s.repairs
}

// buildFileList collects the files shown in a feature's detail view: the
// top-level context docs followed by each stage's output files. Stage outputs
// are discovered by scanning the feature dir's subfolders rather than assuming
// fixed stage names — each stage writes to a subfolder matching its name, which
// is policy in orc.yaml, not something the Workspace view should hardcode. Subfolders are
// ordered by the ticket's own stage history (pipeline order), with any
// remaining folders appended alphabetically.
func buildFileList(featureDir string, s *state.State) []detailFile {
	topLevel := []detailFile{
		{"TICKET.md", filepath.Join(featureDir, "TICKET.md")},
		{"SPEC.md", filepath.Join(featureDir, "SPEC.md")},
		{"PLAN.md", filepath.Join(featureDir, "PLAN.md")},
		{"DECISIONS.md", filepath.Join(featureDir, "DECISIONS.md")},
	}
	core := map[string]bool{"TICKET.md": true, "SPEC.md": true, "PLAN.md": true, "DECISIONS.md": true}
	var out []detailFile
	for _, f := range topLevel {
		if fileExists(f.path) || core[f.label] {
			out = append(out, f)
		}
	}

	for _, dir := range orderedStageDirs(featureDir, s) {
		matches, _ := filepath.Glob(filepath.Join(featureDir, dir, "*.md"))
		sort.Strings(matches)
		for _, p := range matches {
			out = append(out, detailFile{label: dir + "/" + filepath.Base(p), path: p})
		}
	}
	return out
}

// orderedStageDirs returns the feature dir's stage subfolders in pipeline order:
// those the ticket has visited (per history, then the current stage) first, then
// any other present folders alphabetically. Hidden and `_`-prefixed folders are
// skipped.
func orderedStageDirs(featureDir string, s *state.State) []string {
	present := map[string]bool{}
	if entries, err := os.ReadDir(featureDir); err == nil {
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() && !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_") {
				present[name] = true
			}
		}
	}

	var ordered []string
	seen := map[string]bool{}
	add := func(name string) {
		if present[name] && !seen[name] {
			ordered = append(ordered, name)
			seen[name] = true
		}
	}
	if s != nil {
		for _, h := range s.History {
			add(h.Stage)
		}
		add(s.Stage.Name)
	}

	var rest []string
	for name := range present {
		if !seen[name] {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}
