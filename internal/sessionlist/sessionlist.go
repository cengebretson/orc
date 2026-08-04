// Package sessionlist joins Orc's durable feature state with optional
// multiplexer and provider telemetry. Durable state always wins when the
// sources disagree.
package sessionlist

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/gitmeta"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/tmux"
)

const (
	KindManaged   = "managed"
	KindOrphaned  = "orphaned"
	KindUnmanaged = "unmanaged"
)

const (
	ReconciliationLive      = "live"
	ReconciliationResumable = "resumable"
	ReconciliationReplaced  = "replaced"
	ReconciliationOrphaned  = "orphaned"
	ReconciliationUnknown   = "unknown"
)

type Target struct {
	Backend string `json:"backend,omitempty"`
	Session string `json:"session"`
	Window  string `json:"window"`
	Pane    string `json:"pane,omitempty"`
}

type Repository struct {
	Name     string `json:"name"`
	Branch   string `json:"branch,omitempty"`
	Worktree string `json:"worktree,omitempty"`
}

type Session struct {
	Kind              string          `json:"kind"`
	Running           bool            `json:"running"`
	Ticket            string          `json:"ticket,omitempty"`
	Stage             string          `json:"stage,omitempty"`
	Status            string          `json:"status,omitempty"`
	Worker            string          `json:"worker,omitempty"`
	Engine            string          `json:"engine,omitempty"`
	AgentID           string          `json:"agent_id,omitempty"`
	AgentInstance     string          `json:"agent_instance,omitempty"`
	FeatureDir        string          `json:"feature_dir,omitempty"`
	Attention         string          `json:"attention,omitempty"`
	AttentionSource   string          `json:"attention_source,omitempty"`
	AttentionSince    int64           `json:"attention_since,omitempty"`
	Lifecycle         string          `json:"lifecycle,omitempty"`
	LifecycleSource   string          `json:"lifecycle_source,omitempty"`
	ObservedLifecycle string          `json:"observed_lifecycle,omitempty"`
	ObservationSource string          `json:"observation_source,omitempty"`
	ObservationSince  int64           `json:"observation_since,omitempty"`
	DisplayTitle      string          `json:"display_title,omitempty"`
	Reconciliation    string          `json:"reconciliation,omitempty"`
	LifecycleSince    int64           `json:"lifecycle_since,omitempty"`
	StateChangeSeq    uint64          `json:"state_change_seq,omitempty"`
	Repositories      []Repository    `json:"repositories,omitempty"`
	Target            *Target         `json:"target,omitempty"`
	Live              *telemetry.Live `json:"live,omitempty"`
}

// ManagedRuntime is the authoritative multiplexer observation for one managed
// feature, optionally enriched with provider telemetry.
type ManagedRuntime struct {
	Live              telemetry.Live
	HasTelemetry      bool
	Attention         string
	Lifecycle         string
	LifecycleSince    int64
	StateChangeSeq    uint64
	AttentionSource   string
	AttentionSince    int64
	LifecycleSource   string
	ObservedLifecycle string
	ObservationSource string
	ObservationSince  int64
	DisplayTitle      string
	Reconciliation    string
}

type Options struct {
	IncludeUnmanaged bool
	Home             string
	Features         []*featurelist.Feature
	Panes            []mux.Pane
	Telemetry        []telemetry.Live
	LoadFeatures     func(string, featurelist.Options) ([]*featurelist.Feature, error)
	// Mux supplies the live pane inventory. Defaults to tmux when unset; tests
	// usually bypass it entirely by passing Panes directly.
	Mux        mux.Backend
	Discover   func(string) ([]telemetry.Live, error)
	ResolveGit func(string) (gitmeta.Metadata, bool)
}

