package orchestrator

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/artifactcheck"
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

	if opts.Stage != "" && workflowCfg.IsLoopStage(workflow, opts.Stage) {
		owner, _ := workflowCfg.OwnerStage(workflow, opts.Stage)
		if owner != prevStage {
			return nil, fmt.Errorf("stage %q is a loop stage owned by %q, not %q", opts.Stage, owner, prevStage)
		}
		if loopDef, ok := workflowCfg.LoopConfig(workflow, prevStage); ok && loopDef.Max > 0 {
			count := s.StageCounts[opts.Stage]
			if count >= loopDef.Max {
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
		}
	}

	// Checked last so agents get actionable guidance first: a manual stage says
	// "use pause", and a loop at max still pauses/fails instead of erroring here.
	if workflowCfg.ArtifactPolicy() == "block" {
		if issues := artifactcheck.Check(opts.FeatureDir, append(artifactcheck.CoreDocs, stageCfg.RequiredArtifacts...)); len(issues) > 0 {
			return nil, fmt.Errorf("artifact_policy=block: required artifacts are not ready:\n  - %s", artifactIssueDetails(issues))
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
		AutoArchive: autoArchive,
	}, nil
}

func artifactIssueDetails(issues []artifactcheck.Issue) string {
	parts := make([]string, len(issues))
	for i, issue := range issues {
		parts[i] = issue.Detail()
	}
	return strings.Join(parts, "\n  - ")
}
