package watch

import (
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncateUsesTerminalCellWidth(t *testing.T) {
	tests := []struct {
		name  string
		value string
		width int
		want  string
	}{
		{name: "ascii", value: "abcdef", width: 4, want: "abc…"},
		{name: "multibyte", value: "café", width: 4, want: "café"},
		{name: "wide", value: "界abc", width: 4, want: "界a…"},
		{name: "ellipsis only", value: "界", width: 1, want: "…"},
		{name: "trim", value: "  abc  ", width: 3, want: "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.value, tt.width)
			if got != tt.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tt.value, tt.width, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("truncate(%q, %d) returned invalid UTF-8", tt.value, tt.width)
			}
			if lipgloss.Width(got) > tt.width {
				t.Fatalf("truncate(%q, %d) uses %d cells", tt.value, tt.width, lipgloss.Width(got))
			}
		})
	}
}
