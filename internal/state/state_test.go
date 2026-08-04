package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/state"
)

func fixtureWorkspace() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "workspace")
}

func TestLoad(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := filepath.Join(ws, "features", "STORY-123-add-user-auth")

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if s.Ticket != "STORY-123" {
		t.Errorf("ticket = %q, want STORY-123", s.Ticket)
	}
	if s.Status != "active" {
		t.Errorf("status = %q, want active", s.Status)
	}
	if s.Stage.Name != "default:develop" {
		t.Errorf("stage.name = %q, want default:develop", s.Stage.Name)
	}
	if s.Stage.Worker != "default:bob" {
		t.Errorf("stage.worker = %q, want default:bob", s.Stage.Worker)
	}
	if s.NextAction.Worker != "default:bob" {
		t.Errorf("next_action.worker = %q, want default:bob", s.NextAction.Worker)
	}
}

func TestCreate_WritesLoadableState(t *testing.T) {
	dir := t.TempDir()
	s := &state.State{
		SchemaVersion: state.SchemaVersion,
		Ticket:        "TEST-9",
		Slug:          "TEST-9-create",
		Status:        "pending",
		Stage:         state.Stage{Name: "intake"},
		History: []state.HistoryEntry{
			{At: time.Now().Format(time.RFC3339), Stage: "intake", Worker: "agent", Result: "created"},
		},
	}
	if err := state.Create(dir, s); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := state.Load(dir)
	if err != nil {
		t.Fatalf("Load after Create: %v", err)
	}
	if got.Ticket != "TEST-9" || got.Stage.Name != "intake" {
		t.Errorf("round-trip mismatch: ticket=%q stage=%q", got.Ticket, got.Stage.Name)
	}
	if len(got.History) != 1 || got.History[0].Worker != "agent" {
		t.Errorf("history round-trip mismatch: %+v", got.History)
	}
}

func TestCreate_ReplacesPlaceholder(t *testing.T) {
	dir := t.TempDir()
	placeholder := filepath.Join(dir, state.Filename)
	if err := os.WriteFile(placeholder, []byte("ticket: TICKET-0000\nslug: placeholder\nstatus: pending\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := state.Create(dir, &state.State{Ticket: "TEST-10", Slug: "TEST-10-x", Status: "pending"}); err != nil {
		t.Fatalf("Create over placeholder: %v", err)
	}
	got, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ticket != "TEST-10" {
		t.Errorf("ticket = %q, want TEST-10 (placeholder not replaced)", got.Ticket)
	}
}

func TestLoad_DefaultsMissingSchemaVersionToV1(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "LEGACY-1")
	writeLegacyState(t, featureDir, `
ticket: LEGACY-1
slug: LEGACY-1
status: pending
stage:
  worker: bob-developer
  name: develop
`)

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.SchemaVersion != state.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", s.SchemaVersion, state.SchemaVersion)
	}
}

func TestLoad_Missing(t *testing.T) {
	_, err := state.Load("/nonexistent/feature")
	if err == nil {
		t.Fatal("expected error for missing STATE.yaml, got nil")
	}
}

func TestFindFeatureDir_ExactSlug(t *testing.T) {
	ws := fixtureWorkspace()
	dir, err := state.FindFeatureDir(ws, "STORY-123-add-user-auth")
	if err != nil {
		t.Fatalf("FindFeatureDir: %v", err)
	}
	if filepath.Base(dir) != "STORY-123-add-user-auth" {
		t.Errorf("dir = %q, want STORY-123-add-user-auth", filepath.Base(dir))
	}
}

func TestFindFeatureDir_TicketPrefix(t *testing.T) {
	ws := fixtureWorkspace()
	dir, err := state.FindFeatureDir(ws, "STORY-456")
	if err != nil {
		t.Fatalf("FindFeatureDir: %v", err)
	}
	if filepath.Base(dir) != "STORY-456-export-api" {
		t.Errorf("dir = %q, want STORY-456-export-api", filepath.Base(dir))
	}
}

func TestFindFeatureDir_NotFound(t *testing.T) {
	ws := fixtureWorkspace()
	_, err := state.FindFeatureDir(ws, "NOTREAL-999")
	if err == nil {
		t.Fatal("expected error for missing feature, got nil")
	}
}

func TestFindFeatureDir_Ambiguous(t *testing.T) {
	ws := fixtureWorkspace()
	// "STORY" matches STORY-123, STORY-456, and STORY-789
	_, err := state.FindFeatureDir(ws, "STORY")
	if err == nil {
		t.Fatal("expected error for ambiguous slug, got nil")
	}
}

