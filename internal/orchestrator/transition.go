package orchestrator

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/artifactcheck"
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
)

type AdvanceOutcome string

const (
	AdvanceOutcomeAdvanced AdvanceOutcome = "advanced"
	AdvanceOutcomePaused   AdvanceOutcome = "paused"
	AdvanceOutcomeDone     AdvanceOutcome = "done"
)

type AdvanceOptions struct {
	Root       string
	FeatureDir string
	Stage      string
	Worker     string
	Result     string
	// Force is the human override for artifact_policy=block: advance even when
	// required artifacts are not ready. The override is recorded in history.
	Force bool
}

type AdvanceResult struct {
	Ticket      string
	Previous    string
	Next        string
	Worker      string
	Outcome     AdvanceOutcome
	Reason      string
	AutoArchive bool
}

func Advance(opts AdvanceOptions) (*AdvanceResult, error) {
	s, err := state.Load(opts.FeatureDir)
	if err != nil {
		return nil, err
	}

	if s.Status == "archived" || s.Status == "done" {
		return nil, fmt.Errorf("ticket %s is %s — cannot advance", s.Ticket, s.Status)
	}
	if s.Status == "pending" {
		return nil, fmt.Errorf("ticket %s is pending — run `orc next %s` or `orc mark %s start` before marking next", s.Ticket, s.Ticket, s.Ticket)
	}
	if err := state.ValidateRepos(s, opts.Root); err != nil {
		return nil, err
	}

	ctx, validationErrs, err := workspacectx.LoadValidated(opts.Root)
	if err != nil {
		return nil, err
	}
	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("invalid workspace config: %w", validationErrs)
	}
	workflowCfg := ctx.Config
	allWorkers := ctx.Workers
	if opts.Worker != "" && workers.FindByID(allWorkers, opts.Worker) == nil {
		return nil, fmt.Errorf("worker %q not found in workers/", opts.Worker)
	}

	workflow := s.Workflow
	if workflow == "" {
		workflow = workflowCfg.DefaultWorkflow()
	}
	if _, ok := workflowCfg.Workflows[workflow]; !ok {
		return nil, fmt.Errorf("workflow %q not found in orc.yaml", workflow)
	}
	prevStage := s.Stage.Name
	stageCfg, ok := workflowCfg.StageConfig(workflow, prevStage)
	if !ok {
		return nil, fmt.Errorf("current stage %q not found in workflow %q — check STATE.yaml.stage.name", prevStage, workflow)
	}

	if opts.Stage != "" {
		if _, ok := workflowCfg.StageConfig(workflow, opts.Stage); !ok {
			return nil, fmt.Errorf("stage %q not found in workflow %q — check orc.yaml", opts.Stage, workflow)
		}
	}

	nextStage := opts.Stage
	if nextStage == "" {
		nextStage = workflowCfg.NextStage(workflow, prevStage)
		if nextStage == "" {
			if owner, ok := workflowCfg.OwnerStage(workflow, prevStage); ok {
				nextStage = owner
			}
		}
	}

	if nextStage != "" && !workflowCfg.IsLoopStage(workflow, prevStage) && !workflowCfg.IsLoopStage(workflow, nextStage) {
		if stageCfg.Advance == "manual" {
			return nil, fmt.Errorf(
				"stage %q has advance: manual — use `orc mark %s pause \"<reason>\"` so a human can review before continuing",
				prevStage, s.Ticket,
			)
		}
	}

	if res, err := handleLoopLimit(opts, workflowCfg, workflow, prevStage, s); err != nil || res != nil {
		return res, err
	}

	// Checked last so agents get actionable guidance first: a manual stage says
	// "use pause", and a loop at max still pauses/fails instead of erroring here.
	forcedNote := ""
	if workflowCfg.ArtifactPolicy() == "block" {
		if issues := blockingArtifactIssues(opts.FeatureDir, opts.Root, stageCfg.RequiredArtifacts); len(issues) > 0 {
			if !opts.Force {
				return nil, fmt.Errorf(
					"artifact_policy=block: required artifacts are not ready:\n  - %s\nComplete them first, or `orc mark %s pause \"<reason>\"` for human review",
					artifactIssueDetails(issues), s.Ticket,
				)
			}
			forcedNote = "forced past artifact_policy=block: " + strings.Join(artifactIssueList(issues), ", ")
		}
	}

	result := opts.Result
	if result == "" {
		if nextStage != "" && nextStage != prevStage {
			result = fmt.Sprintf("advanced from %s to %s", prevStage, nextStage)
		} else {
			result = fmt.Sprintf("completed %s", prevStage)
		}
	}
	if forcedNote != "" {
		result += " (" + forcedNote + ")"
	}

	if err := state.Next(opts.FeatureDir, nextStage, opts.Worker, result); err != nil {
		return nil, err
	}

	out := AdvanceOutcomeAdvanced
	if nextStage == "" {
		out = AdvanceOutcomeDone
	}

	autoArchive := false
	if nextStage == "" {
		autoArchive = workflowCfg.Settings.AutoArchive
	}

	return &AdvanceResult{
		Ticket:      s.Ticket,
		Previous:    prevStage,
		Next:        nextStage,
		Worker:      opts.Worker,
		Outcome:     out,
		Reason:      forcedNote,
		AutoArchive: autoArchive,
	}, nil
}

