package validate_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/validate"
)

func fixtureWorkspace() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "workspace")
}

func fixtureFeatureDir(ws, ticket string) string {
	entries, _ := filepath.Glob(filepath.Join(ws, "features", ticket+"*"))
	if len(entries) == 0 {
		return ""
	}
	return entries[0]
}

func TestRun_ValidTicketPasses(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	report := validate.Run(ws, featureDir)

	if report.Ticket == "" {
		t.Error("Ticket is empty")
	}
	// Core checks must pass; worktree checks may fail since test env has no real worktrees.
	coreChecks := map[string]bool{
		"STATE.yaml":              false,
		"orc.yaml":                false,
		"orc.yaml.config":         false,
		"STATE.yaml.workflow":     false,
		"STATE.yaml.stage.name":   false,
		"STATE.yaml.stage.worker": false,
	}
	for _, c := range report.Checks {
		if _, isCoreCheck := coreChecks[c.Name]; isCoreCheck {
			coreChecks[c.Name] = true
			if c.Status == validate.Fail {
				t.Errorf("core check failed: %s — %s", c.Name, c.Detail)
			}
		}
	}
	for name, seen := range coreChecks {
		if !seen {
			t.Errorf("core check %q not present in report", name)
		}
	}
}

func TestRun_ReturnsTicketID(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	report := validate.Run(ws, featureDir)

	if report.Ticket != "STORY-123" {
		t.Errorf("Ticket = %q, want STORY-123", report.Ticket)
	}
}

func TestRun_MissingFeatureDirFails(t *testing.T) {
	ws := fixtureWorkspace()
	report := validate.Run(ws, "/nonexistent/feature/dir")

	if report.OK() {
		t.Error("expected report to fail for missing feature dir")
	}
}

func TestRun_WorkflowCheckPresent(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	report := validate.Run(ws, featureDir)

	var found bool
	for _, c := range report.Checks {
		if c.Name == "STATE.yaml.workflow" {
			found = true
			if c.Status == validate.Fail {
				t.Errorf("workflow check failed: %s", c.Detail)
			}
		}
	}
	if !found {
		t.Error("workflow check not present in report")
	}
}

func TestRun_StageCheckPresent(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	report := validate.Run(ws, featureDir)

	var found bool
	for _, c := range report.Checks {
		if c.Name == "STATE.yaml.stage.name" {
			found = true
			if c.Status == validate.Fail {
				t.Errorf("stage check failed: %s", c.Detail)
			}
		}
	}
	if !found {
		t.Error("stage check not present in report")
	}
}

func TestRun_InvalidWorkspaceFailsConfig(t *testing.T) {
	report := validate.Run("/nonexistent/workspace", "/nonexistent/workspace/features/X-1")

	if report.OK() {
		t.Error("expected report to fail for invalid workspace")
	}
}

func TestRun_StateShapeFailures(t *testing.T) {
	root := writeValidateWorkspace(t)
	featureDir := writeValidateFeature(t, root, `
schema_version: 999
ticket: ""
slug: ""
status: mystery
stage:
  name: ""
  worker: missing-worker
next_action:
  worker: missing-next-worker
  cwd: .
repos: {}
`)

	report := validate.Run(root, featureDir)

	assertCheck(t, report, "STATE.yaml.schema_version", validate.Fail, "unsupported schema version")
	assertCheck(t, report, "STATE.yaml.ticket", validate.Fail, "ticket is required")
	assertCheck(t, report, "STATE.yaml.slug", validate.Fail, "slug is required")
	assertCheck(t, report, "STATE.yaml.status", validate.Fail, "not a valid status")
	assertCheck(t, report, "STATE.yaml.stage.name", validate.Fail, "stage name is required")
	assertCheck(t, report, "STATE.yaml.stage.worker", validate.Fail, "missing-worker")
	assertCheck(t, report, "STATE.yaml.next_action.worker", validate.Fail, "missing-next-worker")
}

