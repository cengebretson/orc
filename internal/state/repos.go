package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
