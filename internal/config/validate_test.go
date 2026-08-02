package config_test

import (
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/config"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{DefaultWorkflow: "default"},
		Workflows: map[string]config.WorkflowDef{
			"default": {
				Stages: []config.StageDef{
					{Name: "intake", Worker: "fred-documentor", Advance: "auto"},
					{
						Name:    "develop",
						Worker:  "bob-developer",
						Advance: "manual",
						Loop:    &config.LoopDef{Via: "code-review", Worker: "zach-reviewer", Max: 3, OnMax: "pause"},
					},
				},
			},
		},
	}

	errs := config.Validate(cfg, []string{"fred-documentor", "bob-developer", "zach-reviewer"})
	if len(errs) != 0 {
		t.Fatalf("Validate returned errors: %v", errs)
	}
}

func TestValidate_DefaultWorkflowMustExist(t *testing.T) {
	cfg := &config.Config{
		Settings:  config.Settings{DefaultWorkflow: "missing"},
		Workflows: map[string]config.WorkflowDef{"default": {Stages: []config.StageDef{{Name: "intake", Worker: "fred", Advance: "auto"}}}},
	}

	assertValidationError(t, config.Validate(cfg, []string{"fred"}), "settings.default_workflow", `workflow "missing" not found`)
}

func TestValidate_DefaultWorkflowMayUseAlias(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{DefaultWorkflow: "default"},
		Aliases: config.Aliases{
			Workflows: map[string]string{"default": "default:standard"},
		},
		Workflows: map[string]config.WorkflowDef{
			"default:standard": {Stages: []config.StageDef{{Name: "default:intake", Worker: "default:fred", Advance: "auto"}}},
		},
	}

	errs := config.Validate(cfg, []string{"default:fred"})
	if len(errs) != 0 {
		t.Fatalf("Validate returned errors: %v", errs)
	}
}

func TestValidate_ArtifactPolicy(t *testing.T) {
	cfg := &config.Config{
		Settings:  config.Settings{DefaultWorkflow: "default", ArtifactPolicy: "strict"},
		Workflows: map[string]config.WorkflowDef{"default": {Stages: []config.StageDef{{Name: "intake", Worker: "fred", Advance: "auto"}}}},
	}

	assertValidationError(t, config.Validate(cfg, []string{"fred"}), "settings.artifact_policy", `artifact_policy must be "warn" or "block"`)
}

func TestValidate_WorkspaceRefreshCannotBeNegative(t *testing.T) {
	cfg := &config.Config{Settings: config.Settings{WorkspaceRefresh: -1}}
	assertValidationError(t, config.Validate(cfg, nil), "settings.workspace_refresh", "workspace_refresh must be zero or greater")
}

func TestValidate_NotifyEvents(t *testing.T) {
	cfg := &config.Config{Settings: config.Settings{Notify: config.NotifySettings{On: []string{"blocked", "bogus"}}}}
	assertValidationError(t, config.Validate(cfg, nil), "settings.notify.on[1]", `event must be "blocked", "complete", "error", or "all"`)
}

func TestValidate_RepoIdentity(t *testing.T) {
	cfg := &config.Config{Repos: []config.Repo{
		{},
		{Name: "app", Path: "../app"},
		{Name: "app", Path: "../app/../app"},
	}}
	errs := config.Validate(cfg, nil)
	assertValidationError(t, errs, "repos[0].name", "repo name is required")
	assertValidationError(t, errs, "repos[0].path", "repo path is required")
	assertValidationError(t, errs, "repos[2].name", `duplicate repo name "app"`)
	assertValidationError(t, errs, "repos[2].path", `duplicate repo path "../app/../app"`)
}

func TestValidate_ContextPressureThresholdOrder(t *testing.T) {
	cfg := &config.Config{Settings: config.Settings{ContextPressure: &config.ContextPressureSettings{Green: 70, Yellow: 70, Red: 90}}}
	assertValidationError(t, config.Validate(cfg, nil), "settings.context_pressure", "thresholds must satisfy 0 <= green < yellow < red <= 100")
}

func TestValidate_ContextPressureThresholdRange(t *testing.T) {
	cfg := &config.Config{Settings: config.Settings{ContextPressure: &config.ContextPressureSettings{Green: 0, Yellow: 70, Red: 101}}}
	assertValidationError(t, config.Validate(cfg, nil), "settings.context_pressure", "thresholds must satisfy 0 <= green < yellow < red <= 100")
}

func TestValidate_DefaultWorkflowRequiredWhenWorkflowsExist(t *testing.T) {
	cfg := &config.Config{
		Workflows: map[string]config.WorkflowDef{"default": {Stages: []config.StageDef{{Name: "intake", Worker: "fred", Advance: "auto"}}}},
	}

	assertValidationError(t, config.Validate(cfg, []string{"fred"}), "settings.default_workflow", "default workflow is required")
}

