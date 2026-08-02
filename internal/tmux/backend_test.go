package tmux

import (
	"errors"
	"testing"

	"github.com/cengebretson/orc/internal/mux"
)

// The tmux backend is the reference implementation of mux.Backend. These tests
// cover the contract a second backend would also have to satisfy, rather than
// re-testing the tmux functions themselves — those are covered directly in
// process_test.go and tmux_live_test.go.

func TestBackendDelegatesToTmuxCommands(t *testing.T) {
	backend := New()

	tests := []struct {
		name     string
		call     func()
		wantArgs []string
	}{
		{
			name:     "SessionExists",
			call:     func() { backend.SessionExists("orc") },
			wantArgs: []string{"has-session", "-t", "orc"},
		},
		{
			name:     "KillSession",
			call:     func() { _ = backend.KillSession("orc") },
			wantArgs: []string{"kill-session", "-t", "orc"},
		},
		{
			name:     "SetPaneMetadata",
			call:     func() { _ = backend.SetPaneMetadata("%1", mux.Metadata{Ticket: "ENG-42"}) },
			wantArgs: []string{"set-option", "-p", "-t", "%1", "@orc_agent", "1"},
		},
		{
			name:     "Attention",
			call:     func() { backend.Attention("orc", "develop") },
			wantArgs: []string{"show-options", "-w", "-qv", "-t", "orc:develop", "@agent_attention"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := stubCommands(t, commandResult{})
			test.call()
			if len(*calls) == 0 {
				t.Fatalf("%s issued no tmux command", test.name)
			}
			first := (*calls)[0]
			if first.name != "tmux" {
				t.Fatalf("command = %q, want tmux", first.name)
			}
			for i, want := range test.wantArgs {
				if i >= len(first.args) || first.args[i] != want {
					t.Fatalf("args = %v, want prefix %v", first.args, test.wantArgs)
				}
			}
		})
	}
}

// A backend must treat a missing multiplexer as an empty inventory rather than
// an error. Orc's read paths run on every dashboard refresh, so a backend that
// errors when no server is running turns "nothing to show" into a failure.
func TestBackendMissingMultiplexerIsEmptyNotError(t *testing.T) {
	original := findExecutable
	t.Cleanup(func() { findExecutable = original })
	findExecutable = func(string) (string, error) { return "", errors.New("missing") }

	backend := New()

	if backend.Available() {
		t.Fatal("Available() = true with no tmux on PATH")
	}

	panes, err := backend.ListPanes()
	if err != nil {
		t.Fatalf("ListPanes() error = %v, want nil when tmux is absent", err)
	}
	if len(panes) != 0 {
		t.Fatalf("ListPanes() = %d panes, want none", len(panes))
	}
}

func TestBackendListSessionsIsEmptyWhenNoServerRunning(t *testing.T) {
	stubCommands(t, commandResult{exit: 1})

	if got := New().ListSessions(); len(got) != 0 {
		t.Fatalf("ListSessions() = %v, want none when no server is running", got)
	}
}

func TestBackendNameIdentifiesTheMultiplexer(t *testing.T) {
	if got := New().Name(); got != "tmux" {
		t.Fatalf("Name() = %q, want tmux", got)
	}
}
