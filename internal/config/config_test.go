package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cengebretson/orc/internal/config"
)

func writeOrcYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "orc.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultWorkflow_ReturnsEmptyWhenNotSet(t *testing.T) {
	cfg := &config.Config{}
	if got := cfg.DefaultWorkflow(); got != "" {
		t.Errorf("DefaultWorkflow() = %q, want \"\"", got)
	}
}

func TestDefaultWorkflow_UsesConfiguredValue(t *testing.T) {
	cfg := &config.Config{Settings: config.Settings{DefaultWorkflow: "hotfix"}}
	if got := cfg.DefaultWorkflow(); got != "hotfix" {
		t.Errorf("DefaultWorkflow() = %q, want \"hotfix\"", got)
	}
}

func TestLoad_Settings(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
settings:
  default_workflow: hotfix
  auto_archive: true
repos: []
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Settings.DefaultWorkflow != "hotfix" {
		t.Errorf("default_workflow = %q, want \"hotfix\"", cfg.Settings.DefaultWorkflow)
	}
	if !cfg.Settings.AutoArchive {
		t.Error("auto_archive = false, want true")
	}
	if got := cfg.DefaultWorkflow(); got != "hotfix" {
		t.Errorf("DefaultWorkflow() = %q, want \"hotfix\"", got)
	}
}

func TestLoad_RepoWorktreeSetup(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
repos:
  - name: my-app
    path: ../my-app
    purpose: Application code
    worktree_setup: "../my-app/setup.sh -b {{branch}} --path {{worktree_path}}"
    agent_hints:
      - Use the repo Makefile before direct tool commands.
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("repos len = %d, want 1", len(cfg.Repos))
	}
	if got := cfg.Repos[0].WorktreeSetup; got != "../my-app/setup.sh -b {{branch}} --path {{worktree_path}}" {
		t.Fatalf("WorktreeSetup = %q", got)
	}
	if got := cfg.Repos[0].AgentHints; len(got) != 1 || got[0] != "Use the repo Makefile before direct tool commands." {
		t.Fatalf("AgentHints = %#v", got)
	}
}

func TestLoad_MissingFile_ReturnsEmptyConfig(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("expected empty repos, got %d", len(cfg.Repos))
	}
	if cfg.DefaultWorkflow() != "" {
		t.Errorf("DefaultWorkflow() = %q, want \"\"", cfg.DefaultWorkflow())
	}
	if len(cfg.Names()) != 0 {
		t.Errorf("expected no workflows, got %d", len(cfg.Names()))
	}
}

func TestLoad_BasicWorkflow(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
workflows:
  default:
    stages:
      - name: intake
        worker: fred-documentor
        advance: auto
      - name: develop
        worker: bob-developer
        advance: manual
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := cfg.Names()
	if len(names) != 1 || names[0] != "default" {
		t.Errorf("Names() = %v, want [default]", names)
	}

	stages := cfg.StageNames("default")
	if len(stages) != 2 || stages[0] != "intake" || stages[1] != "develop" {
		t.Errorf("StageNames(default) = %v, want [intake develop]", stages)
	}
}

func TestLoad_WorkflowDescription(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
workflows:
  default:
    description: General feature workflow — intake → develop
    stages:
      - name: intake
        worker: fred-documentor
        advance: auto
  bare:
    stages:
      - name: intake
        worker: fred-documentor
        advance: auto
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.WorkflowDescription("default"); got != "General feature workflow — intake → develop" {
		t.Errorf("WorkflowDescription(default) = %q, want the description", got)
	}
	if got := cfg.WorkflowDescription("bare"); got != "" {
		t.Errorf("WorkflowDescription(bare) = %q, want empty (no description set)", got)
	}
}

