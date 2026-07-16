package main

import (
	"testing"

	"github.com/cengebretson/orc/internal/sessionlist"
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