// handleLoopLimit governs an explicit `--stage` jump into a loop stage. It
// rejects a jump whose loop stage is owned by a different pipeline stage, and
// when the loop's max iterations are reached it terminates the advance: on_max
// "fail" closes the ticket, otherwise it pauses back to the owner. A nil result
// with nil error means the jump is within limits and Advance should proceed.
func handleLoopLimit(opts AdvanceOptions, cfg *config.Config, workflow, prevStage string, s *state.State) (*AdvanceResult, error) {
	if opts.Stage == "" || !cfg.IsLoopStage(workflow, opts.Stage) {
		return nil, nil
	}
	owner, _ := cfg.OwnerStage(workflow, opts.Stage)
	if owner != prevStage {
		return nil, fmt.Errorf("stage %q is a loop stage owned by %q, not %q", opts.Stage, owner, prevStage)
	}
	loopDef, ok := cfg.LoopConfig(workflow, prevStage)
	if !ok || loopDef.Max <= 0 {
		return nil, nil
	}
	count := s.StageCounts[opts.Stage]
	if count < loopDef.Max {
		return nil, nil
	}

	reason := fmt.Sprintf("loop limit reached (%d/%d for %s)", count, loopDef.Max, opts.Stage)
	if loopDef.OnMax == "fail" {
		result := opts.Result
		if result == "" {
			result = reason
		}
		if err := state.Done(opts.FeatureDir, result); err != nil {
			return nil, err
		}
		return &AdvanceResult{
			Ticket:   s.Ticket,
			Previous: prevStage,
			Next:     "",
			Outcome:  AdvanceOutcomeDone,
			Reason:   reason,
		}, nil
	}
	if err := state.Pause(opts.FeatureDir, reason); err != nil {
		return nil, err
	}
	return &AdvanceResult{
		Ticket:   s.Ticket,
		Previous: prevStage,
		Next:     prevStage,
		Outcome:  AdvanceOutcomePaused,
		Reason:   reason,
	}, nil
}

// blockingArtifactIssues gathers the readiness problems that gate an advance.
// Core docs must exist and be non-empty; the stage's required artifacts must
// additionally differ from the feature template — they are the stage's actual
// output contract. Core docs not required by the stage never block on content,
// so an untouched DECISIONS.md cannot wedge a workflow.
func blockingArtifactIssues(featureDir, root string, required []string) []artifactcheck.Issue {
	inRequired := make(map[string]bool, len(required))
	for _, artifact := range required {
		inRequired[artifact] = true
	}
	var coreOnly []string
	for _, doc := range artifactcheck.CoreDocs {
		if !inRequired[doc] {
			coreOnly = append(coreOnly, doc)
		}
	}
	issues := artifactcheck.Check(featureDir, "", coreOnly)
	return append(issues, artifactcheck.Check(featureDir, artifactcheck.TemplateDir(root), required)...)
}

func artifactIssueDetails(issues []artifactcheck.Issue) string {
	return strings.Join(artifactIssueList(issues), "\n  - ")
}

func artifactIssueList(issues []artifactcheck.Issue) []string {
	parts := make([]string, len(issues))
	for i, issue := range issues {
		parts[i] = issue.Detail()
	}
	return parts
}
