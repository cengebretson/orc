package ui

import (
	"testing"
	"time"
)

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name string
		then time.Time
		want string
	}{
		{"zero renders as unknown, not now", time.Time{}, "-"},
		{"seconds", now.Add(-30 * time.Second), "now"},
		{"minutes", now.Add(-12 * time.Minute), "12m"},
		{"hours", now.Add(-3 * time.Hour), "3h"},
		{"days", now.Add(-50 * time.Hour), "2d"},
		{"future clamps to now", now.Add(time.Hour), "now"},
	} {
		if got := RelativeTime(now, test.then); got != test.want {
			t.Errorf("%s: RelativeTime() = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestEmptyDash(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"", "-"},
		{"   ", "-"},
		{"\t", "-"},
		{"value", "value"},
	} {
		if got := EmptyDash(test.in); got != test.want {
			t.Errorf("EmptyDash(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}
