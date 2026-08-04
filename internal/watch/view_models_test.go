package watch

import (
	"testing"
	"time"
)

func TestLiveOverviewFor(t *testing.T) {
	lastLoad := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	view := liveOverviewFor([]row{
		{status: "active", tmuxState: "live"},
		{status: "paused", tmuxState: "stopped"},
		{status: "active", tmuxState: "stopped"},
	}, lastLoad, lastLoad.Add(4*time.Second))

	if view.running != 1 || view.paused != 1 || view.needs != 1 {
		t.Fatalf("counts = running %d, paused %d, needs %d; want 1, 1, 1", view.running, view.paused, view.needs)
	}
	if view.refreshAge != "↺ 4s ago" {
		t.Fatalf("refreshAge = %q, want %q", view.refreshAge, "↺ 4s ago")
	}
}
