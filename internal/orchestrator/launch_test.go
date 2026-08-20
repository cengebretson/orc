package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/mux/muxtest"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
)

type nativeTargetFake struct {
	*muxtest.Fake
	createTarget         func(name, dir string, tabs []string) (mux.Target, error)
	createWorktreeTarget func(spec mux.WorktreeTargetSpec) (mux.Target, error)
	sendTarget           func(target mux.Target, tab, dir, runDir string, argv []string) (mux.Target, error)
	setTargetMetadata    func(target mux.Target, meta mux.Metadata) error
	configureTaskCell    func(target mux.Target, spec mux.TaskCellSpec) error
}

type agentLaunchFake struct {
	*nativeTargetFake
	prepareAgentLaunch func(target mux.Target, tab, dir string, meta mux.Metadata, argv []string) (mux.Target, []string, error)
}

func (f *agentLaunchFake) PrepareAgentLaunch(target mux.Target, tab, dir string, meta mux.Metadata, argv []string) (mux.Target, []string, error) {
	return f.prepareAgentLaunch(target, tab, dir, meta, argv)
}

func (f *nativeTargetFake) CreateTarget(name, dir string, tabs []string) (mux.Target, error) {
	return f.createTarget(name, dir, tabs)
}

func (f *nativeTargetFake) CreateWorktreeTarget(spec mux.WorktreeTargetSpec) (mux.Target, error) {
	return f.createWorktreeTarget(spec)
}

func (f *nativeTargetFake) SendTarget(target mux.Target, tab, dir, runDir string, argv []string) (mux.Target, error) {
	return f.sendTarget(target, tab, dir, runDir, argv)
}

func (f *nativeTargetFake) SetTargetMetadata(target mux.Target, meta mux.Metadata) error {
	if f.setTargetMetadata == nil {
		return nil
	}
	return f.setTargetMetadata(target, meta)
}

func (f *nativeTargetFake) ConfigureTaskCell(target mux.Target, spec mux.TaskCellSpec) error {
	if f.configureTaskCell == nil {
		return nil
	}
	return f.configureTaskCell(target, spec)
}

func (f *nativeTargetFake) AttachTarget(mux.Target) error { return nil }

func (f *nativeTargetFake) AttachTargetHint(target mux.Target) string {
	return "herdr agent attach " + target.Pane
}