func TestValidate_WorkflowMustHaveStages(t *testing.T) {
	cfg := &config.Config{
		Settings:  config.Settings{DefaultWorkflow: "default"},
		Workflows: map[string]config.WorkflowDef{"default": {}},
	}

	assertValidationError(t, config.Validate(cfg, nil), "workflows.default.stages", "workflow must define at least one stage")
}

func TestValidate_StageFields(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{DefaultWorkflow: "default"},
		Workflows: map[string]config.WorkflowDef{
			"default": {
				Stages: []config.StageDef{
					{Name: "", Worker: "", Advance: "sometimes"},
					{Name: "develop", Worker: "missing", Advance: "manual"},
					{Name: "develop", Worker: "bob-developer", Advance: "auto"},
				},
			},
		},
	}
	errs := config.Validate(cfg, []string{"bob-developer"})

	assertValidationError(t, errs, "workflows.default.stages[0].name", "stage name is required")
	assertValidationError(t, errs, "workflows.default.stages[0].worker", "worker is required")
	assertValidationError(t, errs, "workflows.default.stages[0].advance", `advance must be "auto" or "manual"`)
	assertValidationError(t, errs, "workflows.default.stages[1].worker", `worker "missing" not found`)
	assertValidationError(t, errs, "workflows.default.stages[2].name", `duplicate stage name "develop"`)
}

func TestValidate_LoopFields(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{DefaultWorkflow: "default"},
		Workflows: map[string]config.WorkflowDef{
			"default": {
				Stages: []config.StageDef{
					{
						Name:    "develop",
						Worker:  "bob-developer",
						Advance: "manual",
						Loop:    &config.LoopDef{Via: "", Worker: "", Max: -1, OnMax: "invalid"},
					},
					{
						Name:    "pr-open",
						Worker:  "bob-developer",
						Advance: "manual",
						Loop:    &config.LoopDef{Via: "develop", Worker: "missing"},
					},
				},
			},
		},
	}
	errs := config.Validate(cfg, []string{"bob-developer"})

	assertValidationError(t, errs, "workflows.default.stages[0].loop.via", "loop stage name is required")
	assertValidationError(t, errs, "workflows.default.stages[0].loop.worker", "worker is required")
	assertValidationError(t, errs, "workflows.default.stages[0].loop.max", "loop max must be zero or greater")
	assertValidationError(t, errs, "workflows.default.stages[0].loop.on_max", `loop on_max must be "pause" or "fail"`)
	assertValidationError(t, errs, "workflows.default.stages[1].loop.via", `duplicate stage name "develop"`)
	assertValidationError(t, errs, "workflows.default.stages[1].loop.worker", `worker "missing" not found`)
}

func TestValidate_LoopOnMaxValidValues(t *testing.T) {
	for _, onMax := range []string{"", "pause", "fail"} {
		cfg := &config.Config{
			Settings: config.Settings{DefaultWorkflow: "default"},
			Workflows: map[string]config.WorkflowDef{
				"default": {
					Stages: []config.StageDef{
						{
							Name:    "develop",
							Worker:  "bob-developer",
							Advance: "auto",
							Loop:    &config.LoopDef{Via: "code-review", Worker: "bob-developer", Max: 3, OnMax: onMax},
						},
					},
				},
			},
		}
		errs := config.Validate(cfg, []string{"bob-developer"})
		for _, err := range errs {
			if err.Path == "workflows.default.stages[0].loop.on_max" {
				t.Errorf("on_max=%q got unexpected error: %s", onMax, err.Message)
			}
		}
	}
}

func TestValidate_RepoRoutingValues(t *testing.T) {
	cfg := &config.Config{
		Repos: []config.Repo{{Name: "app", Path: "../app"}},
		Routing: []config.RepoRoute{
			{Labels: []string{"frontend", " "}, Repos: []string{"app", "app", "missing"}},
			{Labels: []string{"FRONTEND"}, Components: []string{"web", "WEB"}},
			{},
		},
	}
	errs := config.Validate(cfg, nil)

	assertValidationError(t, errs, "routing[0].labels[1]", "routing value is required")
	assertValidationError(t, errs, "routing[0].repos[1]", `duplicate repo "app"`)
	assertValidationError(t, errs, "routing[0].repos[2]", `repo "missing" not found`)
	assertValidationError(t, errs, "routing[1].labels[0]", `routing value "FRONTEND" is already used`)
	assertValidationError(t, errs, "routing[1].components[1]", `routing value "WEB" is already used`)
	assertValidationError(t, errs, "routing[1].repos", "route must select at least one repo")
	assertValidationError(t, errs, "routing[2]", "route must define at least one label or component")
}

func assertValidationError(t *testing.T, errs config.ValidationErrors, path, message string) {
	t.Helper()
	for _, err := range errs {
		if err.Path == path && strings.Contains(err.Message, message) {
			return
		}
	}
	t.Fatalf("missing validation error path=%q message containing %q in %#v", path, message, errs)
}
