// Package events projects immutable workspace snapshots into a deterministic
// stream of changes for CLI consumers.
package events

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/cengebretson/orc/internal/workspacesnapshot"
)

type Type string

const (
	FeatureChanged   Type = "feature.changed"
	AttentionChanged Type = "attention.changed"
	SessionChanged   Type = "session.changed"
	StageChanged     Type = "stage.changed"
)

const defaultInterval = 5 * time.Second

type FeatureState struct {
	Ticket            string   `json:"ticket,omitempty"`
	Slug              string   `json:"slug,omitempty"`
	FeatureDir        string   `json:"feature_dir"`
	Status            string   `json:"status,omitempty"`
	Workflow          string   `json:"workflow,omitempty"`
	Archived          bool     `json:"archived"`
	HasIssues         bool     `json:"has_issues"`
	LoadError         string   `json:"load_error,omitempty"`
	RequiredArtifacts []string `json:"required_artifacts,omitempty"`
}

type StageState struct {
	Name       string `json:"name,omitempty"`
	Label      string `json:"label,omitempty"`
	LoopLabel  string `json:"loop_label,omitempty"`
	WorkerID   string `json:"worker_id,omitempty"`
	WorkerName string `json:"worker_name,omitempty"`
}

type AttentionState struct {
	State  string `json:"state,omitempty"`
	Source string `json:"source,omitempty"`
	Since  string `json:"since,omitempty"`
}

type SessionState struct {
	Running             bool   `json:"running"`
	Backend             string `json:"backend,omitempty"`
	Workspace           string `json:"workspace,omitempty"`
	Tab                 string `json:"tab,omitempty"`
	Pane                string `json:"pane,omitempty"`
	AgentID             string `json:"agent_id,omitempty"`
	AgentInstance       string `json:"agent_instance,omitempty"`
	Engine              string `json:"engine,omitempty"`
	ProviderSessionID   string `json:"provider_session_id,omitempty"`
	ObservedSessionID   string `json:"observed_session_id,omitempty"`
	Correlation         string `json:"correlation,omitempty"`
	Model               string `json:"model,omitempty"`
	Effort              string `json:"effort,omitempty"`
	ProviderState       string `json:"provider_state,omitempty"`
	Lifecycle           string `json:"lifecycle,omitempty"`
	LifecycleSource     string `json:"lifecycle_source,omitempty"`
	ObservedLifecycle   string `json:"observed_lifecycle,omitempty"`
	ObservationSource   string `json:"observation_source,omitempty"`
	ObservationSince    string `json:"observation_since,omitempty"`
	Reconciliation      string `json:"reconciliation,omitempty"`
	StateChangeSequence uint64 `json:"state_change_sequence,omitempty"`
	ContextUsed         uint64 `json:"context_used,omitempty"`
	ContextLimit        uint64 `json:"context_limit,omitempty"`
	LastActive          string `json:"last_active,omitempty"`
}

type ItemState struct {
	Feature   FeatureState   `json:"feature"`
	Stage     StageState     `json:"stage"`
	Attention AttentionState `json:"attention"`
	Session   SessionState   `json:"session"`
}

type Event struct {
	Type       Type       `json:"type"`
	At         time.Time  `json:"at"`
	Ticket     string     `json:"ticket,omitempty"`
	FeatureDir string     `json:"feature_dir,omitempty"`
	Before     *ItemState `json:"before"`
	After      *ItemState `json:"after"`
}

// State is the normalized, comparable event projection of one workspace
// snapshot. Keys prefer the durable ticket ID so an archive move remains one
// feature change rather than a removal followed by an addition.
type State map[string]ItemState