func TestLoad_StageConfig(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
workflows:
  default:
    stages:
      - name: intake
        worker: fred-documentor
        advance: auto
      - name: develop
        worker: bob-developer
        advance: manual
        required_artifacts:
          - PLAN.md
          - develop/HANDOFF.md
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sc, ok := cfg.StageConfig("default", "intake")
	if !ok {
		t.Fatal("StageConfig(default, intake) not found")
	}
	if sc.Worker != "fred-documentor" {
		t.Errorf("Worker = %q, want fred-documentor", sc.Worker)
	}
	if sc.Advance != "auto" {
		t.Errorf("Advance = %q, want auto", sc.Advance)
	}
	sc, ok = cfg.StageConfig("default", "develop")
	if !ok {
		t.Fatal("StageConfig(default, develop) not found")
	}
	if got := sc.RequiredArtifacts; len(got) != 2 || got[0] != "PLAN.md" || got[1] != "develop/HANDOFF.md" {
		t.Fatalf("RequiredArtifacts = %#v", got)
	}
}

func TestLoad_WorkflowAliasAccessors(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
aliases:
  workflows:
    default: default:standard
  stages:
    intake: default:intake
    develop: default:develop
    code-review: default:code-review
workflows:
  default:standard:
    description: Namespaced default workflow
    stages:
      - name: default:intake
        worker: default:fred
        advance: auto
      - name: default:develop
        worker: default:bob
        advance: manual
        loop:
          via: default:code-review
          worker: default:zach
          max: 3
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := cfg.WorkflowDescription("default"); got != "Namespaced default workflow" {
		t.Fatalf("WorkflowDescription(default) = %q", got)
	}
	if got := cfg.StageNames("default"); len(got) != 2 || got[0] != "default:intake" || got[1] != "default:develop" {
		t.Fatalf("StageNames(default) = %v", got)
	}
	if got := cfg.NextStage("default", "default:intake"); got != "default:develop" {
		t.Fatalf("NextStage(default, default:intake) = %q", got)
	}
	sc, ok := cfg.StageConfig("default", "default:code-review")
	if !ok || sc.Worker != "default:zach" {
		t.Fatalf("StageConfig alias loop = %+v, %v", sc, ok)
	}
	if !cfg.IsLoopStage("default", "default:code-review") {
		t.Fatal("IsLoopStage alias = false")
	}
	owner, ok := cfg.OwnerStage("default", "default:code-review")
	if !ok || owner != "default:develop" {
		t.Fatalf("OwnerStage alias = %q, %v", owner, ok)
	}
	if got := cfg.NextStage("default", "intake"); got != "default:develop" {
		t.Fatalf("NextStage with stage alias = %q", got)
	}
	sc, ok = cfg.StageConfig("default", "develop")
	if !ok || sc.Name != "default:develop" {
		t.Fatalf("StageConfig with stage alias = %+v, %v", sc, ok)
	}
}

func TestDisplayNamesUseUnambiguousAliases(t *testing.T) {
	cfg := &config.Config{
		Aliases: config.Aliases{
			Workflows: map[string]string{"default": "default:standard"},
			Stages:    map[string]string{"develop": "default:develop"},
			Workers:   map[string]string{"bob": "default:bob"},
		},
	}

	if got := cfg.WorkflowDisplayName("default:standard"); got != "default" {
		t.Fatalf("WorkflowDisplayName = %q, want default", got)
	}
	if got := cfg.StageDisplayName("default:develop"); got != "develop" {
		t.Fatalf("StageDisplayName = %q, want develop", got)
	}
	if got := cfg.WorkerDisplayName("default:bob"); got != "bob" {
		t.Fatalf("WorkerDisplayName = %q, want bob", got)
	}
}

func TestValidate_AliasTargetsMustBeUnique(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{DefaultWorkflow: "default:standard"},
		Aliases: config.Aliases{
			Workflows: map[string]string{
				"default": "default:standard",
				"main":    "default:standard",
			},
			Stages: map[string]string{
				"develop": "default:develop",
				"dev":     "default:develop",
			},
			Workers: map[string]string{
				"bob":       "default:bob",
				"developer": "default:bob",
			},
		},
		Workflows: map[string]config.WorkflowDef{
			"default:standard": {Stages: []config.StageDef{{Name: "default:develop", Worker: "default:bob", Advance: "auto"}}},
		},
	}

	errs := config.Validate(cfg, []string{"default:bob"})
	assertValidationError(t, errs, "aliases.workflows.main", `alias target "default:standard" is already used by alias "default"`)
	assertValidationError(t, errs, "aliases.stages.develop", `alias target "default:develop" is already used by alias "dev"`)
	assertValidationError(t, errs, "aliases.workers.developer", `alias target "default:bob" is already used by alias "bob"`)
}

