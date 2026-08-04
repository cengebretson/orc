package sessionlist

import (
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/gitmeta"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
)

func TestCollectClassifiesManagedOrphanedAndUnmanaged(t *testing.T) {
	features := []*featurelist.Feature{{
		State: &state.State{
			Ticket: "ORC-1", Status: "active", Stage: state.Stage{Name: "develop"},
			Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}},
		},
		FeatureDir: "/work/orc-1", WorkerID: "default:dev", Engine: "codex",
	}}
	panes := []mux.Pane{
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

func TestCollectReconcilesRecordedAgentIdentity(t *testing.T) {
	base := func() *featurelist.Feature {
		return &featurelist.Feature{
			State: &state.State{
				Ticket: "ORC-6", Stage: state.Stage{Name: "develop"},
				Runtime: state.Runtime{
					Mux:   &state.MuxRuntime{Backend: "tmux", Workspace: "orc-6", Tab: "develop", Pane: "%6"},
					Agent: &state.AgentRuntime{ID: "agent-6", Instance: "instance-6", Engine: "codex", ProviderSessionID: "provider-6"},
				},
			},
			FeatureDir: "/work/orc-6", Engine: "codex",
		}
	}
	tests := []struct {
		name  string
		panes []mux.Pane
		live  []telemetry.Live
		want  string
	}{
		{name: "live", panes: []mux.Pane{{Backend: "tmux", ID: "%6", Session: "orc-6", Window: "develop", Agent: true, AgentID: "agent-6", AgentInstance: "instance-6"}}, want: ReconciliationLive},
		{name: "resumable", live: []telemetry.Live{{Engine: "codex", ProviderSessionID: "provider-6"}}, want: ReconciliationResumable},
		{name: "replaced", panes: []mux.Pane{{Backend: "tmux", ID: "%6", Session: "orc-6", Window: "develop", Agent: true, AgentID: "agent-6", AgentInstance: "instance-new"}}, want: ReconciliationReplaced},
		{name: "orphaned", want: ReconciliationOrphaned},
		{name: "unknown", panes: []mux.Pane{{Backend: "tmux", ID: "%6", Session: "orc-6", Window: "develop", Agent: true}}, want: ReconciliationUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Collect("/work", Options{Features: []*featurelist.Feature{base()}, Panes: tt.panes, Telemetry: tt.live})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].Reconciliation != tt.want {
				t.Fatalf("sessions = %#v, want reconciliation %s", got, tt.want)
			}
			if tt.want == ReconciliationReplaced && got[0].Lifecycle != "" {
				t.Fatalf("replacement leaked pane lifecycle: %#v", got[0])
			}
		})
	}
}

