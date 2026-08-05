package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspace"
)

func TestSelectRunWorkerNonInteractiveListsAvailableWorkers(t *testing.T) {
	resetCommandGlobals(t)
	runInputIsTTY = func() bool { return false }
	available := []*workers.Worker{{ID: "default:bob"}, {ID: "default:ada"}}

	_, err := selectRunWorker("", available)
	if err == nil {
		t.Fatal("selectRunWorker succeeded without --worker in non-interactive mode")
	}
	for _, want := range []string{"non-interactive", "default:ada, default:bob", "--worker <id>"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err, want)
		}
	}
}

func TestSelectRunWorkerPromptsWhenOmitted(t *testing.T) {
	resetCommandGlobals(t)
	runInputIsTTY = func() bool { return true }
	runChoose = func(title string, choices []runChoice) (string, error) {
		if title != "Choose a worker:" || len(choices) != 2 {
			t.Fatalf("picker = %q, %+v", title, choices)
		}
		if choices[0].Value != "default:ada" || choices[1].Value != "default:bob" {
			t.Fatalf("worker choices are not sorted: %+v", choices)
		}
		return "default:bob", nil
	}

	got, err := selectRunWorker("", []*workers.Worker{
		{ID: "default:bob", Name: "Bob", Engine: "codex", Model: "gpt-5"},
		{ID: "default:ada", Name: "Ada", Engine: "claude", Model: "sonnet"},
	})
	if err != nil || got != "default:bob" {
		t.Fatalf("selectRunWorker = %q, %v", got, err)
	}
}

func TestSelectRunRepositoryPromptsWithWorkspaceRootChoice(t *testing.T) {
	resetCommandGlobals(t)
	root := t.TempDir()
	for _, name := range []string{"api", "web"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	runInputIsTTY = func() bool { return true }
	runChoose = func(title string, choices []runChoice) (string, error) {
		if title != "Choose a repository:" || len(choices) != 3 {
			t.Fatalf("picker = %q, %+v", title, choices)
		}
		if choices[0].Value != "" || choices[0].Label != "Workspace root" {
			t.Fatalf("first choice = %+v", choices[0])
		}
		return "", nil
	}

	selected, err := selectRunRepositoryForCommand(root, root, "", []config.Repo{
		{Name: "web", Path: "web"},
		{Name: "api", Path: "api"},
	})
	if err != nil || selected != nil {
		t.Fatalf("repository selection = %+v, %v", selected, err)
	}
}

func TestRunLocalCancellationDoesNotAllocateLocalID(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: globalWorkspace}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	runInputIsTTY = func() bool { return true }
	runChoose = func(string, []runChoice) (string, error) {
		return "", errRunSelectionCancelled
	}

	out, err := captureStdout(func() error {
		return runLocal(nil, []string{"Investigate the timeout"})
	})
	if err != nil || !strings.Contains(out, "Cancelled.") {
		t.Fatalf("runLocal output/error = %q, %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(globalWorkspace, ".local-sequence")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("local sequence exists after cancellation: %v", err)
	}
}
