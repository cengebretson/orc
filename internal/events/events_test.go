package events

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/workspacesnapshot"
)

func TestCaptureProjectsDurableAndLiveState(t *testing.T) {
	lastActive := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	snapshot := testSnapshot("ORC-1", "develop", "active")
	item := snapshot.Items[0]
	item.Attention = "blocked"
	item.AttentionSource = "hook"
	item.Feature.RequiredArtifacts = []string{"PLAN.md"}
	item.Feature.TmuxLive = true
	item.Feature.State.Runtime.Mux = &state.MuxRuntime{Backend: "tmux", Workspace: "orc-1", Tab: "develop", Pane: "%7"}
	item.Feature.State.Runtime.Agent = &state.AgentRuntime{ID: "agent-1", Instance: "instance-1", Engine: "codex"}
	item.Lifecycle = "working"
	item.LifecycleSource = "hook"
	item.HasTelemetry = true
	item.Live = telemetry.Live{
		Engine: "codex", ProviderSessionID: "provider-1", Model: "gpt-5",
		State: "working", ContextUsed: 42, ContextLimit: 100, LastActive: lastActive,
		PaneTarget: "%8",
	}

	got := Capture(snapshot)
	projected, ok := got["ticket:ORC-1"]
	if !ok {
		t.Fatalf("capture keys = %#v, want ticket:ORC-1", got)
	}
	if projected.Feature.Status != "active" || projected.Feature.FeatureDir != filepath.Clean(item.Feature.FeatureDir) {
		t.Fatalf("feature = %#v", projected.Feature)
	}
	if projected.Stage.Name != "develop" || projected.Stage.WorkerID != "default:bob" {
		t.Fatalf("stage = %#v", projected.Stage)
	}
	if projected.Attention.State != "blocked" || projected.Attention.Source != "hook" {
		t.Fatalf("attention = %#v", projected.Attention)
	}
	if !projected.Session.Running || projected.Session.Backend != "tmux" || projected.Session.Workspace != "orc-1" || projected.Session.Pane != "%8" {
		t.Fatalf("session target = %#v", projected.Session)
	}
	if projected.Session.AgentID != "agent-1" || projected.Session.Lifecycle != "working" || projected.Session.ProviderSessionID != "provider-1" || projected.Session.LastActive != lastActive.Format(time.RFC3339Nano) {
		t.Fatalf("session telemetry = %#v", projected.Session)
	}
}

func TestDiffEmitsDeterministicDomainEvents(t *testing.T) {
	at := time.Date(2026, 8, 18, 12, 1, 0, 0, time.UTC)
	item := onlyItem(Capture(testSnapshot("ORC-1", "develop", "active")))
	before := State{"ticket:ORC-1": item}
	afterItem := item
	afterItem.Feature.Status = "paused"
	afterItem.Attention.State = "input"
	afterItem.Session.Running = true
	afterItem.Stage.Name = "review"
	after := State{"ticket:ORC-1": afterItem}

	got := Diff(before, after, at)
	want := []Type{FeatureChanged, AttentionChanged, SessionChanged, StageChanged}
	if len(got) != len(want) {
		t.Fatalf("events = %#v, want %d", got, len(want))
	}
	for i, eventType := range want {
		if got[i].Type != eventType {
			t.Errorf("event %d type = %s, want %s", i, got[i].Type, eventType)
		}
		if got[i].At != at || got[i].Ticket != "ORC-1" || got[i].Before == nil || got[i].After == nil {
			t.Errorf("event %d envelope = %#v", i, got[i])
		}
	}
}

func TestDiffUsesFeatureEventsForAddAndRemove(t *testing.T) {
	at := time.Date(2026, 8, 18, 12, 2, 0, 0, time.UTC)
	current := Capture(testSnapshot("ORC-1", "develop", "active"))

	added := Diff(nil, current, at)
	if len(added) != 1 || added[0].Type != FeatureChanged || added[0].Before != nil || added[0].After == nil {
		t.Fatalf("added events = %#v", added)
	}
	removed := Diff(current, State{}, at)
	if len(removed) != 1 || removed[0].Type != FeatureChanged || removed[0].Before == nil || removed[0].After != nil {
		t.Fatalf("removed events = %#v", removed)
	}
}

func TestStreamEmitsBaselineThenFollowedChanges(t *testing.T) {
	initialAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	changedAt := initialAt.Add(time.Minute)
	snapshots := []*workspacesnapshot.Snapshot{
		testSnapshot("ORC-1", "develop", "active"),
		testSnapshot("ORC-1", "review", "active"),
	}
	loadCalls := 0
	load := func() (*workspacesnapshot.Snapshot, error) {
		if loadCalls >= len(snapshots) {
			return nil, errors.New("unexpected extra load")
		}
		value := snapshots[loadCalls]
		loadCalls++
		return value, nil
	}
	poll := make(chan time.Time, 1)
	poll <- changedAt
	close(poll)

	var got []Event
	err := Stream(context.Background(), load, StreamOptions{
		Follow: true,
		Now:    func() time.Time { return initialAt },
		Poll:   poll,
	}, func(event Event) error {
		got = append(got, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Type != FeatureChanged || got[1].Type != StageChanged {
		t.Fatalf("events = %#v", got)
	}
	if got[0].At != initialAt || got[1].At != changedAt || loadCalls != 2 {
		t.Fatalf("timing/calls = %#v, calls %d", got, loadCalls)
	}
}

func testSnapshot(ticket, stageName, status string) *workspacesnapshot.Snapshot {
	featureDir := filepath.Join("workspace", "features", ticket)
	return &workspacesnapshot.Snapshot{Items: []*workspacesnapshot.WorkItem{{
		Feature: &featurelist.Feature{
			State: &state.State{
				Ticket: ticket, Slug: ticket, Status: status,
				Stage: state.Stage{Name: stageName, Worker: "default:bob"},
			},
			FeatureDir: featureDir, Workflow: "default:standard", Stage: stageName,
			StageLabel: stageName, WorkerID: "default:bob", WorkerName: "Bob",
		},
	}}}
}

func onlyItem(state State) ItemState {
	for _, item := range state {
		return item
	}
	return ItemState{}
}
