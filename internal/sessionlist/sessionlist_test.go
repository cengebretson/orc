package sessionlist

import (
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/tmux"
)

func TestCollectClassifiesManagedOrphanedAndUnmanaged(t *testing.T) {
	features := []*featurelist.Feature{{
		State: &state.State{
			Ticket: "ORC-1", Status: "active", Stage: state.Stage{Name: "develop"},
			Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}},
		},
		FeatureDir: "/work/orc-1", WorkerID: "default:dev", Engine: "codex",
	}}
	panes := []tmux.Pane{
		{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/orc-1", Agent: true, Ticket: "ORC-1"},
		{ID: "%2", Session: "old", Window: "review", CWD: "/work/old", Agent: true, Ticket: "ORC-OLD", Engine: "claude"},
	}
	now := time.Now()
	live := []telemetry.Live{
		{Engine: "codex", ProviderSessionID: "codex-1", CWD: "/work/orc-1", LastActive: now},
		{Engine: "claude", ProviderSessionID: "claude-old", CWD: "/work/old", LastActive: now.Add(-time.Minute)},
		{Engine: "codex", ProviderSessionID: "personal", CWD: "/work/personal", LastActive: now.Add(-time.Hour)},
	}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: features, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3: %#v", len(got), got)
	}
	if got[0].Kind != KindManaged || got[0].Ticket != "ORC-1" || got[0].Target.Pane != "%1" {
		t.Fatalf("managed = %#v", got[0])
	}
	if got[0].Live == nil || !got[0].Live.Managed || got[0].Live.Ticket != "ORC-1" {
		t.Fatalf("managed live = %#v", got[0].Live)
	}
	if got[1].Kind != KindOrphaned || got[1].Ticket != "ORC-OLD" || got[1].Live.ProviderSessionID != "claude-old" {
		t.Fatalf("orphaned = %#v", got[1])
	}
	if got[2].Kind != KindUnmanaged || got[2].Live.ProviderSessionID != "personal" {
		t.Fatalf("unmanaged = %#v", got[2])
	}
}

func TestCollectOmitsUnmanagedByDefault(t *testing.T) {
	got, err := Collect("/work", Options{
		Features:  []*featurelist.Feature{},
		Panes:     []tmux.Pane{},
		Telemetry: []telemetry.Live{{Engine: "codex", ProviderSessionID: "personal"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v, want empty", got)
	}
}

func TestCollectPrefersDurableTmuxTargetOverStaleMetadata(t *testing.T) {
	feature := &featurelist.Feature{
		State: &state.State{
			Ticket: "ORC-1", Stage: state.Stage{Name: "develop"},
			Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "current"}},
		},
		FeatureDir: "/work/current",
	}
	panes := []tmux.Pane{
		{ID: "%1", Session: "stale", Window: "develop", Agent: true, Ticket: "ORC-1"},
		{ID: "%2", Session: "current", Window: "develop", Agent: true},
	}
	got, err := Collect("/work", Options{Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: []telemetry.Live{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != KindManaged || got[0].Target.Pane != "%2" || got[1].Kind != KindOrphaned || got[1].Target.Pane != "%1" {
		t.Fatalf("sessions = %#v", got)
	}
}

func TestCollectPrefersExactProviderIDOverNewerCWDMatch(t *testing.T) {
	feature := &featurelist.Feature{
		State:      &state.State{Ticket: "ORC-1", Stage: state.Stage{Name: "develop"}, Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}}},
		FeatureDir: "/work/shared", Engine: "codex",
	}
	now := time.Now()
	panes := []tmux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", Agent: true, ProviderEngine: "codex", ProviderSessionID: "exact"}}
	live := []telemetry.Live{
		{Engine: "codex", ProviderSessionID: "wrong-newer", CWD: "/work/shared", Model: "wrong", LastActive: now},
		{Engine: "codex", ProviderSessionID: "exact", CWD: "/work/shared", Model: "right", LastActive: now.Add(-time.Minute)},
	}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Live == nil || got[0].Live.ProviderSessionID != "exact" || got[0].Live.Model != "right" || got[0].Live.Correlation != "provider_id" {
		t.Fatalf("sessions = %#v", got)
	}
	if got[1].Kind != KindUnmanaged || got[1].Live.ProviderSessionID != "wrong-newer" {
		t.Fatalf("unmanaged = %#v", got[1])
	}
}

func TestCollectMergesExactResumeIdentityWithLivePanePID(t *testing.T) {
	feature := &featurelist.Feature{
		State:      &state.State{Ticket: "ORC-1", Stage: state.Stage{Name: "develop"}, Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}}},
		FeatureDir: "/work/shared", Engine: "claude",
	}
	now := time.Now()
	panes := []tmux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", PID: 4242, Agent: true, ProviderEngine: "claude", ProviderSessionID: "original"}}
	live := []telemetry.Live{
		{Engine: "claude", ProviderSessionID: "current", CWD: "/work/shared", PID: 4242, State: "active", LastActive: now},
		{Engine: "claude", ProviderSessionID: "original", CWD: "/work/shared", Model: "opus", ContextUsed: 1200, State: "idle", LastActive: now.Add(-time.Minute)},
	}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Live == nil {
		t.Fatalf("sessions = %#v", got)
	}
	merged := got[0].Live
	if merged.ProviderSessionID != "original" || merged.ObservedSessionID != "current" || merged.Correlation != "provider_id+pid" || merged.PID != 4242 || merged.State != "active" || merged.Model != "opus" || merged.ContextUsed != 1200 {
		t.Fatalf("merged live = %#v", merged)
	}
}

