package tmux

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/mux"
)

type commandCall struct {
	name string
	args []string
}

type commandResult struct {
	output string
	exit   int
}

func stubCommands(t *testing.T, results ...commandResult) *[]commandCall {
	t.Helper()
	original := newCommand
	realCommand := exec.Command
	calls := make([]commandCall, 0, len(results))
	next := 0
	newCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, commandCall{name: name, args: append([]string(nil), args...)})
		result := commandResult{}
		if next < len(results) {
			result = results[next]
		}
		next++
		cmd := realCommand("sh", "-c", `printf '%s' "$ORC_TMUX_TEST_OUTPUT"; exit "$ORC_TMUX_TEST_EXIT"`)
		cmd.Env = append(os.Environ(),
			"ORC_TMUX_TEST_OUTPUT="+result.output,
			"ORC_TMUX_TEST_EXIT="+strconv.Itoa(result.exit),
		)
		return cmd
	}
	t.Cleanup(func() { newCommand = original })
	return &calls
}

func TestAvailableUsesExecutableBoundary(t *testing.T) {
	original := findExecutable
	t.Cleanup(func() { findExecutable = original })

	findExecutable = func(name string) (string, error) {
		if name != "tmux" {
			t.Fatalf("lookup = %q, want tmux", name)
		}
		return "/usr/bin/tmux", nil
	}
	if !Available() {
		t.Fatal("Available() = false, want true")
	}

	findExecutable = func(string) (string, error) {
		return "", errors.New("missing")
	}
	if Available() {
		t.Fatal("Available() = true, want false")
	}
}

func TestCaptureTargetUsesExactPane(t *testing.T) {
	calls := stubCommands(t,
		commandResult{output: "orc\tbuild\n"},
		commandResult{output: "zero\none\ntwo\n"},
	)

	got, err := CaptureTarget(mux.Target{
		Backend: "tmux", Workspace: "orc", Tab: "build", Pane: "%7",
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != "one\ntwo\n" {
		t.Fatalf("capture = %q", got)
	}
	if len(*calls) != 2 || !reflect.DeepEqual((*calls)[1].args, []string{"capture-pane", "-p", "-J", "-t", "%7", "-S", "-2"}) {
		t.Fatalf("calls = %#v", *calls)
	}
}

func TestResolvePaneTarget(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    string
		wantErr bool
	}{
		{name: "marked pane", output: "%1\t\n%2\t1\n", want: "%2"},
		{name: "only pane", output: "%1\t\n", want: "%1"},
		{name: "ambiguous", output: "%1\t\n%2\t\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := stubCommands(t, commandResult{output: tt.output})
			got, err := ResolvePaneTarget("orc", "build")
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolvePaneTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ResolvePaneTarget() = %q, want %q", got, tt.want)
			}
			wantArgs := []string{"list-panes", "-t", "orc:build", "-F", "#{pane_id}\t#{@orc_agent}"}
			if len(*calls) != 1 || (*calls)[0].name != "tmux" || !reflect.DeepEqual((*calls)[0].args, wantArgs) {
				t.Fatalf("calls = %#v, want tmux %#v", *calls, wantArgs)
			}
		})
	}
}

func TestValidatePaneTargetAndFallback(t *testing.T) {
	t.Run("current", func(t *testing.T) {
		calls := stubCommands(t, commandResult{output: "orc\tbuild\n"})
		got, err := ValidatePaneTarget("orc", "build", "%7")
		if err != nil || got != "%7" {
			t.Fatalf("ValidatePaneTarget() = %q, %v; want %%7, nil", got, err)
		}
		if len(*calls) != 1 {
			t.Fatalf("calls = %d, want 1", len(*calls))
		}
	})

	t.Run("stale", func(t *testing.T) {
		calls := stubCommands(t,
			commandResult{output: "other\twindow\n"},
			commandResult{output: "%9\t1\n"},
		)
		got, err := ValidatePaneTarget("orc", "build", "%7")
		if err != nil || got != "%9" {
			t.Fatalf("ValidatePaneTarget() = %q, %v; want %%9, nil", got, err)
		}
		if len(*calls) != 2 {
			t.Fatalf("calls = %d, want 2", len(*calls))
		}
	})
}

func TestValidateAgentTargetRequiresExactInstance(t *testing.T) {
	target := mux.Target{Backend: "tmux", Workspace: "orc", Tab: "build", Pane: "%7"}

	t.Run("matching", func(t *testing.T) {
		calls := stubCommands(t, commandResult{output: "orc\tbuild\tagent-1\tinstance-1\n"})
		got, err := ValidateAgentTarget(target, "agent-1", "instance-1")
		if err != nil || got != "%7" {
			t.Fatalf("ValidateAgentTarget() = %q, %v; want %%7, nil", got, err)
		}
		if len(*calls) != 1 {
			t.Fatalf("calls = %d, want 1", len(*calls))
		}
	})

	t.Run("replaced", func(t *testing.T) {
		calls := stubCommands(t, commandResult{output: "orc\tbuild\tagent-1\tinstance-2\n"})
		if _, err := ValidateAgentTarget(target, "agent-1", "instance-1"); err == nil || !strings.Contains(err.Error(), "different agent instance") {
			t.Fatalf("ValidateAgentTarget() error = %v, want replacement error", err)
		}
		if len(*calls) != 1 {
			t.Fatalf("calls = %d, want no fallback lookup", len(*calls))
		}
	})

	t.Run("wrong window", func(t *testing.T) {
		stubCommands(t, commandResult{output: "orc\treview\tagent-1\tinstance-1\n"})
		if _, err := ValidateAgentTarget(target, "agent-1", "instance-1"); err == nil || !strings.Contains(err.Error(), "no longer belongs") {
			t.Fatalf("ValidateAgentTarget() error = %v, want target error", err)
		}
	})

	t.Run("missing identity", func(t *testing.T) {
		calls := stubCommands(t)
		if _, err := ValidateAgentTarget(target, "", ""); err == nil {
			t.Fatal("ValidateAgentTarget() should reject missing identity")
		}
		if len(*calls) != 0 {
			t.Fatalf("calls = %d, want validation before tmux", len(*calls))
		}
	})
}

