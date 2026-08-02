package ticketview_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cengebretson/orc/internal/mux/muxtest"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticketview"
)

func TestBuildResolvesWorkerWorkflowNextAndRuntime(t *testing.T) {
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
          worker: default:zach
          max: 3
      - name: default:qa
        worker: default:brian
        advance: manual
aliases:
  workflows:
    default: default:standard
  stages:
    code-review: default:code-review
    qa: default:qa
`)
	writeFile(t, filepath.Join(root, "workers", "default", "zach.md"), `---
id: default:zach
name: Zach Reviewer
engine: codex
model: gpt-5
---
`)
	s := &state.State{
		Ticket:      "TICKET-1",
		Slug:        "TICKET-1",
		Status:      "active",
		Stage:       state.Stage{Name: "default:code-review"},
		StageCounts: map[string]int{"default:code-review": 2},
		Runtime: state.Runtime{
			Tmux: &state.TmuxRuntime{Session: "TICKET-1"},
		},
	}

	summary := ticketview.Build(root, filepath.Join(root, "features", "TICKET-1"), s, ticketview.Options{
		Mux: &muxtest.Fake{
			AvailableFunc:     func() bool { return true },
			SessionExistsFunc: func(session string) bool { return session == "TICKET-1" },
		},
	})

	if summary.Workflow != "default:standard" {
		t.Fatalf("Workflow = %q, want default:standard", summary.Workflow)
	}
	if summary.WorkerID != "default:zach" {
		t.Fatalf("WorkerID = %q, want default:zach", summary.WorkerID)
	}
	if summary.WorkerName != "Zach Reviewer" {
		t.Fatalf("WorkerName = %q, want Zach Reviewer", summary.WorkerName)
	}
	if summary.WorkerEngine != "codex" || summary.WorkerModel != "gpt-5" {
		t.Fatalf("worker metadata = %q/%q", summary.WorkerEngine, summary.WorkerModel)
	}
	if summary.StageLoopLabel != " (2/3)" {
		t.Fatalf("StageLoopLabel = %q, want (2/3)", summary.StageLoopLabel)
	}
	if summary.TmuxAttachHint != "TICKET-1:default:code-review" || !summary.TmuxLive {
		t.Fatalf("tmux = live:%v hint:%q", summary.TmuxLive, summary.TmuxAttachHint)
	}
}

func TestBuildSummarizesPausedAndDeadTmux(t *testing.T) {
	root := t.TempDir()
	s := &state.State{
		Ticket: "TICKET-1",
		Slug:   "TICKET-1",
		Status: "paused",
		Stage:  state.Stage{Worker: "default:bob", Name: "develop"},
		Runtime: state.Runtime{
			Tmux: &state.TmuxRuntime{Session: "TICKET-1"},
		},
		History: []state.HistoryEntry{{Result: "paused for review"}},
	}

	summary := ticketview.Build(root, filepath.Join(root, "features", "TICKET-1"), s, ticketview.Options{
		Mux: &muxtest.Fake{},
	})

	if summary.PausedReason != "paused for review" {
		t.Fatalf("PausedReason = %q", summary.PausedReason)
	}
	if !summary.TmuxConfigured || summary.TmuxAvailable || summary.TmuxLive {
		t.Fatalf("tmux configured/available/live = %v/%v/%v", summary.TmuxConfigured, summary.TmuxAvailable, summary.TmuxLive)
	}
	if summary.TmuxRestart != "run `orc next TICKET-1` to restart" {
		t.Fatalf("TmuxRestart = %q", summary.TmuxRestart)
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
