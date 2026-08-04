package watch

import (
	"fmt"
	"strings"
	"time"

	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) previewContent() string {
	if r, ok := m.selectedWork(); ok {
		return m.workPreviewContent(r)
	}
	return mutedStyle.Render("No item selected.")
}

func renderRailDetailAt(r row, width int, now time.Time) string {
	icon, label := displayState(r)
	var b strings.Builder
	b.WriteString(stateBadge(label).Render(icon + " " + strings.ToUpper(truncate(label, width-2))))
	for _, location := range compactDetailLocations(r) {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate(location, max(1, width-2))))
	}
	if r.worker != "" && r.worker != "-" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate(r.worker, max(1, width-2))))
	}
	if r.displayTitle != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate(r.displayTitle, max(1, width-2))))
	}
	if r.tmuxState != "" && r.tmuxState != "-" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate("tmux "+r.tmuxState, max(1, width-2))))
	}
	if r.attention != "" {
		b.WriteString("\n")
		attention := "attention " + r.attention
		if r.attentionSource != "" {
			attention += " · " + r.attentionSource
		}
		b.WriteString(mutedStyle.Render("  " + truncate(attention, max(1, width-2))))
	}
	if r.reconciliation != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate("runtime "+r.reconciliation, max(1, width-2))))
	}
	if r.woken {
		b.WriteString("\n")
		b.WriteString(accentStyle.Render(truncate("↟ woken from parking · "+r.wakeReason, width)))
	}
	if r.context.Observed {
		b.WriteString("\n")
		b.WriteString("  " + renderContextVisual(r, max(3, min(8, width-10))))
	}
	if age := activityAge(r, now); age != "" {
		b.WriteString("\n")
		prefix := "updated "
		if !r.lastActive.IsZero() {
			prefix = "active "
		}
		b.WriteString(mutedStyle.Render("  " + prefix + age))
	}
	if len(r.workflowSteps) > 0 {
		b.WriteString("\n\n")
		label := "FLOW"
		if r.workflowLabel != "" {
			label += " · " + r.workflowLabel
		}
		b.WriteString(sectionStyle.Render(truncate(label, width)))
		b.WriteString("\n")
		b.WriteString(renderWorkflowFlow(r, width))
	}
	if r.next != "" {
		b.WriteString("\n\n")
		b.WriteString(sectionStyle.Render(promptLabel(r)))
		b.WriteString("\n")
		b.WriteString(wrap(r.next, width))
	}
	return b.String()
}

func stateBadge(label string) lipgloss.Style {
	return stateStyle(label).Background(lipgloss.Color("#313244")).Bold(true).Padding(0, 1)
}

func renderWatchStatusLines(view liveOverviewView, width int) []string {
	runningText := fmt.Sprintf("● %d RUNNING", view.running)
	pausedText := fmt.Sprintf("◐ %d PAUSED", view.paused)
	needsText := fmt.Sprintf("! %d NEEDS YOU", view.needs)
	runningView := activeStyle.Bold(true).Render(runningText)
	pausedView := pendingStyle.Bold(true).Render(pausedText)
	needsView := blockedStyle.Bold(true).Render(needsText)

	all := runningView + "  " + pausedView
	if view.needs > 0 {
		all += "  " + needsView
	}
	if lipgloss.Width(all) <= width {
		return []string{all}
	}
	primary := runningView + "  " + pausedView
	if lipgloss.Width(primary) <= width {
		lines := []string{primary}
		if view.needs > 0 {
			lines = append(lines, needsView)
		}
		return lines
	}

	compact := activeStyle.Bold(true).Render(fmt.Sprintf("● %d", view.running)) + "  " +
		pendingStyle.Bold(true).Render(fmt.Sprintf("◐ %d", view.paused))
	lines := []string{compact}
	if view.needs > 0 {
		lines = append(lines, blockedStyle.Bold(true).Render(fmt.Sprintf("! %d", view.needs)))
	}
	return lines
}

func renderWatchOverview(rows []row, width int, lastLoad, now time.Time) string {
	view := liveOverviewFor(rows, lastLoad, now)
	outerW := max(12, width)
	innerW := outerW - 2
	title := titleStyle.Render("orc") + mutedStyle.Render("  workspace orchestrator")
	if innerW < 29 {
		title = titleStyle.Render("orc") + mutedStyle.Render("  workspace")
	}
	if innerW < 17 {
		title = titleStyle.Render("orc")
	}

	statusLines := renderWatchStatusLines(view, max(8, innerW-2))
	refresh := mutedStyle.Render(view.refreshAge)
	lines := make([]string, 0, len(statusLines)+1)
	for _, line := range statusLines {
		lines = append(lines, " "+line)
	}
	last := len(lines) - 1
	combined := lines[last] + mutedStyle.Render("  ·  ") + refresh
	if lipgloss.Width(combined) <= innerW {
		lines[last] = combined
	} else {
		lines = append(lines, " "+refresh)
	}
	return renderWatchLabeledBox(title, lines, outerW)
}

func watchRefreshAge(lastLoad, now time.Time) string {
	if lastLoad.IsZero() {
		return "↺ loading"
	}
	if now.IsZero() {
		now = time.Now()
	}
	age := now.Sub(lastLoad)
	if age < 0 {
		age = 0
	}
	return fmt.Sprintf("↺ %s ago", age.Round(time.Second))
}

func renderWatchLabeledBox(title string, content []string, outerW int) string {
	return terminalui.RenderLabeledPanel(terminalui.LabeledPanelOptions{
		Title: title, Lines: content, Width: outerW, Border: mutedStyle,
	})
}
