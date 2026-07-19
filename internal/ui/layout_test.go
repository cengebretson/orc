package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestLayoutPrimitivesRespectTerminalCellsAndANSI(t *testing.T) {
	styled := lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Render("界 café")
	for width := 1; width <= 8; width++ {
		got := Truncate(styled, width)
		if !utf8.ValidString(got) {
			t.Fatalf("Truncate width %d returned invalid UTF-8: %q", width, got)
		}
		if gotWidth := Width(got); gotWidth > width {
			t.Fatalf("Truncate width %d occupied %d cells: %q", width, gotWidth, got)
		}
		if gotWidth := Width(Cell(styled, width)); gotWidth != width {
			t.Fatalf("PadRight width %d occupied %d cells", width, gotWidth)
		}
	}
	for _, line := range strings.Split(Wrap("界界 alpha beta", 6), "\n") {
		if gotWidth := Width(line); gotWidth > 6 {
			t.Fatalf("wrapped line occupied %d cells: %q", gotWidth, line)
		}
	}
}

func TestFitPreservesANSIAndUnicodeAtEveryWidth(t *testing.T) {
	value := "\x1b[35;1m界 ORC DASHBOARD\x1b[0m"
	for width := 1; width <= 20; width++ {
		got := Fit(value, width)
		if !utf8.ValidString(got) || lipgloss.Width(got) > width {
			t.Fatalf("Fit width %d = %q (%d cells)", width, got, lipgloss.Width(got))
		}
		if !utf8.ValidString(ansi.Strip(got)) {
			t.Fatalf("Fit width %d produced invalid stripped text: %q", width, ansi.Strip(got))
		}
	}
}

func TestLayoutPrimitiveEdgeCases(t *testing.T) {
	if got := Truncate("hello", 0); got != "" {
		t.Fatalf("Truncate(_, 0) = %q, want empty", got)
	}
	if got := Truncate("hello", -5); got != "" {
		t.Fatalf("Truncate(_, negative) = %q, want empty", got)
	}
	if got := Truncate("hello world", 6); got != "hello…" {
		t.Fatalf("Truncate long value = %q, want %q", got, "hello…")
	}
	if got := PadRight("abcdef", 3); got != "abcdef" {
		t.Fatalf("PadRight should not truncate: %q", got)
	}
	if got, want := Wrap("one two three four", 9), "one two\nthree\nfour"; got != want {
		t.Fatalf("Wrap = %q, want %q", got, want)
	}
	if got := Wrap("", 10); got != "" {
		t.Fatalf("Wrap empty = %q, want empty", got)
	}
}