func TestCollectKeepsFallbackSeparateFromAuthoritativeLifecycle(t *testing.T) {
	feature := &featurelist.Feature{
		State: &state.State{
			Ticket: "ORC-6", Stage: state.Stage{Name: "develop"},
			Runtime: state.Runtime{
				Mux:   &state.MuxRuntime{Backend: "tmux", Workspace: "orc-6", Tab: "develop", Pane: "%6"},
				Agent: &state.AgentRuntime{ID: "agent-6", Instance: "instance-6", Engine: "codex"},
			},
		},
		FeatureDir: "/work/orc-6", Engine: "codex",
	}
	pane := mux.Pane{
		Backend: "tmux", ID: "%6", Session: "orc-6", Window: "develop", Agent: true,
		AgentID: "agent-6", AgentInstance: "instance-6", Lifecycle: "unknown", LifecycleSource: "launch",
		ObservedLifecycle: "blocked", ObservationSource: "screen", Attention: "blocked", AttentionSource: "screen",
	}
	got, err := Collect("/work", Options{Features: []*featurelist.Feature{feature}, Panes: []mux.Pane{pane}, Telemetry: []telemetry.Live{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Lifecycle != "unknown" || got[0].LifecycleSource != "launch" || got[0].ObservedLifecycle != "blocked" || got[0].ObservationSource != "screen" || got[0].AttentionSource != "screen" {
		t.Fatalf("session = %#v", got)
	}
}

func TestManagedTelemetryFromSessionsUsesManagedFeatureDirectory(t *testing.T) {
	live := telemetry.Live{Engine: "codex", ContextUsed: 70, ContextLimit: 100}
	got := managedTelemetryFromSessions([]Session{
		{Kind: KindManaged, FeatureDir: "/work/one/../one", Live: &live},
		{Kind: KindOrphaned, FeatureDir: "/work/orphan", Live: &live},
		{Kind: KindManaged, FeatureDir: "/work/no-live"},
	})
	value, ok := got["/work/one"]
	if !ok || value.ContextUsed != 70 || value.ContextLimit != 100 {
		t.Fatalf("managed telemetry = %#v, want normalized managed feature overlay", got)
	}
	if _, ok := got["/work/orphan"]; ok {
		t.Fatalf("orphaned telemetry should not be projected by feature: %#v", got)
	}
}

func TestCollectUsesExactLiveWorkerOverrideAndEngine(t *testing.T) {
	feature := &featurelist.Feature{
		State: &state.State{
			Ticket: "ORC-9", Stage: state.Stage{Name: "intake"},
			Runtime: state.Runtime{Mux: &state.MuxRuntime{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"}},
		},
		FeatureDir: "/work/orc-9", WorkerID: "default:fred", Engine: "claude",
	}
	panes := []mux.Pane{{
		Backend: "herdr", ID: "w9:p1", Session: "w9", Window: "w9:t1", Agent: true,
		Ticket: "ORC-9", Worker: "default:bob", ProviderEngine: "codex", Lifecycle: "idle",
	}}

	got, err := Collect("/work", Options{Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: []telemetry.Live{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Worker != "default:bob" || got[0].Engine != "codex" || got[0].Lifecycle != "idle" {
		t.Fatalf("managed live identity = %#v", got)
	}
}

func TestCollectUsesDurableManagedRepositoriesWithoutGit(t *testing.T) {
	feature := &featurelist.Feature{
		State: &state.State{
			Ticket: "ORC-1", Stage: state.Stage{Name: "develop"},
			Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1"}},
			Repos: map[string]state.Repo{
				"los-app": {Main: "/repos/los-app", Worktree: "/worktrees/los-app-feature", Branch: "feature/orc-1"},
				"qa":      {Main: "/repos/qa", Worktree: "/repos/qa", Branch: "main"},
			},
		},
		FeatureDir: "/features/orc-1",
	}
	gitCalls := 0
	got, err := Collect("/work", Options{
		Features: []*featurelist.Feature{feature}, Panes: []mux.Pane{}, Telemetry: []telemetry.Live{},
		ResolveGit: func(string) (gitmeta.Metadata, bool) {
			gitCalls++
			return gitmeta.Metadata{}, false
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gitCalls != 0 {
		t.Fatalf("managed session performed %d Git lookups", gitCalls)
	}
	if len(got) != 1 || len(got[0].Repositories) != 2 {
		t.Fatalf("sessions = %#v", got)
	}
	if got[0].Repositories[0] != (Repository{Name: "los-app", Branch: "feature/orc-1", Worktree: "los-app-feature"}) {
		t.Fatalf("first repository = %#v", got[0].Repositories[0])
	}
	if got[0].Repositories[1] != (Repository{Name: "qa", Branch: "main", Worktree: "."}) {
		t.Fatalf("second repository = %#v", got[0].Repositories[1])
	}
}

func TestCollectResolvesGitOnlyForOrphanedAndUnmanagedSessions(t *testing.T) {
	panes := []mux.Pane{{ID: "%1", Session: "old", Window: "review", CWD: "/work/orc", Agent: true, Ticket: "ORC-OLD"}}
	live := []telemetry.Live{
		{Engine: "codex", ProviderSessionID: "orphan", CWD: "/work/orc"},
		{Engine: "claude", ProviderSessionID: "personal", CWD: "/work/personal"},
	}
	lookups := map[string]int{}
	resolve := func(cwd string) (gitmeta.Metadata, bool) {
		lookups[cwd]++
		if cwd == "/work/orc" {
			return gitmeta.Metadata{Repository: "orc", Branch: "feature/grouping", Worktree: "orc-wt"}, true
		}
		return gitmeta.Metadata{Repository: "personal", Branch: "main", Worktree: "."}, true
	}
	got, err := Collect("/work", Options{
		IncludeUnmanaged: true, Features: []*featurelist.Feature{}, Panes: panes, Telemetry: live, ResolveGit: resolve,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != KindOrphaned || got[1].Kind != KindUnmanaged {
		t.Fatalf("sessions = %#v", got)
	}
	if got[0].Repositories[0].Name != "orc" || got[1].Repositories[0].Name != "personal" {
		t.Fatalf("repositories = %#v / %#v", got[0].Repositories, got[1].Repositories)
	}
	if lookups["/work/orc"] != 1 || lookups["/work/personal"] != 1 {
		t.Fatalf("lookups = %#v", lookups)
	}
}

func TestCollectOmitsUnmanagedByDefault(t *testing.T) {
	got, err := Collect("/work", Options{
		Features:  []*featurelist.Feature{},
		Panes:     []mux.Pane{},
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
	panes := []mux.Pane{
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
	panes := []mux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", Agent: true, ProviderEngine: "codex", ProviderSessionID: "exact"}}
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
	panes := []mux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", PID: 4242, Agent: true, ProviderEngine: "claude", ProviderSessionID: "original"}}
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
	panes := []mux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", Agent: true, ProviderEngine: "codex", ProviderSessionID: "missing"}}
	live := []telemetry.Live{{Engine: "codex", ProviderSessionID: "wrong", CWD: "/work/shared"}}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: []*featurelist.Feature{feature}, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Live != nil || got[1].Kind != KindUnmanaged || got[1].Live.ProviderSessionID != "wrong" {
		t.Fatalf("sessions = %#v", got)
	}
}

func TestCollectDoesNotGuessTelemetryWhenPaneCWDIsEmpty(t *testing.T) {
	panes := []mux.Pane{{ID: "%1", Session: "orphan", Window: "shell", Agent: true, ProviderEngine: "codex"}}
	live := []telemetry.Live{{Engine: "codex", ProviderSessionID: "unrelated", CWD: "/work/elsewhere", LastActive: time.Now()}}

	got, err := Collect("/work", Options{IncludeUnmanaged: true, Features: []*featurelist.Feature{}, Panes: panes, Telemetry: live})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Kind != KindOrphaned || got[0].Live != nil || got[1].Kind != KindUnmanaged || got[1].Live.ProviderSessionID != "unrelated" {
		t.Fatalf("sessions = %#v", got)
	}
}

func TestCollectPrefersExactPanePIDWithoutProviderMarker(t *testing.T) {
	feature := &featurelist.Feature{
		State:      &state.State{Ticket: "ORC-1", Stage: state.Stage{Name: "develop"}, Runtime: state.Runtime{Tmux: &state.TmuxRuntime{Session: "orc-1", Pane: "%1"}}},
		FeatureDir: "/work/shared", Engine: "claude",
	}
	now := time.Now()
	panes := []mux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", PID: 4242, Agent: true, ProviderEngine: "claude"}}
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
	panes := []mux.Pane{{ID: "%1", Session: "orc-1", Window: "develop", CWD: "/work/shared", Agent: true, ProviderEngine: "codex", ProviderSessionID: "duplicate"}}
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