func TestResolveCWD_Dot(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := filepath.Join(ws, "features", "STORY-456-export-api")
	s, _ := state.Load(featureDir)

	cwd := s.ResolveCWD(ws)
	if cwd != ws {
		t.Errorf("cwd = %q, want workspace root %q", cwd, ws)
	}
}

func TestResolveCWD_Relative(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := filepath.Join(ws, "features", "STORY-123-add-user-auth")
	s, _ := state.Load(featureDir)

	// cwd in fixture is the workspace-relative "worktrees/my-app/STORY-123-add-user-auth";
	// it must resolve against the workspace root, not the feature dir.
	cwd := s.ResolveCWD(ws)
	want := filepath.Join(ws, "worktrees", "my-app", "STORY-123-add-user-auth")
	if cwd != want {
		t.Errorf("cwd = %q, want %q", cwd, want)
	}
}

func TestResolveCWD_Absolute(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "some", "worktree")
	s := &state.State{NextAction: state.NextAction{CWD: abs}}
	if got := s.ResolveCWD("/unused/workspace"); got != abs {
		t.Errorf("cwd = %q, want absolute path passed through %q", got, abs)
	}
}

func writeLegacyState(t *testing.T, featureDir, content string) {
	t.Helper()
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "STATE.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateWritesStateAndRemovesLock(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "TICKET-1")
	writeLegacyState(t, featureDir, `
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  worker: bob-developer
  name: develop
`)

	if err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "active"
		s.History = append(s.History, state.HistoryEntry{Result: "updated"})
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load after Update: %v", err)
	}
	if s.Status != "active" {
		t.Errorf("status = %q, want active", s.Status)
	}
	if len(s.History) != 1 || s.History[0].Result != "updated" {
		t.Errorf("history = %#v, want one updated entry", s.History)
	}
	assertNoStateArtifacts(t, featureDir)
}

func TestUpdateMutationErrorDoesNotRewriteState(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "TICKET-1")
	writeLegacyState(t, featureDir, `
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  worker: bob-developer
  name: develop
`)

	wantErr := errors.New("stop")
	err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "active"
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Update error = %v, want %v", err, wantErr)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load after Update: %v", err)
	}
	if s.Status != "pending" {
		t.Errorf("status = %q, want pending", s.Status)
	}
	assertNoStateArtifacts(t, featureDir)
}

func TestUpdateRemovesLockFromDeadPID(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "TICKET-1")
	writeLegacyState(t, featureDir, `
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  worker: bob-developer
  name: develop
`)
	if err := os.WriteFile(filepath.Join(featureDir, "STATE.yaml.lock"), []byte("999999999\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "active"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load after Update: %v", err)
	}
	if s.Status != "active" {
		t.Errorf("status = %q, want active", s.Status)
	}
	assertNoStateArtifacts(t, featureDir)
}

func TestUpdateRemovesOldLockWithoutPID(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "TICKET-1")
	writeLegacyState(t, featureDir, `
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  worker: bob-developer
  name: develop
`)
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}

	if err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "active"
		return nil
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	assertNoStateArtifacts(t, featureDir)
}

func TestClearStaleLockRemovesDeadPIDLock(t *testing.T) {
	featureDir := t.TempDir()
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte("999999999\n"), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := state.ClearStaleLock(featureDir)
	if err != nil {
		t.Fatalf("ClearStaleLock: %v", err)
	}
	if !removed {
		t.Fatal("removed = false, want true")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock should be gone, stat err = %v", err)
	}
}

func TestClearStaleLockKeepsLiveLock(t *testing.T) {
	featureDir := t.TempDir()
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	removed, err := state.ClearStaleLock(featureDir)
	if err != nil {
		t.Fatalf("ClearStaleLock: %v", err)
	}
	if removed {
		t.Fatal("removed = true, want false")
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("live lock should remain: %v", err)
	}
}

func TestClearStaleLockNoLockIsNoOp(t *testing.T) {
	featureDir := t.TempDir()

	removed, err := state.ClearStaleLock(featureDir)
	if err != nil {
		t.Fatalf("ClearStaleLock: %v", err)
	}
	if removed {
		t.Fatal("removed = true, want false")
	}
}

func assertNoStateArtifacts(t *testing.T, featureDir string) {
	t.Helper()
	entries, err := os.ReadDir(featureDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == "STATE.yaml.lock" || filepath.Ext(name) == ".tmp" {
			t.Fatalf("unexpected state artifact left behind: %s", name)
		}
	}
}

