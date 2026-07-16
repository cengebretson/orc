package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/parking"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/tmux"
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

func TestRestoredPaneRequiresExactParkedIdentity(t *testing.T) {
	entry := parking.Entry{
		Ticket: "ORC-1", Stage: "develop", Engine: "codex", ProviderSessionID: "provider-1",
		TmuxSession: "orc-1", TmuxWindow: "develop",
	}
	panes := []tmux.Pane{
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
	panes := []tmux.Pane{
		{ID: "%1", Agent: true, Session: "orc-1", Window: "develop", Ticket: "ORC-OTHER", Stage: "develop", ProviderEngine: "codex", ProviderSessionID: "provider-1"},
		{ID: "%2", Agent: false, Session: "orc-1", Window: "develop", Ticket: "ORC-1", Stage: "develop", ProviderEngine: "codex", ProviderSessionID: "provider-1"},
	}

	if pane, ok := restoredPane(entry, panes); ok {
		t.Fatalf("restoredPane accepted collision at %s", pane)
	}
}

func TestUnparkEntryReconcilesMatchingExistingSession(t *testing.T) {
	if !tmux.Available() {
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
	for !tmux.SessionExists(entry.TmuxSession) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !tmux.SessionExists(entry.TmuxSession) {
		t.Fatal("test tmux session did not become available")
	}
	metadata := tmux.WindowMetadata{
		Ticket: entry.Ticket, Stage: entry.Stage, Worker: entry.Worker, Engine: entry.Engine,
		ProviderSessionID: entry.ProviderSessionID, FeatureDir: entry.FeatureDir,
	}
	if err := tmux.SetWindowMetadata(entry.TmuxSession, entry.TmuxWindow, metadata); err != nil {
		t.Fatal(err)
	}
	pane, err := tmux.ResolvePaneTarget(entry.TmuxSession, entry.TmuxWindow)
	if err != nil {
		t.Fatal(err)
	}
	if err := tmux.SetPaneMetadata(pane, metadata); err != nil {
		t.Fatal(err)
	}
	panes, err := tmux.ListPanesDetailed()
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
	if got.Runtime.Tmux == nil || got.Runtime.Tmux.Session != entry.TmuxSession || got.Runtime.Tmux.Pane != pane {
		t.Fatalf("runtime target = %#v", got.Runtime.Tmux)
	}
}
