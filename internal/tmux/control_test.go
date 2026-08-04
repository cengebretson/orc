package tmux

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

func TestStateAgentReadsExactInstanceLifecycle(t *testing.T) {
	calls := stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tworking\t14\t14\t1\t0\n"})
	target := controlTestTarget()
	got, err := (Backend{}).StateAgent(target)
	if err != nil {
		t.Fatal(err)
	}
	if got.Backend != "tmux" || got.Target != target || got.Agent != "agent-1" || got.Lifecycle != mux.LifecycleWorking || got.StateChangeSeq != 14 {
		t.Fatalf("StateAgent() = %#v", got)
	}
	want := []string{"display-message", "-p", "-t", "%7", "#{session_name}\t#{window_name}\t#{@orc_agent_id}\t#{@orc_agent_instance}\t#{@orc_agent_state}\t#{@orc_agent_state_seq}\t#{@orc_agent_event_seq}\t#{bracket_paste_flag}\t#{pane_dead}"}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0].args, want) {
		t.Fatalf("calls = %#v", *calls)
	}
}

func TestStateAgentReportsUnknownBeforeFirstHook(t *testing.T) {
	stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\t\t0\t0\t1\t0\n"})
	got, err := (Backend{}).StateAgent(controlTestTarget())
	if err != nil {
		t.Fatal(err)
	}
	if got.Lifecycle != mux.LifecycleUnknown || got.StateChangeSeq != 0 {
		t.Fatalf("StateAgent() = %#v", got)
	}
}

