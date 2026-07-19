package workspaceui

import "time"

type workspaceOverviewView struct {
	features   int
	active     int
	paused     int
	broken     int
	refreshAge time.Duration
}

func workspaceOverviewFor(features []*featureRow, lastRefresh, now time.Time) workspaceOverviewView {
	view := workspaceOverviewView{features: len(features)}
	if !lastRefresh.IsZero() && !now.IsZero() {
		view.refreshAge = max(time.Duration(0), now.Sub(lastRefresh))
	}
	for _, feature := range features {
		if feature.s == nil {
			view.broken++
			continue
		}
		switch feature.s.Status {
		case "active":
			view.active++
		case "paused":
			view.paused++
		}
	}
	return view
}
