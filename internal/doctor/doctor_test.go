package doctor_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/doctor"
)

func fixtureWorkspace() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "workspace")
}

func TestRunWithOptionsRequiredToolsPresent(t *testing.T) {
	report := doctor.RunWithOptions(fixtureWorkspace(), doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	if !report.OK() {
		for _, c := range report.Checks {
			if c.Status == doctor.Fail {
				t.Errorf("unexpected failure: %s/%s %s", c.Group, c.Name, c.Detail)
			}
		}
	}
}

func TestRunWithOptionsMissingRequiredWorkerEngineFails(t *testing.T) {
	report := doctor.RunWithOptions(fixtureWorkspace(), doctor.Options{
		LookPath: func(name string) (string, error) {
			if name == "codex" {
				return "", errors.New("missing")
			}
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "tools", "codex")
	if check == nil {
		t.Fatal("codex check not found")
	}
	if check.Status != doctor.Fail {
		t.Fatalf("codex status = %v, want Fail", check.Status)
	}
	if report.OK() {
		t.Fatal("report should fail when required worker engine is missing")
	}
}

func TestRunWithOptionsMissingTmuxWarns(t *testing.T) {
	report := doctor.RunWithOptions(fixtureWorkspace(), doctor.Options{
		LookPath: func(name string) (string, error) {
			if name == "tmux" {
				return "", errors.New("missing")
			}
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "tools", "tmux")
	if check == nil {
		t.Fatal("tmux check not found")
	}
	if check.Status != doctor.Warning {
		t.Fatalf("tmux status = %v, want Warning", check.Status)
	}
	if !report.OK() {
		t.Fatal("report should remain OK when only tmux is missing")
	}
}

func TestRunSystemWithOptionsReportsInstallReadiness(t *testing.T) {
	report := doctor.RunSystemWithOptions(doctor.Options{
		Version: "1.2.3",
		LookPath: func(name string) (string, error) {
			switch name {
			case "orc":
				return "/usr/local/bin/orc", nil
			case "tmux", "chafa", "claude", "codex", "cursor":
				return "", errors.New("missing")
			default:
				return "", errors.New("unexpected lookup")
			}
		},
	})

	if report.Label != "System" {
		t.Fatalf("report label = %q, want System", report.Label)
	}

	if check := findCheck(report, "install", "version"); check == nil || check.Detail != "1.2.3" || check.Status != doctor.OK {
		t.Fatalf("version check = %#v, want ok version 1.2.3", check)
	}
	if check := findCheck(report, "install", "orc"); check == nil || check.Status != doctor.OK {
		t.Fatalf("orc check = %#v, want ok", check)
	}
	for _, name := range []string{"tmux", "chafa", "claude", "codex", "cursor"} {
		check := findCheck(report, "tools", name)
		if check == nil {
			t.Fatalf("%s check not found", name)
		}
		if check.Status != doctor.Warning {
			t.Fatalf("%s status = %v, want Warning", name, check.Status)
		}
	}
	if !report.OK() {
		t.Fatal("system report should remain OK when only optional tools are missing")
	}
}

func TestRunWithOptionsReportsNoStateLocks(t *testing.T) {
	report := doctor.RunWithOptions(fixtureWorkspace(), doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "state locks", "STATE.yaml.lock")
	if check == nil {
		t.Fatal("state lock check not found")
	}
	if check.Status != doctor.OK {
		t.Fatalf("state lock status = %v, want OK", check.Status)
	}
	if check.Detail != "none found" {
		t.Fatalf("state lock detail = %q, want none found", check.Detail)
	}
}

func TestRunWithOptionsReportsValidConfig(t *testing.T) {
	report := doctor.RunWithOptions(fixtureWorkspace(), doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "config", "orc.yaml")
	if check == nil {
		t.Fatal("config check not found")
	}
	if check.Status != doctor.OK {
		t.Fatalf("config status = %v, want OK: %s", check.Status, check.Detail)
	}
	if check.Detail != "valid" {
		t.Fatalf("config detail = %q, want valid", check.Detail)
	}
}

func TestRunWithOptionsWarnsWhenWorktreeSetupOmitsWorktreePath(t *testing.T) {
	root := t.TempDir()
	writeDoctorFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
repos:
  - name: app
    path: ../app
    purpose: Application
    worktree_setup: "../app/setup-worktree.sh -b {{branch}}"
workflows:
  default:
    stages:
      - name: develop
        worker: default:bob
        advance: auto
`)
	writeDoctorFile(t, filepath.Join(root, "workers", "default", "bob.md"), `---
id: default:bob
name: Bob
engine: codex
---
`)

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "config", "repos.app.worktree_setup")
	if check == nil {
		t.Fatal("worktree_setup warning not found")
	}
	if check.Status != doctor.Warning {
		t.Fatalf("status = %v, want Warning", check.Status)
	}
	if check.Detail != "does not include {{worktree_path}}; command may create a worktree outside orc state" {
		t.Fatalf("detail = %q", check.Detail)
	}
}

func TestRunWithOptionsChecksWorktreeSetupReadiness(t *testing.T) {
	root := t.TempDir()
	writeDoctorFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
repos:
  - name: app
    path: ../app
    purpose: Application
    worktree_setup: "../app/setup-worktree.sh -b {{branch}} --path {{worktree_path}}"
workflows:
  default:
    stages:
      - name: develop
        worker: default:bob
        advance: auto
`)
	writeDoctorFile(t, filepath.Join(root, "workers", "default", "bob.md"), `---
id: default:bob
name: Bob
engine: codex
---
`)

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	command := findCheck(report, "config", "repos.app.worktree_setup.command")
	if command == nil {
		t.Fatal("worktree_setup command check not found")
	}
	if command.Status != doctor.Warning || command.Detail != "command path not found: ../app/setup-worktree.sh" {
		t.Fatalf("command check = %#v, want missing path warning", command)
	}

	hints := findCheck(report, "config", "repos.app.agent_hints")
	if hints == nil {
		t.Fatal("agent_hints warning not found")
	}
	if hints.Status != doctor.Warning {
		t.Fatalf("agent_hints status = %v, want Warning", hints.Status)
	}

	worktrees := findCheck(report, "config", "worktrees/")
	if worktrees == nil {
		t.Fatal("worktrees readiness warning not found")
	}
	if worktrees.Status != doctor.Warning {
		t.Fatalf("worktrees status = %v, want Warning", worktrees.Status)
	}
}

func TestRunWithOptionsAcceptsExecutableWorktreeSetup(t *testing.T) {
	root := t.TempDir()
	setupPath := filepath.Join(root, "scripts", "setup-worktree.sh")
	writeDoctorFile(t, setupPath, "#!/usr/bin/env bash\n")
	if err := os.Chmod(setupPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "worktrees"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeDoctorFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
repos:
  - name: app
    path: ../app
    purpose: Application
    agent_hints:
      - Run make test.
    worktree_setup: "scripts/setup-worktree.sh -b {{branch}} --path {{worktree_path}}"
workflows:
  default:
    stages:
      - name: develop
        worker: default:bob
        advance: auto
`)
	writeDoctorFile(t, filepath.Join(root, "workers", "default", "bob.md"), `---
id: default:bob
name: Bob
engine: codex
---
`)

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	command := findCheck(report, "config", "repos.app.worktree_setup.command")
	if command == nil {
		t.Fatal("worktree_setup command check not found")
	}
	if command.Status != doctor.OK || command.Detail != "command found: scripts/setup-worktree.sh" {
		t.Fatalf("command check = %#v, want executable ok", command)
	}
	if check := findCheck(report, "config", "repos.app.agent_hints"); check != nil {
		t.Fatalf("agent_hints warning should not appear when hints exist: %#v", check)
	}
	if check := findCheck(report, "config", "worktrees/"); check != nil {
		t.Fatalf("config worktrees warning should not appear when directory exists: %#v", check)
	}
}

func TestRunWithOptionsWarnsWhenRecordedWorktreeMissing(t *testing.T) {
	root := t.TempDir()
	writeDoctorFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: develop
        worker: default:bob
        advance: auto
`)
	writeDoctorFile(t, filepath.Join(root, "workers", "default", "bob.md"), `---
id: default:bob
name: Bob
engine: codex
---
`)
	writeDoctorFile(t, filepath.Join(root, "features", "TICKET-1", "STATE.yaml"), `
schema_version: 1
ticket: TICKET-1
slug: TICKET-1
status: active
stage:
  name: develop
repos:
  app:
    worktree: worktrees/app/TICKET-1
    branch: feature/ticket-1
next_action:
  worker: human
  cwd: .
`)

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "feature state", "TICKET-1.app")
	if check == nil {
		t.Fatal("feature worktree warning not found")
	}
	if check.Status != doctor.Warning {
		t.Fatalf("status = %v, want Warning", check.Status)
	}
	if check.Detail != "recorded worktree missing: worktrees/app/TICKET-1" {
		t.Fatalf("detail = %q", check.Detail)
	}
}

func TestRunWithOptionsReportsInvalidConfig(t *testing.T) {
	root := t.TempDir()
	writeDoctorFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: intake
        worker: missing-worker
        advance: auto
`)
	writeDoctorFile(t, filepath.Join(root, "workers", "default", "fred.md"), `---
id: default:fred
name: Fred
engine: claude
---
`)

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "config", "workflows.default.stages[0].worker")
	if check == nil {
		t.Fatal("invalid config check not found")
	}
	if check.Status != doctor.Fail {
		t.Fatalf("config status = %v, want Fail", check.Status)
	}
	if check.Detail != `worker "missing-worker" not found in workers/` {
		t.Fatalf("config detail = %q", check.Detail)
	}
	if report.OK() {
		t.Fatal("report should fail when config is invalid")
	}
}

func TestRunWithOptionsReportsDuplicateAliasTargets(t *testing.T) {
	root := t.TempDir()
	writeDoctorFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default:standard
workflows:
  default:standard:
    stages:
      - name: default:develop
        worker: default:bob
        advance: auto
aliases:
  stages:
    develop: default:develop
    dev: default:develop
`)
	writeDoctorFile(t, filepath.Join(root, "workers", "default", "bob.md"), `---
id: default:bob
name: Bob
engine: codex
---
`)

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "config", "aliases.stages.develop")
	if check == nil {
		t.Fatal("duplicate alias target check not found")
	}
	if check.Status != doctor.Fail {
		t.Fatalf("config status = %v, want Fail", check.Status)
	}
	if check.Detail != `alias target "default:develop" is already used by alias "dev"` {
		t.Fatalf("config detail = %q", check.Detail)
	}
}

func TestRunWithOptionsReportsStaleStateLock(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
	})

	check := findCheck(report, "state locks", "TICKET-1")
	if check == nil {
		t.Fatal("stale state lock check not found")
	}
	if check.Status != doctor.Warning {
		t.Fatalf("state lock status = %v, want Warning", check.Status)
	}
	if check.Detail != "old lock without a valid PID — will be recovered on next state write" {
		t.Fatalf("state lock detail = %q", check.Detail)
	}
}

