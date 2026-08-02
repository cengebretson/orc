package herdr

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/mux"
)

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
			return response(`{"panes":[{"pane_id":"p1","workspace_id":"w9","tab_id":"t1","agent":"codex","agent_status":"blocked","foreground_cwd":"/work/orc-9","agent_session":{"value":"session-9"},"tokens":{"ticket":"ORC-9","stage":"develop","worker":"builder","feature_dir":"/work/orc-9"}}]}`), nil
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
	if pane.Backend != "herdr" || pane.Session != "w9" || pane.Window != "t1" || pane.ID != "p1" || pane.Lifecycle != "blocked" || pane.Attention != mux.AttentionBlocked {
		t.Fatalf("pane target/lifecycle = %#v", pane)
	}
	if pane.Ticket != "ORC-9" || pane.Stage != "develop" || pane.Worker != "builder" || pane.ProviderSessionID != "session-9" {
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
	meta := mux.Metadata{Ticket: "ORC-9", Stage: "develop", Worker: "builder", Engine: "codex", FeatureDir: "/work/orc-9"}
	if err := b.SetTargetMetadata(mux.Target{Backend: "herdr", Workspace: "w9", Tab: "t1", Pane: "p1"}, meta); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(calls, "\n")
	for _, want := range []string{"workspace report-metadata w9 --source orc --token owned=1", "pane report-metadata p1 --source orc --display-agent builder", "--token ticket=ORC-9", "--token stage=develop", "--token feature_dir=/work/orc-9"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("calls missing %q:\n%s", want, joined)
		}
	}
}

func response(result string) []byte {
	return []byte(`{"result":` + result + `}`)
}