func TestRun_UsesWorkflowWorkerWhenStateWorkerMissing(t *testing.T) {
	root := writeValidateWorkspace(t)
	featureDir := writeValidateFeature(t, root, `
schema_version: 1
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  name: intake
next_action:
  worker: default:fred
  cwd: .
repos: {}
`)

	report := validate.Run(root, featureDir)

	assertCheck(t, report, "STATE.yaml.stage.worker", validate.OK, "default:fred")
	if !report.OK() {
		for _, check := range report.Checks {
			if check.Status == validate.Fail {
				t.Errorf("unexpected failure: %s %s", check.Name, check.Detail)
			}
		}
	}
}

func TestRun_InvalidConfigBlocksTicketValidation(t *testing.T) {
	root := t.TempDir()
	writeValidateFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: intake
        worker: missing-worker
        advance: auto
`)
	writeWorker(t, root, "default:fred")
	featureDir := writeValidateFeature(t, root, `
schema_version: 1
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  name: intake
next_action:
  worker: human
  cwd: .
repos: {}
`)

	report := validate.Run(root, featureDir)

	assertCheck(t, report, "orc.yaml.workflows.default.stages[0].worker", validate.Fail, `worker "missing-worker" not found`)
}

func TestRun_RepoValidationFailure(t *testing.T) {
	root := writeValidateWorkspace(t)
	featureDir := writeValidateFeature(t, root, `
schema_version: 1
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  name: intake
  worker: default:fred
next_action:
  worker: human
  cwd: somewhere-else
repos:
  app:
    main: /definitely/not/a/repo
    worktree: outside-worktrees/TICKET-1
    branch: ""
`)

	report := validate.Run(root, featureDir)

	assertCheck(t, report, "STATE.yaml.repos.app.main", validate.Fail, "does not exist")
	assertCheck(t, report, "STATE.yaml.repos.app.worktree", validate.Fail, "is not under worktrees/")
	assertCheck(t, report, "STATE.yaml.repos.app.branch", validate.Fail, "empty but worktree is set")
	assertCheck(t, report, "STATE.yaml.next_action.cwd", validate.Fail, "does not match any recorded worktree")
}

func writeValidateWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeValidateFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: intake
        worker: default:fred
        advance: auto
      - name: develop
        worker: default:bob
        advance: manual
        loop:
          via: code-review
          worker: default:zach
          max: 3
          on_max: pause
`)
	for _, id := range []string{"default:fred", "default:bob", "default:zach"} {
		writeWorker(t, root, id)
	}
	writeValidateFile(t, filepath.Join(root, "stages", "intake.md"), "# intake\n")
	writeValidateFile(t, filepath.Join(root, "stages", "develop.md"), "# develop\n")
	writeValidateFile(t, filepath.Join(root, "stages", "code-review.md"), "# code review\n")
	return root
}

func writeWorker(t *testing.T, root, id string) {
	t.Helper()
	writeValidateFile(t, filepath.Join(root, "workers", strings.ReplaceAll(id, ":", "/")+".md"), `---
id: `+id+`
name: `+id+`
engine: claude
---
`)
}

func writeValidateFeature(t *testing.T, root, stateYAML string) string {
	t.Helper()
	featureDir := filepath.Join(root, "features", strings.ReplaceAll(t.Name(), "/", "-"))
	writeValidateFile(t, filepath.Join(featureDir, "STATE.yaml"), stateYAML)
	return featureDir
}

func writeValidateFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertCheck(t *testing.T, report *validate.Report, name string, status validate.Status, detailContains string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name != name {
			continue
		}
		if check.Status != status {
			t.Fatalf("%s status = %v, want %v (%s)", name, check.Status, status, check.Detail)
		}
		if detailContains != "" && !strings.Contains(check.Detail, detailContains) {
			t.Fatalf("%s detail = %q, want containing %q", name, check.Detail, detailContains)
		}
		return
	}
	t.Fatalf("check %q not found in %#v", name, report.Checks)
}
