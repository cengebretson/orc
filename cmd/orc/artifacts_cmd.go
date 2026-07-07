package main

import (
	"fmt"

	"github.com/cengebretson/orc/internal/artifactcheck"
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/cengebretson/orc/internal/workspacectx"
	"github.com/spf13/cobra"
)

type artifactReport struct {
	Ticket   string          `json:"ticket"`
	Slug     string          `json:"slug"`
	Workflow string          `json:"workflow"`
	Stage    string          `json:"stage"`
	Policy   string          `json:"policy"`
	Scope    string          `json:"scope"`
	OK       bool            `json:"ok"`
	Groups   []artifactGroup `json:"groups"`
}

type artifactGroup struct {
	Name      string                   `json:"name"`
	Artifacts []artifactcheck.Artifact `json:"artifacts"`
}

func runArtifacts(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	featureDir, err := ticket.Resolve(root, args[0])
	if err != nil {
		return err
	}
	s, err := state.Load(featureDir)
	if err != nil {
		return err
	}
	ctx, validationErrs, err := workspacectx.LoadValidated(root)
	if err != nil {
		return err
	}
	if len(validationErrs) > 0 {
		return fmt.Errorf("invalid workspace config: %w", validationErrs)
	}

	report, err := buildArtifactReport(featureDir, artifactcheck.TemplateDir(root), s, ctx.Config, artifactsAll)
	if err != nil {
		return err
	}
	if artifactsJSON {
		if err := printJSON(report); err != nil {
			return err
		}
	} else {
		printArtifactReport(report)
	}
	if !report.OK {
		return fmt.Errorf("artifacts not ready")
	}
	return nil
}

func buildArtifactReport(featureDir, templateDir string, s *state.State, cfg *config.Config, all bool) (*artifactReport, error) {
	workflow := s.Workflow
	if workflow == "" {
		workflow = cfg.DefaultWorkflow()
	}
	if _, ok := cfg.Workflows[workflow]; !ok {
		return nil, fmt.Errorf("workflow %q not found in orc.yaml", workflow)
	}

	// Core docs are checked for presence only; stage artifacts are the stage's
	// output contract and must also differ from the feature template. This
	// mirrors the artifact_policy=block gate in orc mark next.
	scope := "current"
	var groups []artifactGroup
	if all {
		scope = "all"
		groups = append(groups, inspectArtifactGroup(featureDir, "", "core", artifactcheck.CoreDocs))
		for _, stage := range cfg.Stages(workflow) {
			if len(stage.RequiredArtifacts) > 0 {
				groups = append(groups, inspectArtifactGroup(featureDir, templateDir, cfg.StageDisplayName(stage.Name), stage.RequiredArtifacts))
			}
			if stage.Loop != nil && len(stage.Loop.RequiredArtifacts) > 0 {
				groups = append(groups, inspectArtifactGroup(featureDir, templateDir, cfg.StageDisplayName(stage.Loop.Via), stage.Loop.RequiredArtifacts))
			}
		}
	} else {
		stageCfg, ok := cfg.StageConfig(workflow, s.Stage.Name)
		if !ok {
			return nil, fmt.Errorf("current stage %q not found in workflow %q", s.Stage.Name, workflow)
		}
		groups = append(groups, artifactGroup{
			Name: "current stage",
			Artifacts: append(
				artifactcheck.Inspect(featureDir, "", artifactcheck.CoreDocs),
				artifactcheck.Inspect(featureDir, templateDir, stageCfg.RequiredArtifacts)...,
			),
		})
	}

	ok := true
	for _, group := range groups {
		for _, artifact := range group.Artifacts {
			if !artifact.Ready {
				ok = false
			}
		}
	}

	return &artifactReport{
		Ticket:   s.Ticket,
		Slug:     s.Slug,
		Workflow: workflow,
		Stage:    s.Stage.Name,
		Policy:   cfg.ArtifactPolicy(),
		Scope:    scope,
		OK:       ok,
		Groups:   groups,
	}, nil
}

func inspectArtifactGroup(featureDir, templateDir, name string, artifacts []string) artifactGroup {
	return artifactGroup{Name: name, Artifacts: artifactcheck.Inspect(featureDir, templateDir, artifacts)}
}

func printArtifactReport(report *artifactReport) {
	fmt.Printf("Artifacts: %s · %s/%s\n", report.Ticket, report.Workflow, report.Stage)
	fmt.Printf("Policy:    %s\n", report.Policy)
	fmt.Printf("Scope:     %s\n\n", report.Scope)
	for i, group := range report.Groups {
		if i > 0 {
			fmt.Println()
		}
		fmt.Println(group.Name)
		for _, artifact := range group.Artifacts {
			icon := "✓"
			text := artifact.Path
			if !artifact.Ready {
				icon = "✗"
				text = artifact.Detail
			}
			fmt.Printf("  %s  %s\n", icon, text)
		}
	}
	if !report.OK {
		fmt.Println()
		fmt.Println("Some artifacts are missing, empty, or directories.")
	}
}