func TestStateAgentHidesUncommittedLifecycle(t *testing.T) {
	stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tdone\t3\t4\t1\t0\n"})
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
		{name: "different instance", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-2\tidle\t2\t2\t1\t0\n"}, code: "agent_replaced"},
		{name: "dead pane", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tidle\t2\t2\t1\t1\n"}, code: "agent_offline"},
		{name: "bad lifecycle", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tguessing\t2\t2\t1\t0\n"}, code: "invalid_state"},
		{name: "bad sequence", target: controlTestTarget(), result: commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tidle\tnope\t2\t1\t0\n"}, code: "invalid_state"},
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
		stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tblocked\t3\t3\t1\t0\n"})
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
			commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tworking\t3\t3\t1\t0\n"},
			commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tidle\t4\t4\t1\t0\n"},
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
		stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-1\tworking\t3\t3\t1\t0\n"})
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
		stubCommands(t, commandResult{output: "orc\tdevelop\tagent-1\tinstance-2\tidle\t2\t2\t1\t0\n"})
		_, err := (Backend{}).WaitAgent(controlTestTarget(), mux.AgentControlOptions{Timeout: time.Second})
		assertAgentControlCode(t, err, "agent_replaced")
	})
}

func TestPromptAgentDeliversLiteralTextThroughPrivateBuffer(t *testing.T) {
	prompt := "review 'quotes' $(touch nope); then\ncheck\tunicode: 🐈"
	loadedName, loadedText := stubAgentPromptBuffer(t)
	calls := stubCommands(t,
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{},
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{},
	)
	got, err := (Backend{}).PromptAgent(controlTestTarget(), prompt, false, mux.AgentControlOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Lifecycle != mux.LifecycleIdle || got.StateChangeSeq != 1 {
		t.Fatalf("PromptAgent() = %#v", got)
	}
	if *loadedName != "orc-prompt-test" || *loadedText != prompt {
		t.Fatalf("loaded buffer = %q / %q", *loadedName, *loadedText)
	}
	if len(*calls) != 5 {
		t.Fatalf("calls = %#v", *calls)
	}
	if got := (*calls)[2].args; !reflect.DeepEqual(got, []string{"paste-buffer", "-dprS", "-b", "orc-prompt-test", "-t", "%7"}) {
		t.Fatalf("paste args = %#v", got)
	}
	if got := (*calls)[4].args; !reflect.DeepEqual(got, []string{"send-keys", "-t", "%7", "Enter"}) {
		t.Fatalf("submit args = %#v", got)
	}
	for _, call := range *calls {
		if strings.Contains(strings.Join(call.args, " "), prompt) {
			t.Fatalf("prompt leaked into command argv: %#v", call)
		}
	}
}

func TestPromptAgentRequiresBracketedPasteAndCleansBuffer(t *testing.T) {
	stubAgentPromptBuffer(t)
	calls := stubCommands(t,
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{output: agentStateOutput("idle", 1, "instance-1", false)},
		commandResult{},
	)
	_, err := (Backend{}).PromptAgent(controlTestTarget(), "safe text", false, mux.AgentControlOptions{})
	assertAgentControlCode(t, err, "bracketed_paste_unavailable")
	if len(*calls) != 3 || !reflect.DeepEqual((*calls)[2].args, []string{"delete-buffer", "-b", "orc-prompt-test"}) {
		t.Fatalf("calls = %#v", *calls)
	}
}

func TestPromptAgentCleansBufferWhenLoadFails(t *testing.T) {
	stubAgentPromptBuffer(t)
	loadAgentPromptBuffer = func(string, string) error { return errors.New("load failed") }
	calls := stubCommands(t,
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{},
	)
	_, err := (Backend{}).PromptAgent(controlTestTarget(), "safe text", false, mux.AgentControlOptions{})
	assertAgentControlCode(t, err, "backend_unavailable")
	if len(*calls) != 2 || !reflect.DeepEqual((*calls)[1].args, []string{"delete-buffer", "-b", "orc-prompt-test"}) {
		t.Fatalf("calls = %#v", *calls)
	}
}

func TestPromptAgentDetectsReplacementBeforeDelivery(t *testing.T) {
	stubAgentPromptBuffer(t)
	calls := stubCommands(t,
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{output: agentStateOutput("idle", 1, "instance-2", true)},
		commandResult{},
	)
	_, err := (Backend{}).PromptAgent(controlTestTarget(), "safe text", false, mux.AgentControlOptions{})
	assertAgentControlCode(t, err, "agent_replaced")
	for _, call := range *calls {
		if len(call.args) > 0 && (call.args[0] == "paste-buffer" || call.args[0] == "send-keys") {
			t.Fatalf("replacement received input command: %#v", call)
		}
	}
}

func TestPromptAgentDoesNotSubmitAfterReplacementDuringDelivery(t *testing.T) {
	stubAgentPromptBuffer(t)
	calls := stubCommands(t,
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{},
		commandResult{output: agentStateOutput("idle", 1, "instance-2", true)},
	)
	_, err := (Backend{}).PromptAgent(controlTestTarget(), "safe text", false, mux.AgentControlOptions{})
	assertAgentControlCode(t, err, "agent_replaced")
	for _, call := range *calls {
		if len(call.args) > 0 && call.args[0] == "send-keys" {
			t.Fatalf("replacement received submit key: %#v", call)
		}
	}
}

func TestPromptAgentWaitRequiresMovementThenSettlement(t *testing.T) {
	stubAgentPromptBuffer(t)
	calls := stubCommands(t,
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{},
		commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		commandResult{},
		commandResult{output: agentStateOutput("working", 2, "instance-1", true)},
		commandResult{output: agentStateOutput("done", 3, "instance-1", true)},
	)
	got, err := (Backend{}).PromptAgent(controlTestTarget(), "finish it", true, mux.AgentControlOptions{
		Until: []string{mux.LifecycleDone}, Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Lifecycle != mux.LifecycleDone || got.StateChangeSeq != 3 {
		t.Fatalf("PromptAgent() = %#v", got)
	}
	if len(*calls) != 7 {
		t.Fatalf("calls = %#v", *calls)
	}
}

func TestPromptAgentReportsStallAndAgentExit(t *testing.T) {
	t.Run("stalled", func(t *testing.T) {
		stubAgentPromptBuffer(t)
		oldInterval, oldGrace := agentWaitPollInterval, agentPromptStartupGrace
		agentWaitPollInterval, agentPromptStartupGrace = 20*time.Millisecond, time.Millisecond
		t.Cleanup(func() { agentWaitPollInterval, agentPromptStartupGrace = oldInterval, oldGrace })
		stubCommands(t,
			commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
			commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
			commandResult{},
			commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
			commandResult{},
			commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
		)
		_, err := (Backend{}).PromptAgent(controlTestTarget(), "wake up", true, mux.AgentControlOptions{Timeout: time.Second})
		assertAgentControlCode(t, err, "agent_prompt_stalled")
	})

	t.Run("agent exit", func(t *testing.T) {
		stubAgentPromptBuffer(t)
		stubCommands(t,
			commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
			commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
			commandResult{},
			commandResult{output: agentStateOutput("idle", 1, "instance-1", true)},
			commandResult{},
			commandResult{exit: 1},
		)
		_, err := (Backend{}).PromptAgent(controlTestTarget(), "wake up", true, mux.AgentControlOptions{Timeout: time.Second})
		assertAgentControlCode(t, err, "agent_offline")
	})
}

func TestPromptAgentRejectsUnsafeInputBeforeTargeting(t *testing.T) {
	tests := []string{"", " \n\t", "nul\x00byte", "escape\x1bcode", string([]byte{0xff}), strings.Repeat("x", mux.MaxAgentPromptBytes+1)}
	for _, prompt := range tests {
		calls := stubCommands(t)
		_, err := (Backend{}).PromptAgent(controlTestTarget(), prompt, false, mux.AgentControlOptions{})
		assertAgentControlCode(t, err, "invalid_argument")
		if len(*calls) != 0 {
			t.Fatalf("prompt %q made calls: %#v", prompt, *calls)
		}
	}
}

func TestPromptAgentHonorsCancelledContextBeforeTargeting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := stubCommands(t)
	_, err := (Backend{}).PromptAgent(controlTestTarget(), "safe text", true, mux.AgentControlOptions{Context: ctx})
	assertAgentControlCode(t, err, "cancelled")
	if len(*calls) != 0 {
		t.Fatalf("calls = %#v, want none", *calls)
	}
}

func stubAgentPromptBuffer(t *testing.T) (*string, *string) {
	t.Helper()
	oldName, oldLoad := newAgentPromptBufferName, loadAgentPromptBuffer
	name, text := "", ""
	newAgentPromptBufferName = func() (string, error) { return "orc-prompt-test", nil }
	loadAgentPromptBuffer = func(gotName, gotText string) error {
		name, text = gotName, gotText
		return nil
	}
	t.Cleanup(func() {
		newAgentPromptBufferName, loadAgentPromptBuffer = oldName, oldLoad
	})
	return &name, &text
}

func agentStateOutput(lifecycle string, sequence uint64, instance string, bracketed bool) string {
	bracket := "0"
	if bracketed {
		bracket = "1"
	}
	return "orc\tdevelop\tagent-1\t" + instance + "\t" + lifecycle + "\t" +
		strconv.FormatUint(sequence, 10) + "\t" + strconv.FormatUint(sequence, 10) + "\t" + bracket + "\t0\n"
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
