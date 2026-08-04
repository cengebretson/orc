package watch

import (
	"time"
)

type liveOverviewView struct {
	running    int
	paused     int
	needs      int
	refreshAge string
}

func liveOverviewFor(rows []row, lastLoad, now time.Time) liveOverviewView {
	view := liveOverviewView{refreshAge: watchRefreshAge(lastLoad, now)}
	for _, row := range rows {
		if row.tmuxState == "live" {
			view.running++
		}
		if row.status == "paused" {
			view.paused++
		}
		if attentionNeeded(row) {
			view.needs++
		}
	}
	return view
}
