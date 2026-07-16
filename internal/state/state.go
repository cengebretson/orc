package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

func timeNow() string {
	return time.Now().Format(time.RFC3339)
}

const Filename = "STATE.yaml"

const SchemaVersion = 1

const lockTimeout = 5 * time.Second

const staleLockAge = 30 * time.Second

type LockStatus int

const (
	LockMissing LockStatus = iota
	LockActive
	LockStale
)

type LockInfo struct {
	Path   string
	Status LockStatus
	PID    int
	Age    time.Duration
	Detail string
}

type State struct {
	SchemaVersion int    `yaml:"schema_version,omitempty"`
	Ticket        string `yaml:"ticket"`
	Slug          string `yaml:"slug"`
	Status        string `yaml:"status"`
	Workflow      string `yaml:"workflow,omitempty"`

	Stage Stage `yaml:"stage"`

	StageCounts map[string]int `yaml:"stage_counts,omitempty"`

	Runtime Runtime `yaml:"runtime,omitempty"`

	Repos map[string]Repo `yaml:"repos"`

	Inputs  IOSet `yaml:"inputs"`
	Outputs IOSet `yaml:"outputs"`

	NextAction NextAction `yaml:"next_action"`

	History []HistoryEntry `yaml:"history"`
}

type Runtime struct {
	Tmux *TmuxRuntime `yaml:"tmux,omitempty"`
	JIT  *JITRuntime  `yaml:"jit,omitempty"`
}

type TmuxRuntime struct {
	Session string `yaml:"session"`
	Pane    string `yaml:"pane,omitempty"`
}

type JITRuntime struct {
	Worker    string `yaml:"worker"`
	Task      string `yaml:"task"`
	StartedAt string `yaml:"started_at"`
}

type Stage struct {
	Worker string `yaml:"worker"`
	Name   string `yaml:"name"`
}

type Repo struct {
	Main     string `yaml:"main"`
	Worktree string `yaml:"worktree"`
	Branch   string `yaml:"branch"`
}

type IOSet struct {
	Ready     []string `yaml:"ready"`
	Required  []string `yaml:"required"`
	Completed []string `yaml:"completed"`
}

type NextAction struct {
	Worker string `yaml:"worker"`
	Prompt string `yaml:"prompt"`
	CWD    string `yaml:"cwd"`
}

type HistoryEntry struct {
	At     string `yaml:"at"`
	Stage  string `yaml:"stage"`
	Worker string `yaml:"worker"`
	Result string `yaml:"result"`
}

// Load reads STATE.yaml from the given feature directory.
func Load(featureDir string) (*State, error) {
	path := filepath.Join(featureDir, Filename)
	return loadPath(path)
}

func loadPath(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var s State
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}

	return &s, nil
}

// Update loads STATE.yaml, applies mutate, and writes the file back atomically.
func Update(featureDir string, mutate func(*State) error) error {
	path := filepath.Join(featureDir, Filename)
	unlock, err := lockPath(path)
	if err != nil {
		return err
	}
	defer unlock()

	s, err := loadPath(path)
	if err != nil {
		return err
	}
	if err := mutate(s); err != nil {
		return err
	}
	return savePath(path, s)
}

// Create writes a fresh STATE.yaml in featureDir through the same locked,
// atomic temp-file path as Update. Used when scaffolding a feature, where the
// file holds template placeholders (or does not exist) and there is no prior
// state to read.
func Create(featureDir string, s *State) error {
	path := filepath.Join(featureDir, Filename)
	unlock, err := lockPath(path)
	if err != nil {
		return err
	}
	defer unlock()
	return savePath(path, s)
}

func savePath(path string, s *State) error {
	out, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+Filename+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp state file: %w", err)
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp state file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	cleanup = false
	return nil
}