func TestValidate_WorktreeSetupUnknownPlaceholder(t *testing.T) {
	cfg := &config.Config{
		Repos: []config.Repo{
			{
				Name:          "my-app",
				Path:          "../my-app",
				WorktreeSetup: "../my-app/setup.sh --path {{worktree_path}} --bad {{unknown}}",
			},
		},
	}

	errs := config.Validate(cfg, nil)
	assertValidationError(t, errs, "repos[0].worktree_setup", "unknown placeholder(s): unknown")
}

func TestValidate_RequiredArtifactMustBeRelative(t *testing.T) {
	cfg := &config.Config{
		Settings: config.Settings{DefaultWorkflow: "default"},
		Workflows: map[string]config.WorkflowDef{
			"default": {
				Stages: []config.StageDef{
					{Name: "develop", Worker: "default:bob", Advance: "auto", RequiredArtifacts: []string{"/tmp/out.md", "../escape.md"}},
				},
			},
		},
	}

	errs := config.Validate(cfg, []string{"default:bob"})
	assertValidationError(t, errs, "workflows.default.stages[0].required_artifacts[0]", "must be relative")
	assertValidationError(t, errs, "workflows.default.stages[0].required_artifacts[1]", "cannot contain ..")
}

func TestLoad_NextStage(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
workflows:
  default:
    stages:
      - name: intake
        advance: auto
      - name: develop
        advance: manual
      - name: pr-open
        advance: auto
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if next := cfg.NextStage("default", "intake"); next != "develop" {
		t.Errorf("NextStage(intake) = %q, want develop", next)
	}
	if next := cfg.NextStage("default", "develop"); next != "pr-open" {
		t.Errorf("NextStage(develop) = %q, want pr-open", next)
	}
	if next := cfg.NextStage("default", "pr-open"); next != "" {
		t.Errorf("NextStage(pr-open) = %q, want empty (last stage)", next)
	}
}

func TestLoad_LoopStages(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
workflows:
  default:
    stages:
      - name: pr-open
        worker: bob-developer
        advance: auto
        loop:
          via: pr-repair
          worker: bob-developer
          max: 3
          on_max: pause
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loop, ok := cfg.LoopConfig("default", "pr-open")
	if !ok {
		t.Fatal("LoopConfig(pr-open) not found")
	}
	if loop.Via != "pr-repair" {
		t.Errorf("Via = %q, want pr-repair", loop.Via)
	}
	if loop.Max != 3 {
		t.Errorf("Max = %d, want 3", loop.Max)
	}
	if loop.OnMax != "pause" {
		t.Errorf("OnMax = %q, want pause", loop.OnMax)
	}

	if !cfg.IsLoopStage("default", "pr-repair") {
		t.Error("IsLoopStage(pr-repair) = false, want true")
	}
	if cfg.IsLoopStage("default", "pr-open") {
		t.Error("IsLoopStage(pr-open) = true, want false")
	}

	owner, ok := cfg.OwnerStage("default", "pr-repair")
	if !ok {
		t.Fatal("OwnerStage(pr-repair) not found")
	}
	if owner != "pr-open" {
		t.Errorf("OwnerStage = %q, want pr-open", owner)
	}

	// Loop stage resolves via StageConfig.
	sc, ok := cfg.StageConfig("default", "pr-repair")
	if !ok {
		t.Fatal("StageConfig(pr-repair) not found")
	}
	if sc.Worker != "bob-developer" {
		t.Errorf("StageConfig worker = %q, want bob-developer", sc.Worker)
	}
}

func TestLoad_MultipleWorkflows(t *testing.T) {
	dir := t.TempDir()
	writeOrcYAML(t, dir, `
workflows:
  default:
    stages:
      - name: intake
      - name: develop
  hotfix:
    stages:
      - name: develop
      - name: pr-open
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := cfg.Names()
	if len(names) != 2 {
		t.Errorf("Names() = %v, want 2 workflows", names)
	}

	if stages := cfg.StageNames("hotfix"); len(stages) != 2 || stages[0] != "develop" {
		t.Errorf("StageNames(hotfix) = %v, want [develop pr-open]", stages)
	}
}
