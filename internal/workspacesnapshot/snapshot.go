// Package workspacesnapshot loads one immutable view of the workspace for
// presentation layers. Consumers project the snapshot into their own rows.
package workspacesnapshot

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
)

type Snapshot struct {
	Config  *config.Config
	Workers []*workers.Worker
	Items   []*WorkItem
	Health  []doctor.Check
}

// WorkItem is the presentation-neutral projection shared by Orc's Live and
// Workspace interfaces. Feature contains durable state and resolved workspace
// metadata; Live and Context contain the current runtime observation.
type WorkItem struct {
	Feature        *featurelist.Feature
	Live           telemetry.Live
	HasTelemetry   bool
	Context        contextpressure.Pressure
	Attention      string
	Lifecycle      string
	LifecycleSince time.Time
	StateChangeSeq uint64
}

func Load(root string) (*Snapshot, error) {
	return LoadWithMux(root, nil)
}

// LoadWithMux builds a snapshot from the selected multiplexer backend.
func LoadWithMux(root string, backend mux.Backend) (*Snapshot, error) {
	ctx, err := workspacectx.Load(root)
	if err != nil {
		return nil, err
	}
	items, err := LoadItemsWithMux(root, ctx.Config, ctx.Workers, backend)
	if err != nil {
		return nil, err
	}
	return &Snapshot{
		Config:  ctx.Config,
		Workers: ctx.Workers,
		Items:   items,
		Health:  doctor.Run(root).Checks,
	}, nil
}

// LoadItems refreshes durable feature state and live session telemetry without
// rerunning the slower workspace health checks used by a full Snapshot.
func LoadItems(root string, cfg *config.Config, allWorkers []*workers.Worker) ([]*WorkItem, error) {
	return LoadItemsWithMux(root, cfg, allWorkers, nil)
}

// LoadItemsWithMux refreshes items using the selected multiplexer backend.
func LoadItemsWithMux(root string, cfg *config.Config, allWorkers []*workers.Worker, backend mux.Backend) ([]*WorkItem, error) {
	features, err := featurelist.Collect(root, featurelist.Options{
		IncludeArchived: true,
		Config:          cfg,
		Workers:         allWorkers,
		Mux:             backend,
	})
	if err != nil {
		return nil, fmt.Errorf("loading features: %w", err)
	}
	runtimeByFeature := sessionlist.ManagedRuntimeWithMux(root, features, backend)
	thresholds := cfg.ContextPressureThresholds()
	items := buildItems(features, runtimeByFeature, thresholds)
	return items, nil
}

func buildItems(features []*featurelist.Feature, runtimeByFeature map[string]sessionlist.ManagedRuntime, thresholds contextpressure.Thresholds) []*WorkItem {
	items := make([]*WorkItem, 0, len(features))
	for _, feature := range features {
		item := &WorkItem{Feature: feature}
		if runtime, ok := runtimeByFeature[filepath.Clean(feature.FeatureDir)]; ok {
			item.Attention = runtime.Attention
			item.Lifecycle = runtime.Lifecycle
			item.StateChangeSeq = runtime.StateChangeSeq
			if runtime.LifecycleSince > 0 {
				item.LifecycleSince = time.Unix(runtime.LifecycleSince, 0)
			}
			if runtime.HasTelemetry {
				item.Live = runtime.Live
				item.HasTelemetry = true
				item.Context = contextpressure.Evaluate(runtime.Live.ContextUsed, runtime.Live.ContextLimit, thresholds)
			}
		}
		items = append(items, item)
	}
	return items
}