func TestLauncherAssignsAgentIdentityBeforeLaunchAndPersistsItAtomically(t *testing.T) {
	target := mux.Target{Backend: "tmux", Workspace: "orc-9", Tab: "develop", Pane: "%9"}
	s := &state.State{
		Ticket: "ORC-9", Slug: "orc-9", Stage: state.Stage{Name: "develop"},
		Runtime: state.Runtime{Agent: &state.AgentRuntime{
			ID: "agent-existing", Instance: "instance-old", Engine: "codex", ProviderSessionID: "provider-9",
		}},
	}
	plan := &runner.Plan{
		CWD: "/work", LaunchArgv: []string{"codex", "build this"},
		Worker: &workers.Worker{ID: "dev", Engine: "codex"},
	}
	var prepared mux.Metadata
	var sentArgv []string
	var persistedTarget state.MuxRuntime
	var persistedAgent state.AgentRuntime
	base := &nativeTargetFake{
		Fake: &muxtest.Fake{
			NameFunc: func() string { return "tmux" }, AvailableFunc: func() bool { return true },
			SessionExistsFunc: func(string) bool { return false },
		},
		createTarget: func(string, string, []string) (mux.Target, error) { return target, nil },
		sendTarget: func(got mux.Target, _ string, _ string, _ string, argv []string) (mux.Target, error) {
			sentArgv = append([]string(nil), argv...)
			return got, nil
		},
		setTargetMetadata: func(_ mux.Target, meta mux.Metadata) error {
			prepared = meta
			return nil
		},
	}
	fake := &agentLaunchFake{
		nativeTargetFake: base,
		prepareAgentLaunch: func(got mux.Target, _ string, _ string, meta mux.Metadata, argv []string) (mux.Target, []string, error) {
			prepared = meta
			return got, append([]string{"prepared"}, argv...), nil
		},
	}
	launcher := Launcher{
		Mux: fake,
		SetMuxAgentRuntime: func(_ string, gotTarget state.MuxRuntime, gotAgent state.AgentRuntime) error {
			persistedTarget, persistedAgent = gotTarget, gotAgent
			return nil
		},
		NewAgentID: func() (string, error) {
			t.Fatal("existing durable agent id should be reused")
			return "", nil
		},
		NewInstanceID: func() (string, error) { return "instance-new", nil },
		AppendHistory: func(string, string, string, string) error { return nil },
		RunForeground: func(LaunchOptions) error {
			t.Fatal("foreground should not run")
			return nil
		},
	}
	result, err := launcher.Launch(LaunchOptions{FeatureDir: "/feature", State: s, Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if result.Pane != "%9" || !reflect.DeepEqual(sentArgv, []string{"prepared", "codex", "build this"}) {
		t.Fatalf("result/argv = %+v / %#v", result, sentArgv)
	}
	if prepared.AgentID != "agent-existing" || prepared.AgentInstance != "instance-new" || prepared.ProviderSessionID != "provider-9" {
		t.Fatalf("prepared metadata = %+v", prepared)
	}
	if persistedTarget.Pane != "%9" || persistedAgent.ID != "agent-existing" || persistedAgent.Instance != "instance-new" || persistedAgent.Stage != "develop" || persistedAgent.ProviderSessionID != "provider-9" {
		t.Fatalf("persisted target/agent = %+v / %+v", persistedTarget, persistedAgent)
	}
}

func TestNextAgentRuntimeReplacesIdentityWhenEngineChanges(t *testing.T) {
	launcher := Launcher{
		NewAgentID:    func() (string, error) { return "agent-new", nil },
		NewInstanceID: func() (string, error) { return "instance-new", nil },
	}
	got, err := launcher.nextAgentRuntime(LaunchOptions{
		State: &state.State{Runtime: state.Runtime{Agent: &state.AgentRuntime{
			ID: "agent-old", Instance: "instance-old", Engine: "claude", ProviderSessionID: "provider-old",
		}}},
		Plan: &runner.Plan{Worker: &workers.Worker{Engine: "codex"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "agent-new" || got.Instance != "instance-new" || got.Stage != "" || got.Engine != "codex" || got.ProviderSessionID != "" {
		t.Fatalf("next agent runtime = %+v", got)
	}
}

func TestNextAgentRuntimeReplacesIdentityWhenStageChanges(t *testing.T) {
	launcher := Launcher{
		NewAgentID:    func() (string, error) { return "agent-new", nil },
		NewInstanceID: func() (string, error) { return "instance-new", nil },
	}
	got, err := launcher.nextAgentRuntime(LaunchOptions{
		State: &state.State{
			Stage: state.Stage{Name: "review"},
			Runtime: state.Runtime{Agent: &state.AgentRuntime{
				ID: "agent-old", Instance: "instance-old", Stage: "develop", Engine: "codex", ProviderSessionID: "provider-old",
			}},
		},
		Plan: &runner.Plan{Worker: &workers.Worker{Engine: "codex"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "agent-new" || got.Instance != "instance-new" || got.Stage != "review" || got.Engine != "codex" || got.ProviderSessionID != "" {
		t.Fatalf("next agent runtime = %+v", got)
	}
}

func TestLauncherCreatesNativeWorktreeTarget(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	featureDir := filepath.Join(root, "features", "ORC-9-native")
	for _, dir := range []string{source, featureDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configYAML := `
settings:
  default_workflow: default
  herdr:
    task_cell:
      test_command: make test
      watch: true
repos:
  - name: app
    path: source
workflows:
  default:
    stages:
      - name: develop
        worker: dev
      - name: review
        worker: reviewer
`
	if err := os.WriteFile(filepath.Join(root, "orc.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	s := &state.State{
		Ticket: "ORC-9", Slug: "ORC-9-native", Workflow: "default",
		Stage: state.Stage{Name: "develop"}, NextAction: state.NextAction{CWD: "."},
	}
	plan := &runner.Plan{
		CWD: root, Prompt: "build this", LaunchArgv: []string{"codex", "--cd", root, "build this"},
		Worker: &workers.Worker{ID: "dev", Name: "Dev", Engine: "codex", Model: "gpt-5"},
	}

	var created mux.WorktreeTargetSpec
	var recorded worktreeLaunch
	var contextDir, contextProject, contextSlug string
	var sentDir, sentRunDir string
	var sentArgv []string
	var taskCell mux.TaskCellSpec
	target := mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"}
	fake := &nativeTargetFake{
		Fake: &muxtest.Fake{
			NameFunc: func() string { return "herdr" }, AvailableFunc: func() bool { return true },
			SessionExistsFunc: func(string) bool { return false },
		},
		createTarget: func(name, dir string, tabs []string) (mux.Target, error) {
			t.Fatal("generic target creation should not run")
			return mux.Target{}, nil
		},
		createWorktreeTarget: func(spec mux.WorktreeTargetSpec) (mux.Target, error) {
			created = spec
			return target, nil
		},
		sendTarget: func(got mux.Target, tab, dir, runDir string, argv []string) (mux.Target, error) {
			sentDir, sentRunDir, sentArgv = dir, runDir, append([]string(nil), argv...)
			return got, nil
		},
		configureTaskCell: func(got mux.Target, spec mux.TaskCellSpec) error {
			taskCell = spec
			return nil
		},
	}
	launcher := Launcher{
		Mux: fake,
		RecordWorktree: func(featureDir, root string, launch worktreeLaunch) error {
			recorded = launch
			return nil
		},
		WriteWorktreeContext: func(worktreeDir, project, featureSlug string) error {
			contextDir, contextProject, contextSlug = worktreeDir, project, featureSlug
			return nil
		},
		SetMuxRuntime: func(string, state.MuxRuntime) error { return nil },
		AppendHistory: func(string, string, string, string) error { return nil },
		RunForeground: func(LaunchOptions) error {
			t.Fatal("foreground should not run")
			return nil
		},
	}

	result, err := launcher.Launch(LaunchOptions{Root: root, FeatureDir: featureDir, State: s, Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	wantWorktree := filepath.Join(root, "worktrees", "app", "ORC-9-native")
	if created.SourceDir != source || created.WorktreeDir != wantWorktree || created.Branch != "feature/orc-9-native" || !reflect.DeepEqual(created.Tabs, []string{"develop", "review"}) {
		t.Fatalf("created spec = %#v", created)
	}
	if recorded.Spec.WorktreeDir != wantWorktree || sentDir != wantWorktree || sentRunDir != wantWorktree {
		t.Fatalf("recorded/sent paths = %#v, %q, %q", recorded, sentDir, sentRunDir)
	}
	if contextDir != wantWorktree || contextProject != "ORC-9" || contextSlug != "ORC-9-native" {
		t.Fatalf("tmux-attention context = %q, %q, %q", contextDir, contextProject, contextSlug)
	}
	if joined := strings.Join(sentArgv, " "); !strings.Contains(joined, "--cd "+wantWorktree) || sentArgv[len(sentArgv)-1] != "build this" {
		t.Fatalf("sent argv = %#v", sentArgv)
	}
	if taskCell.CWD != wantWorktree || taskCell.TestCommand != "make test" || taskCell.WatchCommand != "orc --workspace '"+root+"' --mux herdr watch 'ORC-9'" {
		t.Fatalf("task cell = %#v", taskCell)
	}
	if taskCell.Metadata.Ticket != "ORC-9" || taskCell.Metadata.FeatureSlug != "ORC-9-native" || taskCell.Metadata.Worker != "dev" {
		t.Fatalf("task cell metadata = %#v", taskCell.Metadata)
	}
	if s.Repos["app"].Worktree != filepath.Join("worktrees", "app", "ORC-9-native") || s.NextAction.CWD != filepath.Join("worktrees", "app", "ORC-9-native") {
		t.Fatalf("state worktree = %#v, cwd = %q", s.Repos, s.NextAction.CWD)
	}
	if result.Session != "w9" || result.Pane != "p1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestLauncherLaunchesInTmux(t *testing.T) {
	s := &state.State{
		Ticket: "TICKET-1",
		Slug:   "TICKET-1",
		Stage: state.Stage{
			Name: "develop",
		},
	}
	plan := &runner.Plan{
		CWD:        "/workspace",
		LaunchArgv: []string{"codex", "do it"},
		Worker:     &workers.Worker{Name: "Dev", Engine: "codex"},
	}

	var createdSession string
	var sent []string
	var sendEvent []string
	var history []string
	var metadata mux.Metadata

	launcher := Launcher{
		Mux: &muxtest.Fake{
			AvailableFunc:     func() bool { return true },
			SessionExistsFunc: func(string) bool { return false },
			CreateSessionFunc: func(name, dir string, windows []string) error {
				createdSession = name
				return nil
			},
			SendCommandFunc: func(session, window, pane, dir, runDir string, argv []string) (string, error) {
				sent = []string{session, window, runDir}
				return "%1", nil
			},
			SetWindowMetadataFunc: func(session, window string, got mux.Metadata) error {
				if session != "TICKET-1" || window != "develop" {
					t.Fatalf("SetWindowMetadata target = %s:%s", session, window)
				}
				metadata = got
				return nil
			},
		},
		SetRuntime: func(featureDir, tmuxSession string) error { return nil },
		AppendHistory: func(featureDir, stage, workerID, result string) error {
			history = []string{stage, workerID, result}
			return nil
		},
		RunForeground: func(opts LaunchOptions) error {
			t.Fatal("foreground should not run")
			return nil
		},
	}

	result, err := launcher.Launch(LaunchOptions{
		FeatureDir: "/feature",
		State:      s,
		Plan:       plan,
		OnTmuxSend: func(session, window string) {
			sendEvent = []string{session, window}
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}

	if result.Mode != LaunchModeTmux {
		t.Fatalf("Mode = %q, want %q", result.Mode, LaunchModeTmux)
	}
	if createdSession != "TICKET-1" {
		t.Errorf("createdSession = %q, want TICKET-1", createdSession)
	}
	if !reflect.DeepEqual(sent, []string{"TICKET-1", "develop", "/workspace"}) {
		t.Errorf("sent = %#v", sent)
	}
	if !reflect.DeepEqual(sendEvent, []string{"TICKET-1", "develop"}) {
		t.Errorf("sendEvent = %#v", sendEvent)
	}
	if metadata.Ticket != "TICKET-1" || metadata.Stage != "develop" || metadata.Worker != "Dev" || metadata.Engine != "codex" || metadata.FeatureDir != "/feature" {
		t.Errorf("metadata = %#v", metadata)
	}
	if result.AttachHint != "TICKET-1:develop" {
		t.Errorf("AttachHint = %q", result.AttachHint)
	}
	if result.Pane != "%1" {
		t.Errorf("Pane = %q, want %%1", result.Pane)
	}
	if !reflect.DeepEqual(history, []string{"develop", "", "launched in tmux session TICKET-1:develop"}) {
		t.Errorf("history = %#v", history)
	}
}

func TestLauncherAdoptsExistingSessionWhenRuntimeUnset(t *testing.T) {
	// A prior launch created the session but failed to persist runtime, so
	// STATE.yaml has no runtime.tmux yet the session is live. The next launch
	// must adopt it (and re-persist runtime), not re-create it.
	s := &state.State{
		Slug:  "TICKET-1",
		Stage: state.Stage{Name: "develop"},
	}
	plan := &runner.Plan{
		CWD:        "/workspace",
		LaunchArgv: []string{"codex", "do it"},
		Worker:     &workers.Worker{Name: "Dev", Engine: "codex"},
	}

	var setRuntime []string
	var sent []string
	launcher := Launcher{
		Mux: &muxtest.Fake{
			AvailableFunc:     func() bool { return true },
			SessionExistsFunc: func(session string) bool { return session == "TICKET-1" },
			CreateSessionFunc: func(name, dir string, windows []string) error {
				t.Fatal("should adopt existing session, not create a new one")
				return nil
			},
			SendCommandFunc: func(session, window, pane, dir, runDir string, argv []string) (string, error) {
				sent = []string{session, window, runDir}
				return "%1", nil
			},
		},
		SetRuntime: func(featureDir, tmuxSession string) error {
			setRuntime = []string{featureDir, tmuxSession}
			return nil
		},
		AppendHistory: func(featureDir, stage, workerID, result string) error { return nil },
		RunForeground: func(opts LaunchOptions) error {
			t.Fatal("foreground should not run")
			return nil
		},
	}

	result, err := launcher.Launch(LaunchOptions{
		FeatureDir: "/feature",
		State:      s,
		Plan:       plan,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if result.Mode != LaunchModeTmux {
		t.Fatalf("Mode = %q, want %q", result.Mode, LaunchModeTmux)
	}
	if !reflect.DeepEqual(setRuntime, []string{"/feature", "TICKET-1"}) {
		t.Errorf("setRuntime = %#v, want runtime re-persisted for adopted session", setRuntime)
	}
	if !reflect.DeepEqual(sent, []string{"TICKET-1", "develop", "/workspace"}) {
		t.Errorf("sent = %#v", sent)
	}
}

func TestLauncherFallsBackToForegroundWhenTmuxCreateFails(t *testing.T) {
	s := &state.State{
		Slug:  "TICKET-1",
		Stage: state.Stage{Name: "develop"},
	}
	plan := &runner.Plan{
		CWD:        "/workspace",
		LaunchArgv: []string{"codex", "do it"},
		Worker:     &workers.Worker{Name: "Dev", Engine: "codex"},
	}

	var foregroundRan bool
	var fallbackMessages []string
	var history []string
	createErr := errors.New("no tmux")

	launcher := Launcher{
		Mux: &muxtest.Fake{
			AvailableFunc: func() bool { return true },
			CreateSessionFunc: func(name, dir string, windows []string) error {
				return createErr
			},
			SendCommandFunc: func(session, window, pane, dir, runDir string, argv []string) (string, error) {
				t.Fatal("send should not run after create failure")
				return "", nil
			},
		},
		RunForeground: func(opts LaunchOptions) error {
			foregroundRan = true
			return nil
		},
		AppendHistory: func(featureDir, stage, workerID, result string) error {
			history = []string{stage, workerID, result}
			return nil
		},
	}

	result, err := launcher.Launch(LaunchOptions{
		FeatureDir: "/feature",
		State:      s,
		Plan:       plan,
		OnFallback: func(message string) {
			fallbackMessages = append(fallbackMessages, message)
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if result.Mode != LaunchModeForeground {
		t.Fatalf("Mode = %q, want %q", result.Mode, LaunchModeForeground)
	}
	if !foregroundRan {
		t.Fatal("foreground did not run")
	}
	if len(fallbackMessages) != 1 || fallbackMessages[0] != "tmux session create failed (no tmux)" {
		t.Fatalf("fallbackMessages = %#v", fallbackMessages)
	}
	if !reflect.DeepEqual(history, []string{"develop", "", "launched in foreground"}) {
		t.Errorf("history = %#v", history)
	}
}

func TestLauncherUsesExistingTmuxWindowOverride(t *testing.T) {
	s := &state.State{
		Slug: "TICKET-1",
		Stage: state.Stage{
			Name: "develop",
		},
		Runtime: state.Runtime{
			Tmux: &state.TmuxRuntime{Session: "existing"},
		},
	}
	plan := &runner.Plan{
		CWD:        "/feature",
		LaunchArgv: []string{"codex", "do it"},
		Worker:     &workers.Worker{Name: "Dev", Engine: "codex"},
	}

	var sent []string
	var history []string
	launcher := Launcher{
		Mux: &muxtest.Fake{
			AvailableFunc:     func() bool { return true },
			SessionExistsFunc: func(session string) bool { return session == "existing" },
			CreateSessionFunc: func(name, dir string, windows []string) error {
				t.Fatal("existing-session launch should not create a session")
				return nil
			},
			SendCommandFunc: func(session, window, pane, dir, runDir string, argv []string) (string, error) {
				sent = []string{session, window, runDir}
				return "%2", nil
			},
		},
		AppendHistory: func(featureDir, stage, workerID, result string) error {
			history = []string{stage, workerID, result}
			return nil
		},
		RunForeground: func(opts LaunchOptions) error {
			t.Fatal("foreground should not run")
			return nil
		},
	}

	result, err := launcher.Launch(LaunchOptions{
		FeatureDir:          "/feature",
		State:               s,
		Plan:                plan,
		Window:              "jit",
		RequireExistingTmux: true,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if result.Mode != LaunchModeTmux {
		t.Fatalf("Mode = %q, want %q", result.Mode, LaunchModeTmux)
	}
	if !reflect.DeepEqual(sent, []string{"existing", "jit", "/feature"}) {
		t.Errorf("sent = %#v", sent)
	}
	if !reflect.DeepEqual(history, []string{"jit", "", "launched in tmux session existing:jit"}) {
		t.Errorf("history = %#v", history)
	}
}

func TestLauncherSkipsTmuxWhenDisabled(t *testing.T) {
	s := &state.State{Slug: "TICKET-1", Stage: state.Stage{Name: "develop"}}
	plan := &runner.Plan{
		CWD:        "/feature",
		LaunchArgv: []string{"codex", "do it"},
		Worker:     &workers.Worker{Name: "Dev", Engine: "codex"},
	}

	var foregroundRan bool
	var history []string
	launcher := Launcher{
		Mux: &muxtest.Fake{
			AvailableFunc: func() bool {
				t.Fatal("tmux availability should not be checked")
				return true
			},
		},
		RunForeground: func(opts LaunchOptions) error {
			foregroundRan = true
			return nil
		},
		AppendHistory: func(featureDir, stage, workerID, result string) error {
			history = []string{stage, workerID, result}
			return nil
		},
	}

	result, err := launcher.Launch(LaunchOptions{
		FeatureDir:  "/feature",
		State:       s,
		Plan:        plan,
		DisableTmux: true,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if result.Mode != LaunchModeForeground {
		t.Fatalf("Mode = %q, want %q", result.Mode, LaunchModeForeground)
	}
	if !foregroundRan {
		t.Fatal("foreground did not run")
	}
	if !reflect.DeepEqual(history, []string{"develop", "", "launched in foreground"}) {
		t.Errorf("history = %#v", history)
	}
}

func TestLauncherRecordsForegroundFailure(t *testing.T) {
	s := &state.State{Slug: "TICKET-1", Stage: state.Stage{Name: "develop"}}
	plan := &runner.Plan{
		CWD:        "/feature",
		LaunchArgv: []string{"codex", "do it"},
		Worker:     &workers.Worker{ID: "dev", Name: "Dev", Engine: "codex"},
	}

	runErr := errors.New("agent failed")
	var history []string
	launcher := Launcher{
		Mux: &muxtest.Fake{},
		RunForeground: func(opts LaunchOptions) error {
			return runErr
		},
		AppendHistory: func(featureDir, stage, workerID, result string) error {
			history = []string{stage, workerID, result}
			return nil
		},
	}

	result, err := launcher.Launch(LaunchOptions{
		FeatureDir: "/feature",
		State:      s,
		Plan:       plan,
	})
	if !errors.Is(err, runErr) {
		t.Fatalf("Launch error = %v, want %v", err, runErr)
	}
	if result.Mode != LaunchModeForeground {
		t.Fatalf("Mode = %q, want %q", result.Mode, LaunchModeForeground)
	}
	if !reflect.DeepEqual(history, []string{"develop", "dev", "launch failed in foreground: agent failed"}) {
		t.Errorf("history = %#v", history)
	}
}