func lockPath(path string) (func(), error) {
	lockName := path + ".lock"
	deadline := time.Now().Add(lockTimeout)
	for {
		f, err := os.OpenFile(lockName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(lockName) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("creating state lock: %w", err)
		}
		if removed, err := removeStaleLock(lockName); err != nil {
			return nil, err
		} else if removed {
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for %s — if no orc process is running, remove the lock and retry", lockName)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func InspectLock(featureDir string) (LockInfo, error) {
	return inspectLockPath(filepath.Join(featureDir, Filename+".lock"))
}

// ClearStaleLock removes the feature's state lock only when it is provably
// stale — dead PID, or old without a valid PID. It reports whether a lock was
// removed; live or ambiguous locks are left untouched.
func ClearStaleLock(featureDir string) (bool, error) {
	lockName := filepath.Join(featureDir, Filename+".lock")
	lock, err := inspectLockPath(lockName)
	if err != nil {
		return false, err
	}
	if lock.Status != LockStale {
		return false, nil
	}
	if err := os.Remove(lockName); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("removing stale state lock %s: %w", lockName, err)
	}
	return true, nil
}

func removeStaleLock(lockName string) (bool, error) {
	lock, err := inspectLockPath(lockName)
	if err != nil {
		return false, err
	}
	if lock.Status == LockMissing {
		return true, nil
	}
	if lock.Status != LockStale {
		return false, nil
	}
	if err := os.Remove(lockName); err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("removing stale state lock %s: %w", lockName, err)
	}
	return true, nil
}

func inspectLockPath(lockName string) (LockInfo, error) {
	lock := LockInfo{Path: lockName, Status: LockActive}
	info, err := os.Stat(lockName)
	if err != nil {
		if os.IsNotExist(err) {
			lock.Status = LockMissing
			lock.Detail = "not present"
			return lock, nil
		}
		return lock, fmt.Errorf("checking state lock: %w", err)
	}

	lock.Age = time.Since(info.ModTime())
	data, err := os.ReadFile(lockName)
	if err != nil {
		lock.Detail = "cannot read PID"
		if lock.Age > staleLockAge {
			lock.Status = LockStale
			lock.Detail = "old lock with unreadable PID"
		}
		return lock, nil
	}

	pidText := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidText)
	if err != nil || pid <= 0 {
		lock.Detail = "lock exists without a valid PID"
		if lock.Age > staleLockAge {
			lock.Status = LockStale
			lock.Detail = "old lock without a valid PID"
		}
		return lock, nil
	}
	lock.PID = pid
	if processExists(pid) {
		lock.Detail = fmt.Sprintf("held by pid %d", pid)
		return lock, nil
	}
	lock.Status = LockStale
	lock.Detail = fmt.Sprintf("pid %d is not running", pid)
	return lock, nil
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// Start marks the feature as active and records a history entry.
func Start(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: "started",
		})
		s.Status = "active"
		return nil
	})
}

// Resume marks a paused feature as active again and records the continuation.
// It clears the human-directed NextAction that Pause sets so the agent can write fresh context.
func Resume(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: "resumed",
		})
		s.Status = "active"
		s.NextAction = NextAction{}
		return nil
	})
}

// Pause marks the feature as paused (waiting for human input or external blocker).
func Pause(featureDir, reason string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: "paused — " + reason,
		})

		s.Status = "paused"
		s.NextAction.Worker = "human"
		s.NextAction.Prompt = reason
		return nil
	})
}

// Next advances the feature to the next stage, records a history entry, and saves STATE.yaml.
// When stageName is empty (no stages remain), status is set to "done".
func Next(featureDir, stageName, worker, result string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: result,
		})

		if stageName != "" {
			s.Stage.Name = stageName
			if s.StageCounts == nil {
				s.StageCounts = map[string]int{}
			}
			s.StageCounts[stageName]++
			s.Status = "pending"
		} else {
			s.Status = "done"
		}
		if worker != "" {
			s.Stage.Worker = worker
		}
		s.NextAction = NextAction{}
		return nil
	})
}