// Legacy STATE.yaml files (written before the workflow field existed) have no workflow key.
// Advance must still work and must not overwrite the stage when stageName is empty.
func TestNext_LegacyStateNoWorkflowField(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "OLD-001")
	writeLegacyState(t, featureDir, `
ticket: OLD-001
slug: OLD-001
status: active
stage:
  worker: bob-developer
  name: develop
`)

	if err := state.Next(featureDir, "pr-open", "bob-developer", "done"); err != nil {
		t.Fatalf("Next: %v", err)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load after Next: %v", err)
	}
	if s.Stage.Name != "pr-open" {
		t.Errorf("stage.name = %q, want pr-open", s.Stage.Name)
	}
	if s.Status != "pending" {
		t.Errorf("status = %q, want pending", s.Status)
	}
	// workflow field should remain empty — it was never set on this legacy ticket
	if s.Workflow != "" {
		t.Errorf("workflow = %q, want empty (legacy ticket)", s.Workflow)
	}
}

func TestValidateRepos_NoRepos(t *testing.T) {
	s := &state.State{}
	if err := state.ValidateRepos(s, t.TempDir()); err != nil {
		t.Errorf("expected nil for empty repos, got %v", err)
	}
}

func TestValidateRepos_ValidWorktree(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "worktrees", "my-app", "TICKET-1")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "my-app")
	if err := os.MkdirAll(mainPath, 0755); err != nil {
		t.Fatal(err)
	}

	s := &state.State{
		Repos: map[string]state.Repo{
			"my-app": {Main: mainPath, Worktree: wt, Branch: "feature/x"},
		},
		NextAction: state.NextAction{CWD: wt},
	}
	if err := state.ValidateRepos(s, root); err != nil {
		t.Errorf("expected nil for valid state, got %v", err)
	}
}

func TestValidateRepos_MissingMain(t *testing.T) {
	root := t.TempDir()
	s := &state.State{
		Repos: map[string]state.Repo{
			"my-app": {Main: "/nonexistent/path", Worktree: "", Branch: ""},
		},
	}
	if err := state.ValidateRepos(s, root); err == nil {
		t.Error("expected error for missing main path, got nil")
	}
}

func TestValidateRepos_WorktreeOutsideWorktreesDir(t *testing.T) {
	root := t.TempDir()
	outsidePath := filepath.Join(root, "somewhere-else", "TICKET-1")
	if err := os.MkdirAll(outsidePath, 0755); err != nil {
		t.Fatal(err)
	}
	s := &state.State{
		Repos: map[string]state.Repo{
			"my-app": {Worktree: outsidePath, Branch: "feature/x"},
		},
	}
	if err := state.ValidateRepos(s, root); err == nil {
		t.Error("expected error for worktree outside worktrees/, got nil")
	}
}

func TestValidateRepos_MissingBranchWhenWorktreeSet(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "worktrees", "my-app", "TICKET-1")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	s := &state.State{
		Repos: map[string]state.Repo{
			"my-app": {Worktree: wt, Branch: ""},
		},
	}
	if err := state.ValidateRepos(s, root); err == nil {
		t.Error("expected error for empty branch with worktree set, got nil")
	}
}

func TestValidateRepos_CWDNotUnderWorktree(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "worktrees", "my-app", "TICKET-1")
	if err := os.MkdirAll(wt, 0755); err != nil {
		t.Fatal(err)
	}
	s := &state.State{
		Repos: map[string]state.Repo{
			"my-app": {Worktree: wt, Branch: "feature/x"},
		},
		NextAction: state.NextAction{CWD: "/some/other/path"},
	}
	if err := state.ValidateRepos(s, root); err == nil {
		t.Error("expected error for cwd not under any worktree, got nil")
	}
}

func TestValidateRepos_CWDSkippedWhenNoWorktrees(t *testing.T) {
	root := t.TempDir()
	// Repos set but no worktrees recorded — cwd check should be skipped
	s := &state.State{
		Repos: map[string]state.Repo{
			"my-app": {Main: "", Worktree: "", Branch: ""},
		},
		NextAction: state.NextAction{CWD: "/some/other/path"},
	}
	if err := state.ValidateRepos(s, root); err != nil {
		t.Errorf("expected nil when no worktrees set, got %v", err)
	}
}

// When stageName is empty (last stage in pipeline), Next should set status to "done".
func TestNext_EmptyStageSetsDone(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "DONE-001")
	writeLegacyState(t, featureDir, `
ticket: DONE-001
slug: DONE-001
status: active
workflow: default
stage:
  worker: bob-developer
  name: qa-automation
`)

	if err := state.Next(featureDir, "", "bob-developer", "all tests pass"); err != nil {
		t.Fatalf("Next: %v", err)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load after Next: %v", err)
	}
	if s.Stage.Name != "qa-automation" {
		t.Errorf("stage.name = %q, want qa-automation (unchanged)", s.Stage.Name)
	}
	if s.Status != "done" {
		t.Errorf("status = %q, want done", s.Status)
	}
}

