package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
)

type worktreeLaunch struct {
	Spec        mux.WorktreeTargetSpec
	WorktreeRef string
}

type worktreeRepo struct {
	Config config.Repo
	State  state.Repo
}

// resolveWorktreeLaunch chooses only an unambiguous repository. A single
// configured repo is safe for a new ticket; multi-repo tickets must already
// identify the target repo by recording it in STATE.yaml and selecting a cwd
// inside its worktree.
func resolveWorktreeLaunch(opts LaunchOptions, tabs []string) (worktreeLaunch, bool) {
	cfg, err := config.Load(opts.Root)
	if err != nil {
		return worktreeLaunch{}, false
	}

	repos := worktreeRepos(cfg, opts.State)
	if len(repos) == 0 {
		return worktreeLaunch{}, false
	}
	repo, ok := selectWorktreeRepo(opts, repos)
	if !ok {
		return worktreeLaunch{}, false
	}

	source := repo.State.Main
	if source == "" {
		source = repo.Config.Path
	}
	if source == "" {
		return worktreeLaunch{}, false
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(opts.Root, source)
	}
	source = filepath.Clean(source)

	worktreeRef := repo.State.Worktree
	if worktreeRef == "" {
		worktreeRef = filepath.Join("worktrees", repo.Config.Name, opts.State.Slug)
	}
	worktreeDir := worktreeRef
	if !filepath.IsAbs(worktreeDir) {
		worktreeDir = filepath.Join(opts.Root, worktreeDir)
	}
	worktreeDir = filepath.Clean(worktreeDir)
	if strings.TrimSpace(repo.Config.WorktreeSetup) != "" {
		if _, err := os.Stat(worktreeDir); err != nil {
			// A repository-owned setup command can do more than
			// `git worktree add`. Preserve it for missing checkouts instead of
			// partially reproducing it in Herdr. Existing checkouts are safe to
			// reopen natively.
			return worktreeLaunch{}, false
		}
	}
	if rel, err := filepath.Rel(opts.Root, worktreeDir); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		worktreeRef = rel
	}

	branch := repo.State.Branch
	if branch == "" {
		branch = "feature/" + strings.ToLower(opts.State.Slug)
	}

	return worktreeLaunch{
		Spec: mux.WorktreeTargetSpec{
			Name:        opts.State.Slug,
			Repository:  repo.Config.Name,
			SourceDir:   source,
			WorktreeDir: worktreeDir,
			Branch:      branch,
			Tabs:        tabs,
		},
		WorktreeRef: worktreeRef,
	}, true
}

func worktreeRepos(cfg *config.Config, s *state.State) []worktreeRepo {
	if len(s.Repos) == 0 {
		if len(cfg.Repos) == 1 && cfg.Repos[0].Name != "" {
			return []worktreeRepo{{Config: cfg.Repos[0]}}
		}
		return nil
	}

	byName := make(map[string]config.Repo, len(cfg.Repos))
	for _, repo := range cfg.Repos {
		byName[repo.Name] = repo
	}
	names := make([]string, 0, len(s.Repos))
	for name := range s.Repos {
		names = append(names, name)
	}
	sort.Strings(names)

	var repos []worktreeRepo
	for _, name := range names {
		if configured, ok := byName[name]; ok {
			repos = append(repos, worktreeRepo{Config: configured, State: s.Repos[name]})
		}
	}
	return repos
}

func selectWorktreeRepo(opts LaunchOptions, repos []worktreeRepo) (worktreeRepo, bool) {
	if len(repos) == 1 {
		return repos[0], true
	}
	for _, repo := range repos {
		if repo.State.Worktree == "" {
			continue
		}
		path := repo.State.Worktree
		if !filepath.IsAbs(path) {
			path = filepath.Join(opts.Root, path)
		}
		if pathWithin(opts.Plan.CWD, filepath.Clean(path)) {
			return repo, true
		}
	}
	return worktreeRepo{}, false
}

func pathWithin(path, root string) bool {
	if path == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func recordWorktree(featureDir, root string, launch worktreeLaunch) error {
	return state.Update(featureDir, func(s *state.State) error {
		applyWorktreeState(s, launch)
		if err := state.ValidateRepos(s, root); err != nil {
			return fmt.Errorf("validate native worktree state: %w", err)
		}
		return nil
	})
}

func applyWorktreeState(s *state.State, launch worktreeLaunch) {
	if s.Repos == nil {
		s.Repos = make(map[string]state.Repo)
	}
	s.Repos[launch.Spec.Repository] = state.Repo{
		Main: launch.Spec.SourceDir, Worktree: launch.WorktreeRef, Branch: launch.Spec.Branch,
	}
	cwd := s.NextAction.CWD
	cwdInWorktree := pathWithin(filepath.Clean(cwd), filepath.Clean(launch.WorktreeRef))
	if filepath.IsAbs(cwd) {
		cwdInWorktree = pathWithin(cwd, launch.Spec.WorktreeDir)
	}
	if cwd == "" || cwd == "." || !cwdInWorktree {
		s.NextAction.CWD = launch.WorktreeRef
	}
}