func TestRunWithOptionsFixRemovesStaleLock(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
		Fix: true,
	})

	check := findCheck(report, "state locks", "TICKET-1")
	if check == nil {
		t.Fatal("state lock check not found")
	}
	if check.Status != doctor.OK {
		t.Fatalf("state lock status = %v, want OK: %s", check.Status, check.Detail)
	}
	if check.Detail != "old lock without a valid PID — stale lock removed" {
		t.Fatalf("state lock detail = %q", check.Detail)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock should be gone, stat err = %v", err)
	}
}

func TestRunWithOptionsFixKeepsLiveLock(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	report := doctor.RunWithOptions(root, doctor.Options{
		LookPath: func(name string) (string, error) {
			return "/bin/" + name, nil
		},
		Fix: true,
	})

	check := findCheck(report, "state locks", "TICKET-1")
	if check == nil {
		t.Fatal("state lock check not found")
	}
	if check.Status != doctor.Warning {
		t.Fatalf("state lock status = %v, want Warning: %s", check.Status, check.Detail)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("live lock should remain: %v", err)
	}
}

func findCheck(report *doctor.Report, group, name string) *doctor.Check {
	for i := range report.Checks {
		if report.Checks[i].Group == group && report.Checks[i].Name == name {
			return &report.Checks[i]
		}
	}
	return nil
}

func writeDoctorFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
