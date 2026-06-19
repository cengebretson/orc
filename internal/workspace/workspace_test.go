package workspace_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspace"
)

func TestInit_CreatesExpectedFiles(t *testing.T) {
	dir := t.TempDir()

	err := workspace.Init(workspace.InitOptions{Root: dir})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	required := []string{
		"AGENTS.md",
		"CLAUDE.md",
		"ROUTER.md",
		"TOOLS.md",
		"RULES.md",
		"SETUP.md",
		"features/_template/STATE.yaml",
		"features/_template/TICKET.md",
		"workers/_template.md",
		"orc.yaml",
		"ORC.md",
		"packs/default/pack.yaml",
		"packs/default/.orc-pack.yaml",
		"packs/default/workflow.yaml",
		"stages/default/intake.md",
		"stages/default/develop.md",
		"stages/default/pr-open.md",
		"stages/default/pr-repair.md",
		"stages/default/qa-automation.md",
	}

	for _, rel := range required {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing expected file: %s", rel)
		}
	}
}

func TestInit_DryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()

	err := workspace.Init(workspace.InitOptions{Root: dir, DryRun: true})
	if err != nil {
		t.Fatalf("Init dry-run: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("dry-run wrote %d files, want 0", len(entries))
	}
}

func TestInit_PrintsNextSteps(t *testing.T) {
	dir := t.TempDir()

	out := captureStdout(t, func() {
		if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
			t.Fatalf("Init: %v", err)
		}
	})

	for _, want := range []string{
		"Workspace ready at: " + dir,
		"Next:",
		"  cd " + dir,
		`  claude "Read SETUP.md and follow the setup instructions"`,
		"  # or:",
		`  codex "Read SETUP.md and follow the setup instructions"`,
		"  orc doctor",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("init output missing %q:\n%s", want, out)
		}
	}
}

func TestInit_DryRunPrintsNextSteps(t *testing.T) {
	dir := t.TempDir()

	out := captureStdout(t, func() {
		if err := workspace.Init(workspace.InitOptions{Root: dir, DryRun: true}); err != nil {
			t.Fatalf("Init dry-run: %v", err)
		}
	})

	for _, want := range []string{
		"Dry run",
		"Would create workspace at: " + dir,
		"Would run next:",
		"  cd " + dir,
		`  claude "Read SETUP.md and follow the setup instructions"`,
		"  # or:",
		`  codex "Read SETUP.md and follow the setup instructions"`,
		"  orc doctor",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, out)
		}
	}
}

func TestInit_SkipsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()

	// write once
	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	// overwrite a file to detect if it gets reset
	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}

	// write again without force
	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("second Init: %v", err)
	}

	data, _ := os.ReadFile(agentsPath)
	if string(data) != "custom" {
		t.Error("Init without --force overwrote an existing file")
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = orig
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return string(out)
}

func TestInit_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	agentsPath := filepath.Join(dir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("custom"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := workspace.Init(workspace.InitOptions{Root: dir, Force: true}); err != nil {
		t.Fatalf("forced Init: %v", err)
	}

	data, _ := os.ReadFile(agentsPath)
	if string(data) == "custom" {
		t.Error("Init --force did not overwrite existing file")
	}
}

func TestInit_DefaultPackInstallsWorkers(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	workers := []string{
		"workers/default/bob.md",
		"workers/default/fred.md",
		"workers/default/zach.md",
	}
	for _, rel := range workers {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing pack worker: %s", rel)
		}
	}
}

func TestInit_SkipDefaultPackIsBaseOnly(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir, SkipDefaultPack: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// base files present
	if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err != nil {
		t.Errorf("base file AGENTS.md missing: %v", err)
	}
	// pack content absent
	for _, rel := range []string{"workers/default/bob.md", "stages/default/develop.md", "packs/default/pack.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
			t.Errorf("--skip-default-pack should not install %s", rel)
		}
	}
}

func TestPackInstall_LocalPackMaterializesRuntimeFiles(t *testing.T) {
	packDir := writeLocalPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workflows:
    hotfix: hotfix:standard
  workers:
    bob: hotfix:bob
  stages:
    develop: hotfix:develop
`)
	writePackFile(t, filepath.Join(packDir, "workflow.yaml"), `workflows:
  "hotfix:standard":
    description: Fast production fix workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)

	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir, SkipDefaultPack: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: packDir}); err != nil {
		t.Fatalf("InstallPack: %v", err)
	}

	for _, rel := range []string{
		"packs/hotfix/pack.yaml",
		"packs/hotfix/.orc-pack.yaml",
		"packs/hotfix/workflow.yaml",
		"stages/hotfix/develop.md",
		"workers/hotfix/bob.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}

	orcYAML, err := os.ReadFile(filepath.Join(dir, "orc.yaml"))
	if err != nil {
		t.Fatalf("read orc.yaml: %v", err)
	}
	for _, want := range []string{
		"default_workflow: hotfix:standard",
		`"hotfix:standard":`,
		"name: hotfix:develop",
		"worker: hotfix:bob",
		"aliases:",
	} {
		if !strings.Contains(string(orcYAML), want) {
			t.Fatalf("orc.yaml missing %q:\n%s", want, string(orcYAML))
		}
	}
}

