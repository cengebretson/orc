// Package workspacesnapshot loads one immutable view of the workspace for
// presentation layers. Consumers project the snapshot into their own rows.
package workspacesnapshot

import (
	"fmt"
	"path/filepath"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/featurelist"
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
	Feature      *featurelist.Feature
	Live         telemetry.Live
	HasTelemetry bool
	Context      contextpressure.Pressure
}

func Load(root string) (*Snapshot, error) {
	ctx, err := workspacectx.Load(root)
	if err != nil {
		return nil, err
	}
	items, err := LoadItems(root, ctx.Config, ctx.Workers)
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
	features, err := featurelist.Collect(root, featurelist.Options{
		IncludeArchived: true,
		Config:          cfg,
		Workers:         allWorkers,
	})
	if err != nil {
		return nil, fmt.Errorf("loading features: %w", err)
	}
	liveByFeature := sessionlist.ManagedTelemetry(root, features)
	thresholds := cfg.ContextPressureThresholds()
	items := buildItems(features, liveByFeature, thresholds)
	return items, nil
}

func buildItems(features []*featurelist.Feature, liveByFeature map[string]telemetry.Live, thresholds contextpressure.Thresholds) []*WorkItem {
	items := make([]*WorkItem, 0, len(features))
	for _, feature := range features {
		item := &WorkItem{Feature: feature}
		if live, ok := liveByFeature[filepath.Clean(feature.FeatureDir)]; ok {
			item.Live = live
			item.HasTelemetry = true
			item.Context = contextpressure.Evaluate(live.ContextUsed, live.ContextLimit, thresholds)
		}
		items = append(items, item)
	}
	return items
}
