package parking

import (
	"os"
	"testing"
	"time"
)

func TestSnapshotRoundTrip(t *testing.T) {
	path, err := Path("/workspace", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{Workspace: "/workspace", ParkedAt: time.Now().UTC(), Sessions: []Entry{{Ticket: "ORC-1", Engine: "codex", ProviderSessionID: "abc"}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != Version || len(got.Sessions) != 1 || got.Sessions[0].ProviderSessionID != "abc" {
		t.Fatalf("snapshot = %#v", got)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, err=%v", info.Mode().Perm(), err)
	}
}
