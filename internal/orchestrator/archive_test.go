package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cengebretson/orc/internal/mux/muxtest"
	"github.com/cengebretson/orc/internal/state"
)

func TestArchiverArchivesFeatureAndCleansRuntime(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	s := &state.State{
		Slug:   "TICKET-1",
		Status: "done",
		Runtime: state.Runtime{
			Tmux: &state.TmuxRuntime{Session: "custom-session"},
		},
		Repos: map[string]state.Repo{
			"app": {
				Main:     filepath.Join(root, "app"),
				Worktree: "worktrees/app/TICKET-1",
				Branch:   "feature/TICKET-1",
			},
		},
	}

	var removed []string
	var renamed []string
	var killedSession string
	archiver := Archiver{
		RemoveWorktree: func(repoMain, worktreePath string) error {
			removed = []string{repoMain, worktreePath}
			return nil
		},
		SetStatus: func(featureDir, status string) error {
			if status != "archived" {
				t.Fatalf("status = %q, want archived", status)
			}
			return nil
		},
		MkdirAll: func(path string, perm os.FileMode) error {
			return os.MkdirAll(path, perm)
		},
		Rename: func(oldpath, newpath string) error {
			renamed = []string{oldpath, newpath}
			return nil
		},
		Mux: &muxtest.Fake{
			AvailableFunc:     func() bool { return true },
			SessionExistsFunc: func(session string) bool { return session == "custom-session" },
			KillSessionFunc: func(session string) error {
				killedSession = session
				return nil
			},
		},
		ClearRuntime: func(featureDir string) error { return nil },
	}

	result, err := archiver.Archive(ArchiveOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
	})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}

	if !reflect.DeepEqual(removed, []string{filepath.Join(root, "app"), filepath.Join(root, "worktrees/app/TICKET-1")}) {
		t.Errorf("removed = %#v", removed)
	}
	wantDest := filepath.Join(root, "features", "_archive", "TICKET-1")
	if !reflect.DeepEqual(renamed, []string{featureDir, wantDest}) {
		t.Errorf("renamed = %#v", renamed)
	}
	if result.Destination != wantDest {
		t.Errorf("Destination = %q, want %q", result.Destination, wantDest)
	}
	if !result.KilledTmux {
		t.Error("KilledTmux = false, want true")
	}
	if killedSession != "custom-session" {
		t.Fatalf("killedSession = %q, want custom-session", killedSession)
	}
	if result.TmuxSession != "custom-session" {
		t.Fatalf("TmuxSession = %q, want custom-session", result.TmuxSession)
	}
	if !result.RuntimeCleared {
		t.Error("RuntimeCleared = false, want true")
	}
}

func TestArchiverKeepsGoingAfterCleanupWarnings(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	s := &state.State{
		Slug:   "TICKET-1",
		Status: "done",
		Repos: map[string]state.Repo{
			"app": {Main: "/repo", Worktree: "worktrees/app/TICKET-1", Branch: "feature/TICKET-1"},
		},
	}

	archiver := Archiver{
		RemoveWorktree: func(repoMain, worktreePath string) error {
			return errors.New("worktree busy")
		},
		SetStatus: func(featureDir, status string) error { return nil },
		MkdirAll:  func(path string, perm os.FileMode) error { return nil },
		Rename:    func(oldpath, newpath string) error { return nil },
		Mux: &muxtest.Fake{
			AvailableFunc:     func() bool { return true },
			SessionExistsFunc: func(session string) bool { return true },
			KillSessionFunc: func(session string) error {
				return errors.New("tmux busy")
			},
		},
		ClearRuntime: func(featureDir string) error {
			return errors.New("state busy")
		},
	}

	result, err := archiver.Archive(ArchiveOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
	})
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if len(result.Worktrees) != 1 || result.Worktrees[0].Warning != "worktree busy" {
		t.Fatalf("Worktrees = %#v", result.Worktrees)
	}
	if result.TmuxKillWarn != "could not kill tmux session TICKET-1: tmux busy" {
		t.Fatalf("TmuxKillWarn = %q", result.TmuxKillWarn)
	}
	if result.RuntimeClearWarn != "could not clear runtime from STATE.yaml: state busy" {
		t.Fatalf("RuntimeClearWarn = %q", result.RuntimeClearWarn)
	}
}

func TestArchiverFailsBeforeMoveWhenStatusCannotBeSet(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	s := &state.State{Slug: "TICKET-1", Status: "done"}

	var removed bool
	var renamed bool
	archiver := Archiver{
		RemoveWorktree: func(repoMain, worktreePath string) error {
			removed = true
			return nil
		},
		SetStatus: func(featureDir, status string) error {
			return errors.New("readonly")
		},
		Rename: func(oldpath, newpath string) error {
			renamed = true
			return nil
		},
	}

	_, err := archiver.Archive(ArchiveOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
	})
	if err == nil {
		t.Fatal("expected archive error")
	}
	if removed {
		t.Fatal("worktree was removed despite status failure")
	}
	if renamed {
		t.Fatal("feature was moved despite status failure")
	}
}

func TestArchiverRejectsNonDoneTicket(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	s := &state.State{Slug: "TICKET-1", Status: "active"}

	var removed bool
	archiver := Archiver{
		RemoveWorktree: func(repoMain, worktreePath string) error {
			removed = true
			return nil
		},
	}

	_, err := archiver.Archive(ArchiveOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
	})
	if err == nil {
		t.Fatal("expected archive refusal")
	}
	if removed {
		t.Fatal("worktree was removed despite non-done status")
	}
}

func TestArchiverFailsBeforeCleanupWhenDestinationExists(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	dest := filepath.Join(root, "features", "_archive", "TICKET-1")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	s := &state.State{
		Slug:   "TICKET-1",
		Status: "done",
		Repos: map[string]state.Repo{
			"app": {Main: "/repo", Worktree: "worktrees/app/TICKET-1", Branch: "feature/TICKET-1"},
		},
	}

	var removed bool
	var statusSet bool
	var renamed bool
	archiver := Archiver{
		RemoveWorktree: func(repoMain, worktreePath string) error {
			removed = true
			return nil
		},
		SetStatus: func(featureDir, status string) error {
			statusSet = true
			return nil
		},
		Rename: func(oldpath, newpath string) error {
			renamed = true
			return nil
		},
	}

	_, err := archiver.Archive(ArchiveOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
	})
	if err == nil {
		t.Fatal("expected destination error")
	}
	if removed || statusSet || renamed {
		t.Fatalf("mutated despite destination error: removed=%v statusSet=%v renamed=%v", removed, statusSet, renamed)
	}
}
