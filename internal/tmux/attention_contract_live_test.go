package tmux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// findAttentionCLI locates an installed tmux-attention CLI, or reports that
// there is none.
//
// The plugin is an optional dependency (ADR-0005), so its absence must skip
// rather than fail — a machine without it is a supported configuration, not a
// broken one.
func findAttentionCLI(t *testing.T) string {
	t.Helper()
	// An explicitly pointed-at CLI that does not work is operator error, not a
	// missing optional dependency: say so rather than skipping and looking green.
	if override := os.Getenv("ORC_TMUX_ATTENTION_CLI"); override != "" {
		info, err := os.Stat(override)
		if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
			t.Fatalf("ORC_TMUX_ATTENTION_CLI=%s is not an executable file", override)
		}
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, candidate := range []string{
		filepath.Join(home, ".config/tmux/plugins/tmux-attention/scripts/tmux-attention"),
		filepath.Join(home, ".tmux/plugins/tmux-attention/scripts/tmux-attention"),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}

// TestLiveAttentionPluginContract drives the real tmux-attention CLI and asserts
// Orc reads back what it actually wrote.
//
// Every other test in this package asserts against option names Orc hardcodes,
// which is exactly how the two projects silently disagreed: Orc read
// @agent_attention_since while the plugin had always written
// @agent_pane_attention_updated_at, and both test suites stayed green because
// neither ever saw the other's output. A rename on either side is invisible to
// unit tests by construction. This one fails.
func TestLiveAttentionPluginContract(t *testing.T) {
	if !Available() {
		t.Skip("tmux is not installed")
	}
	cli := findAttentionCLI(t)
	if cli == "" {
		t.Skip("tmux-attention is not installed; it is an optional dependency (ADR-0005)")
	}

	socketDir, err := os.MkdirTemp("/tmp", "orc-attn-contract-")
	if err != nil {
		t.Fatalf("create tmux socket dir: %v", err)
	}
	defer os.RemoveAll(socketDir) //nolint:errcheck
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX_TMPDIR", socketDir)
	defer exec.Command("tmux", "kill-server").Run() //nolint:errcheck

	root := t.TempDir()
	requireTmuxServer(t, root)

	const session = "orc-attn-contract"
	if err := CreateSession(session, root, []string{"develop"}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	pane, err := exec.Command("tmux", "list-panes", "-t", session+":develop", "-F", "#{pane_id}").Output()
	if err != nil {
		t.Fatalf("list panes: %v", err)
	}
	target := strings.TrimSpace(string(pane))
	if target == "" {
		t.Fatal("no pane in the test window")
	}

	attention := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(cli, args...).CombinedOutput(); err != nil {
			t.Fatalf("tmux-attention %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
		}
	}

	paneNamed := func(id string) *mux0Pane {
		t.Helper()
		panes, err := ListPanesDetailed()
		if err != nil {
			t.Fatalf("ListPanesDetailed: %v", err)
		}
		for i := range panes {
			if panes[i].ID == id {
				return &mux0Pane{panes[i].Attention, panes[i].AttentionSince, panes[i].AttentionSource, panes[i].ContextActive}
			}
		}
		t.Fatalf("pane %s not in the inventory", id)
		return nil
	}

	// A marker the plugin wrote must arrive with its state, its age, and its
	// source. The age is the field that silently read as zero for as long as the
	// two disagreed.
	attention("blocked", "--target", target, "--source", "claude", "--reason", "permission_prompt")
	got := paneNamed(target)
	if got.attention != "blocked" {
		t.Errorf("Attention = %q, want blocked from the plugin's marker", got.attention)
	}
	if got.since <= 0 {
		t.Errorf("AttentionSince = %d, want the plugin's timestamp — the field names have diverged again", got.since)
	}
	if got.source != "claude" {
		t.Errorf("AttentionSource = %q, want claude", got.source)
	}

	// An active turn must surface as ContextActive, which feeds the `context`
	// observation source.
	attention("turn-start", "--target", target, "--project", "ORC-CONTRACT")
	if got := paneNamed(target); !got.contextActive {
		t.Error("ContextActive = false after turn-start; the active-turn option has been renamed or rescoped")
	}

	// turn-start clears the marker it found, and Orc must see that too rather
	// than holding a stale state.
	if got := paneNamed(target); got.attention != "" {
		t.Errorf("Attention = %q after turn-start, want cleared", got.attention)
	}

	// turn-done ends the turn and leaves a done marker.
	attention("turn-done", "--target", target)
	got = paneNamed(target)
	if got.contextActive {
		t.Error("ContextActive = true after turn-done, want false")
	}
	if got.attention != "done" {
		t.Errorf("Attention = %q after turn-done, want done", got.attention)
	}

	// Clearing must reach Orc as an absent marker, not a stale one.
	attention("clear", "--target", target)
	if got := paneNamed(target); got.attention != "" || got.since != 0 {
		t.Errorf("after clear: attention=%q since=%d, want empty/0", got.attention, got.since)
	}
}

// mux0Pane is the small slice of mux.Pane this contract cares about, kept local
// so the assertions read as the contract rather than as struct plumbing.
type mux0Pane struct {
	attention     string
	since         int64
	source        string
	contextActive bool
}
