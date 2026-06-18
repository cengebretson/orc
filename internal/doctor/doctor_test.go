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
