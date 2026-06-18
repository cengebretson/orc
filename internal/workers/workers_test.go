package workers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cengebretson/orc/internal/workers"
)

func TestLoad(t *testing.T) {
	all, err := workers.Load(fixtureWorkersDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("loaded %d workers, want 2", len(all))
	}
}

func TestLoad_ParsesFrontmatter(t *testing.T) {
	all, err := workers.Load(fixtureWorkersDir(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	bob := findWorker(all, "custom:bob")
	if bob == nil {
		t.Fatal("custom:bob not found")
	}
	if bob.Engine != "codex" {
		t.Errorf("product = %q, want codex", bob.Engine)
	}
	if bob.Args["service_tier"] != "medium" {
		t.Errorf("args.service_tier = %q, want medium", bob.Args["service_tier"])
	}
	if bob.Args["reasoning_effort"] != "high" {
		t.Errorf("args.reasoning_effort = %q, want high", bob.Args["reasoning_effort"])
	}
}

func TestLoad_UsesNamespacedPathAsDefaultID(t *testing.T) {
	root := t.TempDir()
	writeWorker(t, root, "custom", "chris", "")

	all, err := workers.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if findWorker(all, "custom:chris") == nil {
		t.Fatal("custom:chris not found")
	}
}

func TestLoad_IgnoresRootWorkerFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "chris.md"), `---
id: custom:chris
name: Chris
engine: codex
---
`)

	all, err := workers.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("loaded %d workers, want 0", len(all))
	}
}

func TestLoad_RejectsIDThatDoesNotMatchPath(t *testing.T) {
	root := t.TempDir()
	writeWorker(t, root, "custom", "chris", "other:chris")

	_, err := workers.Load(root)
	if err == nil {
		t.Fatal("Load returned nil error")
	}
}

func TestFindByID_Found(t *testing.T) {
	all, _ := workers.Load(fixtureWorkersDir(t))

	w := workers.FindByID(all, "custom:bob")
	if w == nil {
		t.Fatal("expected custom:bob, got nil")
	}
	if w.ID != "custom:bob" {
		t.Errorf("id = %q, want custom:bob", w.ID)
	}
}

func TestFindByID_NotFound(t *testing.T) {
	all, _ := workers.Load(fixtureWorkersDir(t))

	w := workers.FindByID(all, "nonexistent-worker")
	if w != nil {
		t.Errorf("expected nil, got %q", w.ID)
	}
}

func TestLaunchCommand_Codex(t *testing.T) {
	all, _ := workers.Load(fixtureWorkersDir(t))
	bob := findWorker(all, "custom:bob")

	cmd := workers.LaunchCommand(bob, "/workspace", "/workspace/worktrees/app/FLYWL-123", "do the thing")
	if cmd == "" {
		t.Error("expected non-empty launch command")
	}
}

func TestLaunchCommand_Claude(t *testing.T) {
	all, _ := workers.Load(fixtureWorkersDir(t))
	fred := findWorker(all, "custom:fred")

	cmd := workers.LaunchCommand(fred, "/workspace", "/workspace", "do the thing")
	if cmd == "" {
		t.Error("expected non-empty launch command")
	}
}

func findWorker(list []*workers.Worker, id string) *workers.Worker {
	for _, w := range list {
		if w.ID == id {
			return w
		}
	}
	return nil
}

func fixtureWorkersDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeWorker(t, root, "custom", "bob", "custom:bob")
	writeWorker(t, root, "custom", "fred", "custom:fred")
	return root
}

func writeWorker(t *testing.T, root, namespace, name, id string) {
	t.Helper()
	if id == "" {
		id = "\n"
	} else {
		id = "id: " + id + "\n"
	}
	writeFile(t, filepath.Join(root, namespace, name+".md"), `---
`+id+`name: `+name+`
engine: codex
args:
  service_tier: medium
  reasoning_effort: high
---
`)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
