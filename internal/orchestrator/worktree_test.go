package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
)

func TestRecordWorktreePersistsOwnedCheckout(t *testing.T) {
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "ORC-9-native")
	source := filepath.Join(root, "source")
	worktreeRef := filepath.Join("worktrees", "app", "ORC-9-native")
	worktreeDir := filepath.Join(root, worktreeRef)
	for _, dir := range []string{featureDir, source, worktreeDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.Create(featureDir, &state.State{
		Ticket: "ORC-9", Slug: "ORC-9-native", NextAction: state.NextAction{CWD: "."},
	}); err != nil {
		t.Fatal(err)
	}

	launch := worktreeLaunch{
		Spec: mux.WorktreeTargetSpec{
			Repository: "app", SourceDir: source, WorktreeDir: worktreeDir, Branch: "feature/orc-9-native",
		},
		WorktreeRef: worktreeRef,
	}
	if err := recordWorktree(featureDir, root, launch); err != nil {
		t.Fatal(err)
	}
	got, err := state.Load(featureDir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repos["app"].Main != source || got.Repos["app"].Worktree != worktreeRef || got.Repos["app"].Branch != "feature/orc-9-native" {
		t.Fatalf("repo state = %#v", got.Repos["app"])
	}
	if got.NextAction.CWD != worktreeRef {
		t.Fatalf("next_action.cwd = %q, want %q", got.NextAction.CWD, worktreeRef)
	}
}

func TestResolveWorktreeLaunchPreservesCustomSetup(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "orc.yaml"), []byte(`
repos:
  - name: app
    path: source
    worktree_setup: ./setup-worktree {{branch}} {{worktree_path}}
workflows: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &state.State{Slug: "ORC-9-native"}
	if _, ok := resolveWorktreeLaunch(LaunchOptions{Root: root, State: s, Plan: &runner.Plan{CWD: root}}, nil); ok {
		t.Fatal("custom worktree_setup should keep ownership of checkout creation")
	}
	existing := filepath.Join(root, "worktrees", "app", s.Slug)
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	launch, ok := resolveWorktreeLaunch(LaunchOptions{Root: root, State: s, Plan: &runner.Plan{CWD: root}}, nil)
	if !ok || launch.Spec.WorktreeDir != existing {
		t.Fatalf("existing custom checkout should reopen natively: launch = %#v, ok = %v", launch, ok)
	}
}

func TestResolveWorktreeLaunchUsesCWDToChooseRecordedRepo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "orc.yaml"), []byte(`
repos:
  - name: app
    path: app-main
  - name: api
    path: api-main
workflows: {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	apiRef := filepath.Join("worktrees", "api", "ORC-9-native")
	s := &state.State{
		Slug: "ORC-9-native",
		Repos: map[string]state.Repo{
			"app": {Main: filepath.Join(root, "app-main"), Worktree: filepath.Join("worktrees", "app", "ORC-9-native"), Branch: "feature/app"},
			"api": {Main: filepath.Join(root, "api-main"), Worktree: apiRef, Branch: "feature/api"},
		},
	}
	launch, ok := resolveWorktreeLaunch(LaunchOptions{
		Root: root, State: s, Plan: &runner.Plan{CWD: filepath.Join(root, apiRef, "pkg")},
	}, []string{"develop"})
	if !ok || launch.Spec.Repository != "api" || launch.Spec.Branch != "feature/api" {
		t.Fatalf("launch = %#v, ok = %v", launch, ok)
	}
}
