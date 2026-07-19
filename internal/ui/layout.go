// Package ui contains terminal presentation primitives shared by Orc's Live
// and Workspace views.
package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Width returns the number of terminal cells occupied by value, excluding ANSI
// control sequences.
func Width(value string) int {
	return ansi.StringWidth(value)
}

// Truncate shortens value to width terminal cells without splitting ANSI
// sequences, UTF-8, or wide characters.
func Truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(value, width, "…")
}

// Fit constrains value to width terminal cells while preserving styling.
func Fit(value string, width int) string {
	if width <= 0 || Width(value) <= width {
		return value
	}
	return Truncate(value, width)
}

// PadRight pads values shorter than width and leaves wider values unchanged.
func PadRight(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return value + strings.Repeat(" ", max(0, width-Width(value)))
}

// Cell truncates or pads value to exactly width terminal cells.
func Cell(value string, width int) string {
	return PadRight(Truncate(value, width), width)
}

// Wrap word-wraps value using terminal-cell width and preserves ANSI styling.
func Wrap(value string, width int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	width = max(1, width)
	return ansi.Hardwrap(ansi.Wordwrap(value, width, ""), width, false)
}