// Done marks the feature as done (all stages complete or explicitly closed).
func Done(featureDir, result string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: result,
		})
		s.Status = "done"
		s.NextAction = NextAction{}
		return nil
	})
}

// SetStatus updates only the status field in STATE.yaml, preserving all other content.
func SetStatus(featureDir, status string) error {
	return Update(featureDir, func(s *State) error {
		s.Status = status
		return nil
	})
}

// AppendHistory loads STATE.yaml, appends a history entry, and saves — no other fields touched.
func AppendHistory(featureDir, stage, workerID, result string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  stage,
			Worker: workerID,
			Result: result,
		})
		return nil
	})
}

// FindFeatureDir locates the feature directory for the given slug or ticket ID.
// Supports full slug match or prefix match on ticket ID (e.g. "FLYWL-123").
func FindFeatureDir(workspaceRoot, query string) (string, error) {
	featuresDir := filepath.Join(workspaceRoot, "features")
	if _, err := os.ReadDir(featuresDir); err != nil {
		return "", fmt.Errorf("reading features/: %w", err)
	}
	notFound := func(q string) error {
		return fmt.Errorf("no feature found matching %q — create one with `orc work %s`", q, q)
	}
	return matchFeature(query, notFound, featuresDir)
}

// FindFeatureDirWithArchive searches both features/ and features/_archive/ for a ticket match.
func FindFeatureDirWithArchive(workspaceRoot, query string) (string, error) {
	featuresDir := filepath.Join(workspaceRoot, "features")
	notFound := func(q string) error {
		return fmt.Errorf("no feature found matching %q", q)
	}
	return matchFeature(query, notFound, featuresDir, filepath.Join(featuresDir, "_archive"))
}

// matchFeature scans the given directories for a feature folder whose name
// equals or is prefixed by query (case-insensitive), skipping the _template and
// _archive scaffolding dirs. notFound builds the error for the zero-match case
// from the normalized query. Unreadable dirs are skipped.
func matchFeature(query string, notFound func(q string) error, dirs ...string) (string, error) {
	query = strings.ToUpper(strings.TrimSpace(query))

	var matches []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "_template" || e.Name() == "_archive" {
				continue
			}
			upper := strings.ToUpper(e.Name())
			if upper == query || strings.HasPrefix(upper, query) {
				matches = append(matches, filepath.Join(dir, e.Name()))
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", notFound(query)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = filepath.Base(m)
		}
		return "", fmt.Errorf("ambiguous slug %q matches multiple features:\n  %s\nUse the full slug", query, strings.Join(names, "\n  "))
	}
}

// SetRuntime writes the runtime.tmux session name to STATE.yaml.
func SetRuntime(featureDir, tmuxSession string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.Tmux = &TmuxRuntime{Session: tmuxSession}
		return nil
	})
}

// SetRuntimeTarget records the exact pane used by the active agent. The pane is
// optional for compatibility with existing STATE.yaml files.
func SetRuntimeTarget(featureDir, tmuxSession, pane string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.Tmux = &TmuxRuntime{Session: tmuxSession, Pane: pane}
		return nil
	})
}

// ClearRuntime removes the runtime block from STATE.yaml.
func ClearRuntime(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime = Runtime{}
		return nil
	})
}

// SetJIT writes runtime.jit to STATE.yaml before a jit task launches.
func SetJIT(featureDir, workerID, task string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.JIT = &JITRuntime{
			Worker:    workerID,
			Task:      task,
			StartedAt: timeNow(),
		}
		return nil
	})
}

// ClearJIT removes runtime.jit from STATE.yaml after a jit task completes.
func ClearJIT(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.JIT = nil
		return nil
	})
}

// RepoError is a single structured problem found by ValidateRepos.
type RepoError struct {
	Field   string // dotted path, e.g. "repos.myrepo.main" or "next_action.cwd"
	Message string // human-readable detail
}

