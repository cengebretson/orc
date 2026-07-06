package runner

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
)

// Plan is the fully resolved next action for a ticket.
type Plan struct {
	Ticket         string
	Workflow       string
	Stage          string
	Worker         *workers.Worker
	WorkerReason   string // "flag override" | "stage owner" | "workflow default"
	Prompt         string
	LaunchCommand  string   // shell string for --dry display
	LaunchArgv     []string // for exec.Command
	CWD            string
	EndInstruction string // the orc mark command the agent should run
}

// Compute resolves the next action plan for a ticket.
// workerOverride is the --worker flag value; pass "" to use the default resolution order.
func Compute(root, featureDir, workerOverride string) (*Plan, error) {
	s, err := state.Load(featureDir)
	if err != nil {
		return nil, err
	}

	ctx, validationErrs, err := workspacectx.LoadValidated(root)
	if err != nil {
		return nil, err
	}
	if len(validationErrs) > 0 {
		return nil, fmt.Errorf("invalid workspace config: %w", validationErrs)
	}
	cfg := ctx.Config
	allWorkers := ctx.Workers

	workflow, err := resolveWorkflow(cfg, s.Workflow)
	if err != nil {
		return nil, err
	}
	stageCfg, _ := cfg.StageConfig(workflow, s.Stage.Name)
	nextStage := cfg.NextStage(workflow, s.Stage.Name)
	// If current stage is a loop stage, the "next" stage is the owner stage.
	if nextStage == "" {
		if owner, ok := cfg.OwnerStage(workflow, s.Stage.Name); ok {
			nextStage = owner
		}
	}
	loopDef, _ := cfg.LoopConfig(workflow, s.Stage.Name)
	isLoopStage := cfg.IsLoopStage(workflow, s.Stage.Name)

	worker, reason, err := resolveWorker(allWorkers, workerOverride, s.Stage.Worker, stageCfg.Worker, s.Stage.Name)
	if err != nil {
		return nil, err
	}

	cwd := s.ResolveCWD(root)
	prompt := buildPrompt(
		s,
		nextStage,
		stageCfg.Advance,
		loopDef,
		isLoopStage,
		worktreeSetupPrompt(root, cfg, s),
		artifactPrompt(s.Slug, stageCfg.RequiredArtifacts),
		repoHintsPrompt(cfg, s),
	)

	plan := &Plan{
		Ticket:         s.Ticket,
		Workflow:       workflow,
		Stage:          s.Stage.Name,
		Worker:         worker,
		WorkerReason:   reason,
		Prompt:         prompt,
		LaunchCommand:  workers.LaunchCommand(worker, root, cwd, prompt),
		LaunchArgv:     workers.LaunchArgs(worker, root, cwd, prompt),
		CWD:            cwd,
		EndInstruction: endInstruction(s.Ticket, nextStage, stageCfg.Advance, loopDef, isLoopStage),
	}
	return plan, nil
}

// ResolveWorkflow returns the ticket's workflow name, using the workspace default if unset.
// Returns an error if the resolved workflow is not defined in orc.yaml.
// Exported so callers that already have a config can avoid a second load.
func ResolveWorkflow(cfg *config.Config, ticketWorkflow string) (string, error) {
	return resolveWorkflow(cfg, ticketWorkflow)
}

func resolveWorkflow(cfg *config.Config, ticketWorkflow string) (string, error) {
	name := ticketWorkflow
	if name == "" {
		name = cfg.DefaultWorkflow()
	}
	if name == "" {
		known := cfg.Names()
		if len(known) > 0 {
			return "", fmt.Errorf("no default_workflow set in orc.yaml (available: %s)", strings.Join(known, ", "))
		}
		return "", fmt.Errorf("no default_workflow set in orc.yaml")
	}
	if _, ok := cfg.Workflows[name]; !ok {
		known := cfg.Names()
		if len(known) > 0 {
			return "", fmt.Errorf("workflow %q not found in orc.yaml (available: %s)", name, strings.Join(known, ", "))
		}
		return "", fmt.Errorf("workflow %q not found in orc.yaml", name)
	}
	return name, nil
}

