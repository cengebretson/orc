package parking

import (
	"os"
	"path/filepath"
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

func TestRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "parked.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Remove(path); err != nil {
		t.Fatalf("Remove(existing) error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("removed file stat error = %v, want not-exist", err)
	}
	if err := Remove(path); err != nil {
		t.Fatalf("Remove(missing) error = %v", err)
	}
}
