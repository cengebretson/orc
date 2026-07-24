package workspaceui

import "time"

type workspaceOverviewView struct {
	features   int
	running    int
	paused     int
	needs      int
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
		if feature.tmuxLive {
			view.running++
		}
		if feature.s.Status == "paused" {
			view.paused++
		}
		if featureNeedsAttention(feature) {
			view.needs++
		}
	}
	return view
}