func TestPackInstall_PreservesOrcYAMLShape(t *testing.T) {
	packDir := writeLocalPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workflows:
    hotfix: hotfix:standard
`)
	writePackFile(t, filepath.Join(packDir, "workflow.yaml"), `workflows:
  "hotfix:standard":
    description: Fast production fix workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)

	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir, SkipDefaultPack: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeOrcYAML(t, dir, `# keep this workspace comment
settings:
  default_workflow: default # keep this inline comment
  quotes:
    - custom

repos: []

workflows: {}

aliases: {}
`)

	if err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: packDir}); err != nil {
		t.Fatalf("InstallPack: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "orc.yaml"))
	if err != nil {
		t.Fatalf("read orc.yaml: %v", err)
	}
	orcYAML := string(data)
	for _, want := range []string{
		"# keep this workspace comment",
		"default_workflow: hotfix:standard # keep this inline comment",
		"repos: []",
		"workflows:\n  \"hotfix:standard\":",
		"aliases:\n  workflows:",
		`"hotfix:standard":`,
		"hotfix: hotfix:standard",
	} {
		if !strings.Contains(orcYAML, want) {
			t.Fatalf("orc.yaml missing %q:\n%s", want, orcYAML)
		}
	}
	for _, unwanted := range []string{"auto_archive: false", `theme: ""`, "tui_refresh: 0"} {
		if strings.Contains(orcYAML, unwanted) {
			t.Fatalf("orc.yaml contains generated zero value %q:\n%s", unwanted, orcYAML)
		}
	}
}

func TestPackInstall_RejectsWorkflowManifestMismatch(t *testing.T) {
	packDir := writeLocalPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
`)
	writePackFile(t, filepath.Join(packDir, "workflow.yaml"), `workflows:
  "hotfix:other":
    description: Wrong workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)

	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir, SkipDefaultPack: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: packDir})
	if err == nil {
		t.Fatal("InstallPack returned nil error")
	}
	if !strings.Contains(err.Error(), `workflow workflow.yaml does not define declared workflow "hotfix:standard"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackInstall_BuiltInPackInstallsAfterSkipDefault(t *testing.T) {
	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir, SkipDefaultPack: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: "default"}); err != nil {
		t.Fatalf("InstallPack: %v", err)
	}

	for _, rel := range []string{
		"packs/default/pack.yaml",
		"packs/default/.orc-pack.yaml",
		"stages/default/develop.md",
		"workers/default/bob.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Fatalf("missing %s: %v", rel, err)
		}
	}

	orcYAML, err := os.ReadFile(filepath.Join(dir, "orc.yaml"))
	if err != nil {
		t.Fatalf("read orc.yaml: %v", err)
	}
	if !strings.Contains(string(orcYAML), "default_workflow: default:standard") {
		t.Fatalf("orc.yaml did not set installed default workflow:\n%s", string(orcYAML))
	}
}

func TestPackInstall_RejectsAlreadyInstalledPack(t *testing.T) {
	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: "default"})
	if err == nil {
		t.Fatal("InstallPack returned nil error")
	}
	if !strings.Contains(err.Error(), `pack "default" is already installed`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackInstall_RejectsAliasConflict(t *testing.T) {
	packDir := writeLocalPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workers:
    bob: hotfix:bob
`)
	writePackFile(t, filepath.Join(packDir, "workflow.yaml"), `workflows:
  "hotfix:standard":
    description: Fast production fix workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)

	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: packDir})
	if err == nil {
		t.Fatal("InstallPack returned nil error")
	}
	if !strings.Contains(err.Error(), `worker alias "bob" points to both default:bob and hotfix:bob`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackInstall_RejectsDuplicateAliasTargets(t *testing.T) {
	packDir := writeLocalPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workers:
    bob: hotfix:bob
    developer: hotfix:bob
`)
	writePackFile(t, filepath.Join(packDir, "workflow.yaml"), `workflows:
  "hotfix:standard":
    description: Fast production fix workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)

	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: packDir})
	if err == nil {
		t.Fatal("InstallPack returned nil error")
	}
	if !strings.Contains(err.Error(), `worker aliases "bob" and "developer" both point to hotfix:bob`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWork_CreatesFeatureFolder(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	result, err := workspace.Work(workspace.WorkOptions{Root: dir, Ticket: "TEST-001"})
	if err != nil {
		t.Fatalf("Work: %v", err)
	}

	if _, err := os.Stat(result.FeatureDir); err != nil {
		t.Errorf("feature dir not created: %v", err)
	}

	stateFile := filepath.Join(result.FeatureDir, "STATE.yaml")
	if _, err := os.Stat(stateFile); err != nil {
		t.Error("STATE.yaml not created")
	}

	// The stamped file must round-trip through the canonical schema — the old
	// hand-rolled marshal wrote stage.owner/history[].owner, which state.Load
	// silently dropped.
	st, err := state.Load(result.FeatureDir)
	if err != nil {
		t.Fatalf("loading stamped STATE.yaml: %v", err)
	}
	if st.Ticket != "TEST-001" {
		t.Errorf("ticket = %q, want TEST-001", st.Ticket)
	}
	if st.Status != "pending" {
		t.Errorf("status = %q, want pending", st.Status)
	}
	if st.Stage.Name == "" {
		t.Error("stage.name not stamped")
	}
	if len(st.History) != 1 || st.History[0].Worker != "agent" {
		t.Errorf("history not round-tripped: %+v", st.History)
	}
}

func TestWork_UsesConfiguredDefaultWorkflow(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeOrcYAML(t, dir, `
settings:
  default_workflow: hotfix

repos: []

workflows:
  default:
    stages:
      - name: intake
  hotfix:
    stages:
      - name: develop
      - name: pr-open
`)

	result, err := workspace.Work(workspace.WorkOptions{Root: dir, Ticket: "TEST-003"})
	if err != nil {
		t.Fatalf("Work: %v", err)
	}

	s, err := state.Load(result.FeatureDir)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if s.Workflow != "hotfix" {
		t.Fatalf("workflow = %q, want hotfix", s.Workflow)
	}
	if s.Stage.Name != "develop" {
		t.Fatalf("stage = %q, want develop", s.Stage.Name)
	}
}

func TestWork_ExplicitWorkflowOverridesConfiguredDefault(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	writeOrcYAML(t, dir, `
settings:
  default_workflow: hotfix

repos: []

workflows:
  default:
    stages:
      - name: intake
  hotfix:
    stages:
      - name: develop
`)

	result, err := workspace.Work(workspace.WorkOptions{
		Root:     dir,
		Ticket:   "TEST-004",
		Workflow: "default",
	})
	if err != nil {
		t.Fatalf("Work: %v", err)
	}

	s, err := state.Load(result.FeatureDir)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if s.Workflow != "default" {
		t.Fatalf("workflow = %q, want default", s.Workflow)
	}
	if s.Stage.Name != "intake" {
		t.Fatalf("stage = %q, want intake", s.Stage.Name)
	}
}

func TestWork_InvalidWorkflowDoesNotCreateFeatureDir(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	_, err := workspace.Work(workspace.WorkOptions{
		Root:     dir,
		Ticket:   "TEST-005",
		Workflow: "missing",
	})
	if err == nil {
		t.Fatal("expected missing workflow error, got nil")
	}
	if !strings.Contains(err.Error(), `workflow "missing" not found`) {
		t.Fatalf("error = %q, want missing workflow", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "features", "TEST-005")); !os.IsNotExist(statErr) {
		t.Fatalf("feature dir exists after failed Work: %v", statErr)
	}
}

func TestWork_RejectsDuplicate(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if _, err := workspace.Work(workspace.WorkOptions{Root: dir, Ticket: "TEST-001"}); err != nil {
		t.Fatalf("first Work: %v", err)
	}

	_, err := workspace.Work(workspace.WorkOptions{Root: dir, Ticket: "TEST-001"})
	if err == nil {
		t.Error("expected error for duplicate ticket, got nil")
	}
}

func TestWork_SlugOverride(t *testing.T) {
	dir := t.TempDir()

	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	result, err := workspace.Work(workspace.WorkOptions{
		Root:   dir,
		Ticket: "TEST-002",
		Slug:   "add-login",
	})
	if err != nil {
		t.Fatalf("Work: %v", err)
	}

	if result.Slug != "TEST-002-add-login" {
		t.Errorf("slug = %q, want TEST-002-add-login", result.Slug)
	}
}

func writeOrcYAML(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "orc.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
