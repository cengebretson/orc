package worktreecheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
)

// initRepo creates a git repo with one commit so worktrees can be added.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-b", "main")
	git(t, dir, "commit", "--allow-empty", "-m", "init")
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	base := []string{"-C", dir,
		"-c", "user.name=test", "-c", "user.email=test@example.com",
		"-c", "commit.gpgsign=false",
	}
	cmd := exec.Command("git", append(base, args...)...)
	// Drop inherited GIT_* vars (GIT_DIR, GIT_INDEX_FILE, ...) so the tests
	// work when the suite itself runs inside a git hook, and pin config to
	// nothing so user/system gitconfig cannot interfere.
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// addWorktree creates a real git worktree on branch at root/relPath.
func addWorktree(t *testing.T, repoDir, root, relPath, branch string) {
	t.Helper()
	git(t, repoDir, "worktree", "add", "-b", branch, filepath.Join(root, relPath))
}

func stateWithRepo(name, worktree, branch string) *state.State {
	return &state.State{Repos: map[string]state.Repo{
		name: {Worktree: worktree, Branch: branch},
	}}
}

func TestReconcileMatchingWorktreeHasNoFindings(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	initRepo(t, repoDir)
	addWorktree(t, repoDir, root, "worktrees/app/T-1", "feature/t-1")

	findings := Reconcile(root, stateWithRepo("app", "worktrees/app/T-1", "feature/t-1"))
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
}

func TestReconcileReportsBranchMismatch(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	initRepo(t, repoDir)
	addWorktree(t, repoDir, root, "worktrees/app/T-1", "feature/actual")

	findings := Reconcile(root, stateWithRepo("app", "worktrees/app/T-1", "feature/recorded"))
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one branch mismatch", findings)
	}
	f := findings[0]
	if f.Field != "repos.app.branch" || f.Severity != Warning {
		t.Fatalf("finding = %+v", f)
	}
	if !strings.Contains(f.Message, `"feature/recorded"`) || !strings.Contains(f.Message, `"feature/actual"`) {
		t.Fatalf("message = %q, want both branches named", f.Message)
	}
}

func TestReconcileReportsDetachedHead(t *testing.T) {
	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	initRepo(t, repoDir)
	addWorktree(t, repoDir, root, "worktrees/app/T-1", "feature/t-1")
	git(t, filepath.Join(root, "worktrees/app/T-1"), "checkout", "--detach")

	findings := Reconcile(root, stateWithRepo("app", "worktrees/app/T-1", "feature/t-1"))
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one detached warning", findings)
	}
	f := findings[0]
	if f.Field != "repos.app.branch" || f.Severity != Warning || !strings.Contains(f.Message, "detached") {
		t.Fatalf("finding = %+v", f)
	}
}

func TestReconcileReportsPlainDirectoryAsNotAWorktree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "worktrees/app/T-1"), 0755); err != nil {
		t.Fatal(err)
	}

	findings := Reconcile(root, stateWithRepo("app", "worktrees/app/T-1", "feature/t-1"))
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one not-a-worktree warning", findings)
	}
	f := findings[0]
	if f.Field != "repos.app.worktree" || f.Severity != Warning || !strings.Contains(f.Message, "not a git worktree") {
		t.Fatalf("finding = %+v", f)
	}
}

func TestReconcileReportsFileAsFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "worktrees/app"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "worktrees/app/T-1"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	findings := Reconcile(root, stateWithRepo("app", "worktrees/app/T-1", "feature/t-1"))
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one not-a-directory failure", findings)
	}
	f := findings[0]
	if f.Field != "repos.app.worktree" || f.Severity != Fail || !strings.Contains(f.Message, "not a directory") {
		t.Fatalf("finding = %+v", f)
	}
}

// Missing worktree paths are state.ValidateRepos territory — Reconcile must
// stay silent to avoid duplicate findings.
func TestReconcileSkipsMissingWorktreeAndEmptyFields(t *testing.T) {
	root := t.TempDir()
	s := &state.State{Repos: map[string]state.Repo{
		"gone":  {Worktree: "worktrees/gone/T-1", Branch: "feature/t-1"},
		"unset": {Branch: "feature/t-1"},
	}}

	if findings := Reconcile(root, s); len(findings) != 0 {
		t.Fatalf("findings = %+v, want none", findings)
	}
	if findings := Reconcile(root, nil); findings != nil {
		t.Fatalf("nil state findings = %+v, want nil", findings)
	}
}

// Findings must come back in sorted repo-name order so doctor output is stable.
func TestReconcileOrdersFindingsByRepoName(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"zeta", "alpha", "mid"} {
		if err := os.MkdirAll(filepath.Join(root, "worktrees", name, "T-1"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	s := &state.State{Repos: map[string]state.Repo{
		"zeta":  {Worktree: "worktrees/zeta/T-1"},
		"alpha": {Worktree: "worktrees/alpha/T-1"},
		"mid":   {Worktree: "worktrees/mid/T-1"},
	}}

	findings := Reconcile(root, s)
	if len(findings) != 3 {
		t.Fatalf("findings = %+v, want three", findings)
	}
	for i, want := range []string{"alpha", "mid", "zeta"} {
		if findings[i].RepoName != want {
			t.Fatalf("findings[%d].RepoName = %q, want %q", i, findings[i].RepoName, want)
		}
	}
}
