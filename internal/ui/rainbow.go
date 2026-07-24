package ui

import "time"

const (
	RainbowSteps    = 48
	RainbowInterval = 80 * time.Millisecond
)

var rainbowPalette = [...]string{
	"#cba6f7", // mauve
	"#f5c2e7", // pink
	"#f2cdcd", // flamingo
	"#f38ba8", // red
	"#fab387", // peach
	"#f9e2af", // yellow
	"#a6e3a1", // green
	"#94e2d5", // teal
	"#89dceb", // sky
	"#74c7ec", // sapphire
	"#89b4fa", // blue
	"#b4befe", // lavender
}

// RainbowColor returns the shared Orc easter-egg color for a countdown step.
func RainbowColor(step, offset int) string {
	index := (RainbowSteps - step + offset) % len(rainbowPalette)
	if index < 0 {
		index += len(rainbowPalette)
	}
	return rainbowPalette[index]
}
