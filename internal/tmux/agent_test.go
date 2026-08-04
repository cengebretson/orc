package tmux

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

func TestPrepareAgentLaunchStampsBeforeReturningWrappedCommand(t *testing.T) {
	calls := stubCommands(t,
		commandResult{},
		commandResult{output: "orc\tdevelop\n"},
	)
	target, argv, err := (Backend{}).PrepareAgentLaunch(
		mux.Target{Backend: "tmux", Workspace: "orc", Tab: "develop", Pane: "%7"},
		"develop",
		"/work",
		mux.Metadata{AgentID: "agent-1", AgentInstance: "instance-1", FeatureDir: "/work"},
		[]string{"codex", "build this"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if target.Pane != "%7" {
		t.Fatalf("target = %+v", target)
	}
	wantArgv := []string{
		"env", "ORC_AGENT_ID=agent-1", "ORC_AGENT_INSTANCE=instance-1", "ORC_FEATURE_DIR=/work",
		"codex", "build this",
	}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Fatalf("argv = %#v, want %#v", argv, wantArgv)
	}
	var stampedID, stampedInstance bool
	for _, call := range *calls {
		joined := strings.Join(call.args, " ")
		stampedID = stampedID || joined == "set-option -p -t %7 @orc_agent_id agent-1"
		stampedInstance = stampedInstance || joined == "set-option -p -t %7 @orc_agent_instance instance-1"
	}
	if !stampedID || !stampedInstance {
		t.Fatalf("identity stamp calls missing: %#v", *calls)
	}
}

func TestApplyAgentEventCommitsSequenceLast(t *testing.T) {
	calls := stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tidle\t4\told-event\t4\n"})
	result, err := applyAgentEventAt("%7", mux.AgentEvent{
		AgentID: "agent-1", AgentInstance: "instance-1", Engine: "codex",
		ProviderSessionID: "provider-1", Lifecycle: mux.LifecycleBlocked, EventID: "event-5",
	}, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if result.Target.Pane != "%7" || result.Lifecycle != mux.LifecycleBlocked || result.StateChangeSeq != 5 || result.Duplicate {
		t.Fatalf("result = %+v", result)
	}
	last := (*calls)[len(*calls)-1]
	if got := strings.Join(last.args, " "); got != "set-option -p -t %7 @orc_agent_state_seq 5" {
		t.Fatalf("last command = %q", got)
	}
	if got := strings.Join((*calls)[len(*calls)-2].args, " "); got != "set-option -p -t %7 @orc_agent_event_id event-5" {
		t.Fatalf("event commit command = %q", got)
	}
	if got := strings.Join((*calls)[len(*calls)-3].args, " "); got != "set-option -p -t %7 @orc_agent_event_seq 5" {
		t.Fatalf("event sequence command = %q", got)
	}
	wantOptions := map[string]string{
		"@orc_agent_state":        "blocked",
		"@orc_agent_state_since":  "1700000000",
		"@orc_agent_state_source": "hook",
		"@orc_provider_engine":    "codex",
		"@orc_provider_session":   "provider-1",
		"@agent_attention":        "blocked",
		"@agent_attention_since":  "1700000000",
		"@agent_attention_source": "hook",
	}
	for _, call := range (*calls)[1:] {
		if len(call.args) == 6 && call.args[0] == "set-option" {
			delete(wantOptions, call.args[4])
		}
	}
	if len(wantOptions) != 0 {
		t.Fatalf("missing option updates: %#v; calls: %#v", wantOptions, *calls)
	}
}

func TestApplyAgentEventIsIdempotent(t *testing.T) {
	calls := stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tdone\t8\tevent-8\t8\n"})
	result, err := applyAgentEventAt("%7", mux.AgentEvent{
		AgentID: "agent-1", AgentInstance: "instance-1", Engine: "codex",
		Lifecycle: mux.LifecycleDone, EventID: "event-8",
	}, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Duplicate || result.StateChangeSeq != 8 || len(*calls) != 1 {
		t.Fatalf("result/calls = %+v / %#v", result, *calls)
	}
}

func TestApplyAgentEventCompletesInterruptedSequenceCommit(t *testing.T) {
	calls := stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tdone\t7\tevent-8\t8\n"})
	result, err := applyAgentEventAt("%7", mux.AgentEvent{
		AgentID: "agent-1", AgentInstance: "instance-1", Engine: "codex",
		Lifecycle: mux.LifecycleDone, EventID: "event-8",
	}, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Duplicate || result.StateChangeSeq != 8 || len(*calls) != 2 {
		t.Fatalf("result/calls = %+v / %#v", result, *calls)
	}
	if got := strings.Join((*calls)[1].args, " "); got != "set-option -p -t %7 @orc_agent_state_seq 8" {
		t.Fatalf("recovery command = %q", got)
	}
}

func TestApplyAgentEventRejectsReplacementAndInvalidState(t *testing.T) {
	t.Run("replacement", func(t *testing.T) {
		calls := stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-2\tidle\t1\told\t1\n"})
		_, err := ApplyAgentEvent("%7", mux.AgentEvent{
			AgentID: "agent-1", AgentInstance: "instance-1", Engine: "codex",
			Lifecycle: mux.LifecycleWorking, EventID: "event-2",
		})
		if err == nil || !strings.Contains(err.Error(), "different agent instance") {
			t.Fatalf("error = %v", err)
		}
		if len(*calls) != 1 {
			t.Fatalf("replacement wrote metadata: %#v", *calls)
		}
	})

	t.Run("invalid lifecycle", func(t *testing.T) {
		calls := stubCommands(t)
		_, err := ApplyAgentEvent("%7", mux.AgentEvent{
			AgentID: "agent-1", AgentInstance: "instance-1", Engine: "codex",
			Lifecycle: "finished-ish", EventID: "event-2",
		})
		if err == nil || !strings.Contains(err.Error(), "unsupported agent lifecycle") {
			t.Fatalf("error = %v", err)
		}
		if len(*calls) != 0 {
			t.Fatalf("invalid event reached tmux: %#v", *calls)
		}
	})
}