func resolveWorker(allWorkers []*workers.Worker, flagOverride, stageOwner, configWorker, stageName string) (*workers.Worker, string, error) {
	if flagOverride != "" {
		if w := workers.FindByID(allWorkers, flagOverride); w != nil {
			return w, "flag override", nil
		}
		return nil, "", fmt.Errorf("worker %q not found in workers/", flagOverride)
	}
	if stageOwner != "" {
		if w := workers.FindByID(allWorkers, stageOwner); w != nil {
			return w, "stage owner", nil
		}
	}
	if configWorker != "" {
		if w := workers.FindByID(allWorkers, configWorker); w != nil {
			return w, "workflow default", nil
		}
		return nil, "", fmt.Errorf("worker %q assigned to stage %q in orc.yaml not found in workers/", configWorker, stageName)
	}
	return nil, "", fmt.Errorf("no worker assigned for stage %q — set worker: in orc.yaml", stageName)
}

func buildPrompt(s *state.State, nextStage, advanceMode string, loopDef *config.LoopDef, isLoopStage bool, worktreeSetup, artifacts, repoHints string) string {
	prompt := s.NextAction.Prompt
	if prompt == "" {
		prompt = fmt.Sprintf(
			"Continue %s — stage: %s\n\nFeature context: features/%s/STATE.yaml\nStage: stages/%s.md",
			s.Ticket, s.Stage.Name, s.Slug, strings.TrimSuffix(config.ResourcePath(s.Stage.Name), ".md"),
		)
	}

	markAction := "start"
	if s.Status == "paused" {
		markAction = "resume"
	}
	preamble := fmt.Sprintf(
		"Before starting: read AGENTS.md and ORC.md. Run `orc mark %s %s` to mark as active.\n\n",
		s.Ticket, markAction,
	)

	if worktreeSetup != "" {
		prompt = worktreeSetup + prompt
	}
	if artifacts != "" {
		prompt = artifacts + prompt
	}
	if repoHints != "" {
		prompt = repoHints + prompt
	}

	return preamble + prompt + endInstruction(s.Ticket, nextStage, advanceMode, loopDef, isLoopStage)
}

func artifactPrompt(slug string, required []string) string {
	if len(required) == 0 {
		return ""
	}
	if slug == "" {
		slug = "<slug>"
	}
	var b strings.Builder
	b.WriteString("## Required artifacts\n\n")
	b.WriteString("Before completing this stage, make sure these feature-folder artifacts exist and are current:\n\n")
	for _, artifact := range required {
		fmt.Fprintf(&b, "- `features/%s/%s`\n", slug, artifact)
	}
	b.WriteString("\n")
	return b.String()
}

func repoHintsPrompt(cfg *config.Config, s *state.State) string {
	repos := targetRepos(cfg, s)
	var b strings.Builder
	for _, repo := range repos {
		if len(repo.Config.AgentHints) == 0 {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("## Repo hints\n\n")
		}
		fmt.Fprintf(&b, "Repo `%s`:\n", repo.Config.Name)
		for _, hint := range repo.Config.AgentHints {
			if strings.TrimSpace(hint) == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s\n", hint)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func endInstruction(ticket, nextStage, advanceMode string, loopDef *config.LoopDef, isLoopStage bool) string {
	if nextStage == "" {
		return ""
	}
	// Loop stage: fixed return to owner, no branching.
	if isLoopStage {
		return fmt.Sprintf(
			"\n\nWhen your work is complete, run:\n  orc mark %s next --result \"<summary of what was done>\"",
			ticket,
		)
	}
	// Stage with a loop: branching end instruction.
	if loopDef != nil {
		forward := fmt.Sprintf("orc mark %s next --result \"<summary>\"", ticket)
		loop := fmt.Sprintf("orc mark %s next --stage %s --result \"<what needs fixing>\"", ticket, loopDef.Via)
		if advanceMode == "manual" {
			forward = fmt.Sprintf("orc mark %s pause \"<summary — human will review before advancing to %s>\"", ticket, nextStage)
		}
		return fmt.Sprintf(
			"\n\nWhen this stage is complete, run ONE of:\n\n  %s\n    → work is good, advance to %s\n\n  %s\n    → issues found, enter %s loop",
			forward, nextStage, loop, loopDef.Via,
		)
	}
	// Normal stage.
	if advanceMode == "manual" {
		return fmt.Sprintf(
			"\n\nWhen this stage is complete, run:\n  orc mark %s pause \"<summary — human will review before advancing to %s>\"",
			ticket, nextStage,
		)
	}
	return fmt.Sprintf(
		"\n\nWhen this stage is complete, run:\n  orc mark %s next --result \"<summary>\"",
		ticket,
	)
}
