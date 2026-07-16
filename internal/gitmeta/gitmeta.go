// Package gitmeta provides bounded, best-effort repository metadata for live
// sessions that do not already have durable Orc repository state.
package gitmeta

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultTTL     = 5 * time.Second
	inspectTimeout = 300 * time.Millisecond
)

type Metadata struct {
	Repository string `json:"repository"`
	Branch     string `json:"branch,omitempty"`
	Worktree   string `json:"worktree,omitempty"`
}

type InspectFunc func(cwd string) (Metadata, error)

type cacheEntry struct {
	metadata Metadata
	ok       bool
	expires  time.Time
}

type Resolver struct {
	mu      sync.Mutex
	ttl     time.Duration
	now     func() time.Time
	inspect InspectFunc
	cache   map[string]cacheEntry
}

func New(ttl time.Duration, inspect InspectFunc) *Resolver {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if inspect == nil {
		inspect = inspectGit
	}
	return &Resolver{
		ttl: ttl, now: time.Now, inspect: inspect, cache: make(map[string]cacheEntry),
	}
}

func (r *Resolver) Resolve(cwd string) (Metadata, bool) {
	cwd = filepath.Clean(strings.TrimSpace(cwd))
	if cwd == "" || cwd == "." {
		return Metadata{}, false
	}
	now := r.now()
	r.mu.Lock()
	entry, found := r.cache[cwd]
	r.mu.Unlock()
	if found && now.Before(entry.expires) {
		return entry.metadata, entry.ok
	}

	metadata, err := r.inspect(cwd)
	ok := err == nil && metadata.Repository != ""
	r.mu.Lock()
	r.cache[cwd] = cacheEntry{metadata: metadata, ok: ok, expires: now.Add(r.ttl)}
	r.mu.Unlock()
	return metadata, ok
}

var defaultResolver = New(defaultTTL, nil)

func Resolve(cwd string) (Metadata, bool) {
	return defaultResolver.Resolve(cwd)
}

func inspectGit(cwd string) (Metadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), inspectTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", cwd, "rev-parse", "--path-format=absolute", "--git-common-dir", "--show-toplevel", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return Metadata{}, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 3 {
		return Metadata{}, errors.New("unexpected git metadata output")
	}
	commonDir := filepath.Clean(lines[0])
	worktreeRoot := filepath.Clean(lines[1])
	repositoryRoot := commonDir
	if filepath.Base(commonDir) == ".git" {
		repositoryRoot = filepath.Dir(commonDir)
	}
	worktree := "."
	if !samePath(repositoryRoot, worktreeRoot) {
		worktree = filepath.Base(worktreeRoot)
	}
	return Metadata{
		Repository: filepath.Base(repositoryRoot),
		Branch:     strings.TrimSpace(lines[2]),
		Worktree:   worktree,
	}, nil
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return filepath.Clean(left) == filepath.Clean(right)
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}
