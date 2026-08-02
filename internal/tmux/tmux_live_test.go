package tmux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

func TestLiveTmuxWindowMetadataAttentionAndExactSend(t *testing.T) {
	if !Available() {
		t.Skip("tmux is not installed")
	}

	// Keep the socket path short enough for macOS's Unix-domain socket limit.
	socketDir, err := os.MkdirTemp("/tmp", "orc-tmux-test-")
	if err != nil {
		t.Fatalf("create tmux socket dir: %v", err)
	}
	defer os.RemoveAll(socketDir) //nolint:errcheck
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX_TMPDIR", socketDir)
	defer exec.Command("tmux", "kill-server").Run() //nolint:errcheck

	root := t.TempDir()
	const session = "orc-live-test"
	if err := CreateSession(session, root, []string{"develop", "review"}); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	metadata := mux.Metadata{
		Ticket:            "ORC-123",
		Stage:             "review",
		Worker:            "default:ada",
		Engine:            "codex",
		ProviderSessionID: "provider-123",
		FeatureDir:        root,
	}
	if err := SetWindowMetadata(session, "review", metadata); err != nil {
		t.Fatalf("SetWindowMetadata: %v", err)
	}
	pane, err := ResolvePaneTarget(session, "review")
	if err != nil {
		t.Fatalf("ResolvePaneTarget: %v", err)
	}
	if err := SetPaneMetadata(pane, metadata); err != nil {
		t.Fatalf("SetPaneMetadata: %v", err)
	}
	withoutProviderID := metadata
	withoutProviderID.ProviderSessionID = ""
	if err := SetWindowMetadata(session, "review", withoutProviderID); err != nil {
		t.Fatalf("clear window provider metadata: %v", err)
	}
	if err := SetPaneMetadata(pane, withoutProviderID); err != nil {
		t.Fatalf("clear pane provider metadata: %v", err)
	}
	if got, err := WindowOption(session, "review", "@orc_provider_session"); err != nil || got != "" {
		t.Fatalf("cleared window provider session = %q, %v", got, err)
	}
	if err := SetWindowMetadata(session, "review", metadata); err != nil {
		t.Fatalf("restore window provider metadata: %v", err)
	}
	if err := SetPaneMetadata(pane, metadata); err != nil {
		t.Fatalf("restore pane provider metadata: %v", err)
	}
	for option, want := range map[string]string{
		"@orc_ticket":           metadata.Ticket,
		"@orc_stage":            metadata.Stage,
		"@orc_worker":           metadata.Worker,
		"@orc_engine":           metadata.Engine,
		"@orc_provider_engine":  metadata.Engine,
		"@orc_provider_session": metadata.ProviderSessionID,
		"@orc_feature_dir":      metadata.FeatureDir,
	} {
		got, err := WindowOption(session, "review", option)
		if err != nil {
			t.Fatalf("WindowOption(%s): %v", option, err)
		}
		if got != want {
			t.Fatalf("WindowOption(%s) = %q, want %q", option, got, want)
		}
	}
	if err := SetSessionEnvironment(session, mux.EnvResumedFrom, metadata.ProviderSessionID); err != nil {
		t.Fatalf("SetSessionEnvironment: %v", err)
	}
	if got, err := SessionEnvironment(session, mux.EnvResumedFrom); err != nil || got != metadata.ProviderSessionID {
		t.Fatalf("SessionEnvironment = %q, %v; want %q", got, err, metadata.ProviderSessionID)
	}

	if err := exec.Command("tmux", "set-option", "-w", "-t", session+":review", "@agent_attention", mux.AttentionInput).Run(); err != nil {
		t.Fatalf("set attention: %v", err)
	}
	if got := WindowAttention(session, "review"); got != mux.AttentionInput {
		t.Fatalf("WindowAttention = %q, want %q", got, mux.AttentionInput)
	}

	// Split the agent window and activate a different pane. Exact pane identity
	// must keep subsequent commands away from the newly-active shell.
	secondOut, err := exec.Command("tmux", "split-window", "-d", "-P", "-F", "#{pane_id}", "-t", session+":review", "sleep 30").Output()
	if err != nil {
		t.Fatalf("split review window: %v", err)
	}
	secondPane := strings.TrimSpace(string(secondOut))
	if secondPane == "" || secondPane == pane {
		t.Fatalf("split pane = %q, first pane %q", secondPane, pane)
	}
	if err := exec.Command("tmux", "select-pane", "-t", secondPane).Run(); err != nil {
		t.Fatalf("select second review pane: %v", err)
	}
	if resolved, err := ResolvePaneTarget(session, "review"); err != nil || resolved != pane {
		t.Fatalf("ResolvePaneTarget after split = %q, %v; want %q", resolved, err, pane)
	}
	if resolved, err := ValidatePaneTarget(session, "review", "%999999"); err != nil || resolved != pane {
		t.Fatalf("ValidatePaneTarget stale pane = %q, %v; want %q", resolved, err, pane)
	}
	output := filepath.Join(root, "review-ready")
	pidFile := filepath.Join(root, "provider-pid")
	gotPane, err := SendCommandTarget(session, "review", pane, root, root, []string{"sh", "-c", `printf '%s' "$$" > "$1"; printf ready > "$2"; sleep 30`, "sh", pidFile, output})
	if err != nil {
		t.Fatalf("SendCommandTarget: %v", err)
	}
	if gotPane != pane {
		t.Fatalf("SendCommandTarget pane = %q, want %q", gotPane, pane)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(output)
		if err == nil {
			if string(data) != "ready" {
				t.Fatalf("output = %q, want ready", data)
			}
			providerPIDBytes, readErr := os.ReadFile(pidFile)
			if readErr != nil {
				t.Fatalf("read provider pid: %v", readErr)
			}
			providerPID, parseErr := strconv.Atoi(string(providerPIDBytes))
			if parseErr != nil {
				t.Fatalf("parse provider pid: %v", parseErr)
			}
			panePIDOut, tmuxErr := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{pane_pid}").Output()
			if tmuxErr != nil {
				t.Fatalf("read pane pid: %v", tmuxErr)
			}
			panePID, parseErr := strconv.Atoi(strings.TrimSpace(string(panePIDOut)))
			if parseErr != nil || panePID != providerPID {
				t.Fatalf("pane pid = %d (%v), provider pid = %d", panePID, parseErr, providerPID)
			}
			panes, listErr := ListPanesDetailed()
			if listErr != nil {
				t.Fatalf("ListPanesDetailed: %v", listErr)
			}
			found := false
			for _, listed := range panes {
				if listed.ID != pane {
					continue
				}
				found = true
				if listed.PID != providerPID || listed.ProviderEngine != metadata.Engine || listed.ProviderSessionID != metadata.ProviderSessionID {
					t.Fatalf("listed pane = %#v", listed)
				}
			}
			if !found {
				t.Fatalf("pane %s missing from detailed inventory", pane)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for command in review window")
}
