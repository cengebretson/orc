package health

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
)

type Status int

const (
	OK      Status = iota
	Missing        // expected but not found
	Empty          // exists but has no content worth noting
)

func (s Status) String() string {
	switch s {
	case OK:
		return "✓"
	case Empty:
		return "⚠"
	default:
		return "✗"
	}
}

type Result struct {
	Name   string
	Status Status
	Detail string
	Group  string // section header — printed once when the group changes
}

type Report struct {
	Root    string
	Results []Result
}

func (r *Report) OK() bool {
	for _, res := range r.Results {
		if res.Status == Missing {
			return false
		}
	}
	return true
}

// Run checks the workspace filesystem state at root.
func Run(root string) *Report {
	report := &Report{Root: root}

	// root-level docs
	for _, f := range []string{"AGENTS.md", "TOOLS.md", "RULES.md", "ROUTER.md"} {
		report.Results = append(report.Results, checkFile(root, f))
	}

	// setup completion
	report.Results = append(report.Results, checkSetup(root))

	// features/
	report.Results = append(report.Results, checkFeatures(root))

	// workers/
	report.Results = append(report.Results, checkDirWithCount(root, "workers", "*.md", "worker"))

	// optional dirs
	report.Results = append(report.Results, checkOptionalDir(root, "worktrees"))

	// orc.yaml (repos + workflows) — grouped under their own section
	for _, r := range []Result{
		checkOrcConfig(root),
		checkRepoPaths(root),
		checkWorkflowRefs(root),
		checkDirWithCount(root, "stages", "*.md", "stage"),
	} {
		r.Group = "orc.yaml"
		report.Results = append(report.Results, r)
	}

	// loop counts — warn when a ticket is approaching its loop max
	if r := checkLoopCounts(root); r.Name != "" {
		r.Group = "orc.yaml"
		report.Results = append(report.Results, r)
	}

	return report
}

// Print renders the report to stdout.
func Print(r *Report) {
	fmt.Printf("Workspace: %s\n\n", r.Root)
	var currentGroup string
	for _, res := range r.Results {
		if res.Group != currentGroup {
			currentGroup = res.Group
			if currentGroup != "" {
				fmt.Printf("\n  %s\n", currentGroup)
			}
		}
		indent := "  "
		if res.Group != "" {
			indent = "    "
		}
		if res.Detail != "" {
			fmt.Printf("%s%s  %-20s %s\n", indent, res.Status, res.Name, res.Detail)
		} else {
			fmt.Printf("%s%s  %s\n", indent, res.Status, res.Name)
		}
	}
}

func checkSetup(root string) Result {
	path := filepath.Join(root, "SETUP.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{Name: "SETUP.md", Status: Missing, Detail: "missing — run `orc init`"}
	}
	content := string(data)

	shared := setupSectionComplete(content, "shared")
	claude := setupSectionComplete(content, "claude")
	codex := setupSectionComplete(content, "codex")

	if shared && (claude || codex) {
		var done []string
		if claude {
			done = append(done, "claude")
		}
		if codex {
			done = append(done, "codex")
		}
		return Result{Name: "SETUP.md", Status: OK, Detail: "complete (" + strings.Join(done, ", ") + ")"}
	}

	var pending []string
	if !shared {
		pending = append(pending, "shared")
	}
	if !claude {
		pending = append(pending, "claude")
	}
	if !codex {
		pending = append(pending, "codex")
	}
	return Result{Name: "SETUP.md", Status: Empty, Detail: "pending: " + strings.Join(pending, ", ") + " — run an agent on SETUP.md"}
}

