package featurelist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cengebretson/orc/internal/featurelist"
)

func TestCollectResolvesWorkerAndTmuxState(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: develop
        worker: default:bob
        loop:
          via: code-review
          worker: default:bob
          max: 3
`)
	writeFile(t, filepath.Join(root, "workers", "default", "bob.md"), `---
id: default:bob
name: Bob Developer
engine: codex
---
`)
	writeFile(t, filepath.Join(root, "features", "TICKET-1", "STATE.yaml"), `
ticket: TICKET-1
slug: TICKET-1
status: active
stage:
  name: code-review
stage_counts:
  code-review: 2
runtime:
  tmux:
    session: TICKET-1
`)

	features, err := featurelist.Collect(root, featurelist.Options{
		TmuxAvailable: func() bool { return true },
		ListSessions:  func() []string { return []string{"TICKET-1"} },
	})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(features) != 1 {
		t.Fatalf("len(features) = %d, want 1", len(features))
	}
	f := features[0]
	if f.WorkerID != "default:bob" {
		t.Fatalf("WorkerID = %q, want default:bob", f.WorkerID)
	}
	if f.WorkerName != "Bob Developer" {
		t.Fatalf("WorkerName = %q, want Bob Developer", f.WorkerName)
	}
	if f.Workflow != "default" {
		t.Fatalf("Workflow = %q, want default", f.Workflow)
	}
	if f.StageLoopLabel != " (2/3)" {
		t.Fatalf("StageLoopLabel = %q, want (2/3)", f.StageLoopLabel)
	}
	if !f.TmuxLive {
		t.Fatal("TmuxLive = false, want true")
	}
}

func TestCollectIncludesArchivedAndLoadErrors(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "features", "_archive", "TICKET-2", "STATE.yaml"), `
ticket: TICKET-2
slug: TICKET-2
status: archived
stage:
  worker: default:bob
  name: develop
`)
	writeFile(t, filepath.Join(root, "features", "BROKEN", "STATE.yaml"), `: bad yaml`)

	features, err := featurelist.Collect(root, featurelist.Options{IncludeArchived: true})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(features) != 2 {
		t.Fatalf("len(features) = %d, want 2", len(features))
	}

	var archived, broken bool
	for _, f := range features {
		if f.Archived && f.State != nil && f.State.Ticket == "TICKET-2" {
			archived = true
		}
		if f.LoadError != nil && filepath.Base(f.FeatureDir) == "BROKEN" {
			broken = true
		}
	}
	if !archived {
		t.Fatal("archived feature not collected")
	}
	if !broken {
		t.Fatal("broken feature load error not collected")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
