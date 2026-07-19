// Package workspacesnapshot loads one immutable view of the workspace for
// presentation layers. Consumers project the snapshot into their own rows.
package workspacesnapshot

import (
	"fmt"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspacectx"
)

type Snapshot struct {
	Config    *config.Config
	Workers   []*workers.Worker
	Features  []*featurelist.Feature
	Telemetry map[string]telemetry.Live
	Health    []doctor.Check
}

func Load(root string) (*Snapshot, error) {
	ctx, err := workspacectx.Load(root)
	if err != nil {
		return nil, err
	}
	features, err := featurelist.Collect(root, featurelist.Options{
		IncludeArchived: true,
		Config:          ctx.Config,
		Workers:         ctx.Workers,
	})
	if err != nil {
		return nil, fmt.Errorf("loading features: %w", err)
	}
	return &Snapshot{
		Config:    ctx.Config,
		Workers:   ctx.Workers,
		Features:  features,
		Telemetry: sessionlist.ManagedTelemetry(root, features),
		Health:    doctor.Run(root).Checks,
	}, nil
}