// CollectManagedRuntimeWithMux is the strict runtime projection used by live
// interfaces that must surface malformed local detection overrides.
func CollectManagedRuntimeWithMux(root string, features []*featurelist.Feature, backend mux.Backend) (map[string]ManagedRuntime, error) {
	if len(features) == 0 {
		return nil, nil
	}
	sessions, err := Collect(root, Options{Features: features, Mux: backend})
	if err != nil {
		return nil, err
	}
	result := make(map[string]ManagedRuntime)
	for _, session := range sessions {
		if session.Kind != KindManaged || session.FeatureDir == "" {
			continue
		}
		observation := ManagedRuntime{
			Attention: session.Attention, Lifecycle: session.Lifecycle,
			LifecycleSince: session.LifecycleSince, StateChangeSeq: session.StateChangeSeq,
			AttentionSource: session.AttentionSource, LifecycleSource: session.LifecycleSource,
			AttentionSince:    session.AttentionSince,
			ObservedLifecycle: session.ObservedLifecycle, ObservationSource: session.ObservationSource,
			ObservationSince: session.ObservationSince, Reconciliation: session.Reconciliation,
			DisplayTitle: session.DisplayTitle,
		}
		if session.Live != nil {
			observation.Live = *session.Live
			observation.HasTelemetry = true
		}
		result[filepath.Clean(session.FeatureDir)] = observation
	}
	return result, nil
}

