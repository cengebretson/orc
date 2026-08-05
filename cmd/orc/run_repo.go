package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
)

type runRepository struct {
	Name     string
	Path     string
	Inferred bool
}

func selectRunRepository(root, cwd, requested string, repos []config.Repo) (*runRepository, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		for _, repo := range repos {
			if repo.Name == requested {
				return resolveRunRepository(root, repo, false)
			}
		}
		names := configuredRepoNames(repos)
		if len(names) == 0 {
			return nil, fmt.Errorf("repository %q not found — no repositories are configured", requested)
		}
		return nil, fmt.Errorf("repository %q not found (available: %s)", requested, strings.Join(names, ", "))
	}

	if len(repos) == 1 {
		selected, err := resolveRunRepository(root, repos[0], true)
		if err != nil {
			// A placeholder or temporarily unavailable sole repository should not
			// make an otherwise repository-agnostic run unusable. Explicit
			// selection still reports the path error above.
			return nil, nil
		}
		return selected, nil
	}
	if len(repos) == 0 || strings.TrimSpace(cwd) == "" {
		return nil, nil
	}

	cwd = canonicalRunPath(cwd)
	var matches []config.Repo
	for _, repo := range repos {
		path := resolveConfiguredRepoPath(root, repo.Path)
		if path != "" && pathWithinRunRepo(cwd, canonicalRunPath(path)) {
			matches = append(matches, repo)
		}
	}
	if len(matches) == 0 {
		return nil, nil
	}
	// Nested configured repositories are valid. The deepest matching checkout
	// is the most specific description of the caller's current directory.
	sort.SliceStable(matches, func(i, j int) bool {
		left := resolveConfiguredRepoPath(root, matches[i].Path)
		right := resolveConfiguredRepoPath(root, matches[j].Path)
		return len(left) > len(right)
	})
	return resolveRunRepository(root, matches[0], true)
}

func resolveRunRepository(root string, repo config.Repo, inferred bool) (*runRepository, error) {
	path := resolveConfiguredRepoPath(root, repo.Path)
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("repository %q path %s is unavailable: %w", repo.Name, path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository %q path %s is not a directory", repo.Name, path)
	}
	return &runRepository{Name: repo.Name, Path: canonicalRunPath(path), Inferred: inferred}, nil
}

func resolveConfiguredRepoPath(root, configured string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return ""
	}
	if !filepath.IsAbs(configured) {
		configured = filepath.Join(root, configured)
	}
	return filepath.Clean(configured)
}

func canonicalRunPath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}

func pathWithinRunRepo(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func configuredRepoNames(repos []config.Repo) []string {
	names := make([]string, 0, len(repos))
	for _, repo := range repos {
		if name := strings.TrimSpace(repo.Name); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
