package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/artifactcheck"
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticketview"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
	"github.com/cengebretson/orc/internal/worktreecheck"
)

type Status int

const (
	OK      Status = iota
	Warning        // non-blocking issue
	Fail           // blocking problem
)

func (s Status) String() string {
	switch s {
	case OK:
		return "✓"
	case Warning:
		return "⚠"
	default:
		return "✗"
	}
}

type Check struct {
	Name   string
	Status Status
	Detail string
}

type Report struct {
	Ticket     string
	FeatureDir string
	Checks     []Check
}

func (r *Report) OK() bool {
	for _, c := range r.Checks {
		if c.Status == Fail {
			return false
		}
	}
	return true
}

func ok(name string) Check      { return Check{Name: name, Status: OK} }
func okd(name, d string) Check  { return Check{Name: name, Status: OK, Detail: d} }
func warn(name, d string) Check { return Check{Name: name, Status: Warning, Detail: d} }
func fail(name, d string) Check { return Check{Name: name, Status: Fail, Detail: d} }

var validStatuses = map[string]bool{
	"pending":  true,
	"ready":    true,
	"active":   true,
	"paused":   true,
	"done":     true,
	"archived": true,
}

// Run validates a ticket's STATE.yaml against the workspace.
func Run(root, featureDir string) *Report {
	r := &Report{FeatureDir: featureDir}

	s, err := state.Load(featureDir)
	if err != nil {
		r.Checks = append(r.Checks, fail("STATE.yaml", fmt.Sprintf("cannot load: %v", err)))
		return r
	}
	r.Ticket = s.Ticket
	r.Checks = append(r.Checks, ok("STATE.yaml"))
	summary := ticketview.Build(root, featureDir, s, ticketview.Options{})
	appendStateShapeChecks(r, s)
	appendFeatureSchemaChecks(r, featureDir, s)

	ctx, validationErrs, err := workspacectx.LoadValidated(root)
	if err != nil {
		r.Checks = append(r.Checks, fail("workspace", fmt.Sprintf("cannot load: %v", err)))
		return r
	}
	r.Checks = append(r.Checks, ok("orc.yaml"))
	if len(validationErrs) > 0 {
		for _, err := range validationErrs {
			r.Checks = append(r.Checks, fail("orc.yaml."+err.Path, err.Message))
		}
		return r
	}
	r.Checks = append(r.Checks, okd("orc.yaml.config", "valid"))
	cfg := ctx.Config
	allWorkers := ctx.Workers

	// Resolve workflow name and verify it exists in orc.yaml.
	workflow := s.Workflow
	if workflow == "" {
		workflow = cfg.DefaultWorkflow()
	}
	if workflow == "" {
		known := cfg.Names()
		detail := "no default_workflow set in orc.yaml"
		if len(known) > 0 {
			detail += fmt.Sprintf(" (available: %s)", strings.Join(known, ", "))
		}
		r.Checks = append(r.Checks, fail("STATE.yaml.workflow", detail))
		return r
	}
	if _, ok := cfg.Workflows[workflow]; !ok {
		known := cfg.Names()
		detail := fmt.Sprintf("%q not found in orc.yaml", workflow)
		if len(known) > 0 {
			detail += fmt.Sprintf(" (available: %s)", strings.Join(known, ", "))
		}
		r.Checks = append(r.Checks, fail("STATE.yaml.workflow", detail))
		return r
	}
	r.Checks = append(r.Checks, okd("STATE.yaml.workflow", workflow))
	stageNames := cfg.StageNames(workflow)

	// Current stage exists in the workflow.
	stageName := s.Stage.Name
	stageInWorkflow := false
	for _, sn := range stageNames {
		if sn == stageName {
			stageInWorkflow = true
			break
		}
	}
	// Also check loop stages.
	if !stageInWorkflow {
		if cfg.IsLoopStage(workflow, stageName) {
			stageInWorkflow = true
		}
	}
	if !stageInWorkflow && len(stageNames) > 0 {
		r.Checks = append(r.Checks, fail("STATE.yaml.stage.name", fmt.Sprintf("%q not found in %q pipeline", stageName, workflow)))
	} else if stageInWorkflow {
		r.Checks = append(r.Checks, okd("STATE.yaml.stage.name", stageName))
	} else if stageName != "" {
		r.Checks = append(r.Checks, warn("STATE.yaml.stage.name", fmt.Sprintf("workflow %q has no stages defined", workflow)))
	}

	// Stage markdown file exists.
	stagePath := config.ResourcePath(stageName)
	stageMD := filepath.Join(root, "stages", stagePath)
	if _, err := os.Stat(stageMD); err != nil {
		r.Checks = append(r.Checks, fail("stage file", fmt.Sprintf("stages/%s missing", stagePath)))
	} else {
		r.Checks = append(r.Checks, okd("stage file", fmt.Sprintf("stages/%s", stagePath)))
	}

	// Worker exists.
	sc, _ := cfg.StageConfig(workflow, stageName)
	workerID := s.Stage.Worker
	if workerID == "" {
		workerID = sc.Worker
	}
	if workerID != "" {
		if workers.FindByID(allWorkers, workerID) == nil {
			r.Checks = append(r.Checks, fail("STATE.yaml.stage.worker", fmt.Sprintf("%q not found in workers/", workerID)))
		} else {
			r.Checks = append(r.Checks, okd("STATE.yaml.stage.worker", workerID))
		}
	} else {
		r.Checks = append(r.Checks, fail("STATE.yaml.stage.worker", "no worker assigned for this stage"))
	}

	if s.NextAction.Worker != "" && s.NextAction.Worker != "human" {
		if workers.FindByID(allWorkers, s.NextAction.Worker) == nil {
			r.Checks = append(r.Checks, fail("STATE.yaml.next_action.worker", fmt.Sprintf("%q not found in workers/", s.NextAction.Worker)))
		} else {
			r.Checks = append(r.Checks, okd("STATE.yaml.next_action.worker", s.NextAction.Worker))
		}
	}
	appendRequiredArtifactChecks(r, featureDir, sc.RequiredArtifacts)
	if err := state.ValidateRepos(s, root); err != nil {
		appendRepoValidationChecks(r, err)
	} else if len(s.Repos) > 0 {
		r.Checks = append(r.Checks, ok("STATE.yaml.repos"))
		if s.NextAction.CWD != "" {
			r.Checks = append(r.Checks, okd("STATE.yaml.next_action.cwd", s.ResolveCWD(root)))
		}
	} else if s.NextAction.CWD != "" {
		r.Checks = append(r.Checks, okd("STATE.yaml.next_action.cwd", s.ResolveCWD(root)))
	}

	// Repo worktrees exist.
	if len(s.Repos) > 0 {
		var missing []string
		for name, repo := range s.Repos {
			if repo.Worktree == "" {
				continue
			}
			p := repo.Worktree
			if !filepath.IsAbs(p) {
				p = filepath.Join(root, p)
			}
			if _, err := os.Stat(p); err != nil {
				missing = append(missing, fmt.Sprintf("%s (%s)", name, repo.Worktree))
			}
		}
		if len(missing) > 0 {
			for _, m := range missing {
				r.Checks = append(r.Checks, fail("STATE.yaml.repos.worktree", "not found: "+m))
			}
		} else {
			r.Checks = append(r.Checks, ok("STATE.yaml.repos.worktrees"))
		}
	}
	appendWorktreeReconciliationChecks(r, root, s)

	// Stage output folder exists (warn if missing — agent may not have written it yet).
	stageOutputDir := filepath.Join(featureDir, stageName)
	if _, err := os.Stat(stageOutputDir); err != nil {
		r.Checks = append(r.Checks, warn("stage outputs", fmt.Sprintf("%s/ not yet written", stageName)))
	} else {
		entries, _ := os.ReadDir(stageOutputDir)
		r.Checks = append(r.Checks, okd("stage outputs", fmt.Sprintf("%s/ (%d file(s))", stageName, len(entries))))
	}

	// Tmux session alive (if configured).
	if summary.TmuxConfigured {
		if summary.TmuxAvailable {
			if summary.TmuxLive {
				r.Checks = append(r.Checks, okd("tmux", fmt.Sprintf("session %q active", summary.TmuxSession)))
			} else {
				r.Checks = append(r.Checks, warn("tmux", fmt.Sprintf("session %q configured but not running", summary.TmuxSession)))
			}
		} else {
			r.Checks = append(r.Checks, warn("tmux", "configured but tmux not available"))
		}
	}

	return r
}