// RepoValidationErrors is returned by ValidateRepos when one or more problems are found.
type RepoValidationErrors []RepoError

func (e RepoValidationErrors) Error() string {
	msgs := make([]string, len(e))
	for i, r := range e {
		msgs[i] = r.Field + ": " + r.Message
	}
	return "STATE.yaml repo validation failed:\n  " + strings.Join(msgs, "\n  ")
}

// ValidateRepos checks that repo fields in STATE.yaml are internally consistent
// and point to paths that make sense within the workspace. Returns a non-nil
// RepoValidationErrors listing all problems found. Only validates fields that
// are set — a repo with no worktree recorded is not an error.
func ValidateRepos(s *State, workspaceRoot string) error {
	worktreesRoot := filepath.Join(workspaceRoot, "worktrees")
	var errs RepoValidationErrors

	for name, r := range s.Repos {
		// main must exist if set
		if r.Main != "" {
			if _, err := os.Stat(r.Main); os.IsNotExist(err) {
				errs = append(errs, RepoError{
					Field:   fmt.Sprintf("repos.%s.main", name),
					Message: fmt.Sprintf("%q does not exist", r.Main),
				})
			}
		}

		if r.Worktree != "" {
			// worktree must be under worktrees/ in the workspace
			abs := r.Worktree
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(workspaceRoot, abs)
			}
			rel, err := filepath.Rel(worktreesRoot, abs)
			if err != nil || strings.HasPrefix(rel, "..") {
				errs = append(errs, RepoError{
					Field:   fmt.Sprintf("repos.%s.worktree", name),
					Message: fmt.Sprintf("%q is not under worktrees/ in the workspace", r.Worktree),
				})
			} else if _, err := os.Stat(abs); os.IsNotExist(err) {
				errs = append(errs, RepoError{
					Field:   fmt.Sprintf("repos.%s.worktree", name),
					Message: fmt.Sprintf("%q does not exist", r.Worktree),
				})
			}

			// branch must be non-empty when a worktree is recorded
			if r.Branch == "" {
				errs = append(errs, RepoError{
					Field:   fmt.Sprintf("repos.%s.branch", name),
					Message: "empty but worktree is set",
				})
			}
		}
	}

	// next_action.cwd must be under a recorded worktree when any worktree is set
	hasWorktrees := false
	for _, r := range s.Repos {
		if r.Worktree != "" {
			hasWorktrees = true
			break
		}
	}
	if hasWorktrees && s.NextAction.CWD != "" {
		cwd := s.NextAction.CWD
		if !filepath.IsAbs(cwd) {
			cwd = filepath.Join(workspaceRoot, cwd)
		}
		matched := false
		for _, r := range s.Repos {
			if r.Worktree == "" {
				continue
			}
			wt := r.Worktree
			if !filepath.IsAbs(wt) {
				wt = filepath.Join(workspaceRoot, wt)
			}
			rel, err := filepath.Rel(wt, cwd)
			if err == nil && !strings.HasPrefix(rel, "..") {
				matched = true
				break
			}
		}
		if !matched {
			errs = append(errs, RepoError{
				Field:   "next_action.cwd",
				Message: fmt.Sprintf("%q does not match any recorded worktree", s.NextAction.CWD),
			})
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// ResolveCWD returns an absolute path for the next action cwd, resolving
// relative paths against the workspace root. Worktrees live under
// workspaceRoot/worktrees, so a relative cwd is workspace-relative — this keeps
// resolution consistent with the "." default (which maps to the workspace root)
// and with ValidateRepos, which also anchors cwd at the workspace root.
func (s *State) ResolveCWD(workspaceRoot string) string {
	cwd := s.NextAction.CWD
	if cwd == "" || cwd == "." {
		return workspaceRoot
	}
	if filepath.IsAbs(cwd) {
		return cwd
	}
	return filepath.Clean(filepath.Join(workspaceRoot, cwd))
}
