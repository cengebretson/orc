package workspaceui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
)

func (m Model) viewFile() string {
	outerW := max(4, m.width-2)
	var b strings.Builder
	title := styleDetailTitle.Render(" "+m.viewerContext) +
		styleDim.Render(" · ") +
		styleSubtext.Render(m.viewerTitle) +
		styleDim.Render(fmt.Sprintf("  ·  %.0f%% ", m.viewport.ScrollPercent()*100))
	b.WriteString("\n" + drawBox(title, nil, outerW) + "\n")
	b.WriteString(m.viewport.View())
	if !m.embedded {
		helpItems := []string{
			combinedBindingHelp("scroll", keys.up, keys.down, keys.pageUp, keys.pageDown),
		}
		switch m.viewerReturn {
		case viewDetail:
			helpItems = append(helpItems, combinedBindingHelp("prev/next file", keys.previous, keys.next))
		case viewWorkflowDetail:
			helpItems = append(helpItems, combinedBindingHelp("prev/next stage", keys.previous, keys.next))
		}
		helpItems = append(helpItems,
			bindingHelp(keys.back),
			bindingHelp(keys.quit),
		)
		help := strings.Join(helpItems, "  ")
		b.WriteString("\n" + styleHelp.Render("  "+help))
	}
	return b.String()
}

// ── Workflow detail view ──────────────────────────────────────────

func (m Model) viewWorkflowDetailPage() string {
	outerW := max(4, m.width-2)
	var b strings.Builder
	title := styleDetailTitle.Render(" Workflows") +
		styleDim.Render(" · ") +
		styleSubtext.Render(workflowDisplayWithID(m.wfDetailName, m.workflows)) +
		styleDim.Render(fmt.Sprintf("  ·  %.0f%% ", m.viewport.ScrollPercent()*100))
	b.WriteString("\n" + drawBox(title, nil, outerW) + "\n")
	b.WriteString(m.viewport.View())
	if !m.embedded {
		help := strings.Join([]string{
			combinedBindingHelp("select stage", keys.previous, keys.next),
			combinedBindingHelp("scroll", keys.up, keys.down),
			combinedBindingHelp("scroll page", keys.pageUp, keys.pageDown),
			bindingHelp(keys.open),
			bindingHelp(keys.back),
			bindingHelp(keys.quit),
		}, "  ")
		b.WriteString("\n" + styleHelp.Render("  "+help))
	}
	return b.String()
}

func wfDetailTotal(name string, chains []workflowChain) int {
	for _, c := range chains {
		if c.name == name {
			return len(c.steps) + len(c.repairSteps)
		}
	}
	return 0
}

func wfDetailSelectedStage(name string, idx int, chains []workflowChain) (stageName, advance string, stepNum, total int) {
	for _, c := range chains {
		if c.name != name {
			continue
		}
		total = len(c.steps) + len(c.repairSteps)
		if idx < len(c.steps) {
			s := c.steps[idx]
			return s.name, s.advance, idx + 1, total
		}
		ri := idx - len(c.steps)
		if ri < len(c.repairSteps) {
			rs := c.repairSteps[ri]
			return rs.name, rs.advance, idx + 1, total
		}
	}
	return "", "", 0, 0
}

func stageWorkflowCount(chains []workflowChain, stageName string) int {
	count := 0
	for _, c := range chains {
		for _, s := range c.steps {
			if s.name == stageName {
				count++
				break
			}
		}
		for _, rs := range c.repairSteps {
			if rs.name == stageName {
				count++
				break
			}
		}
	}
	return count
}

func renderFile(path string, width int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if width <= 0 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylesFromJSONBytes(activeTheme.Glamour),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return string(data), nil
	}
	out, err := r.Render(string(data))
	if err != nil {
		return string(data), nil
	}
	return out, nil
}

// renderWorkerFile renders a worker .md file as a frontmatter info box followed
// by the markdown body. width is the available viewport width.
