package workspaceui

import (
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) sectionBox(spec sectionSpec, summary string, content []string, outerW int, focused bool) string {
	borderColor := activeTheme.Palette.Surface1
	if focused {
		borderColor = activeTheme.Palette.Mauve
	}
	bd := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	title := styleSection.Render(spec.title)
	if !m.embedded {
		title = styleDim.Render(spec.shortcut) + " " + title
	}

	if !m.navigation.expanded[spec.id] {
		return terminalui.RenderLabeledPanel(terminalui.LabeledPanelOptions{
			Title: title, Summary: summary, Width: outerW, Collapsed: true, Border: bd,
		})
	}
	return terminalui.RenderLabeledPanel(terminalui.LabeledPanelOptions{
		Title: title, Lines: content, Width: outerW, PaddingLeft: 2, Border: bd,
	})
}

func renderWorkerGroups(groups []workerGroup, maxW int) []string {
	var lines []string
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		chips := make([]string, len(group.items))
		for i, item := range group.items {
			chips[i] = workerAccentStyle(item.id).Render(item.label)
		}
		for _, line := range renderChipList(maxW-2, chips) {
			lines = append(lines, "  "+line)
		}
	}
	return lines
}

func renderWorkflowChainGroups(chains []workflowChain, maxW int) []string {
	grouped := map[string][]workflowChain{}
	for _, chain := range chains {
		grouped[workflowNamespace(chain.name)] = append(grouped[workflowNamespace(chain.name)], chain)
	}

	var lines []string
	for _, groupName := range sortedKeys(grouped) {
		lines = append(lines, styleDim.Render(groupName))
		for _, chain := range grouped[groupName] {
			if chain.name != "" {
				lines = append(lines, "  "+workflowChainHeading(chain)+":")
			}
			for _, line := range renderRouteChain(chain.steps, chain.loops, maxW-2) {
				lines = append(lines, "  "+line)
			}
			lines = append(lines, "")
		}
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func workflowChainHeading(c workflowChain) string {
	return styleDim.Render(labelWithDimID(chainLabel(c), c.name))
}

func renderGroupedWorkerList(groups []workerGroup, cursor int) []string {
	var lines []string
	itemIdx := 0
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		for _, item := range group.items {
			label := coloredWorkerLabel(item.label, item.id)
			if itemIdx == cursor {
				lines = append(lines, styleHealthOK.Render("▶")+"  "+label+
					styleDim.Render("  enter to view"))
			} else {
				lines = append(lines, "   "+label)
			}
			itemIdx++
		}
	}
	if len(lines) == 0 {
		return []string{styleDim.Render("No workers configured.")}
	}
	return lines
}

func renderGroupedWorkflowList(groups []workflowGroup, cursor int) []string {
	var lines []string
	itemIdx := 0
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		for _, item := range group.items {
			label := labelWithDimID(item.label, item.id)
			if itemIdx == cursor {
				lines = append(lines, styleHealthOK.Render("▶")+"  "+styleSubtext.Render(label)+
					styleDim.Render("  enter to view"))
			} else {
				lines = append(lines, "   "+styleDim.Render(label))
			}
			itemIdx++
		}
	}
	if len(lines) == 0 {
		return []string{styleDim.Render("No workflows configured.")}
	}
	return lines
}

// renderNameList wraps a list of plain names with · separators to fit maxW.
func renderNameList(maxW int, names []string) []string {
	chips := make([]string, len(names))
	for i, name := range names {
		chips[i] = styleSubtext.Render(name)
	}
	return renderChipList(maxW, chips)
}

// renderChipList wraps a list of pre-rendered chips with · separators to fit
// maxW, letting callers style each chip differently (e.g. per-worker accent
// colors) while sharing the wrapping logic with renderNameList.
func renderChipList(maxW int, chips []string) []string {
	sep := styleDivider.Render("  ·  ")
	sepW := lipgloss.Width(sep)

	var rows []string
	row := ""
	rowW := 0
	for _, chip := range chips {
		chipW := lipgloss.Width(chip)
		if rowW > 0 && rowW+sepW+chipW > maxW {
			rows = append(rows, row)
			row = ""
			rowW = 0
		}
		if rowW > 0 {
			row += sep
			rowW += sepW
		}
		row += chip
		rowW += chipW
	}
	if row != "" {
		rows = append(rows, row)
	}
	return rows
}

// renderRepoList renders repos as "name — purpose" lines.
