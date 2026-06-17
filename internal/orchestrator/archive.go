package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	TmuxAvailable  func() bool
	SessionExists  func(string) bool
	KillSession    func(string) error
	ClearRuntime   func(featureDir string) error
}

func NewArchiver() Archiver {
	return Archiver{
		RemoveWorktree: removeWorktree,
		SetStatus:      state.SetStatus,
		MkdirAll:       os.MkdirAll,
		Stat:           os.Stat,
		Rename:         os.Rename,
		TmuxAvailable:  tmux.Available,
		SessionExists:  tmux.SessionExists,
		KillSession:    tmux.KillSession,
		ClearRuntime:   state.ClearRuntime,
	}
}

func Archive(opts ArchiveOptions) (*ArchiveResult, error) {
	archiver := NewArchiver()
	return archiver.Archive(opts)
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
	if a.TmuxAvailable == nil {
		a.TmuxAvailable = tmux.Available
	}
	if a.SessionExists == nil {
		a.SessionExists = tmux.SessionExists
	}
	if a.KillSession == nil {
		a.KillSession = tmux.KillSession
	}
	if a.ClearRuntime == nil {
		a.ClearRuntime = state.ClearRuntime
	}

	result := &ArchiveResult{
		Slug:        filepath.Base(opts.FeatureDir),
		TmuxSession: tmuxSessionName(opts.State),
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

	session := tmuxSessionName(opts.State)
	if a.TmuxAvailable() && a.SessionExists(session) {
		if err := a.KillSession(session); err != nil {
			result.TmuxKillWarn = fmt.Sprintf("could not kill tmux session %s: %v", session, err)
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

func tmuxSessionName(s *state.State) string {
	if s.Runtime.Tmux != nil && s.Runtime.Tmux.Session != "" {
		return s.Runtime.Tmux.Session
	}
	return s.Slug
}
