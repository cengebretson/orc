package worktreesetup_test

import (
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/worktreesetup"
)

func TestExpand(t *testing.T) {
	got, err := worktreesetup.Expand("setup {{repo_name}} {{branch}} {{worktree_path}}", worktreesetup.Values{
		"repo_name":     "app",
		"branch":        "feature/test",
		"worktree_path": "/tmp/worktrees/app/test",
	})
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	want := "setup app feature/test /tmp/worktrees/app/test"
	if got != want {
		t.Fatalf("Expand = %q, want %q", got, want)
	}
}

func TestExpandRejectsUnknownPlaceholder(t *testing.T) {
	_, err := worktreesetup.Expand("setup {{unknown}}", worktreesetup.Values{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("error = %q", err)
	}
}

func TestReferencesWorktreePath(t *testing.T) {
	if !worktreesetup.ReferencesWorktreePath("setup --path {{worktree_path}}") {
		t.Fatal("expected worktree_path reference")
	}
	if worktreesetup.ReferencesWorktreePath("setup --branch {{branch}}") {
		t.Fatal("unexpected worktree_path reference")
	}
}
