package workspaceui

import (
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/state"
)

func TestWorkspaceOverviewFor(t *testing.T) {
	lastRefresh := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	view := workspaceOverviewFor([]*featureRow{
		{s: &state.State{Status: "active"}},
		{s: &state.State{Status: "paused"}},
		{s: nil},
	}, lastRefresh, lastRefresh.Add(5*time.Second))

	if view.features != 3 || view.active != 1 || view.paused != 1 || view.broken != 1 {
		t.Fatalf("counts = features %d, active %d, paused %d, broken %d; want 3, 1, 1, 1",
			view.features, view.active, view.paused, view.broken)
	}
	if view.refreshAge != 5*time.Second {
		t.Fatalf("refreshAge = %s, want 5s", view.refreshAge)
	}
}

func TestWorkspaceOverviewForClampsFutureRefresh(t *testing.T) {
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	view := workspaceOverviewFor(nil, now.Add(time.Second), now)
	if view.refreshAge != 0 {
		t.Fatalf("refreshAge = %s, want 0", view.refreshAge)
	}
}