func TestResume_SetsActiveAndClearsNextAction(t *testing.T) {
	dir := t.TempDir()
	featureDir := filepath.Join(dir, "features", "PAUSED-1")
	writeLegacyState(t, featureDir, `
ticket: PAUSED-1
slug: PAUSED-1
status: paused
stage:
  worker: bob-developer
  name: develop
next_action:
  worker: human
  prompt: waiting for product decision
`)

	if err := state.Resume(featureDir); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load after Resume: %v", err)
	}
	if s.Status != "active" {
		t.Errorf("status = %q, want active", s.Status)
	}
	if s.NextAction.Worker != "" || s.NextAction.Prompt != "" {
		t.Errorf("NextAction not cleared: worker=%q prompt=%q", s.NextAction.Worker, s.NextAction.Prompt)
	}
	if len(s.History) == 0 {
		t.Fatal("expected history entry")
	}
	if got := s.History[len(s.History)-1].Result; got != "resumed" {
		t.Errorf("last history result = %q, want resumed", got)
	}
}

func TestLifecycleMutators(t *testing.T) {
	newFeature := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		if err := state.Create(dir, &state.State{
			Ticket: "ORC-1", Slug: "ORC-1-test", Status: "pending",
			Stage:      state.Stage{Name: "develop", Worker: "bob"},
			NextAction: state.NextAction{Worker: "human", Prompt: "old"},
		}); err != nil {
			t.Fatal(err)
		}
		return dir
	}

	t.Run("start", func(t *testing.T) {
		dir := newFeature(t)
		if err := state.Start(dir); err != nil {
			t.Fatal(err)
		}
		got, _ := state.Load(dir)
		if got.Status != "active" || len(got.History) != 1 || got.History[0].Result != "started" {
			t.Fatalf("state after Start = %+v", got)
		}
	})

	t.Run("pause", func(t *testing.T) {
		dir := newFeature(t)
		if err := state.Pause(dir, "needs approval", nil); err != nil {
			t.Fatal(err)
		}
		got, _ := state.Load(dir)
		if got.Status != "paused" || got.NextAction.Worker != "human" || got.NextAction.Prompt != "needs approval" {
			t.Fatalf("state after Pause = %+v", got)
		}
		if len(got.History) != 1 || got.History[0].Result != "paused — needs approval" {
			t.Fatalf("pause history = %+v", got.History)
		}
	})

	t.Run("done", func(t *testing.T) {
		dir := newFeature(t)
		if err := state.Done(dir, "shipped"); err != nil {
			t.Fatal(err)
		}
		got, _ := state.Load(dir)
		if got.Status != "done" || got.NextAction != (state.NextAction{}) || got.History[0].Result != "shipped" {
			t.Fatalf("state after Done = %+v", got)
		}
	})

	t.Run("status and history", func(t *testing.T) {
		dir := newFeature(t)
		if err := state.SetStatus(dir, "blocked"); err != nil {
			t.Fatal(err)
		}
		if err := state.AppendHistory(dir, "review", "fred", "changes requested"); err != nil {
			t.Fatal(err)
		}
		got, _ := state.Load(dir)
		if got.Status != "blocked" || len(got.History) != 1 || got.History[0].Worker != "fred" {
			t.Fatalf("state after SetStatus/AppendHistory = %+v", got)
		}
	})
}

