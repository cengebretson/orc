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

func TestApplyPolicyParksAndWakesBeforeReparking(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	policy := Policy{AutoPark: []string{"paused"}, WakeOn: []string{"status_change", "attention", "stage_change"}}
	now := time.Now().UTC()
	observe := func(observation Observation) Decision {
		t.Helper()
		decisions, err := ApplyPolicy(path, "/workspace", policy, []Observation{observation}, now)
		if err != nil {
			t.Fatal(err)
		}
		return decisions[observation.Ticket]
	}

	if got := observe(Observation{Ticket: "ORC-1", Status: "paused", Stage: "develop"}); !got.Parked {
		t.Fatalf("initial decision = %#v, want parked", got)
	}
	if got := observe(Observation{Ticket: "ORC-1", Status: "paused", Stage: "review"}); !got.Woken || got.WakeReason != "stage_change" {
		t.Fatalf("stage-change decision = %#v, want woken", got)
	}
	if got := observe(Observation{Ticket: "ORC-1", Status: "paused", Stage: "review"}); !got.Woken || got.Parked {
		t.Fatalf("woken suppression decision = %#v", got)
	}
	if got := observe(Observation{Ticket: "ORC-1", Status: "active", Stage: "review"}); got.Parked || got.Woken {
		t.Fatalf("rearm decision = %#v", got)
	}
	if got := observe(Observation{Ticket: "ORC-1", Status: "paused", Stage: "review"}); !got.Parked {
		t.Fatalf("repark decision = %#v, want parked", got)
	}
}

func TestApplyPolicyLeavesAttentionVisibleOnFirstObservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	decisions, err := ApplyPolicy(path, "/workspace", Policy{
		AutoPark: []string{"paused"}, WakeOn: []string{"attention"},
	}, []Observation{{Ticket: "ORC-2", Status: "paused", Stage: "review", Attention: "blocked"}}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if got := decisions["ORC-2"]; !got.Woken || got.Parked || got.WakeReason != "attention" {
		t.Fatalf("decision = %#v, want visible attention wake", got)
	}
}

func TestPolicyPathDoesNotOverlapManualParkingSnapshot(t *testing.T) {
	home := t.TempDir()
	snapshot, err := Path("/workspace", home)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := PolicyPath("/workspace", home)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot == policy || filepath.Dir(snapshot) == filepath.Dir(policy) {
		t.Fatalf("snapshot=%q policy=%q", snapshot, policy)
	}
}
