package tmux

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

func TestStateAgentReadsExactInstanceLifecycle(t *testing.T) {
	calls := stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tworking\t14\t14\t0\n"})
	target := controlTestTarget()
	got, err := (Backend{}).StateAgent(target)
	if err != nil {
		t.Fatal(err)
	}
	if got.Backend != "tmux" || got.Target != target || got.Agent != "agent-1" || got.Lifecycle != mux.LifecycleWorking || got.StateChangeSeq != 14 {
		t.Fatalf("StateAgent() = %#v", got)
	}
	want := []string{"display-message", "-p", "-t", "%7", "#{session_name}\t#{window_name}\t#{@orc_agent_id}\t#{@orc_agent_instance}\t#{@orc_agent_state}\t#{@orc_agent_state_seq}\t#{@orc_agent_event_seq}\t#{pane_dead}"}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0].args, want) {
		t.Fatalf("calls = %#v", *calls)
	}
}

func TestStateAgentReportsUnknownBeforeFirstHook(t *testing.T) {
	stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\t\t0\t0\t0\n"})
	got, err := (Backend{}).StateAgent(controlTestTarget())
	if err != nil {
		t.Fatal(err)
	}
	if got.Lifecycle != mux.LifecycleUnknown || got.StateChangeSeq != 0 {
		t.Fatalf("StateAgent() = %#v", got)
	}
}

func TestStateAgentHidesUncommittedLifecycle(t *testing.T) {
	stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tdone\t3\t4\t0\n"})
	got, err := (Backend{}).StateAgent(controlTestTarget())
	if err != nil {
		t.Fatal(err)
	}
	if got.Lifecycle != mux.LifecycleUnknown || got.StateChangeSeq != 3 {
		t.Fatalf("StateAgent() = %#v", got)
	}
}

func TestStateAgentReturnsStableSafetyErrors(t *testing.T) {
	tests := []struct {
		name   string
		target mux.Target
		result commandResult
		code   string
	}{
		{name: "missing identity", target: mux.Target{Backend: "tmux", Workspace: "orc", Tab: "develop", Pane: "%7"}, code: "agent_identity_missing"},
		{name: "pane unavailable", target: controlTestTarget(), result: commandResult{exit: 1}, code: "agent_offline"},
		{name: "different instance", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-2\tidle\t2\t2\t0\n"}, code: "agent_replaced"},
		{name: "dead pane", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tidle\t2\t2\t1\n"}, code: "agent_offline"},
		{name: "bad lifecycle", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tguessing\t2\t2\t0\n"}, code: "invalid_state"},
		{name: "bad sequence", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tidle\tnope\t2\t0\n"}, code: "invalid_state"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubCommands(t, tt.result)
			_, err := (Backend{}).StateAgent(tt.target)
			assertAgentControlCode(t, err, tt.code)
		})
	}
}

func TestWaitAgentUsesSettledDefaultsAndObservedTransitions(t *testing.T) {
	t.Run("settled immediately", func(t *testing.T) {
		stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tblocked\t3\t3\t0\n"})
		got, err := (Backend{}).WaitAgent(controlTestTarget(), mux.AgentControlOptions{Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if got.Lifecycle != mux.LifecycleBlocked {
			t.Fatalf("WaitAgent() = %#v", got)
		}
	})

	t.Run("transition", func(t *testing.T) {
		oldInterval := agentWaitPollInterval
		agentWaitPollInterval = time.Millisecond
		t.Cleanup(func() { agentWaitPollInterval = oldInterval })
		stubCommands(t,
			commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tworking\t3\t3\t0\n"},
			commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tidle\t4\t4\t0\n"},
		)
		got, err := (Backend{}).WaitAgent(controlTestTarget(), mux.AgentControlOptions{
			Until: []string{mux.LifecycleIdle}, Timeout: time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got.Lifecycle != mux.LifecycleIdle || got.StateChangeSeq != 4 {
			t.Fatalf("WaitAgent() = %#v", got)
		}
	})
}

func TestWaitAgentTimeoutCancellationAndReplacement(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		oldInterval := agentWaitPollInterval
		agentWaitPollInterval = time.Second
		t.Cleanup(func() { agentWaitPollInterval = oldInterval })
		stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tworking\t3\t3\t0\n"})
		_, err := (Backend{}).WaitAgent(controlTestTarget(), mux.AgentControlOptions{
			Until: []string{mux.LifecycleDone}, Timeout: time.Millisecond,
		})
		assertAgentControlCode(t, err, "timeout")
	})

	t.Run("cancelled", func(t *testing.T) {
		calls := stubCommands(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := (Backend{}).WaitAgent(controlTestTarget(), mux.AgentControlOptions{Context: ctx})
		assertAgentControlCode(t, err, "cancelled")
		if len(*calls) != 0 {
			t.Fatalf("calls = %#v, want none", *calls)
		}
	})

	t.Run("replaced", func(t *testing.T) {
		stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-2\tidle\t2\t2\t0\n"})
		_, err := (Backend{}).WaitAgent(controlTestTarget(), mux.AgentControlOptions{Timeout: time.Second})
		assertAgentControlCode(t, err, "agent_replaced")
	})
}

func controlTestTarget() mux.Target {
	return mux.Target{
		Backend: "tmux", Workspace: "orc", Tab: "develop", Pane: "%7",
		AgentID: "agent-1", AgentInstance: "instance-1",
	}
}

func assertAgentControlCode(t *testing.T, err error, code string) {
	t.Helper()
	var controlErr *mux.AgentControlError
	if !errors.As(err, &controlErr) || controlErr.Code != code {
		t.Fatalf("error = %#v, want agent control code %q", err, code)
	}
}
