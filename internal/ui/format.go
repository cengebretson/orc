package ui

import (
	"fmt"
	"strings"
	"time"
)

// RelativeTime renders how long ago then was, relative to now, as a short
// single-token age such as "now", "12m", "3h", or "5d".
//
// A zero timestamp renders as "-": no recorded activity is not the same as
// activity a moment ago, and rendering it as "now" would show a session that
// never reported as the most recently active one.
func RelativeTime(now, then time.Time) string {
	if then.IsZero() {
		return "-"
	}
	d := now.Sub(then)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// EmptyDash renders a placeholder for a value a view has nothing to show for,
// so columns stay aligned and a missing value is visibly missing rather than
// blank. Values that are only whitespace count as empty.
func EmptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