func appendFeatureSchemaChecks(r *Report, featureDir string, s *state.State) {
	if s.Slug != "" {
		if got := filepath.Base(featureDir); got != s.Slug {
			r.Checks = append(r.Checks, warn("feature folder", fmt.Sprintf("folder name %q does not match STATE.yaml.slug %q", got, s.Slug)))
		} else {
			r.Checks = append(r.Checks, okd("feature folder", s.Slug))
		}
	}

	issues := artifactcheck.Check(featureDir, artifactcheck.CoreDocs)
	issueByPath := map[string]artifactcheck.Issue{}
	for _, issue := range issues {
		issueByPath[issue.Path] = issue
	}
	for _, name := range artifactcheck.CoreDocs {
		if issue, found := issueByPath[name]; found {
			r.Checks = append(r.Checks, warn("feature."+name, strings.TrimPrefix(issue.Detail(), name+" ")))
		} else {
			r.Checks = append(r.Checks, ok("feature."+name))
		}
	}
}

func appendRequiredArtifactChecks(r *Report, featureDir string, artifacts []string) {
	if len(artifacts) == 0 {
		return
	}
	issues := artifactcheck.Check(featureDir, artifacts)
	issueByPath := map[string]artifactcheck.Issue{}
	for _, issue := range issues {
		issueByPath[issue.Path] = issue
	}
	for _, artifact := range artifacts {
		if issue, found := issueByPath[artifact]; found {
			r.Checks = append(r.Checks, warn("required artifact", issue.Detail()))
		} else {
			r.Checks = append(r.Checks, okd("required artifact", artifact))
		}
	}
}