// setupSectionComplete reports whether the SETUP.md Status block marks the given
// section complete. The match is anchored to the start of a line and tolerant of
// the whitespace after the colon, so it ignores the literal "<key>: complete"
// strings that appear inside the instruction prose (which are not line-leading)
// and does not depend on the template's cosmetic column alignment.
func setupSectionComplete(content, key string) bool {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:[ \t]*complete\b`)
	return re.MatchString(content)
}

func checkFile(root, name string) Result {
	path := filepath.Join(root, name)
	if _, err := os.Stat(path); err == nil {
		return Result{Name: name, Status: OK}
	}
	return Result{Name: name, Status: Missing, Detail: "missing — run `orc init`"}
}

func checkFeatures(root string) Result {
	path := filepath.Join(root, "features")
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return Result{Name: "features/", Status: Missing, Detail: "missing — run `orc init`"}
	}
	entries, _ := os.ReadDir(path)
	var active, archived int
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_template" {
			continue
		}
		if e.Name() == "_archive" {
			archiveEntries, _ := os.ReadDir(filepath.Join(path, "_archive"))
			for _, a := range archiveEntries {
				if a.IsDir() {
					archived++
				}
			}
			continue
		}
		active++
	}

	if active == 0 && archived == 0 {
		return Result{Name: "features/", Status: Empty, Detail: "no features yet — start one with `orc work <ticket>`"}
	}

	detail := fmt.Sprintf("%d active", active)
	if archived > 0 {
		detail += fmt.Sprintf(", %d archived", archived)
	}
	return Result{Name: "features/", Status: OK, Detail: detail}
}

func checkOptionalDir(root, name string) Result {
	path := filepath.Join(root, name)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return Result{Name: name + "/", Status: OK}
	}
	return Result{Name: name + "/", Status: Empty, Detail: "not created yet"}
}

func checkOrcConfig(root string) Result {
	cfg, err := config.Load(root)
	if err != nil {
		return Result{Name: config.Filename, Status: Missing, Detail: "missing — run `orc init`"}
	}
	if len(cfg.Repos) == 0 {
		return Result{Name: config.Filename, Status: Empty, Detail: "no repos defined — edit orc.yaml"}
	}
	repoNames := make([]string, len(cfg.Repos))
	for i, r := range cfg.Repos {
		repoNames[i] = r.Name
	}
	wfCfg, _ := config.Load(root)
	wfNames := wfCfg.Names()
	detail := fmt.Sprintf("%d repo(s): %s", len(repoNames), strings.Join(repoNames, ", "))
	if len(wfNames) > 0 {
		detail += fmt.Sprintf("  ·  %d workflow(s): %s", len(wfNames), strings.Join(wfNames, ", "))
	} else {
		detail += "  ·  no workflows defined"
	}
	return Result{Name: config.Filename, Status: OK, Detail: detail}
}

func checkDirWithCount(root, dir, pattern, label string) Result {
	path := filepath.Join(root, dir)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return Result{Name: dir + "/", Status: Missing, Detail: "missing — run `orc init`"}
	}

	var matches []string
	_ = filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil || !matched || filepath.Base(path) == "_template.md" {
			return err
		}
		matches = append(matches, path)
		return nil
	})

	switch len(matches) {
	case 0:
		return Result{Name: dir + "/", Status: Empty, Detail: fmt.Sprintf("no %ss defined", label)}
	case 1:
		return Result{Name: dir + "/", Status: OK, Detail: fmt.Sprintf("1 %s", label)}
	default:
		return Result{Name: dir + "/", Status: OK, Detail: fmt.Sprintf("%d %ss", len(matches), label)}
	}
}

func checkRepoPaths(root string) Result {
	cfg, err := config.Load(root)
	if err != nil || len(cfg.Repos) == 0 {
		return Result{Name: "repo paths", Status: Empty, Detail: "no repos to check"}
	}
	var missing []string
	for _, r := range cfg.Repos {
		p := r.Path
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		if _, err := os.Stat(p); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s)", r.Name, r.Path))
		}
	}
	if len(missing) > 0 {
		return Result{Name: "repo paths", Status: Missing, Detail: "not found: " + strings.Join(missing, ", ")}
	}
	return Result{Name: "repo paths", Status: OK, Detail: "all paths exist"}
}

func checkWorkflowRefs(root string) Result {
	wfCfg, err := config.Load(root)
	if err != nil || len(wfCfg.Names()) == 0 {
		return Result{Name: "workflow refs", Status: Empty, Detail: "no workflows to check"}
	}

	// collect known worker IDs by parsing frontmatter
	knownWorkers := map[string]bool{}
	allWorkers, _ := workers.Load(filepath.Join(root, "workers"))
	for _, w := range allWorkers {
		knownWorkers[w.ID] = true
	}

	stagesDir := filepath.Join(root, "stages")
	var errs []string

	for _, wfName := range wfCfg.Names() {
		for _, stageName := range wfCfg.StageNames(wfName) {
			sc, _ := wfCfg.StageConfig(wfName, stageName)

			// check stage file exists
			stageFile := config.ResourcePath(stageName)
			if _, err := os.Stat(filepath.Join(stagesDir, stageFile)); err != nil {
				errs = append(errs, fmt.Sprintf("missing stage file: %s", stageFile))
			}
			// check worker exists
			if sc.Worker != "" && !knownWorkers[sc.Worker] {
				errs = append(errs, fmt.Sprintf("unknown worker: %s (stage: %s)", sc.Worker, stageName))
			}
		}
	}
	// check loop stage refs too
	for _, wfName := range wfCfg.Names() {
		for _, sc := range wfCfg.Stages(wfName) {
			if sc.Loop == nil {
				continue
			}
			stageFile := config.ResourcePath(sc.Loop.Via)
			if _, err := os.Stat(filepath.Join(stagesDir, stageFile)); err != nil {
				errs = append(errs, fmt.Sprintf("missing stage file: %s", stageFile))
			}
			if sc.Loop.Worker != "" && !knownWorkers[sc.Loop.Worker] {
				errs = append(errs, fmt.Sprintf("unknown worker: %s (loop stage: %s)", sc.Loop.Worker, sc.Loop.Via))
			}
		}
	}

	if len(errs) > 0 {
		return Result{Name: "workflow refs", Status: Missing, Detail: strings.Join(errs, "  ·  ")}
	}
	return Result{Name: "workflow refs", Status: OK, Detail: "all workers and stages exist"}
}

// checkLoopCounts warns when any active ticket is at or near its loop stage max.
// Returns an empty Result (Name == "") when there is nothing to report.
func checkLoopCounts(root string) Result {
	cfg, err := config.Load(root)
	if err != nil {
		return Result{}
	}
	featuresDir := filepath.Join(root, "features")
	entries, err := os.ReadDir(featuresDir)
	if err != nil {
		return Result{}
	}

	var warnings []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_template" || e.Name() == "_archive" {
			continue
		}
		s, err := state.Load(filepath.Join(featuresDir, e.Name()))
		if err != nil || s.Status != "active" {
			continue
		}
		workflow := s.Workflow
		if workflow == "" {
			workflow = cfg.DefaultWorkflow()
		}
		stageName := s.Stage.Name
		if !cfg.IsLoopStage(workflow, stageName) {
			continue
		}
		owner, ok := cfg.OwnerStage(workflow, stageName)
		if !ok {
			continue
		}
		loopDef, ok := cfg.LoopConfig(workflow, owner)
		if !ok || loopDef.Max <= 0 {
			continue
		}
		count := s.StageCounts[stageName]
		if count >= loopDef.Max-1 {
			warnings = append(warnings, fmt.Sprintf("%s: %s %d/%d", s.Ticket, stageName, count, loopDef.Max))
		}
	}

	if len(warnings) == 0 {
		return Result{}
	}
	return Result{Name: "loop counts", Status: Empty, Detail: "approaching limit: " + strings.Join(warnings, "  ·  ")}
}
