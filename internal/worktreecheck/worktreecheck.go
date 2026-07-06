package worktreecheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/state"
)

type Severity int

const (
	Warning Severity = iota
	Fail
)

type Finding struct {
	Field    string
	RepoName string
	Severity Severity
	Message  string
}

func Reconcile(root string, s *state.State) []Finding {
	if s == nil || len(s.Repos) == 0 {
		return nil
	}
	var findings []Finding
	for name, repo := range s.Repos {
		if repo.Worktree == "" {
			continue
		}
		field := fmt.Sprintf("repos.%s.worktree", name)
		worktree := repo.Worktree
		if !filepath.IsAbs(worktree) {
			worktree = filepath.Join(root, worktree)
		}
		info, err := os.Stat(worktree)
		if err != nil {
			// state.ValidateRepos reports missing paths. Avoid duplicate failures here.
			continue
		}
		if !info.IsDir() {
			findings = append(findings, Finding{
				Field:    field,
				RepoName: name,
				Severity: Fail,
				Message:  fmt.Sprintf("%q is not a directory", repo.Worktree),
			})
			continue
		}

		if err := gitCheck(worktree, "rev-parse", "--is-inside-work-tree"); err != nil {
			findings = append(findings, Finding{
				Field:    field,
				RepoName: name,
				Severity: Warning,
				Message:  fmt.Sprintf("%q is not a git worktree", repo.Worktree),
			})
			continue
		}

		actualBranch, err := gitOutput(worktree, "branch", "--show-current")
		if err != nil {
			findings = append(findings, Finding{
				Field:    fmt.Sprintf("repos.%s.branch", name),
				RepoName: name,
				Severity: Warning,
				Message:  "could not read current branch",
			})
			continue
		}
		actualBranch = strings.TrimSpace(actualBranch)
		if repo.Branch != "" && actualBranch == "" {
			findings = append(findings, Finding{
				Field:    fmt.Sprintf("repos.%s.branch", name),
				RepoName: name,
				Severity: Warning,
				Message:  fmt.Sprintf("expected %q but worktree is detached", repo.Branch),
			})
			continue
		}
		if repo.Branch != "" && actualBranch != repo.Branch {
			findings = append(findings, Finding{
				Field:    fmt.Sprintf("repos.%s.branch", name),
				RepoName: name,
				Severity: Warning,
				Message:  fmt.Sprintf("STATE.yaml records %q but worktree is on %q", repo.Branch, actualBranch),
			})
		}
	}
	return findings
}

func gitCheck(dir string, args ...string) error {
	_, err := gitOutput(dir, args...)
	return err
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
