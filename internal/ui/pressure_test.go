package ui

import (
	"testing"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/charmbracelet/lipgloss"
)

func TestSparklineScalesSamplesToBlockHeight(t *testing.T) {
	got := Sparkline([]uint64{0, 50, 100}, 0)
	want := string([]rune{sparkBlocks[0], sparkBlocks[3], sparkBlocks[7]})
	if got != want {
		t.Fatalf("Sparkline([0,50,100]) = %q, want %q", got, want)
	}
	if got := Sparkline(nil, 0); got != "" {
		t.Fatalf("Sparkline(nil) = %q, want empty", got)
	}
}

// Percentages above 100 can arrive when a provider reports usage past its
// advertised limit. They have to clamp to the tallest block rather than index
// past the end of the ramp.
func TestSparklineClampsOversizedSamples(t *testing.T) {
	got := Sparkline([]uint64{101, 500, 1 << 20}, 0)
	want := string([]rune{sparkBlocks[7], sparkBlocks[7], sparkBlocks[7]})
	if got != want {
		t.Fatalf("Sparkline(oversized) = %q, want %q", got, want)
	}
}

// The Live rail sizes its trend line to the space it has, so a long history
// has to render its most recent window, not its oldest samples.
func TestSparklineKeepsMostRecentSamplesWithinCells(t *testing.T) {
	got := Sparkline([]uint64{0, 0, 0, 100, 100}, 2)
	want := string([]rune{sparkBlocks[7], sparkBlocks[7]})
	if got != want {
		t.Fatalf("Sparkline(cells=2) = %q, want the two newest samples %q", got, want)
	}
	full := Sparkline([]uint64{0, 100}, 0)
	if len([]rune(full)) != 2 {
		t.Fatalf("Sparkline(cells=0) = %q, want every sample", full)
	}
	if got := Sparkline([]uint64{0, 100}, 9); len([]rune(got)) != 2 {
		t.Fatalf("Sparkline(cells > samples) = %q, want every sample", got)
	}
}

func TestLevelStylesMapEveryLevel(t *testing.T) {
	styles := LevelStyles{
		Unknown: lipgloss.NewStyle().Foreground(lipgloss.Color("#111111")),
		Green:   lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")),
		Yellow:  lipgloss.NewStyle().Foreground(lipgloss.Color("#ffff00")),
		Red:     lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")),
	}
	for _, test := range []struct {
		level contextpressure.Level
		want  lipgloss.Style
	}{
		{contextpressure.LevelGreen, styles.Green},
		{contextpressure.LevelYellow, styles.Yellow},
		{contextpressure.LevelRed, styles.Red},
		// Neither unavailable nor neutral is a graded reading, so both take the
		// same muted treatment as an unrecognized level.
		{contextpressure.LevelUnavailable, styles.Unknown},
		{contextpressure.LevelNeutral, styles.Unknown},
		{contextpressure.Level("something-new"), styles.Unknown},
	} {
		got := styles.For(contextpressure.Pressure{Level: test.level})
		if got.GetForeground() != test.want.GetForeground() {
			t.Errorf("For(%q) foreground = %v, want %v",
				test.level, got.GetForeground(), test.want.GetForeground())
		}
	}
}
