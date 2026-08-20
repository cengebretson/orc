package herdr

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

func TestCreateWorktreeTargetCreatesMissingCheckout(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	worktree := filepath.Join(root, "linked")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}

	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch args[0] + " " + args[1] {
		case "worktree create":
			return response(`{"workspace":{"workspace_id":"w9","label":"ORC-9"},"tab":{"tab_id":"t1","workspace_id":"w9","label":"1"},"root_pane":{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"}}`), nil
		case "tab rename", "tab create":
			return response(`{}`), nil
		default:
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	target, err := b.CreateWorktreeTarget(mux.WorktreeTargetSpec{
		Name: "ORC-9", SourceDir: source, WorktreeDir: worktree,
		Branch: "feature/orc-9", Tabs: []string{"develop", "review"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := (mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"}); !reflect.DeepEqual(target, want) {
		t.Fatalf("target = %#v, want %#v", target, want)
	}
	if !strings.Contains(calls[0], "worktree create --cwd "+source+" --branch feature/orc-9 --path "+worktree+" --label ORC-9 --no-focus --json") {
		t.Fatalf("create call = %q", calls[0])
	}
	if calls[1] != "tab rename t1 develop" || !strings.Contains(calls[2], "tab create --workspace w9 --cwd "+worktree+" --label review") {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestShowNotificationUsesNativeHerdrSurface(t *testing.T) {
	var call string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call = strings.Join(args, " ")
		return []byte("ok"), nil
	}}

	err := b.ShowNotification(mux.Notification{
		Title: "Orc · ORC-9 blocked",
		Body:  "Stage: review · Workflow: default",
		Sound: "request",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "notification show Orc · ORC-9 blocked --body Stage: review · Workflow: default --sound request"
	if call != want {
		t.Fatalf("call = %q, want %q", call, want)
	}
}

func TestShowNotificationRequiresTitle(t *testing.T) {
	b := Backend{run: func(args ...string) ([]byte, error) {
		t.Fatalf("unexpected command: %v", args)
		return nil, nil
	}}
	if err := b.ShowNotification(mux.Notification{}); err == nil {
		t.Fatal("expected empty-title error")
	}
}

func TestStateAgentUsesExactPaneAndReturnsLifecycle(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9"}}`), nil
		case "tab get w9:t1":
			return response(`{"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}`), nil
		case "pane get w9:p1":
			return response(`{"pane":{"pane_id":"w9:p1","workspace_id":"w9","tab_id":"w9:t1"}}`), nil
		case "agent get w9:p1":
			return response(`{"agent":{"name":"builder","agent":"codex","agent_status":"working","state_change_seq":14}}`), nil
		default:
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	result, err := b.StateAgent(mux.Target{
		Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Backend != "herdr" || result.Target.Pane != "w9:p1" || result.Agent != "codex" || result.Name != "builder" || result.Lifecycle != "working" || result.StateChangeSeq != 14 {
		t.Fatalf("result = %#v", result)
	}
	if len(calls) != 4 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestCaptureTargetUsesExactPane(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9"}}`), nil
		case "tab get w9:t1":
			return response(`{"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}`), nil
		case "pane get w9:p1":
			return response(`{"pane":{"pane_id":"w9:p1","workspace_id":"w9","tab_id":"w9:t1"}}`), nil
		case "agent read w9:p1 --source recent-unwrapped --lines 25":
			return []byte("one\ntwo\n"), nil
		default:
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	got, err := b.CaptureTarget(mux.Target{
		Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1",
	}, 25)
	if err != nil {
		t.Fatal(err)
	}
	if got != "one\ntwo\n" || len(calls) != 4 {
		t.Fatalf("capture=%q calls=%#v", got, calls)
	}
}

func TestPromptAgentUsesExactPaneAndHerdrWaitSemantics(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9"}}`), nil
		case "tab get w9:t1":
			return response(`{"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}`), nil
		case "pane get w9:p1":
			return response(`{"pane":{"pane_id":"w9:p1","workspace_id":"w9","tab_id":"w9:t1"}}`), nil
		case "agent prompt w9:p1 review this --wait --until blocked --timeout 120000":
			return response(`{"type":"agent_prompted","agent":{"name":"builder","agent":"codex","agent_status":"blocked","state_change_seq":12}}`), nil
		default:
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	result, err := b.PromptAgent(
		mux.Target{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"},
		"review this", true,
		mux.AgentControlOptions{Until: []string{"blocked"}, Timeout: 2 * time.Minute},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Backend != "herdr" || result.Target.Pane != "w9:p1" || result.Agent != "codex" || result.Name != "builder" || result.Lifecycle != "blocked" || result.StateChangeSeq != 12 {
		t.Fatalf("result = %#v", result)
	}
	if len(calls) != 4 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestWaitAgentUsesHerdrLifecycleWait(t *testing.T) {
	b := Backend{run: func(args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9"}}`), nil
		case "tab get w9:t1":
			return response(`{"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}`), nil
		case "pane get w9:p1":
			return response(`{"pane":{"pane_id":"w9:p1","workspace_id":"w9","tab_id":"w9:t1"}}`), nil
		case "agent wait w9:p1 --until done --timeout 30000":
			return response(`{"agent":{"agent":"claude","agent_status":"done","state_change_seq":18}}`), nil
		default:
			return nil, errors.New("unexpected command: " + strings.Join(args, " "))
		}
	}}

	result, err := b.WaitAgent(
		mux.Target{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"},
		mux.AgentControlOptions{Until: []string{"done"}, Timeout: 30 * time.Second},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Lifecycle != "done" || result.Agent != "claude" {
		t.Fatalf("result = %#v", result)
	}
}

func TestWaitAgentHonorsCancelledContextBeforeCallingHerdr(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	b := Backend{run: func(args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}}
	_, err := b.WaitAgent(mux.Target{
		Backend: "herdr", Workspace: "w1", Tab: "t1", Pane: "p1",
	}, mux.AgentControlOptions{Context: ctx})
	var controlErr *mux.AgentControlError
	if !errors.As(err, &controlErr) || controlErr.Code != "cancelled" {
		t.Fatalf("WaitAgent() error = %#v", err)
	}
	if called {
		t.Fatal("WaitAgent() invoked the Herdr CLI after cancellation")
	}
}

func TestPromptAgentPreservesHerdrStallErrorCode(t *testing.T) {
	b := Backend{run: func(args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9"}}`), nil
		case "tab get w9:t1":
			return response(`{"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}`), nil
		case "pane get w9:p1":
			return response(`{"pane":{"pane_id":"w9:p1","workspace_id":"w9","tab_id":"w9:t1"}}`), nil
		case "agent prompt w9:p1 dropped --wait --timeout 6000":
			return []byte(`{"error":{"code":"agent_prompt_stalled","message":"state_change_seq remained 9"}}`), errors.New("exit status 1")
		default:
			return nil, errors.New("unexpected command: " + strings.Join(args, " "))
		}
	}}

	_, err := b.PromptAgent(
		mux.Target{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"},
		"dropped", true, mux.AgentControlOptions{Timeout: 6 * time.Second},
	)
	var controlErr *mux.AgentControlError
	if !errors.As(err, &controlErr) || controlErr.Code != "agent_prompt_stalled" {
		t.Fatalf("error = %#v", err)
	}
}

func TestWaitAgentRejectsStaleExactPaneWithoutFallback(t *testing.T) {
	b := Backend{run: func(args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9"}}`), nil
		case "tab get w9:t1":
			return response(`{"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}`), nil
		case "pane get w9:p1":
			return response(`{"pane":{"pane_id":"w9:p1","workspace_id":"w9","tab_id":"w9:t2"}}`), nil
		default:
			t.Fatalf("unexpected fallback command: %s", strings.Join(args, " "))
			return nil, nil
		}
	}}

	_, err := b.WaitAgent(
		mux.Target{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"},
		mux.AgentControlOptions{Timeout: time.Second},
	)
	if err == nil || !strings.Contains(err.Error(), "not in recorded tab") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateWorktreeTargetOpensExistingCheckout(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	worktree := filepath.Join(root, "linked")
	for _, dir := range []string{source, worktree} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var call string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call = strings.Join(args, " ")
		return response(`{"workspace":{"workspace_id":"w10","label":"ORC-10"},"tab":{"tab_id":"t2","workspace_id":"w10","label":"develop"},"root_pane":{"pane_id":"p2","workspace_id":"w10","tab_id":"t2"}}`), nil
	}}

	target, err := b.CreateWorktreeTarget(mux.WorktreeTargetSpec{
		Name: "ORC-10", SourceDir: source, WorktreeDir: worktree,
		Branch: "feature/orc-10", Tabs: []string{"develop"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.Workspace != "w10" || target.Tab != "t2" || target.Pane != "p2" {
		t.Fatalf("target = %#v", target)
	}
	want := "worktree open --cwd " + source + " --path " + worktree + " --label ORC-10 --no-focus --json"
	if call != want {
		t.Fatalf("call = %q, want %q", call, want)
	}
}

func TestCreateTargetReturnsExactHerdrIDs(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		switch args[0] + " " + args[1] {
		case "workspace create":
			return response(`{"workspace":{"workspace_id":"w9","label":"ORC-9"},"tab":{"tab_id":"t1","workspace_id":"w9","label":"shell"},"root_pane":{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"}}`), nil
		case "tab rename", "tab create":
			return response(`{}`), nil
		default:
			return nil, errors.New("unexpected command")
		}
	}}

	target, err := b.CreateTarget("ORC-9", "/work/orc-9", []string{"develop", "review"})
	if err != nil {
		t.Fatal(err)
	}
	want := mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"}
	if !reflect.DeepEqual(target, want) {
		t.Fatalf("target = %#v, want %#v", target, want)
	}
	if len(calls) != 3 || !strings.Contains(calls[0], "--env ORC=1 --no-focus") || calls[1] != "tab rename t1 develop" || !strings.Contains(calls[2], "--workspace w9") {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestListPanesMapsLifecycleIdentityAndExactTarget(t *testing.T) {
	b := Backend{run: func(args ...string) ([]byte, error) {
		switch strings.Join(args, " ") {
		case "workspace list":
			return response(`{"workspaces":[{"workspace_id":"w9","label":"ORC-9"}]}`), nil
		case "pane list --workspace w9":
			return response(`{"panes":[{"pane_id":"p1","workspace_id":"w9","tab_id":"t1","agent":"codex","agent_status":"blocked","foreground_cwd":"/work/orc-9","agent_session":{"value":"session-9"},"tokens":{"agent_id":"agent-9","agent_instance":"instance-9","ticket":"ORC-9","stage":"develop","worker":"builder","feature_dir":"/work/orc-9"}}]}`), nil
		default:
			return nil, errors.New("unexpected command: " + strings.Join(args, " "))
		}
	}}

	panes, err := b.ListPanes()
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 {
		t.Fatalf("panes = %#v", panes)
	}
	pane := panes[0]
	if pane.Backend != "herdr" || pane.Session != "w9" || pane.Window != "t1" || pane.ID != "p1" || pane.Lifecycle != "blocked" || pane.LifecycleSource != "native" || pane.Attention != mux.AttentionBlocked || pane.AttentionSource != "native" {
		t.Fatalf("pane target/lifecycle = %#v", pane)
	}
	if pane.AgentID != "agent-9" || pane.AgentInstance != "instance-9" || pane.Ticket != "ORC-9" || pane.Stage != "develop" || pane.Worker != "builder" || pane.ProviderSessionID != "session-9" {
		t.Fatalf("pane identity = %#v", pane)
	}
}

func TestSendTargetStartsAndPromptsHerdrAgent(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9","label":"ORC-9"}}`), nil
		case "tab get t1":
			return response(`{"tab":{"tab_id":"t1","workspace_id":"w9","label":"develop"}}`), nil
		case "pane get p1":
			return response(`{"pane":{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"}}`), nil
		case "agent get p1":
			return nil, errors.New("no agent")
		case "agent start orc-w9-develop --kind codex --pane p1 -- --model gpt-5":
			return response(`{}`), nil
		case "agent prompt p1 build this":
			return response(`{}`), nil
		default:
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	target, err := b.SendTarget(mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"}, "develop", "/work/orc-9", "/work/orc-9", []string{"codex", "--model", "gpt-5", "build this"})
	if err != nil {
		t.Fatal(err)
	}
	if target.Pane != "p1" || target.Tab != "t1" {
		t.Fatalf("target = %#v", target)
	}
	if len(calls) != 8 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestSendTargetRetriesWhileCreatedPaneShellStarts(t *testing.T) {
	starts := 0
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		switch call {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9","label":"ORC-9"}}`), nil
		case "tab get t1":
			return response(`{"tab":{"tab_id":"t1","workspace_id":"w9","label":"develop"}}`), nil
		case "pane get p1":
			return response(`{"pane":{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"}}`), nil
		case "agent get p1":
			return nil, errors.New("no agent")
		case "agent start orc-w9-develop --kind codex --pane p1":
			starts++
			if starts == 1 {
				return nil, errors.New(`{"error":{"code":"agent_pane_busy"}}`)
			}
			return response(`{}`), nil
		case "agent prompt p1 build this":
			return response(`{}`), nil
		default:
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	_, err := b.SendTarget(mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"}, "develop", "/work/orc-9", "/work/orc-9", []string{"codex", "build this"})
	if err != nil {
		t.Fatal(err)
	}
	if starts != 2 {
		t.Fatalf("agent starts = %d, want 2", starts)
	}
}

func TestConfigureTaskCellCreatesOwnedTestAndWatchPanes(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9","label":"ORC-9"}}`), nil
		case "tab get t1":
			return response(`{"tab":{"tab_id":"t1","workspace_id":"w9","label":"develop"}}`), nil
		case "pane get p1":
			return response(`{"pane":{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"}}`), nil
		case "pane list --workspace w9":
			return response(`{"panes":[{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"}]}`), nil
		case "pane split p1 --direction right --ratio 0.35 --cwd /work/orc-9 --env ORC=1 --no-focus":
			return response(`{"pane":{"pane_id":"p2","workspace_id":"w9","tab_id":"t1"}}`), nil
		case "pane split p2 --direction down --ratio 0.5 --cwd /work/orc-9 --env ORC=1 --no-focus":
			return response(`{"pane":{"pane_id":"p3","workspace_id":"w9","tab_id":"t1"}}`), nil
		default:
			if strings.HasPrefix(call, "pane report-metadata ") || strings.HasPrefix(call, "pane rename ") || strings.HasPrefix(call, "pane run ") {
				return response(`{}`), nil
			}
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	err := b.ConfigureTaskCell(
		mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"},
		mux.TaskCellSpec{
			CWD: "/work/orc-9", TestCommand: "make test",
			WatchCommand: "orc --workspace /work watch ORC-9",
			Metadata:     mux.Metadata{Ticket: "ORC-9", Stage: "develop", FeatureDir: "/work/features/orc-9"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{
		"pane split p1 --direction right --ratio 0.35",
		"pane report-metadata p2 --source orc --display-agent tests --token task_cell=tests --token orc_task_cell_owner=/work/features/orc-9",
		"pane rename p2 tests", "pane run p2 make test",
		"pane split p2 --direction down --ratio 0.5",
		"pane report-metadata p3 --source orc --display-agent watch --token task_cell=watch --token orc_task_cell_owner=/work/features/orc-9",
		"pane rename p3 watch", "pane run p3 orc --workspace /work watch ORC-9",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("calls missing %q:\n%s", want, joined)
		}
	}
}

func TestConfigureTaskCellReusesOwnedPanes(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "workspace get w9":
			return response(`{"workspace":{"workspace_id":"w9"}}`), nil
		case "tab get t1":
			return response(`{"tab":{"tab_id":"t1","workspace_id":"w9"}}`), nil
		case "pane get p1":
			return response(`{"pane":{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"}}`), nil
		case "pane list --workspace w9":
			return response(`{"panes":[{"pane_id":"p1","workspace_id":"w9","tab_id":"t1"},{"pane_id":"p2","workspace_id":"w9","tab_id":"t1","tokens":{"task_cell":"tests","orc_task_cell_owner":"/work/features/orc-9"}},{"pane_id":"p3","workspace_id":"w9","tab_id":"t1","tokens":{"task_cell":"watch","orc_task_cell_owner":"/work/features/orc-9"}}]}`), nil
		default:
			return nil, errors.New("unexpected command: " + call)
		}
	}}

	err := b.ConfigureTaskCell(
		mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"},
		mux.TaskCellSpec{
			CWD: "/work", TestCommand: "make test", WatchCommand: "orc watch ORC-9",
			Metadata: mux.Metadata{FeatureDir: "/work/features/orc-9"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("existing task cell should not be recreated; calls = %#v", calls)
	}
}

func TestSetTargetMetadataPublishesSidebarTokens(t *testing.T) {
	var calls []string
	b := Backend{run: func(args ...string) ([]byte, error) {
		call := strings.Join(args, " ")
		calls = append(calls, call)
		if call == "workspace get w9" {
			return response(`{"workspace":{"workspace_id":"w9","label":"ORC-9"}}`), nil
		}
		return response(`{}`), nil
	}}
	meta := mux.Metadata{AgentID: "agent-9", AgentInstance: "instance-9", Ticket: "ORC-9", Stage: "develop", Worker: "builder", Engine: "codex", FeatureDir: "/work/orc-9", FeatureSlug: "ORC-9-native"}
	if err := b.SetTargetMetadata(mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"}, meta); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{"workspace report-metadata w9 --source orc --token owned=1", "pane report-metadata p1 --source orc --display-agent builder", "--token agent_id=agent-9", "--token agent_instance=instance-9", "--token ticket=ORC-9", "--token stage=develop", "--token feature_dir=/work/orc-9", "--token feature_slug=ORC-9-native"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("calls missing %q:\n%s", want, joined)
		}
	}
}

func response(result string) []byte {
	return []byte(`{"result":` + result + `}`)
}