func TestRuntimeMutators(t *testing.T) {
	dir := t.TempDir()
	if err := state.Create(dir, &state.State{Ticket: "ORC-2", Slug: "ORC-2-runtime", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := state.SetRuntime(dir, "ORC-2"); err != nil {
		t.Fatal(err)
	}
	got, _ := state.Load(dir)
	if got.Runtime.Mux == nil || got.Runtime.Mux.Backend != "tmux" || got.Runtime.Mux.Workspace != "ORC-2" || got.Runtime.Tmux != nil {
		t.Fatalf("runtime after SetRuntime = %+v", got.Runtime)
	}
	if err := state.SetRuntimeTarget(dir, "ORC-2", "%7"); err != nil {
		t.Fatal(err)
	}
	if err := state.SetMuxAgentRuntime(
		dir,
		state.MuxRuntime{Backend: "tmux", Workspace: "ORC-2", Tab: "develop", Pane: "%7"},
		state.AgentRuntime{ID: "agent-2", Instance: "instance-2", Stage: "develop", Engine: "codex", ProviderSessionID: "provider-2"},
	); err != nil {
		t.Fatal(err)
	}
	if err := state.SetJIT(dir, "default:fred", "review this"); err != nil {
		t.Fatal(err)
	}
	got, _ = state.Load(dir)
	if got.Runtime.Mux == nil || got.Runtime.Mux.Pane != "%7" || got.Runtime.Agent == nil || got.Runtime.Agent.ID != "agent-2" || got.Runtime.Agent.Instance != "instance-2" || got.Runtime.Agent.Stage != "develop" || got.Runtime.Agent.Engine != "codex" || got.Runtime.Agent.ProviderSessionID != "provider-2" || got.Runtime.JIT == nil || got.Runtime.JIT.Worker != "default:fred" || got.Runtime.JIT.StartedAt == "" {
		t.Fatalf("runtime after target/JIT = %+v", got.Runtime)
	}
	if err := state.ClearJIT(dir); err != nil {
		t.Fatal(err)
	}
	got, _ = state.Load(dir)
	if got.Runtime.JIT != nil || got.Runtime.Mux == nil {
		t.Fatalf("runtime after ClearJIT = %+v", got.Runtime)
	}
	if err := state.ClearRuntime(dir); err != nil {
		t.Fatal(err)
	}
	got, _ = state.Load(dir)
	if got.Runtime != (state.Runtime{}) {
		t.Fatalf("runtime after ClearRuntime = %+v", got.Runtime)
	}
}

func TestSetMuxAgentRuntimeRejectsIncompleteIdentity(t *testing.T) {
	dir := t.TempDir()
	if err := state.Create(dir, &state.State{Ticket: "ORC-3", Slug: "ORC-3-runtime", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	target := state.MuxRuntime{Backend: "tmux", Workspace: "ORC-3", Tab: "develop", Pane: "%3"}
	if err := state.SetMuxAgentRuntime(dir, target, state.AgentRuntime{ID: "agent-3"}); err == nil {
		t.Fatal("SetMuxAgentRuntime should reject a missing instance id")
	}
	got, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Runtime != (state.Runtime{}) {
		t.Fatalf("invalid identity mutated runtime: %+v", got.Runtime)
	}
}

func TestMuxTargetReadsNewAndLegacyRuntime(t *testing.T) {
	t.Run("new", func(t *testing.T) {
		runtime := state.Runtime{Mux: &state.MuxRuntime{Backend: "herdr", Workspace: "w7", Tab: "w7:t2", Pane: "w7:p4"}}
		got, ok := runtime.MuxTarget("develop")
		if !ok || got.Backend != "herdr" || got.Workspace != "w7" || got.Tab != "w7:t2" || got.Pane != "w7:p4" {
			t.Fatalf("MuxTarget() = %+v, %v", got, ok)
		}
	})

	t.Run("legacy tmux", func(t *testing.T) {
		runtime := state.Runtime{Tmux: &state.TmuxRuntime{Session: "ORC-7", Pane: "%4"}}
		got, ok := runtime.MuxTarget("review")
		if !ok || got.Backend != "tmux" || got.Workspace != "ORC-7" || got.Tab != "review" || got.Pane != "%4" {
			t.Fatalf("MuxTarget() = %+v, %v", got, ok)
		}
		if runtime.Mux != nil {
			t.Fatal("legacy projection mutated runtime")
		}
	})
}

func TestInspectLockAndArchivedLookup(t *testing.T) {
	dir := t.TempDir()
	lock, err := state.InspectLock(dir)
	if err != nil || lock.Status != state.LockMissing {
		t.Fatalf("missing lock = %+v, %v", lock, err)
	}
	if err := os.WriteFile(filepath.Join(dir, state.Filename+".lock"), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err = state.InspectLock(dir)
	if err != nil || lock.Status != state.LockActive || lock.PID != os.Getpid() {
		t.Fatalf("active lock = %+v, %v", lock, err)
	}

	workspace := t.TempDir()
	archived := filepath.Join(workspace, "features", "_archive", "ORC-9-old")
	if err := os.MkdirAll(archived, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := state.FindFeatureDirWithArchive(workspace, "ORC-9")
	if err != nil || got != archived {
		t.Fatalf("archived lookup = %q, %v", got, err)
	}

	errs := state.RepoValidationErrors{{Field: "repos.api.main", Message: "missing"}}
	if text := errs.Error(); !strings.Contains(text, "repos.api.main") || !strings.Contains(text, "missing") {
		t.Fatalf("RepoValidationErrors.Error() = %q", text)
	}
}
