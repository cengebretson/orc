// Package sessionlist joins Orc's durable feature state with optional tmux and
// provider telemetry. Durable state always wins when the sources disagree.
package sessionlist

import (
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/tmux"
)

const (
	KindManaged   = "managed"
	KindOrphaned  = "orphaned"
	KindUnmanaged = "unmanaged"
)

type Target struct {
	Session string `json:"session"`
	Window  string `json:"window"`
	Pane    string `json:"pane,omitempty"`
}

type Session struct {
	Kind       string          `json:"kind"`
	Running    bool            `json:"running"`
	Ticket     string          `json:"ticket,omitempty"`
	Stage      string          `json:"stage,omitempty"`
	Status     string          `json:"status,omitempty"`
	Worker     string          `json:"worker,omitempty"`
	Engine     string          `json:"engine,omitempty"`
	FeatureDir string          `json:"feature_dir,omitempty"`
	Attention  string          `json:"attention,omitempty"`
	Target     *Target         `json:"target,omitempty"`
	Live       *telemetry.Live `json:"live,omitempty"`
}

type Options struct {
	IncludeUnmanaged bool
	Home             string
	Features         []*featurelist.Feature
	Panes            []tmux.Pane
	Telemetry        []telemetry.Live
	LoadFeatures     func(string, featurelist.Options) ([]*featurelist.Feature, error)
	ListPanes        func() ([]tmux.Pane, error)
	Discover         func(string) ([]telemetry.Live, error)
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
		list := opts.ListPanes
		if list == nil {
			list = tmux.ListPanesDetailed
		}
		var err error
		panes, err = list()
		if err != nil {
			return nil, err
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
		pane := matchFeaturePane(feature, panes, usedPanes)
		if pane == nil && (s.Runtime.Tmux == nil || s.Runtime.Tmux.Session == "") {
			continue
		}
		entry := Session{
			Kind: KindManaged, Ticket: s.Ticket, Stage: s.Stage.Name, Status: s.Status,
			Worker: feature.WorkerID, Engine: feature.Engine, FeatureDir: feature.FeatureDir,
		}
		if s.Runtime.Tmux != nil {
			entry.Target = &Target{Session: s.Runtime.Tmux.Session, Window: s.Stage.Name, Pane: s.Runtime.Tmux.Pane}
		}
		if pane != nil {
			entry.Running = true
			usedPanes[pane.ID] = true
			entry.Target = &Target{Session: pane.Session, Window: pane.Window, Pane: pane.ID}
			entry.Attention = pane.Attention
			if entry.Engine == "" {
				entry.Engine = pane.ProviderEngine
				if entry.Engine == "" {
					entry.Engine = pane.Engine
				}
			}
		}
		if pane != nil {
			if value, indices, ok := matchTelemetry(entry.Engine, feature.FeatureDir, pane, liveSessions, usedLive); ok {
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
			Engine: pane.Engine, FeatureDir: pane.FeatureDir, Attention: pane.Attention,
			Target: &Target{Session: pane.Session, Window: pane.Window, Pane: pane.ID},
		}
		if entry.Engine == "" {
			entry.Engine = pane.ProviderEngine
		}
		if value, indices, ok := matchTelemetry(entry.Engine, pane.CWD, &pane, liveSessions, usedLive); ok {
			markTelemetryUsed(usedLive, indices)
			value.PaneTarget = pane.ID
			entry.Live = &value
		}
		out = append(out, entry)
	}

	if opts.IncludeUnmanaged {
		for i := range liveSessions {
			if usedLive[i] {
				continue
			}
			value := liveSessions[i]
			out = append(out, Session{Kind: KindUnmanaged, Engine: value.Engine, Live: &value})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if kindRank(out[i].Kind) != kindRank(out[j].Kind) {
			return kindRank(out[i].Kind) < kindRank(out[j].Kind)
		}
		if out[i].Ticket != out[j].Ticket {
			return out[i].Ticket < out[j].Ticket
		}
		return lastActive(out[i]).After(lastActive(out[j]))
	})
	return out, nil
}

func matchFeaturePane(feature *featurelist.Feature, panes []tmux.Pane, used map[string]bool) *tmux.Pane {
	s := feature.State
	if s.Runtime.Tmux != nil && s.Runtime.Tmux.Pane != "" {
		for i := range panes {
			if !used[panes[i].ID] && panes[i].ID == s.Runtime.Tmux.Pane {
				return &panes[i]
			}
		}
	}
	if s.Runtime.Tmux != nil {
		for i := range panes {
			if !used[panes[i].ID] && panes[i].Session == s.Runtime.Tmux.Session && panes[i].Window == s.Stage.Name {
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

func matchTelemetry(engine, cwd string, pane *tmux.Pane, sessions []telemetry.Live, used map[int]bool) (telemetry.Live, []int, bool) {
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

	best := -1
	for i := range sessions {
		if used[i] || !engineMatches(providerEngine, sessions[i].Engine) {
			continue
		}
		if cwd != "" && !samePath(cwd, sessions[i].CWD) {
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

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	a, errA := filepath.Abs(left)
	b, errB := filepath.Abs(right)
	return errA == nil && errB == nil && filepath.Clean(a) == filepath.Clean(b)
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