func TestSetPaneMetadataClearsMissingAgentIdentity(t *testing.T) {
	calls := stubCommands(t)
	if err := SetPaneMetadata("%7", mux.Metadata{Ticket: "ORC-7"}); err != nil {
		t.Fatal(err)
	}
	var clearedID, clearedInstance bool
	for _, call := range *calls {
		joined := strings.Join(call.args, " ")
		clearedID = clearedID || joined == "set-option -p -u -t %7 @orc_agent_id"
		clearedInstance = clearedInstance || joined == "set-option -p -u -t %7 @orc_agent_instance"
	}
	if !clearedID || !clearedInstance {
		t.Fatalf("identity clear calls missing: %#v", *calls)
	}
}

func TestMetadataUsesCommandBoundary(t *testing.T) {
	calls := stubCommands(t)
	err := SetWindowMetadata("orc", "build", mux.Metadata{
		Ticket:            "ENG-42",
		Stage:             "build",
		Worker:            "codex",
		Engine:            "gpt",
		ProviderSessionID: "session-1",
		FeatureDir:        "/tmp/feature",
	})
	if err != nil {
		t.Fatalf("SetWindowMetadata() error = %v", err)
	}
	if len(*calls) == 0 {
		t.Fatal("SetWindowMetadata() did not invoke tmux")
	}
	first := (*calls)[0]
	if first.name != "tmux" || strings.Join(first.args, " ") != "set-option -w -t orc:build @orc_ticket ENG-42" {
		t.Fatalf("first call = %#v", first)
	}
}

func TestSessionEnvironment(t *testing.T) {
	t.Run("rejects invalid name", func(t *testing.T) {
		calls := stubCommands(t)
		if err := SetSessionEnvironment("orc", "BAD-NAME", "value"); err == nil {
			t.Fatal("SetSessionEnvironment() error = nil, want validation error")
		}
		if len(*calls) != 0 {
			t.Fatalf("calls = %d, want 0", len(*calls))
		}
	})

	t.Run("reads value", func(t *testing.T) {
		stubCommands(t, commandResult{output: "ORC_TEST=value\n"})
		got, err := SessionEnvironment("orc", "ORC_TEST")
		if err != nil || got != "value" {
			t.Fatalf("SessionEnvironment() = %q, %v; want value, nil", got, err)
		}
	})
}

func TestSessionAndAttachCommandsUseBoundary(t *testing.T) {
	t.Run("session exists", func(t *testing.T) {
		stubCommands(t, commandResult{}, commandResult{exit: 1})
		if !SessionExists("orc") {
			t.Fatal("SessionExists() = false, want true")
		}
		if SessionExists("missing") {
			t.Fatal("SessionExists() = true, want false")
		}
	})

	t.Run("attach target inside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "active")
		calls := stubCommands(t, commandResult{output: "orc\tbuild\n"}, commandResult{})
		if err := AttachTarget("orc", "build", "%3"); err != nil {
			t.Fatalf("AttachTarget() error = %v", err)
		}
		if len(*calls) != 2 || strings.Join((*calls)[1].args, " ") != "switch-client -t orc ; select-pane -t %3" {
			t.Fatalf("calls = %#v", *calls)
		}
	})

	t.Run("attach command outside tmux", func(t *testing.T) {
		t.Setenv("TMUX", "")
		calls := stubCommands(t, commandResult{output: "orc\tbuild\n"})
		cmd, err := AttachCommandTarget("orc", "build", "%3")
		if err != nil || cmd == nil {
			t.Fatalf("AttachCommandTarget() = %v, %v", cmd, err)
		}
		if len(*calls) != 2 || strings.Join((*calls)[1].args, " ") != "select-pane -t %3 ; attach-session -t orc:build" {
			t.Fatalf("calls = %#v", *calls)
		}
	})

	t.Run("kill and list", func(t *testing.T) {
		calls := stubCommands(t,
			commandResult{},
			commandResult{output: "orc-one\norc-two\n"},
		)
		if err := KillSession("orc-one"); err != nil {
			t.Fatalf("KillSession() error = %v", err)
		}
		got := ListSessions()
		if !reflect.DeepEqual(got, []string{"orc-one", "orc-two"}) {
			t.Fatalf("ListSessions() = %#v", got)
		}
		if len(*calls) != 2 {
			t.Fatalf("calls = %d, want 2", len(*calls))
		}
	})
}

func TestWatchHelpersUseBoundary(t *testing.T) {
	if got := AttachHint("orc", "build"); got != "tmux attach -t orc:build" {
		t.Fatalf("AttachHint() = %q", got)
	}

	t.Run("find marked pane", func(t *testing.T) {
		stubCommands(t, commandResult{output: "%1\t\n%2\t1\n"})
		got, err := findWatchPane()
		if err != nil || got != "%2" {
			t.Fatalf("findWatchPane() = %q, %v; want %%2, nil", got, err)
		}
	})

	t.Run("toggle requires tmux", func(t *testing.T) {
		t.Setenv("TMUX", "")
		calls := stubCommands(t)
		if err := ToggleWatchPane(WatchToggleOptions{}); err == nil {
			t.Fatal("ToggleWatchPane() error = nil, want tmux requirement")
		}
		if len(*calls) != 0 {
			t.Fatalf("calls = %d, want 0", len(*calls))
		}
	})
}
