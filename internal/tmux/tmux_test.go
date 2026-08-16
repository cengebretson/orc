package tmux

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/shellquote"
)

func TestWriteScriptCleansUpAfterCommandFailure(t *testing.T) {
	script, err := writeScript(t.TempDir(), []string{"false"})
	if err != nil {
		t.Fatalf("writeScript: %v", err)
	}

	if err := exec.Command("bash", script).Run(); err == nil {
		t.Fatal("expected script command to fail")
	}
	if _, err := os.Stat(script); !os.IsNotExist(err) {
		t.Fatalf("script still exists after failed command: %v", err)
	}
}

// A pane reporting no attention still emits its trailing fields as empty
// strings. Trimming them would leave the row short and drop an otherwise valid
// pane from the inventory.
func TestParseDetailedPanesPreservesEmptyAttention(t *testing.T) {
	row := strings.Join([]string{
		"%1", "orc-1", "develop", "/work", "codex", "4242", "1",
		"agent-1", "instance-1", "ORC-1", "develop", "default:bob", "codex", "codex", "provider-1", "/feature",
		"", "", "", "", "", "",
	}, "\t") + "\n"

	got := parseDetailedPanes([]byte(row))
	if len(got) != 1 {
		t.Fatalf("panes = %#v", got)
	}
	if got[0].ID != "%1" || got[0].AgentID != "agent-1" || got[0].AgentInstance != "instance-1" || got[0].ProviderSessionID != "provider-1" || got[0].Attention != "" {
		t.Fatalf("pane = %#v", got[0])
	}
	if got[0].AttentionSince != 0 {
		t.Fatalf("AttentionSince = %d, want 0 when unreported", got[0].AttentionSince)
	}
}

// An unrecognized marker is not passed through to the display. Orc defines the
// attention vocabulary; anything else is no signal rather than a fifth state
// every renderer would have to know about.
func TestParseDetailedPanesNormalizesAttention(t *testing.T) {
	rows := func(attention string) string {
		return strings.Join([]string{
			"%1", "orc-1", "develop", "/work", "codex", "4242", "1",
			"agent-1", "instance-1", "ORC-1", "develop", "default:bob", "codex", "codex", "p1", "/feature",
			"working", "7", "1699999999", "hook", attention, "1700000000",
		}, "\t") + "\n"
	}

	for _, test := range []struct{ raw, want string }{
		{"blocked", "blocked"},
		{"BLOCKED", "blocked"},
		{"clear", ""},
		{"nonsense", ""},
	} {
		got := parseDetailedPanes([]byte(rows(test.raw)))
		if len(got) != 1 {
			t.Fatalf("%q: panes = %#v", test.raw, got)
		}
		if got[0].Attention != test.want {
			t.Errorf("%q: Attention = %q, want %q", test.raw, got[0].Attention, test.want)
		}
		if got[0].Lifecycle != "working" || got[0].StateChangeSeq != 7 || got[0].LifecycleSince != 1699999999 || got[0].LifecycleSource != "hook" {
			t.Errorf("%q: lifecycle metadata = %#v", test.raw, got[0])
		}
	}
}

func TestParseDetailedPanesHidesAcknowledgedAttention(t *testing.T) {
	row := strings.Join([]string{
		"%1", "orc-1", "develop", "/work", "codex", "4242", "1",
		"agent-1", "instance-1", "ORC-1", "develop", "default:bob", "codex", "codex", "p1", "/feature",
		"done", "7", "1699999999", "hook", "done", "1700000000", "7",
	}, "\t") + "\n"
	got := parseDetailedPanes([]byte(row))
	if len(got) != 1 || got[0].Attention != "" || got[0].AttentionSince != 0 || got[0].SeenSeq != 7 {
		t.Fatalf("acknowledged pane = %#v", got)
	}
}

// tmux-attention records a marker's age as @agent_pane_attention_updated_at and
// leaves @agent_attention_since empty. Reading only the legacy pair saw such a
// marker's state with no age, so every age-derived display showed 0.
func TestParseDetailedPanesPrefersPluginAttentionFields(t *testing.T) {
	// Legacy state/since empty (fields 20/21) as a plugin-set marker leaves
	// them; the plugin's own pane fields carry the marker.
	row := strings.Join([]string{
		"%1", "orc-1", "develop", "/work", "codex", "4242", "1",
		"agent-1", "instance-1", "ORC-1", "develop", "default:bob", "codex", "codex", "p1", "/feature",
		"working", "0", "1699999999", "hook", "", "", "0",
		"title", "working", "hook", "1699999999", "rule", "claude",
		"blocked", "1700000042",
	}, "\t") + "\n"

	got := parseDetailedPanes([]byte(row))
	if len(got) != 1 {
		t.Fatalf("panes = %#v", got)
	}
	if got[0].Attention != "blocked" {
		t.Errorf("Attention = %q, want %q from the plugin field", got[0].Attention, "blocked")
	}
	if got[0].AttentionSince != 1700000042 {
		t.Errorf("AttentionSince = %d, want 1700000042 from @agent_pane_attention_updated_at", got[0].AttentionSince)
	}
}

