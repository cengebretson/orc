package tmux

import (
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/mux"
)

func resetObservationDebounce(t *testing.T) {
	t.Helper()
	observationDebounce.Lock()
	old := observationDebounce.idle
	observationDebounce.idle = make(map[string]int)
	observationDebounce.Unlock()
	t.Cleanup(func() {
		observationDebounce.Lock()
		observationDebounce.idle = old
		observationDebounce.Unlock()
	})
}

// tmux-attention's active-turn signal is a report from the agent's own hook,
// not a guess about a picture, so it outranks both inference tiers and skips the
// screen capture. It stays an observation: Orc cannot verify who wrote it, so it
// must not read as a registered source.
func TestObserveFallbackPrefersContextActiveOverInference(t *testing.T) {
	resetObservationDebounce(t)
	calls := stubCommands(t)
	panes, err := ObserveFallback(t.TempDir(), []mux.Pane{{
		Backend: "tmux", ID: "%7", Agent: true, AgentInstance: "instance-1",
		ProviderEngine: "codex", Lifecycle: "unknown", LifecycleSource: "launch",
		// A title that would otherwise infer blocked; the running turn wins.
		Title: "⠋ Action Required", ContextActive: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 {
		t.Fatalf("panes = %+v", panes)
	}
	if panes[0].ObservedLifecycle != mux.LifecycleWorking || panes[0].ObservationSource != mux.SourceContext {
		t.Fatalf("observed = %q/%q, want working/context", panes[0].ObservedLifecycle, panes[0].ObservationSource)
	}
	if mux.IsRegisteredSource(panes[0].ObservationSource) {
		t.Error("context source must not count as registration")
	}
	for _, call := range *calls {
		if len(call.args) > 0 && call.args[0] == "capture-pane" {
			t.Fatalf("a reported turn should not capture the screen: %#v", call.args)
		}
	}
}

func TestObserveFallbackPrefersTitleAndPublishesPresentationMetadata(t *testing.T) {
	resetObservationDebounce(t)
	calls := stubCommands(t)
	panes, err := ObserveFallback(t.TempDir(), []mux.Pane{{
		Backend: "tmux", ID: "%7", Agent: true, AgentInstance: "instance-1",
		ProviderEngine: "codex", Lifecycle: "unknown", LifecycleSource: "launch",
		Title: "⠋ Action Required",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 || panes[0].ObservedLifecycle != "blocked" || panes[0].ObservationSource != "title" || panes[0].ObservationRule != "title-action-required" || panes[0].DisplayTitle != "Action Required" || panes[0].AttentionSource != "title" {
		t.Fatalf("observed pane = %+v", panes)
	}
	joined := make([]string, 0, len(*calls))
	for _, call := range *calls {
		joined = append(joined, strings.Join(call.args, " "))
	}
	if !containsString(joined, "set-option -p -t %7 @agent_attention_source title") {
		t.Fatalf("published calls = %#v", joined)
	}
	for _, call := range joined {
		if strings.HasPrefix(call, "capture-pane ") {
			t.Fatalf("title match should not capture screen: %#v", joined)
		}
	}
}

func TestObserveFallbackUsesBoundedScreenAndDebouncesWorkingToIdle(t *testing.T) {
	resetObservationDebounce(t)
	calls := stubCommands(t, commandResult{output: "Would you like to run this command?\n"})
	panes, err := ObserveFallback(t.TempDir(), []mux.Pane{{
		Backend: "tmux", ID: "%8", Agent: true, AgentInstance: "instance-2",
		ProviderEngine: "codex", Lifecycle: "unknown", LifecycleSource: "launch",
		Title: "workspace shell",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if panes[0].ObservedLifecycle != "blocked" || panes[0].ObservationSource != "screen" || panes[0].Attention != "blocked" {
		t.Fatalf("screen observation = %+v", panes[0])
	}
	if got := strings.Join((*calls)[0].args, " "); got != "capture-pane -p -t %8 -S -24" {
		t.Fatalf("capture command = %q", got)
	}

	working := mux.Pane{
		Backend: "tmux", ID: "%9", Agent: true, AgentInstance: "instance-3", ProviderEngine: "codex",
		LifecycleSource: "launch", Title: "Codex", ObservedLifecycle: "working", ObservationSource: "title", ObservationRule: "title-working-spinner", ObservationSince: 10,
	}
	first, err := ObserveFallback(t.TempDir(), []mux.Pane{working})
	if err != nil || first[0].ObservedLifecycle != "working" {
		t.Fatalf("first idle candidate = %+v, %v", first, err)
	}
	second, err := ObserveFallback(t.TempDir(), first)
	if err != nil || second[0].ObservedLifecycle != "idle" {
		t.Fatalf("second idle candidate = %+v, %v", second, err)
	}
}

func TestObserveFallbackNeverTouchesHookLifecycle(t *testing.T) {
	calls := stubCommands(t)
	pane := mux.Pane{Backend: "tmux", ID: "%7", Agent: true, ProviderEngine: "codex", Lifecycle: "working", LifecycleSource: "hook", Title: "Action Required"}
	got, err := ObserveFallback(t.TempDir(), []mux.Pane{pane})
	if err != nil || got[0] != pane || len(*calls) != 0 {
		t.Fatalf("hook observation = %+v, %v, calls %#v", got, err, *calls)
	}
}
