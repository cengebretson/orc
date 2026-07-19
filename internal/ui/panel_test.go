package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderLabeledPanelRespectsCellsPaddingAndCollapse(t *testing.T) {
	border := lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7"))
	for width := 4; width <= 24; width++ {
		panel := RenderLabeledPanel(LabeledPanelOptions{
			Title:        "界 Workspace",
			Lines:        []string{"café content that is deliberately long"},
			Width:        width,
			PaddingLeft:  1,
			PaddingRight: 1,
			Border:       border,
		})
		for _, line := range strings.Split(panel, "\n") {
			if got := Width(line); got != width {
				t.Fatalf("width %d rendered line width %d: %q", width, got, line)
			}
		}
	}

	collapsed := RenderLabeledPanel(LabeledPanelOptions{
		Title: "Repositories", Summary: "2 repos", Width: 24, Collapsed: true, Border: border,
	})
	if got := len(strings.Split(collapsed, "\n")); got != 2 {
		t.Fatalf("collapsed panel lines = %d, want 2", got)
	}
}