func Collect(root string, opts Options) ([]Session, error) {
	features := opts.Features
	if features == nil {
		load := opts.LoadFeatures
		if load == nil {
			load = featurelist.Collect
		}
		var err error
		features, err = load(root, featurelist.Options{})
		if err != nil {
			return nil, err
		}
	}
	panes := opts.Panes
	if panes == nil {
		backend := opts.Mux
		if backend == nil {
			backend = tmux.New()
		}
		var err error
		panes, err = backend.ListPanes()
		if err != nil {
			return nil, err
		}
		if observer, ok := backend.(mux.FallbackObservationBackend); ok {
			panes, err = observer.ObserveFallback(root, panes)
			if err != nil {
				return nil, err
			}
		}
	}
	liveSessions := opts.Telemetry
	if liveSessions == nil {
		discover := opts.Discover
		if discover == nil {
			discover = telemetry.Discover
		}
		var err error
		liveSessions, err = discover(opts.Home)
		if err != nil {
			return nil, err
		}
	}

	usedPanes := make(map[string]bool)
	usedLive := make(map[int]bool)
	var out []Session
	for _, feature := range features {
		if feature.State == nil || feature.Archived || feature.LoadError != nil {
			continue
		}
		s := feature.State
		runtimeTarget, configured := s.Runtime.MuxTarget(s.Stage.Name)
		pane := matchFeaturePane(feature, panes, usedPanes)
		if pane == nil && !configured {
			continue
		}
		entry := Session{
			Kind: KindManaged, Ticket: s.Ticket, Stage: s.Stage.Name, Status: s.Status,
			Worker: feature.WorkerID, Engine: feature.Engine, FeatureDir: feature.FeatureDir,
			Repositories: durableRepositories(s.Repos),
		}
		if entry.Engine == "" && s.Runtime.Agent != nil {
			entry.Engine = s.Runtime.Agent.Engine
		}
		if s.Runtime.Agent != nil {
			entry.AgentID = s.Runtime.Agent.ID
			entry.AgentInstance = s.Runtime.Agent.Instance
		}
		entry.Reconciliation = reconcileFeature(feature, pane, liveSessions)
		if configured {
			entry.Target = &Target{Backend: runtimeTarget.Backend, Session: runtimeTarget.Workspace, Window: runtimeTarget.Tab, Pane: runtimeTarget.Pane}
		}
		usablePane := pane
		if s.Runtime.Agent != nil && entry.Reconciliation != ReconciliationLive {
			usablePane = nil
		}
		if pane != nil {
			entry.Running = true
			usedPanes[pane.ID] = true
		}
		if usablePane != nil {
			entry.Target = &Target{Backend: usablePane.Backend, Session: usablePane.Session, Window: usablePane.Window, Pane: usablePane.ID}
			entry.Attention = usablePane.Attention
			entry.AttentionSource = usablePane.AttentionSource
			entry.AttentionSince = usablePane.AttentionSince
			entry.Lifecycle = usablePane.Lifecycle
			entry.LifecycleSource = usablePane.LifecycleSource
			entry.ObservedLifecycle = usablePane.ObservedLifecycle
			entry.ObservationSource = usablePane.ObservationSource
			entry.ObservationSince = usablePane.ObservationSince
			entry.DisplayTitle = usablePane.DisplayTitle
			entry.LifecycleSince = usablePane.LifecycleSince
			entry.StateChangeSeq = usablePane.StateChangeSeq
			if usablePane.ProviderEngine != "" {
				entry.Engine = usablePane.ProviderEngine
			} else if usablePane.Engine != "" {
				entry.Engine = usablePane.Engine
			}
			if s.Stage.Worker == "" && usablePane.Worker != "" {
				entry.Worker = usablePane.Worker
			}
		}
		if entry.Reconciliation == ReconciliationResumable && s.Runtime.Agent != nil {
			index, ambiguous := uniqueTelemetryIndex(liveSessions, usedLive, func(live telemetry.Live) bool {
				return live.ProviderSessionID == s.Runtime.Agent.ProviderSessionID && engineMatches(s.Runtime.Agent.Engine, live.Engine)
			})
			if !ambiguous && index >= 0 {
				value := liveSessions[index]
				value.Managed = true
				value.Ticket = s.Ticket
				entry.Live = &value
				usedLive[index] = true
			}
		}
		if usablePane != nil {
			if value, indices, ok := matchTelemetry(entry.Engine, feature.FeatureDir, usablePane, liveSessions, usedLive); ok {
				markTelemetryUsed(usedLive, indices)
				value.Managed = true
				value.Ticket = s.Ticket
				if entry.Target != nil {
					value.PaneTarget = entry.Target.Pane
				}
				entry.Live = &value
			}
		}
		out = append(out, entry)
	}

	for _, pane := range panes {
		if usedPanes[pane.ID] || (!pane.Agent && pane.Ticket == "") {
			continue
		}
		entry := Session{
			Kind: KindOrphaned, Running: true, Ticket: pane.Ticket, Stage: pane.Stage, Worker: pane.Worker,
			Reconciliation: ReconciliationOrphaned,
			AgentID:        pane.AgentID, AgentInstance: pane.AgentInstance,
			Engine: pane.Engine, FeatureDir: pane.FeatureDir, Attention: pane.Attention, Lifecycle: pane.Lifecycle,
			AttentionSource: pane.AttentionSource, LifecycleSource: pane.LifecycleSource,
			AttentionSince:    pane.AttentionSince,
			ObservedLifecycle: pane.ObservedLifecycle, ObservationSource: pane.ObservationSource,
			ObservationSince: pane.ObservationSince,
			DisplayTitle:     pane.DisplayTitle,
			LifecycleSince:   pane.LifecycleSince, StateChangeSeq: pane.StateChangeSeq,
			Target: &Target{Backend: pane.Backend, Session: pane.Session, Window: pane.Window, Pane: pane.ID},
		}
		if entry.Engine == "" {
			entry.Engine = pane.ProviderEngine
		}
		if value, indices, ok := matchTelemetry(entry.Engine, pane.CWD, &pane, liveSessions, usedLive); ok {
			markTelemetryUsed(usedLive, indices)
			value.PaneTarget = pane.ID
			entry.Live = &value
		}
		entry.Repositories = resolvedRepository(opts.ResolveGit, firstNonEmpty(pane.CWD, pane.FeatureDir))
		out = append(out, entry)
	}

	if opts.IncludeUnmanaged {
		for i := range liveSessions {
			if usedLive[i] {
				continue
			}
			value := liveSessions[i]
			out = append(out, Session{
				Kind: KindUnmanaged, Engine: value.Engine, Live: &value,
				Repositories: resolvedRepository(opts.ResolveGit, value.CWD),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if kindRank(out[i].Kind) != kindRank(out[j].Kind) {
			return kindRank(out[i].Kind) < kindRank(out[j].Kind)
		}
		if repositoryKey(out[i]) != repositoryKey(out[j]) {
			return repositoryKey(out[i]) < repositoryKey(out[j])
		}
		if branchKey(out[i]) != branchKey(out[j]) {
			return branchKey(out[i]) < branchKey(out[j])
		}
		if out[i].Ticket != out[j].Ticket {
			return out[i].Ticket < out[j].Ticket
		}
		return lastActive(out[i]).After(lastActive(out[j]))
	})
	return out, nil
}

func reconcileFeature(feature *featurelist.Feature, pane *mux.Pane, liveSessions []telemetry.Live) string {
	if feature == nil || feature.State == nil {
		return ReconciliationUnknown
	}
	agent := feature.State.Runtime.Agent
	if agent == nil || agent.ID == "" || agent.Instance == "" {
		return ReconciliationUnknown
	}
	if pane != nil {
		target, configured := feature.State.Runtime.MuxTarget(feature.State.Stage.Name)
		if configured && (!backendMatches(target.Backend, pane.Backend) || target.Workspace != pane.Session || target.Tab != pane.Window || (target.Pane != "" && target.Pane != pane.ID)) {
			return ReconciliationUnknown
		}
		if pane.AgentID == agent.ID && pane.AgentInstance == agent.Instance {
			return ReconciliationLive
		}
		if pane.AgentID == "" || pane.AgentInstance == "" {
			return ReconciliationUnknown
		}
		return ReconciliationReplaced
	}
	if agent.ProviderSessionID == "" {
		return ReconciliationOrphaned
	}
	matches := 0
	for _, live := range liveSessions {
		if live.ProviderSessionID == agent.ProviderSessionID && engineMatches(agent.Engine, live.Engine) {
			matches++
		}
	}
	if matches == 1 {
		return ReconciliationResumable
	}
	if matches > 1 {
		return ReconciliationUnknown
	}
	return ReconciliationOrphaned
}

func durableRepositories(repos map[string]state.Repo) []Repository {
	keys := make([]string, 0, len(repos))
	for name := range repos {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	result := make([]Repository, 0, len(keys))
	for _, name := range keys {
		repo := repos[name]
		resolvedName := name
		if resolvedName == "" {
			resolvedName = filepath.Base(firstNonEmpty(repo.Main, repo.Worktree))
		}
		worktree := ""
		if repo.Worktree != "" {
			worktree = "."
			if repo.Main == "" || !gitmeta.SamePath(repo.Main, repo.Worktree) {
				worktree = filepath.Base(repo.Worktree)
			}
		}
		result = append(result, Repository{Name: resolvedName, Branch: repo.Branch, Worktree: worktree})
	}
	return result
}

func resolvedRepository(resolve func(string) (gitmeta.Metadata, bool), cwd string) []Repository {
	if resolve == nil || cwd == "" {
		return nil
	}
	metadata, ok := resolve(cwd)
	if !ok {
		return nil
	}
	return []Repository{{Name: metadata.Repository, Branch: metadata.Branch, Worktree: metadata.Worktree}}
}

func repositoryKey(session Session) string {
	if len(session.Repositories) == 0 {
		return "~"
	}
	return strings.ToLower(session.Repositories[0].Name)
}

func branchKey(session Session) string {
	if len(session.Repositories) == 0 {
		return "~"
	}
	return strings.ToLower(session.Repositories[0].Branch)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func matchFeaturePane(feature *featurelist.Feature, panes []mux.Pane, used map[string]bool) *mux.Pane {
	s := feature.State
	target, configured := s.Runtime.MuxTarget(s.Stage.Name)
	if configured && target.Pane != "" {
		for i := range panes {
			if !used[panes[i].ID] && panes[i].ID == target.Pane && panes[i].Session == target.Workspace && panes[i].Window == target.Tab && backendMatches(target.Backend, panes[i].Backend) {
				return &panes[i]
			}
		}
	}
	if configured {
		for i := range panes {
			if !used[panes[i].ID] && panes[i].Session == target.Workspace && panes[i].Window == target.Tab && backendMatches(target.Backend, panes[i].Backend) {
				return &panes[i]
			}
		}
	}
	for i := range panes {
		if used[panes[i].ID] {
			continue
		}
		if panes[i].Ticket == s.Ticket && (panes[i].Stage == "" || panes[i].Stage == s.Stage.Name) {
			return &panes[i]
		}
	}
	return nil
}

func backendMatches(target, pane string) bool {
	return target == "" || pane == "" || target == pane
}

func matchTelemetry(engine, cwd string, pane *mux.Pane, sessions []telemetry.Live, used map[int]bool) (telemetry.Live, []int, bool) {
	providerEngine := engine
	providerID := ""
	panePID := 0
	if pane != nil {
		if pane.ProviderEngine != "" {
			providerEngine = pane.ProviderEngine
		}
		providerID = pane.ProviderSessionID
		panePID = pane.PID
	}

	idIndex, idAmbiguous := uniqueTelemetryIndex(sessions, used, func(live telemetry.Live) bool {
		return providerID != "" && live.ProviderSessionID == providerID && engineMatches(providerEngine, live.Engine)
	})
	pidIndex, pidAmbiguous := uniqueTelemetryIndex(sessions, used, func(live telemetry.Live) bool {
		return panePID > 0 && live.PID == panePID && engineMatches(providerEngine, live.Engine)
	})
	if idAmbiguous || pidAmbiguous {
		return telemetry.Live{}, nil, false
	}

	if pidIndex >= 0 {
		value := sessions[pidIndex]
		indices := []int{pidIndex}
		value.Correlation = "pid"
		if idIndex >= 0 && idIndex != pidIndex {
			value = mergeProviderIdentity(value, sessions[idIndex])
			indices = append(indices, idIndex)
		}
		if idIndex >= 0 {
			value.Correlation = "provider_id+pid"
		}
		if providerID != "" {
			if value.ProviderSessionID != "" && value.ProviderSessionID != providerID {
				value.ObservedSessionID = value.ProviderSessionID
			}
			value.ProviderSessionID = providerID
		}
		if providerEngine != "" {
			value.Engine = providerEngine
		}
		return value, indices, true
	}
	if idIndex >= 0 {
		value := sessions[idIndex]
		value.Correlation = "provider_id"
		return value, []int{idIndex}, true
	}
	// An explicit provider identity is stronger than a directory heuristic. If
	// it cannot be resolved, omit the overlay rather than attaching another
	// session from the same working directory.
	if providerID != "" {
		return telemetry.Live{}, nil, false
	}
	if cwd == "" {
		return telemetry.Live{}, nil, false
	}

	best := -1
	for i := range sessions {
		if used[i] || !engineMatches(providerEngine, sessions[i].Engine) {
			continue
		}
		if !gitmeta.SamePath(cwd, sessions[i].CWD) {
			continue
		}
		if best == -1 || sessions[i].LastActive.After(sessions[best].LastActive) {
			best = i
		}
	}
	if best < 0 {
		return telemetry.Live{}, nil, false
	}
	value := sessions[best]
	value.Correlation = "cwd"
	return value, []int{best}, true
}

func uniqueTelemetryIndex(sessions []telemetry.Live, used map[int]bool, matches func(telemetry.Live) bool) (int, bool) {
	index := -1
	for i := range sessions {
		if used[i] || !matches(sessions[i]) {
			continue
		}
		if index >= 0 {
			return -1, true
		}
		index = i
	}
	return index, false
}

func mergeProviderIdentity(process, identity telemetry.Live) telemetry.Live {
	if process.Model == "" {
		process.Model = identity.Model
	}
	if process.Effort == "" {
		process.Effort = identity.Effort
	}
	if process.CWD == "" {
		process.CWD = identity.CWD
	}
	if process.ContextUsed == 0 {
		process.ContextUsed = identity.ContextUsed
	}
	if process.ContextLimit == 0 {
		process.ContextLimit = identity.ContextLimit
	}
	if identity.LastActive.After(process.LastActive) {
		process.LastActive = identity.LastActive
	}
	return process
}

func engineMatches(want, got string) bool {
	return want == "" || strings.EqualFold(want, got)
}

func markTelemetryUsed(used map[int]bool, indices []int) {
	for _, index := range indices {
		used[index] = true
	}
}

func kindRank(kind string) int {
	switch kind {
	case KindManaged:
		return 0
	case KindOrphaned:
		return 1
	case KindUnmanaged:
		return 2
	default:
		return 3
	}
}

func lastActive(session Session) time.Time {
	if session.Live == nil {
		return time.Time{}
	}
	return session.Live.LastActive
}
