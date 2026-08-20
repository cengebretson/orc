package tmuxattention

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplaySlug(t *testing.T) {
	tests := map[string]struct {
		ticket string
		slug   string
		want   string
	}{
		"repeated uppercase ticket": {ticket: "ORC-9", slug: "ORC-9-native-worktree", want: "native-worktree"},
		"repeated lowercase ticket": {ticket: "ORC-9", slug: "orc-9_native worktree", want: "native-worktree"},
		"already concise":           {ticket: "ORC-9", slug: "native-worktree", want: "native-worktree"},
		"ticket only":               {ticket: "ORC-9", slug: "ORC-9", want: ""},
		"prefix is not ticket":      {ticket: "ORC-9", slug: "ORC-90-native", want: "orc-90-native"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := DisplaySlug(test.ticket, test.slug); got != test.want {
				t.Fatalf("DisplaySlug(%q, %q) = %q, want %q", test.ticket, test.slug, got, test.want)
			}
		})
	}
}

func TestWriteWorktreeContextUsesPrivateGitDirectory(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "-c", "user.name=Orc Test", "-c", "user.email=orc@example.com", "commit", "-q", "-m", "initial")

	worktree := filepath.Join(t.TempDir(), "ORC-9-native-worktree")
	runGit(t, repo, "worktree", "add", "-q", "-b", "feature/orc-9-native-worktree", worktree)
	if err := WriteWorktreeContext(worktree, "ORC-9", "ORC-9-native-worktree"); err != nil {
		t.Fatal(err)
	}

	gitDir := runGit(t, worktree, "rev-parse", "--absolute-git-dir")
	contextPath := filepath.Join(gitDir, WorktreeContextFile)
	content, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "branch=feature/orc-9-native-worktree\nproject=ORC-9\nslug=native-worktree\n"
	if string(content) != want {
		t.Fatalf("context = %q, want %q", content, want)
	}
	if strings.HasPrefix(contextPath, worktree+string(filepath.Separator)) {
		t.Fatalf("context file %s was written inside the checkout", contextPath)
	}
	if _, err := os.Stat(filepath.Join(worktree, WorktreeContextFile)); !os.IsNotExist(err) {
		t.Fatalf("checkout contains tmux-attention metadata: %v", err)
	}
}

func TestWriteWorktreeContextRejectsUnsafeOrDetachedContext(t *testing.T) {
	if err := WriteWorktreeContext(t.TempDir(), "ORC-9\nwrong", "safe"); err == nil {
		t.Fatal("control character was accepted")
	}

	repo := t.TempDir()
	runGit(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("tracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "tracked.txt")
	runGit(t, repo, "-c", "user.name=Orc Test", "-c", "user.email=orc@example.com", "commit", "-q", "-m", "initial")
	runGit(t, repo, "checkout", "-q", "--detach")
	if err := WriteWorktreeContext(repo, "ORC-9", "native"); err == nil {
		t.Fatal("detached branch was accepted")
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out))
}
