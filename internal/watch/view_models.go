package watch

import (
	"fmt"
	"strings"
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

func (view liveOverviewView) summary() string {
	parts := []string{fmt.Sprintf("%d running", view.running), fmt.Sprintf("%d paused", view.paused)}
	if view.needs > 0 {
		parts = append(parts, fmt.Sprintf("%d needs you", view.needs))
	}
	return strings.Join(parts, " · ")
}
