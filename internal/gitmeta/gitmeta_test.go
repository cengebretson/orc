package gitmeta

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInspectGitGroupsLinkedWorktreeUnderCommonRepository(t *testing.T) {
	root := filepath.Join(t.TempDir(), "orc")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "init", "-b", "main")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# orc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "-c", "user.name=Orc Test", "-c", "user.email=orc@example.test", "commit", "-m", "initial")

	linked := filepath.Join(t.TempDir(), "orc-feature")
	runGit(t, root, "worktree", "add", "-b", "feature/grouping", linked)
	metadata, err := inspectGit(linked)
	if err != nil {
		t.Fatal(err)
	}
	want := Metadata{Repository: "orc", Branch: "feature/grouping", Worktree: "orc-feature"}
	if metadata != want {
		t.Fatalf("inspectGit() = %#v, want %#v", metadata, want)
	}
}

func runGit(t *testing.T, cwd string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", cwd}, args...)...)
	// Git exports repository-local GIT_* variables to hooks. Tests that create a
	// separate repository must not inherit the parent hook's index or worktree.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestResolverCachesSuccessUntilExpiry(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	calls := 0
	resolver := New(time.Second, func(string) (Metadata, error) {
		calls++
		return Metadata{Repository: "orc", Branch: "main", Worktree: "."}, nil
	})
	resolver.now = func() time.Time { return now }

	for range 2 {
		metadata, ok := resolver.Resolve("/work/orc")
		if !ok || metadata.Repository != "orc" {
			t.Fatalf("Resolve() = %#v, %v", metadata, ok)
		}
	}
	if calls != 1 {
		t.Fatalf("inspect calls = %d, want 1", calls)
	}
	now = now.Add(2 * time.Second)
	_, _ = resolver.Resolve("/work/orc")
	if calls != 2 {
		t.Fatalf("inspect calls after expiry = %d, want 2", calls)
	}
}

func TestResolverCachesFailures(t *testing.T) {
	calls := 0
	resolver := New(time.Second, func(string) (Metadata, error) {
		calls++
		return Metadata{}, errors.New("not a git repository")
	})
	for range 2 {
		if metadata, ok := resolver.Resolve("/tmp/plain"); ok || metadata != (Metadata{}) {
			t.Fatalf("Resolve() = %#v, %v", metadata, ok)
		}
	}
	if calls != 1 {
		t.Fatalf("failure inspect calls = %d, want 1", calls)
	}
}