func appendWorktreeReconciliationChecks(r *Report, root string, s *state.State) {
	for _, finding := range worktreecheck.Reconcile(root, s) {
		status := Warning
		if finding.Severity == worktreecheck.Fail {
			status = Fail
		}
		r.Checks = append(r.Checks, Check{
			Name:   "STATE.yaml." + finding.Field,
			Status: status,
			Detail: finding.Message,
		})
	}
}

func appendStateShapeChecks(r *Report, s *state.State) {
	if s.SchemaVersion > state.SchemaVersion {
		r.Checks = append(r.Checks, fail("STATE.yaml.schema_version", fmt.Sprintf("unsupported schema version %d", s.SchemaVersion)))
	} else {
		r.Checks = append(r.Checks, okd("STATE.yaml.schema_version", fmt.Sprintf("v%d", s.SchemaVersion)))
	}
	if s.Ticket == "" {
		r.Checks = append(r.Checks, fail("STATE.yaml.ticket", "ticket is required"))
	} else {
		r.Checks = append(r.Checks, okd("STATE.yaml.ticket", s.Ticket))
	}
	if s.Slug == "" {
		r.Checks = append(r.Checks, fail("STATE.yaml.slug", "slug is required"))
	} else {
		r.Checks = append(r.Checks, okd("STATE.yaml.slug", s.Slug))
	}
	if s.Status == "" {
		r.Checks = append(r.Checks, fail("STATE.yaml.status", "status is required"))
	} else if !validStatuses[s.Status] {
		r.Checks = append(r.Checks, fail("STATE.yaml.status", fmt.Sprintf("%q is not a valid status", s.Status)))
	} else {
		r.Checks = append(r.Checks, okd("STATE.yaml.status", s.Status))
	}
	if s.Stage.Name == "" {
		r.Checks = append(r.Checks, fail("STATE.yaml.stage.name", "stage name is required"))
	}
}

func appendRepoValidationChecks(r *Report, err error) {
	var repoErrs state.RepoValidationErrors
	if errors.As(err, &repoErrs) {
		for _, e := range repoErrs {
			r.Checks = append(r.Checks, fail("STATE.yaml."+e.Field, e.Message))
		}
		return
	}
	r.Checks = append(r.Checks, fail("STATE.yaml.repos", err.Error()))
}

// Print renders the validation report to stdout.
func Print(r *Report) {
	fmt.Printf("Ticket: %s\n\n", r.Ticket)
	for _, c := range r.Checks {
		if c.Detail != "" {
			fmt.Printf("  %s  %-20s %s\n", c.Status, c.Name, c.Detail)
		} else {
			fmt.Printf("  %s  %s\n", c.Status, c.Name)
		}
	}
	fmt.Println()
	if r.OK() {
		fmt.Println("All checks passed.")
	} else {
		fmt.Println("Some checks failed — ticket may not advance cleanly.")
	}
}