func Capture(snapshot *workspacesnapshot.Snapshot) State {
	state := make(State)
	if snapshot == nil {
		return state
	}
	for _, item := range snapshot.Items {
		if item == nil || item.Feature == nil {
			continue
		}
		feature := item.Feature
		featureDir := filepath.Clean(feature.FeatureDir)
		projected := ItemState{
			Feature: FeatureState{
				FeatureDir:        featureDir,
				Workflow:          feature.Workflow,
				Archived:          feature.Archived,
				HasIssues:         feature.HasIssues,
				RequiredArtifacts: append([]string(nil), feature.RequiredArtifacts...),
			},
			Stage: StageState{
				Name:       feature.Stage,
				Label:      feature.StageLabel,
				LoopLabel:  feature.StageLoopLabel,
				WorkerID:   feature.WorkerID,
				WorkerName: feature.WorkerName,
			},
			Attention: AttentionState{
				State:  firstNonEmpty(item.Attention, feature.Attention),
				Source: item.AttentionSource,
			},
			Session: SessionState{
				Running:             feature.TmuxLive,
				Engine:              feature.Engine,
				Lifecycle:           item.Lifecycle,
				LifecycleSource:     item.LifecycleSource,
				ObservedLifecycle:   item.ObservedLifecycle,
				ObservationSource:   item.ObservationSource,
				Reconciliation:      item.Reconciliation,
				StateChangeSequence: item.StateChangeSeq,
			},
		}
		if !item.AttentionSince.IsZero() {
			projected.Attention.Since = item.AttentionSince.UTC().Format(time.RFC3339Nano)
		}
		if !item.ObservationSince.IsZero() {
			projected.Session.ObservationSince = item.ObservationSince.UTC().Format(time.RFC3339Nano)
		}
		if feature.LoadError != nil {
			projected.Feature.LoadError = feature.LoadError.Error()
		}
		if feature.State != nil {
			projected.Feature.Ticket = feature.State.Ticket
			projected.Feature.Slug = feature.State.Slug
			projected.Feature.Status = feature.State.Status
			if target, ok := feature.State.Runtime.MuxTarget(feature.State.Stage.Name); ok {
				projected.Session.Backend = target.Backend
				projected.Session.Workspace = target.Workspace
				projected.Session.Tab = target.Tab
				projected.Session.Pane = target.Pane
			}
			if agent := feature.State.Runtime.Agent; agent != nil {
				projected.Session.AgentID = agent.ID
				projected.Session.AgentInstance = agent.Instance
				projected.Session.Engine = firstNonEmpty(agent.Engine, projected.Session.Engine)
				projected.Session.ProviderSessionID = agent.ProviderSessionID
			}
		}
		if item.HasTelemetry {
			live := item.Live
			projected.Session.Engine = firstNonEmpty(live.Engine, projected.Session.Engine)
			projected.Session.ProviderSessionID = live.ProviderSessionID
			projected.Session.ObservedSessionID = live.ObservedSessionID
			projected.Session.Correlation = live.Correlation
			projected.Session.Model = live.Model
			projected.Session.Effort = live.Effort
			projected.Session.ProviderState = live.State
			projected.Session.ContextUsed = live.ContextUsed
			projected.Session.ContextLimit = live.ContextLimit
			projected.Session.Pane = firstNonEmpty(live.PaneTarget, projected.Session.Pane)
			if !live.LastActive.IsZero() {
				projected.Session.LastActive = live.LastActive.UTC().Format(time.RFC3339Nano)
			}
		}
		state[itemKey(projected.Feature)] = projected
	}
	return state
}

func Diff(before, after State, at time.Time) []Event {
	keys := make([]string, 0, len(before)+len(after))
	seen := make(map[string]bool, len(before)+len(after))
	for key := range before {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range after {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	var result []Event
	for _, key := range keys {
		previous, hadPrevious := before[key]
		current, hasCurrent := after[key]
		if !hadPrevious || !hasCurrent {
			result = append(result, newEvent(FeatureChanged, previous, hadPrevious, current, hasCurrent, at))
			continue
		}
		if !reflect.DeepEqual(previous.Feature, current.Feature) {
			result = append(result, newEvent(FeatureChanged, previous, true, current, true, at))
		}
		if previous.Attention != current.Attention {
			result = append(result, newEvent(AttentionChanged, previous, true, current, true, at))
		}
		if previous.Session != current.Session {
			result = append(result, newEvent(SessionChanged, previous, true, current, true, at))
		}
		if previous.Stage != current.Stage {
			result = append(result, newEvent(StageChanged, previous, true, current, true, at))
		}
	}
	return result
}

type StreamOptions struct {
	Follow   bool
	Interval time.Duration
	Now      func() time.Time
	Poll     <-chan time.Time
}

// Stream emits the current workspace as feature.changed baseline events, then
// emits deterministic diffs for each refresh when Follow is enabled.
func Stream(
	ctx context.Context,
	load func() (*workspacesnapshot.Snapshot, error),
	opts StreamOptions,
	emit func(Event) error,
) error {
	currentSnapshot, err := load()
	if err != nil {
		return fmt.Errorf("loading initial workspace snapshot: %w", err)
	}
	current := Capture(currentSnapshot)
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	if err := emitAll(Diff(nil, current, now()), emit); err != nil {
		return err
	}
	if !opts.Follow {
		return nil
	}

	poll := opts.Poll
	var ticker *time.Ticker
	if poll == nil {
		interval := opts.Interval
		if interval <= 0 {
			interval = defaultInterval
		}
		ticker = time.NewTicker(interval)
		defer ticker.Stop()
		poll = ticker.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case at, ok := <-poll:
			if !ok {
				return nil
			}
			nextSnapshot, loadErr := load()
			if loadErr != nil {
				return fmt.Errorf("refreshing workspace snapshot: %w", loadErr)
			}
			next := Capture(nextSnapshot)
			if at.IsZero() {
				at = now()
			}
			if err := emitAll(Diff(current, next, at), emit); err != nil {
				return err
			}
			current = next
		}
	}
}

func newEvent(eventType Type, before ItemState, hadBefore bool, after ItemState, hasAfter bool, at time.Time) Event {
	event := Event{Type: eventType, At: at}
	if hadBefore {
		value := before
		event.Before = &value
	}
	if hasAfter {
		value := after
		event.After = &value
	}
	item := event.After
	if item == nil {
		item = event.Before
	}
	if item != nil {
		event.Ticket = item.Feature.Ticket
		event.FeatureDir = item.Feature.FeatureDir
	}
	return event
}

func emitAll(events []Event, emit func(Event) error) error {
	for _, event := range events {
		if err := emit(event); err != nil {
			return fmt.Errorf("emitting %s: %w", event.Type, err)
		}
	}
	return nil
}

func itemKey(feature FeatureState) string {
	if feature.Ticket != "" {
		return "ticket:" + feature.Ticket
	}
	return "path:" + feature.FeatureDir
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
