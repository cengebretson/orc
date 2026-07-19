package watch

import (
	"fmt"
	"strings"

	terminalui "github.com/cengebretson/orc/internal/ui"
)

func (m Model) workPreviewContent(r row) string {
	icon, label := displayState(r)
	width := max(20, m.width)
	panelW := max(12, width-2)
	contentW := max(8, panelW-4)
	now := m.renderNow()

	var b strings.Builder
	var stateLines []string
	if now.Before(r.celebrateUntil) || r.demoCelebration {
		stateLines = append(stateLines, doneFlashStyle.Render(" ✦ COMPLETE ✦ "))
	} else {
		stateLines = append(stateLines, stateBadge(label).Render(icon+" "+strings.ToUpper(label)))
	}
	if r.stage != "" {
		stateLines = append(stateLines, "stage     "+r.stage)
	}
	if r.room != "" && r.room != "workspace" {
		stateLines = append(stateLines, "repo      "+r.room)
	}
	if r.worker != "" && r.worker != "-" {
		stateLines = append(stateLines, "worker    "+r.worker)
	}
	if r.branch != "" {
		stateLines = append(stateLines, "branch    "+r.branch)
	}
	provider := joinNonEmpty(" · ", r.engine, r.model)
	if provider != "" {
		stateLines = append(stateLines, "provider  "+provider)
	}
	if r.tmuxState != "" && r.tmuxState != "-" {
		stateLines = append(stateLines, "tmux      "+r.tmuxState)
		if target := tmuxTargetLabel(r); target != "" {
			stateLines = append(stateLines, "target    "+target)
		}
	}
	if r.attention != "" {
		stateLines = append(stateLines, "attention "+r.attention)
	}
	if r.liveState != "" {
		stateLines = append(stateLines, "live      "+r.liveState)
	}
	if r.context.Observed {
		stateLines = append(stateLines, "context   "+strings.TrimPrefix(renderContextVisual(r, min(10, contentW-12)), "ctx "))
	}
	if age := activityAge(r, now); age != "" {
		stateLines = append(stateLines, "activity  "+age)
	}
	if timing := currentStageTiming(r, now); timing != "" {
		stateLines = append(stateLines, "timing    "+timing)
	}
	detailTitle := featureDetailTitle(r)
	b.WriteString(renderDetailCard(detailTitle, strings.Join(stateLines, "\n"), panelW))

	if len(r.workflowSteps) > 0 {
		workflowTitle := "WORKFLOW"
		if r.workflowLabel != "" {
			workflowTitle += " · " + r.workflowLabel
		}
		if current, total := workflowPosition(r); current > 0 && total > 0 {
			workflowTitle += fmt.Sprintf(" · %d/%d", current, total)
		}
		workflowContent := renderWorkflowFlow(r, contentW)
		if next := nextTransition(r); next != "" {
			workflowContent += "\n\n" + mutedStyle.Render(next)
		}
		b.WriteString("\n\n")
		b.WriteString(renderDetailCard(workflowTitle, workflowContent, panelW))
	}

	if r.loadErr != nil {
		b.WriteString("\n\n")
		b.WriteString(renderDetailCard("ERROR", wrap(r.loadErr.Error(), contentW), panelW))
	} else if r.next != "" {
		b.WriteString("\n\n")
		b.WriteString(renderDetailCard(strings.ToUpper(promptLabel(r)), wrap(r.next, contentW), panelW))
	}
	if len(r.history) > 0 {
		b.WriteString("\n\n")
		historyTitle := fmt.Sprintf("HISTORY · %d EVENTS", len(r.history))
		b.WriteString(renderDetailCard(historyTitle, renderTimelineHistory(r.history, contentW, 12), panelW))
	}
	return b.String()
}

func featureDetailTitle(r row) string {
	name := strings.TrimSpace(r.name)
	ticket := strings.TrimSpace(r.ticket)
	prefix := ticket + "-"
	if ticket != "" && len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
		name = name[len(prefix):]
	}
	if name == "" {
		return ticket
	}
	if ticket == "" || strings.EqualFold(name, ticket) {
		return name
	}
	return name + " · " + ticket
}

func renderDetailCard(title, content string, width int) string {
	outerW := max(12, width)
	innerW := outerW - 2
	title = truncate(title, max(1, innerW-3))
	return terminalui.RenderLabeledPanel(terminalui.LabeledPanelOptions{
		Title: sectionStyle.Render(title), Lines: strings.Split(content, "\n"), Width: outerW,
		PaddingLeft: 1, PaddingRight: 1, Border: accentStyle,
	})
}

func joinNonEmpty(separator string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, separator)
}

func tmuxTargetLabel(r row) string {
	target := r.session
	if r.window != "" {
		if target != "" {
			target += ":"
		}
		target += r.window
	}
	if r.pane != "" {
		if target != "" {
			target += " · "
		}
		target += r.pane
	}
	return target
}

func wrap(s string, width int) string {
	return terminalui.Wrap(s, max(8, width))
}
