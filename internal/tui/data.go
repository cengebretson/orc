package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
	tea "github.com/charmbracelet/bubbletea"
)

func loadData(root string) tea.Cmd {
	return func() tea.Msg {
		features := collectFeatures(root)
		report := doctor.Run(root)

		// build workflow chains from workflows.yaml
		workflowCfg, _ := config.Load(root)
		var chains []workflowChain
		for _, wfName := range workflowCfg.Names() {
			stages := workflowCfg.StageNames(wfName)
			var steps []routeStep
			inThisChain := map[string]bool{}
			for _, stageName := range stages {
				sc, _ := workflowCfg.StageConfig(wfName, stageName)
				steps = append(steps, routeStep{name: stageName, advance: sc.Advance, workerID: sc.Worker})
				inThisChain[stageName] = true
			}
			// loop stages — derived from Loop blocks on pipeline stages
			var loops []repairLoop
			var repairs []repairStep
			for _, sc := range workflowCfg.Stages(wfName) {
				if sc.Loop == nil || !inThisChain[sc.Name] {
					continue
				}
				loops = append(loops, repairLoop{name: sc.Loop.Via, target: sc.Name})
				repairs = append(repairs, repairStep{
					name:       sc.Loop.Via,
					workerID:   sc.Loop.Worker,
					repairs:    sc.Name,
					maxRetries: sc.Loop.Max,
				})
			}
			chains = append(chains, workflowChain{name: wfName, description: workflowCfg.WorkflowDescription(wfName), steps: steps, loops: loops, repairSteps: repairs})
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
		allWorkers, _ := workers.Load(filepath.Join(root, "workers"))
		var workerNames []string
		for _, w := range allWorkers {
			name := w.Name
			if name == "" {
				name = w.ID
			}
			workerNames = append(workerNames, name)
		}

		var repos []config.Repo
		if cfg, err := config.Load(root); err == nil {
			repos = cfg.Repos
		}

		// section items for navigable file viewer
		si := map[string][]sectionItem{}

		// workflows: one entry per workflow chain; path="" signals detail view
		for _, c := range chains {
			si["workflows"] = append(si["workflows"], sectionItem{label: c.name, path: ""})
		}

		// workers: actual namespaced .md files in workers/<namespace>/
		for _, w := range allWorkers {
			if w.FilePath == "" {
				continue
			}
			label := w.ID
			if w.Name != "" {
				label = w.Name
			}
			si["workers"] = append(si["workers"], sectionItem{label: label, path: w.FilePath})
		}

		// routes: ROUTER.md as a single item
		routerPath := filepath.Join(root, "ROUTER.md")
		if _, err := os.Stat(routerPath); err == nil {
			si["routes"] = []sectionItem{{label: "ROUTER.md", path: routerPath}}
		}

		return dataMsg{
			features:        features,
			healthItems:     report.Checks,
			workerNames:     workerNames,
			allWorkers:      allWorkers,
			workflows:       chains,
			repos:           repos,
			sectionItems:    si,
			refreshInterval: workflowCfg.TuiRefreshInterval(),
			quotes:          workflowCfg.Settings.Quotes,
		}
	}
}

func collectFeatures(root string) []*featureRow {
	features, _ := featurelist.Collect(root, featurelist.Options{IncludeArchived: true})
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
		rows = append(rows, &featureRow{
			s:              f.State,
			featureDir:     f.FeatureDir,
			workflow:       f.Workflow,
			stageLoopLabel: f.StageLoopLabel,
			workerName:     f.WorkerName,
			tmuxLive:       f.TmuxLive,
			hasIssues:      f.HasIssues,
		})
	}
	return rows
}

// buildFileList collects the files shown in a feature's detail view: the
// top-level context docs followed by each stage's output files. Stage outputs
// are discovered by scanning the feature dir's subfolders rather than assuming
// fixed stage names — each stage writes to a subfolder matching its name, which
// is policy in orc.yaml, not something the TUI should hardcode. Subfolders are
// ordered by the ticket's own stage history (pipeline order), with any
// remaining folders appended alphabetically.
func buildFileList(featureDir string, s *state.State) []detailFile {
	topLevel := []detailFile{
		{"TICKET.md", filepath.Join(featureDir, "TICKET.md")},
		{"SPEC.md", filepath.Join(featureDir, "SPEC.md")},
		{"PLAN.md", filepath.Join(featureDir, "PLAN.md")},
		{"DECISIONS.md", filepath.Join(featureDir, "DECISIONS.md")},
	}
	core := map[string]bool{"TICKET.md": true, "SPEC.md": true, "PLAN.md": true}
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
