package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/parking"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
)

func TestParkableEntriesRequireRunningManagedResumeMetadata(t *testing.T) {
	sessions := []sessionlist.Session{
		{Kind: sessionlist.KindManaged, Running: true, Ticket: "ORC-1", Stage: "develop", Engine: "codex", FeatureDir: "/feature", Target: &sessionlist.Target{Session: "orc-1", Window: "develop"}, Live: &telemetry.Live{ProviderSessionID: "abc", CWD: "/work"}},
		{Kind: sessionlist.KindManaged, Running: true, Ticket: "ORC-2", Engine: "codex", Target: &sessionlist.Target{Session: "orc-2"}},
		{Kind: sessionlist.KindUnmanaged, Running: true, Engine: "claude", Target: &sessionlist.Target{Session: "personal"}, Live: &telemetry.Live{ProviderSessionID: "def"}},
	}
	entries, skipped := parkableEntries(sessions)
	if len(entries) != 1 || entries[0].Ticket != "ORC-1" || entries[0].ProviderSessionID != "abc" {
		t.Fatalf("entries = %#v", entries)
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}
}

func TestResumedLaunchArgvCarriesProviderIdentity(t *testing.T) {
	got := resumedLaunchArgv("codex", []string{"resume", "abc"}, "abc")
	want := []string{"env", "ORC_RESUMED_FROM=abc", "codex", "resume", "abc"}
	if len(got) != len(want) {
		t.Fatalf("argv = %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNextParkingAgentRuntimeReusesDurableIdentity(t *testing.T) {
	featureDir := t.TempDir()
	if err := state.Create(featureDir, &state.State{
		Ticket: "ORC-1", Slug: "ORC-1", Status: "active",
		Runtime: state.Runtime{Agent: &state.AgentRuntime{
			ID: "agent-1", Instance: "instance-old", Stage: "develop", Engine: "codex", ProviderSessionID: "provider-old",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	oldAgentID, oldInstanceID := newParkingAgentID, newParkingInstanceID
	t.Cleanup(func() {
		newParkingAgentID = oldAgentID
		newParkingInstanceID = oldInstanceID
	})
	newParkingAgentID = func() (string, error) {
		t.Fatal("matching engine should reuse the durable agent id")
		return "", nil
	}
	newParkingInstanceID = func() (string, error) { return "instance-new", nil }
	got, err := nextParkingAgentRuntime(parking.Entry{
		FeatureDir: featureDir, Stage: "develop", Engine: "CODEX", ProviderSessionID: "provider-resumed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "agent-1" || got.Instance != "instance-new" || got.Stage != "develop" || got.Engine != "codex" || got.ProviderSessionID != "provider-resumed" {
		t.Fatalf("agent runtime = %+v", got)
	}
}

func TestRestoredPaneRequiresExactParkedIdentity(t *testing.T) {
	entry := parking.Entry{
		Ticket: "ORC-1", Stage: "develop", Engine: "codex", ProviderSessionID: "provider-1",
		TmuxSession: "orc-1", TmuxWindow: "develop",
	}
	panes := []mux.Pane{
		{ID: "%1", Agent: true, Session: "orc-1", Window: "develop", Ticket: "ORC-1", Stage: "develop", ProviderEngine: "codex", ProviderSessionID: "other"},
		{ID: "%2", Agent: true, Session: "orc-1", Window: "develop", Ticket: "ORC-1", Stage: "develop", ProviderEngine: "CODEX", ProviderSessionID: "provider-1"},
	}

	pane, ok := restoredPane(entry, panes)
	if !ok || pane != "%2" {
		t.Fatalf("restoredPane = %q, %v", pane, ok)
	}
}

func TestRestoredPaneRejectsUnrelatedSessionCollision(t *testing.T) {
	entry := parking.Entry{
		Ticket: "ORC-1", Stage: "develop", Engine: "codex", ProviderSessionID: "provider-1",
		TmuxSession: "orc-1", TmuxWindow: "develop",
	}
	panes := []mux.Pane{
		{ID: "%1", Agent: true, Session: "orc-1", Window: "develop", Ticket: "ORC-OTHER", Stage: "develop", ProviderEngine: "codex", ProviderSessionID: "provider-1"},
		{ID: "%2", Agent: false, Session: "orc-1", Window: "develop", Ticket: "ORC-1", Stage: "develop", ProviderEngine: "codex", ProviderSessionID: "provider-1"},
	}

	if pane, ok := restoredPane(entry, panes); ok {
		t.Fatalf("restoredPane accepted collision at %s", pane)
	}
}

func TestUnparkEntryReconcilesMatchingExistingSession(t *testing.T) {
	if !muxBackend.Available() {
		t.Skip("tmux is not installed")
	}
	socketDir, err := os.MkdirTemp("/tmp", "orc-unpark-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(socketDir) //nolint:errcheck
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_PANE", "")
	t.Setenv("TMUX_TMPDIR", socketDir)
	defer exec.Command("tmux", "kill-server").Run() //nolint:errcheck

	featureDir := t.TempDir()
	stateYAML := "ticket: ORC-1\nslug: ORC-1\nstatus: active\nstage:\n  worker: default:bob\n  name: develop\n"
	if err := os.WriteFile(filepath.Join(featureDir, state.Filename), []byte(stateYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := parking.Entry{
		Ticket: "ORC-1", Stage: "develop", Worker: "default:bob", Engine: "codex",
		ProviderSessionID: "provider-1", CWD: featureDir, FeatureDir: featureDir,
		TmuxSession: "orc-1", TmuxWindow: "develop",
	}
	if err := exec.Command("tmux", "new-session", "-d", "-s", entry.TmuxSession, "-n", entry.TmuxWindow, "-c", featureDir, "sleep 30").Run(); err != nil {
		t.Fatalf("create test session: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for !muxBackend.SessionExists(entry.TmuxSession) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !muxBackend.SessionExists(entry.TmuxSession) {
		t.Fatal("test tmux session did not become available")
	}
	metadata := mux.Metadata{
		Ticket: entry.Ticket, Stage: entry.Stage, Worker: entry.Worker, Engine: entry.Engine,
		ProviderSessionID: entry.ProviderSessionID, FeatureDir: entry.FeatureDir,
	}
	if err := muxBackend.SetWindowMetadata(entry.TmuxSession, entry.TmuxWindow, metadata); err != nil {
		t.Fatal(err)
	}
	pane, err := muxBackend.ResolvePane(entry.TmuxSession, entry.TmuxWindow)
	if err != nil {
		t.Fatal(err)
	}
	if err := muxBackend.SetPaneMetadata(pane, metadata); err != nil {
		t.Fatal(err)
	}
	panes, err := muxBackend.ListPanes()
	if err != nil {
		t.Fatal(err)
	}
	if restored, ok := restoredPane(entry, panes); !ok || restored != pane {
		t.Fatalf("matching restored pane = %q, %v; panes = %#v", restored, ok, panes)
	}

	if err := unparkEntry(entry); err != nil {
		t.Fatalf("unparkEntry: %v", err)
	}
	got, err := state.Load(featureDir)
	if err != nil {
		t.Fatal(err)
	}
	target, ok := got.Runtime.MuxTarget(got.Stage.Name)
	if !ok || target.Backend != "tmux" || target.Workspace != entry.TmuxSession || target.Pane != pane {
		t.Fatalf("runtime target = %#v", target)
	}
}