// Orc's own writes still populate the legacy pair, and a row carrying no plugin
// marker must keep using them rather than being blanked by the empty field.
func TestParseDetailedPanesFallsBackToLegacyAttention(t *testing.T) {
	row := strings.Join([]string{
		"%1", "orc-1", "develop", "/work", "codex", "4242", "1",
		"agent-1", "instance-1", "ORC-1", "develop", "default:bob", "codex", "codex", "p1", "/feature",
		"working", "0", "1699999999", "hook", "review", "1700000000", "0",
		"title", "working", "hook", "1699999999", "rule", "orc",
		"", "",
	}, "\t") + "\n"

	got := parseDetailedPanes([]byte(row))
	if len(got) != 1 {
		t.Fatalf("panes = %#v", got)
	}
	if got[0].Attention != "review" || got[0].AttentionSince != 1700000000 {
		t.Fatalf("legacy attention not preserved: %#v", got[0])
	}
}

// Both schemas are published for every state change: the plugin's pane fields
// so its CLI, clear-on-view, and tmux-fzf-jump's attention view see the marker,
// and orc's original names for its own readers.
func TestAttentionUpdatesWritesBothSchemas(t *testing.T) {
	got := map[string]string{}
	for _, update := range attentionUpdates("blocked", "1700000000", "hook") {
		got[update[0]] = update[1]
	}

	for name, want := range map[string]string{
		"@agent_pane_attention":            "blocked",
		"@agent_pane_attention_updated_at": "1700000000",
		"@agent_pane_attention_source":     "hook",
		"@agent_attention":                 "blocked",
		"@agent_attention_since":           "1700000000",
		"@agent_attention_source":          "hook",
	} {
		if got[name] != want {
			t.Errorf("%s = %q, want %q", name, got[name], want)
		}
	}
	if len(got) != 6 {
		t.Errorf("wrote %d options, want 6: %#v", len(got), got)
	}
}

func TestWriteScriptQuotesArguments(t *testing.T) {
	runDir := t.TempDir()
	script, err := writeScript(runDir, []string{"printf", "%s", "value with 'quotes'"})
	if err != nil {
		t.Fatalf("writeScript: %v", err)
	}
	defer os.Remove(script) //nolint:errcheck

	content, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "trap 'rm -f ") {
		t.Fatalf("script missing cleanup trap:\n%s", text)
	}
	if !strings.Contains(text, "cd "+shellquote.Word(runDir)+" || exit 1") {
		t.Fatalf("script missing guarded cd into run dir:\n%s", text)
	}
	if !strings.Contains(text, shellquote.Word("value with 'quotes'")) {
		t.Fatalf("script missing quoted argument:\n%s", text)
	}
	if !strings.Contains(text, "trap - EXIT\nexec printf") {
		t.Fatalf("script must replace itself with the provider command:\n%s", text)
	}
}

func TestWriteScriptRejectsEmptyCommand(t *testing.T) {
	if _, err := writeScript(t.TempDir(), nil); err == nil {
		t.Fatal("writeScript should reject an empty command")
	}
}

func TestSetSessionEnvironmentRejectsUnsafeName(t *testing.T) {
	for _, name := range []string{"", "-g", "1BAD", "BAD=VALUE", "BAD NAME"} {
		if err := SetSessionEnvironment("unused", name, "value"); err == nil {
			t.Errorf("SetSessionEnvironment should reject %q", name)
		}
		if _, err := SessionEnvironment("unused", name); err == nil {
			t.Errorf("SessionEnvironment should reject %q", name)
		}
	}
}

func TestBuildWatchCommandQuotesArguments(t *testing.T) {
	cmd := buildWatchCommand(WatchToggleOptions{
		ExecPath: "/tmp/orc bin/orc",
		Root:     "/tmp/work space",
		Ticket:   "PROJ-123",
		Interval: "2s",
		Wide:     true,
		Demo:     true,
	})

	for _, want := range []string{
		shellquote.Word("/tmp/orc bin/orc"),
		"--workspace " + shellquote.Word("/tmp/work space"),
		"watch PROJ-123",
		"--interval 2s",
		"--wide",
		"--demo",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("buildWatchCommand() missing %q in %q", want, cmd)
		}
	}
	if strings.Contains(cmd, "--tmux-toggle") {
		t.Fatalf("buildWatchCommand() must not recurse with --tmux-toggle: %q", cmd)
	}
}

func TestBuildWatchCommandOmitsUnsetOptions(t *testing.T) {
	cmd := buildWatchCommand(WatchToggleOptions{ExecPath: "orc"})
	// --tmux-toggle would make the spawned rail spawn another rail.
	for _, flag := range []string{"--interval", "--wide", "--demo", "--tmux-toggle"} {
		if strings.Contains(cmd, flag) {
			t.Fatalf("buildWatchCommand() included unset flag %q in %q", flag, cmd)
		}
	}
}

func TestWatchSplitFlag(t *testing.T) {
	tests := []struct {
		layout string
		want   string
	}{
		{"", "-h"},
		{"right", "-h"},
		{"bottom", "-v"},
	}

	for _, tt := range tests {
		t.Run(tt.layout, func(t *testing.T) {
			got, err := watchSplitFlag(tt.layout)
			if err != nil {
				t.Fatalf("watchSplitFlag(%q): %v", tt.layout, err)
			}
			if got != tt.want {
				t.Fatalf("watchSplitFlag(%q) = %q, want %q", tt.layout, got, tt.want)
			}
		})
	}

	if _, err := watchSplitFlag("left"); err == nil {
		t.Fatal("watchSplitFlag(left) should fail")
	}
}
