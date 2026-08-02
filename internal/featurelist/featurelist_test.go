package featurelist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/mux/muxtest"
)

func TestCollectResolvesWorkerAndTmuxState(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
workflows:
  default:standard:
    stages:
      - name: default:develop
        worker: default:bob
        loop:
          via: default:code-review
          worker: default:bob
          max: 3
aliases:
  workflows:
    default: default:standard
  stages:
    code-review: default:code-review
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
  name: default:code-review
stage_counts:
  default:code-review: 2
runtime:
  tmux:
    session: TICKET-1
`)

	features, err := featurelist.Collect(root, featurelist.Options{
		Mux: &muxtest.Fake{
			AvailableFunc:    func() bool { return true },
			ListSessionsFunc: func() []string { return []string{"TICKET-1"} },
			AttentionFunc: func(session, window string) string {
				if session != "TICKET-1" || window != "default:code-review" {
					t.Fatalf("Attention target = %s:%s", session, window)
				}
				return "review"
			},
		},
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
	if f.Workflow != "default:standard" {
		t.Fatalf("Workflow = %q, want default:standard", f.Workflow)
	}
	if f.WorkflowLabel != "default" {
		t.Fatalf("WorkflowLabel = %q, want default", f.WorkflowLabel)
	}
	if f.Stage != "default:code-review" || f.StageLabel != "code-review" {
		t.Fatalf("stage projection = %q / %q, want default:code-review / code-review", f.Stage, f.StageLabel)
	}
	if f.StageLoopLabel != " (2/3)" {
		t.Fatalf("StageLoopLabel = %q, want (2/3)", f.StageLoopLabel)
	}
	if !f.TmuxLive {
		t.Fatal("TmuxLive = false, want true")
	}
	if f.Attention != "review" {
		t.Fatalf("Attention = %q, want review", f.Attention)
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
		if f.LoadError != nil && f.HasIssues && filepath.Base(f.FeatureDir) == "BROKEN" {
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
