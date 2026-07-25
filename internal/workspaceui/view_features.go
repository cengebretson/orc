package workspaceui

import (
	"strings"

	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

// attentionMarker replaces a non-selected row's leading space when it needs
// attention (blocked/input/review), colored to match its status so a long
// list reads at a glance without needing to scan the Status column text.
const attentionMarker = "▎"

func (m Model) renderTable(rows []*featureRow, w int, selectedIdx int) string {
	const (
		wTicket  = 12
		wStatus  = 12
		wContext = 14 // room for an up-to-8-sample sparkline plus the percentage
		wTmux    = 6
	)
	// fixed overhead: leading space + static columns + separators (6 × "  ")
	fixed := 1 + wTicket + wStatus + wContext + wTmux + 6*2
	flex := w - fixed
	if flex < 48 {
		flex = 48
	}
	// Favor the human-readable feature name, then the workflow/stage cell. The
	// worker column can be narrower without hiding state such as "+ jit".
	wName := flex/2 + 5
	remaining := flex - wName
	wWorkflow := remaining/2 + 4
	wWorker := remaining - wWorkflow

	header := " " +
		ui.PadRight(styleTableHeader.Render("Ticket"), wTicket) + "  " +
		ui.PadRight(styleTableHeader.Render("Name"), wName) + "  " +
		ui.PadRight(styleTableHeader.Render("Status"), wStatus) + "  " +
		ui.PadRight(styleTableHeader.Render("Stage"), wWorkflow) + "  " +
		ui.PadRight(styleTableHeader.Render("Worker"), wWorker) + "  " +
		ui.PadRight(styleTableHeader.Render("Context"), wContext) + "  " +
		ui.PadRight(styleTableHeader.Render("Tmux"), wTmux)

	div := " " + styleDivider.Render(strings.Repeat("─", w-1))

	var lines []string
	lines = append(lines, header, div)

	for i, row := range rows {
		selected := i == selectedIdx

		if row.s == nil {
			lines = append(lines, brokenRow(row, w, wTicket, selected))
			continue
		}
		s := row.s

		icon, displayStatus := featureDisplayState(row)
		name := strings.TrimPrefix(s.Slug, s.Ticket+"-")
		workflowLabel := row.workflowLabel
		if workflowLabel == "" {
			workflowLabel = row.workflow
		}
		stageLabel := row.stageLabel
		if stageLabel == "" {
			stageLabel = s.Stage.Name
		}
		stageCell := workflowLabel + " › " + stageLabel + row.stageLoopLabel
		if s.Runtime.JIT != nil {
			stageCell += " + jit"
		}

		plainWorker := row.workerName
		if plainWorker == "" {
			plainWorker = "—"
		}
		plainTmux := "-"
		if s.Runtime.Tmux != nil {
			if row.tmuxLive {
				plainTmux = "✓"
			} else {
				plainTmux = "✗"
			}
		}

		ticketCell := ui.Truncate(s.Ticket, wTicket)
		if row.hasIssues {
			ticketCell = ui.Truncate("! "+s.Ticket, wTicket)
		}

		history := m.effects.contextHistory[row.ticketID()]

		if selected {
			// Plain unstyled text so styleRowSelected background covers the full row
			contextText := row.context.Label()
			if len(history) >= 2 {
				contextText = sparkline(history) + " " + row.context.Label()
			}
			line := " " +
				ui.PadRight(ticketCell, wTicket) + "  " +
				ui.PadRight(ui.Truncate(name, wName), wName) + "  " +
				ui.PadRight(ui.Truncate(icon+" "+displayStatus, wStatus), wStatus) + "  " +
				ui.PadRight(ui.Truncate(stageCell, wWorkflow), wWorkflow) + "  " +
				ui.PadRight(ui.Truncate(plainWorker, wWorker), wWorker) + "  " +
				ui.PadRight(contextText, wContext) + "  " +
				ui.PadRight(plainTmux, wTmux)
			lines = append(lines, styleRowSelected.Width(w).Render(line))
		} else {
			ticketStyled := ticketCell
			if row.hasIssues {
				ticketStyled = styleHealthWarn.Render(ticketCell)
			}
			statusCell := featureStateStyle(row).Render(icon + " " + displayStatus)
			nameCell := styleDim.Render(ui.Truncate(name, wName))
			workerCell := workerAccentStyle(row.workerID).Render(ui.Truncate(plainWorker, wWorker))
			contextCell := renderContextPressure(row.context)
			if spark := renderContextSparkline(history, row.context); spark != "" {
				contextCell = spark
			}
			var tmuxCell string
			if s.Runtime.Tmux != nil {
				if row.tmuxLive {
					tmuxCell = styleTmuxLive.Render(plainTmux)
				} else {
					tmuxCell = styleTmuxDead.Render(plainTmux)
				}
			} else {
				tmuxCell = styleTmuxNone.Render(plainTmux)
			}
			marker := " "
			if featureNeedsAttention(row) {
				marker = featureStateStyle(row).Render(attentionMarker)
			}
			line := marker +
				ui.PadRight(ticketStyled, wTicket) + "  " +
				ui.PadRight(nameCell, wName) + "  " +
				ui.PadRight(statusCell, wStatus) + "  " +
				ui.PadRight(ui.Truncate(stageCell, wWorkflow), wWorkflow) + "  " +
				ui.PadRight(workerCell, wWorker) + "  " +
				ui.PadRight(contextCell, wContext) + "  " +
				ui.PadRight(tmuxCell, wTmux)
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func featureDisplayState(row *featureRow) (string, string) {
	if row == nil || row.s == nil {
		return "!", "error"
	}
	switch row.s.Status {
	case "paused":
		return "!", "blocked"
	case "done", "archived":
		return "✓", row.s.Status
	case "active":
		if row.s.Runtime.Tmux != nil && !row.tmuxLive {
			return "×", "stopped"
		}
		switch row.attention {
		case tmux.AttentionInput:
			return "!", "input"
		case tmux.AttentionBlocked:
			return "!", "blocked"
		case tmux.AttentionReview:
			return "◆", "review"
		case tmux.AttentionDone:
			return "✓", "done"
		}
		return "●", "active"
	case "ready":
		return "▶", "ready"
	case "pending":
		return "○", "pending"
	default:
		return statusIcon(row.s.Status), row.s.Status
	}
}

func featureStateLabel(row *featureRow) string {
	icon, label := featureDisplayState(row)
	return icon + " " + label
}

func featureStateStyle(row *featureRow) lipgloss.Style {
	_, label := featureDisplayState(row)
	switch label {
	case "blocked", "input", "stopped", "error":
		return styleHealthErr
	case "review":
		return styleStatusInProgress
	case "done":
		return styleHealthOK
	default:
		if row != nil && row.s != nil {
			return statusStyle(row.s.Status)
		}
		return styleSubtext
	}
}

func featureNeedsAttention(row *featureRow) bool {
	_, label := featureDisplayState(row)
	return label == "blocked" || label == "input" || label == "review"
}

// brokenRow renders a feature whose STATE.yaml could not be parsed: ticket from
// the directory name, a red "broken" status, and the parse error in the stage
// column. The "!" health marker flags it like any other issue.
func brokenRow(row *featureRow, w, wTicket int, selected bool) string {
	ticket := ui.Truncate(row.ticketID(), wTicket)
	reason := "unreadable STATE.yaml"
	if row.loadErr != nil {
		reason = row.loadErr.Error()
	}
	const marker = "⚠ broken — "
	// width left for the reason after the leading space, ticket col, separator,
	// and the marker
	reasonW := w - (1 + wTicket + 2 + lipgloss.Width(marker))
	if reasonW < 0 {
		reasonW = 0
	}
	reason = ui.Truncate(reason, reasonW)

	if selected {
		line := " " + ui.PadRight(ticket, wTicket) + "  " + marker + reason
		return styleRowSelected.Width(w).Render(line)
	}
	return " " +
		ui.PadRight(styleHealthErr.Render(ticket), wTicket) + "  " +
		styleHealthErr.Render("⚠ broken") + styleDim.Render(" — "+reason)
}
