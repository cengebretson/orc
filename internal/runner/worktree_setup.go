package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/worktreesetup"
)

func worktreeSetupPrompt(root string, cfg *config.Config, s *state.State) string {
	setups := pendingWorktreeSetups(root, cfg, s)
	if len(setups) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Worktree setup\n\n")
	b.WriteString("The target repo declares a setup command. If the worktree does not already exist, run the matching command before repo edits:\n\n")
	for _, setup := range setups {
		fmt.Fprintf(&b, "Repo `%s`:\n\n```bash\n%s\n```\n\n", setup.RepoName, setup.Command)
		fmt.Fprintf(&b, "Expected worktree: `%s`\n", setup.WorktreePath)
		fmt.Fprintf(&b, "Branch: `%s`\n\n", setup.Branch)
	}
	b.WriteString("After the command succeeds, record the repo main path, worktree path, branch, and next action cwd in `STATE.yaml` so later stages use the same checkout.\n\n")
	return b.String()
}

type pendingSetup struct {
	RepoName     string
	Command      string
	WorktreePath string
	Branch       string
}

func pendingWorktreeSetups(root string, cfg *config.Config, s *state.State) []pendingSetup {
	if cfg == nil || s == nil {
		return nil
	}

	repos := targetRepos(cfg, s)
	var setups []pendingSetup
	for _, repo := range repos {
		if strings.TrimSpace(repo.Config.WorktreeSetup) == "" {
			continue
		}
		worktreePath := repo.State.Worktree
		if worktreePath == "" {
			worktreePath = filepath.Join("worktrees", repo.Config.Name, s.Slug)
		}
		absWorktree := worktreePath
		if !filepath.IsAbs(absWorktree) {
			absWorktree = filepath.Join(root, worktreePath)
		}
		if _, err := os.Stat(absWorktree); err == nil {
			continue
		}

		branch := repo.State.Branch
		if branch == "" {
			branch = "feature/" + strings.ToLower(s.Slug)
		}
		command, err := worktreesetup.Expand(repo.Config.WorktreeSetup, worktreesetup.Values{
			"branch":        branch,
			"repo_name":     repo.Config.Name,
			"repo_path":     repo.Config.Path,
			"slug":          s.Slug,
			"ticket":        s.Ticket,
			"workspace":     root,
			"worktree_path": absWorktree,
		})
		if err != nil {
			continue
		}
		setups = append(setups, pendingSetup{
			RepoName:     repo.Config.Name,
			Command:      command,
			WorktreePath: absWorktree,
			Branch:       branch,
		})
	}
	return setups
}

type targetRepo struct {
	Config config.Repo
	State  state.Repo
}

func targetRepos(cfg *config.Config, s *state.State) []targetRepo {
	byName := make(map[string]config.Repo, len(cfg.Repos))
	for _, repo := range cfg.Repos {
		if repo.Name == "" {
			continue
		}
		byName[repo.Name] = repo
	}

	if len(s.Repos) == 0 {
		if len(cfg.Repos) == 1 && cfg.Repos[0].Name != "" {
			return []targetRepo{{Config: cfg.Repos[0]}}
		}
		return nil
	}

	names := make([]string, 0, len(s.Repos))
	for name := range s.Repos {
		names = append(names, name)
	}
	sort.Strings(names)

	var repos []targetRepo
	for _, name := range names {
		cfgRepo, ok := byName[name]
		if !ok {
			continue
		}
		repos = append(repos, targetRepo{Config: cfgRepo, State: s.Repos[name]})
	}
	return repos
}
