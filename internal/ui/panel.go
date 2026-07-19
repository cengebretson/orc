package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LabeledPanelOptions describes a rounded terminal panel whose title is
// embedded in the top border. Callers retain ownership of styled title and
// content strings; this primitive owns cell-safe sizing and border assembly.
type LabeledPanelOptions struct {
	Title        string
	Summary      string
	Lines        []string
	Width        int
	PaddingLeft  int
	PaddingRight int
	Collapsed    bool
	Border       lipgloss.Style
}

// RenderLabeledPanel renders a width-stable ANSI-aware labeled panel.
func RenderLabeledPanel(opts LabeledPanelOptions) string {
	outerW := max(4, opts.Width)
	innerW := outerW - 2
	paddingLeft := max(0, opts.PaddingLeft)
	paddingRight := max(0, opts.PaddingRight)
	contentW := max(0, innerW-paddingLeft-paddingRight)

	title := opts.Title
	if opts.Summary != "" {
		title += "  " + opts.Summary
	}
	top := opts.Border.Render("╭" + strings.Repeat("─", innerW) + "╮")
	if title != "" && innerW >= 4 {
		label := " " + Truncate(title, innerW-3) + " "
		dashRight := max(0, innerW-1-Width(label))
		top = opts.Border.Render("╭─") + label + opts.Border.Render(strings.Repeat("─", dashRight)+"╮")
	}
	lines := []string{top}
	if !opts.Collapsed {
		for _, line := range opts.Lines {
			line = Truncate(line, contentW)
			fill := max(0, contentW-Width(line))
			lines = append(lines,
				opts.Border.Render("│")+
					strings.Repeat(" ", paddingLeft)+line+strings.Repeat(" ", fill+paddingRight)+
					opts.Border.Render("│"),
			)
		}
	}
	lines = append(lines, opts.Border.Render("╰"+strings.Repeat("─", innerW)+"╯"))
	return strings.Join(lines, "\n")
}