func TestCollectDoesNotFallbackToCWDWhenExplicitProviderIDIsMissing(t *testing.T) {
	feature := &featurelist.Feature{
		State:      &state.State{Ticket: "ORC-1", Stage: state.Stage{Name: "develop"}, Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}}},
		FeatureDir: "/work/shared", Engine: "codex",
	}
	panes := []tmux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", Agent: true, ProviderEngine: "codex", ProviderSessionID: "missing"}}
	live := []telemetry.Live{{Engine: "codex", ProviderSessionID: "wrong", CWD: "/work/shared"}}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Live != nil || got[1].Kind != KindUnmanaged || got[1].Live.ProviderSessionID != "wrong" {
		t.Fatalf("sessions = %#v", got)
	}
}

func TestCollectPrefersExactPanePIDWithoutProviderMarker(t *testing.T) {
	feature := &featurelist.Feature{
		State:      &state.State{Ticket: "ORC-1", Stage: state.Stage{Name: "develop"}, Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}}},
		FeatureDir: "/work/shared", Engine: "claude",
	}
	now := time.Now()
	panes := []tmux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", PID: 4242, Agent: true, ProviderEngine: "claude"}}
	live := []telemetry.Live{
		{Engine: "claude", ProviderSessionID: "wrong-newer", CWD: "/work/shared", PID: 9999, LastActive: now},
		{Engine: "claude", ProviderSessionID: "exact-pid", CWD: "/work/shared", PID: 4242, LastActive: now.Add(-time.Minute)},
	}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Live == nil || got[0].Live.ProviderSessionID != "exact-pid" || got[0].Live.Correlation != "pid" || got[1].Live.ProviderSessionID != "wrong-newer" {
		t.Fatalf("sessions = %#v", got)
	}
}

func TestCollectOmitsAmbiguousExplicitProviderIdentity(t *testing.T) {
	feature := &featurelist.Feature{
		State:      &state.State{Ticket: "ORC-1", Stage: state.Stage{Name: "develop"}, Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}}},
		FeatureDir: "/work/shared", Engine: "codex",
	}
	panes := []tmux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", Agent: true, ProviderEngine: "codex", ProviderSessionID: "duplicate"}}
	live := []telemetry.Live{
		{Engine: "codex", ProviderSessionID: "duplicate", CWD: "/work/shared"},
		{Engine: "codex", ProviderSessionID: "duplicate", CWD: "/work/shared"},
	}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Live != nil || got[1].Kind != KindUnmanaged || got[2].Kind != KindUnmanaged {
		t.Fatalf("sessions = %#v", got)
	}
}
