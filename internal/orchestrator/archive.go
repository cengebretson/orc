package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
)

type ArchiveOptions struct {
	Root       string
	FeatureDir string
	State      *state.State
}

type WorktreeRemoval struct {
	Name         string
	Main         string
	WorktreePath string
	WorktreeRel  string
	Branch       string
	Warning      string
}

type ArchiveResult struct {
	Slug             string
	Destination      string
	Worktrees        []WorktreeRemoval
	KilledTmux       bool
	TmuxSession      string
	RuntimeCleared   bool
	RuntimeClearWarn string
	TmuxKillWarn     string
}

type Archiver struct {
	RemoveWorktree func(repoMain, worktreePath string) error
	SetStatus      func(featureDir, status string) error
	MkdirAll       func(path string, perm os.FileMode) error
	Stat           func(path string) (os.FileInfo, error)
	Rename         func(oldpath, newpath string) error
	Mux            mux.Backend
	ClearRuntime   func(featureDir string) error
}

func NewArchiver() Archiver {
	return Archiver{
		RemoveWorktree: removeWorktree,
		SetStatus:      state.SetStatus,
		MkdirAll:       os.MkdirAll,
		Stat:           os.Stat,
		Rename:         os.Rename,
		Mux:            tmux.New(),
		ClearRuntime:   state.ClearRuntime,
	}
}

func (a Archiver) Archive(opts ArchiveOptions) (*ArchiveResult, error) {
	if opts.State == nil {
		return nil, fmt.Errorf("state is required")
	}
	if opts.State.Status != "done" {
		return nil, fmt.Errorf("cannot archive %q: status is %q (must be done)", opts.State.Slug, opts.State.Status)
	}
	if a.RemoveWorktree == nil {
		a.RemoveWorktree = removeWorktree
	}
	if a.SetStatus == nil {
		a.SetStatus = state.SetStatus
	}
	if a.MkdirAll == nil {
		a.MkdirAll = os.MkdirAll
	}
	if a.Stat == nil {
		a.Stat = os.Stat
	}
	if a.Rename == nil {
		a.Rename = os.Rename
	}
	if a.Mux == nil {
		a.Mux = tmux.New()
	}
	if a.ClearRuntime == nil {
		a.ClearRuntime = state.ClearRuntime
	}

	result := &ArchiveResult{
		Slug:        filepath.Base(opts.FeatureDir),
		TmuxSession: runtimeWorkspaceName(opts.State),
	}

	archiveDir := filepath.Join(opts.Root, "features", "_archive")
	if err := a.MkdirAll(archiveDir, 0755); err != nil {
		return nil, fmt.Errorf("creating _archive dir: %w", err)
	}

	dest := filepath.Join(archiveDir, result.Slug)
	if _, err := a.Stat(dest); err == nil {
		return nil, fmt.Errorf("archive destination already exists: %s", dest)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("checking archive destination: %w", err)
	}

	if err := a.SetStatus(opts.FeatureDir, "archived"); err != nil {
		return nil, fmt.Errorf("updating status: %w", err)
	}

	for name, repo := range opts.State.Repos {
		if repo.Worktree == "" {
			continue
		}
		worktreePath := filepath.Join(opts.Root, repo.Worktree)
		removed := WorktreeRemoval{
			Name:         name,
			Main:         repo.Main,
			WorktreePath: worktreePath,
			WorktreeRel:  repo.Worktree,
			Branch:       repo.Branch,
		}
		if err := a.RemoveWorktree(repo.Main, worktreePath); err != nil {
			removed.Warning = err.Error()
		}
		result.Worktrees = append(result.Worktrees, removed)
	}

	if err := a.Rename(opts.FeatureDir, dest); err != nil {
		return nil, fmt.Errorf("moving feature folder: %w", err)
	}
	result.Destination = dest

	session := runtimeWorkspaceName(opts.State)
	target, configured := opts.State.Runtime.MuxTarget(opts.State.Stage.Name)
	nativeTarget := opts.State.Runtime.Mux != nil
	backendMatches := !nativeTarget || !configured || target.Backend == "" || target.Backend == a.Mux.Name()
	if configured && !backendMatches {
		result.TmuxKillWarn = fmt.Sprintf("could not stop %s workspace %s with selected %s backend", target.Backend, session, a.Mux.Name())
	} else if a.Mux.Available() && a.Mux.SessionExists(session) {
		if err := a.Mux.KillSession(session); err != nil {
			if nativeTarget && target.Backend != "tmux" {
				result.TmuxKillWarn = fmt.Sprintf("could not stop %s workspace %s: %v", a.Mux.Name(), session, err)
			} else {
				result.TmuxKillWarn = fmt.Sprintf("could not kill tmux session %s: %v", session, err)
			}
		} else {
			result.KilledTmux = true
			result.TmuxSession = session
		}
	}

	if err := a.ClearRuntime(dest); err != nil {
		result.RuntimeClearWarn = fmt.Sprintf("could not clear runtime from STATE.yaml: %v", err)
	} else {
		result.RuntimeCleared = true
	}

	return result, nil
}

func removeWorktree(repoMain, worktreePath string) error {
	out, err := exec.Command("git", "-C", repoMain, "worktree", "remove", worktreePath, "--force").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func runtimeWorkspaceName(s *state.State) string {
	if target, ok := s.Runtime.MuxTarget(s.Stage.Name); ok {
		return target.Workspace
	}
	return s.Slug
}
