package ui

import (
	"strings"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/charmbracelet/lipgloss"
)

// sparkBlocks are the eight block heights a sparkline draws from, shortest to
// tallest.
var sparkBlocks = [...]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders 0-100 percentages as one block character each, scaled so 0%
// is the shortest block and 100% the tallest. Values above 100 clamp to the
// tallest block rather than distorting the scale.
//
// When cells is positive and there are more samples than cells, the oldest are
// dropped so the line shows the most recent window. A cells of zero or less
// renders every sample.
func Sparkline(samples []uint64, cells int) string {
	if len(samples) == 0 {
		return ""
	}
	if cells > 0 && len(samples) > cells {
		samples = samples[len(samples)-cells:]
	}
	var b strings.Builder
	b.Grow(len(samples) * 3)
	for _, value := range samples {
		if value > 100 {
			value = 100
		}
		b.WriteRune(sparkBlocks[value*uint64(len(sparkBlocks)-1)/100])
	}
	return b.String()
}

// LevelStyles maps context-pressure levels onto one view's own styles, so the
// Live rail and the Workspace tabs can colour pressure identically while
// keeping their separate style vocabularies. Unknown covers every level that is
// not a graded green/yellow/red reading, including unavailable and neutral.
type LevelStyles struct {
	Unknown lipgloss.Style
	Green   lipgloss.Style
	Yellow  lipgloss.Style
	Red     lipgloss.Style
}

// For returns the style matching a pressure reading's level.
func (s LevelStyles) For(pressure contextpressure.Pressure) lipgloss.Style {
	switch pressure.Level {
	case contextpressure.LevelGreen:
		return s.Green
	case contextpressure.LevelYellow:
		return s.Yellow
	case contextpressure.LevelRed:
		return s.Red
	default:
		return s.Unknown
	}
}
