package workspaceui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderTable(rows []*featureRow, w int, selectedIdx int) string {
	const (
		wTicket  = 12
		wStatus  = 12
		wContext = 8
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
		padRight(styleTableHeader.Render("Ticket"), wTicket) + "  " +
		padRight(styleTableHeader.Render("Name"), wName) + "  " +
		padRight(styleTableHeader.Render("Status"), wStatus) + "  " +
		padRight(styleTableHeader.Render("Stage"), wWorkflow) + "  " +
		padRight(styleTableHeader.Render("Worker"), wWorker) + "  " +
		padRight(styleTableHeader.Render("Context"), wContext) + "  " +
		padRight(styleTableHeader.Render("Tmux"), wTmux)

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

		icon := statusIcon(s.Status)
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

		ticketCell := truncate(s.Ticket, wTicket)
		if row.hasIssues {
			ticketCell = truncate("! "+s.Ticket, wTicket)
		}

		if selected {
			// Plain unstyled text so styleRowSelected background covers the full row
			line := " " +
				padRight(ticketCell, wTicket) + "  " +
				padRight(truncate(name, wName), wName) + "  " +
				padRight(truncate(icon+" "+s.Status, wStatus), wStatus) + "  " +
				padRight(truncate(stageCell, wWorkflow), wWorkflow) + "  " +
				padRight(truncate(plainWorker, wWorker), wWorker) + "  " +
				padRight(row.context.Label(), wContext) + "  " +
				padRight(plainTmux, wTmux)
			lines = append(lines, styleRowSelected.Width(w).Render(line))
		} else {
			ticketStyled := ticketCell
			if row.hasIssues {
				ticketStyled = styleHealthWarn.Render(ticketCell)
			}
			statusCell := statusStyle(s.Status).Render(icon + " " + s.Status)
			nameCell := styleDim.Render(truncate(name, wName))
			workerCell := styleDim.Render(truncate(plainWorker, wWorker))
			contextCell := renderContextPressure(row.context)
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
			line := " " +
				padRight(ticketStyled, wTicket) + "  " +
				padRight(nameCell, wName) + "  " +
				padRight(statusCell, wStatus) + "  " +
				padRight(truncate(stageCell, wWorkflow), wWorkflow) + "  " +
				padRight(workerCell, wWorker) + "  " +
				padRight(contextCell, wContext) + "  " +
				padRight(tmuxCell, wTmux)
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// brokenRow renders a feature whose STATE.yaml could not be parsed: ticket from
// the directory name, a red "broken" status, and the parse error in the stage
// column. The "!" health marker flags it like any other issue.
func brokenRow(row *featureRow, w, wTicket int, selected bool) string {
	ticket := truncate(row.ticketID(), wTicket)
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
	reason = truncate(reason, reasonW)

	if selected {
		line := " " + padRight(ticket, wTicket) + "  " + marker + reason
		return styleRowSelected.Width(w).Render(line)
	}
	return " " +
		padRight(styleHealthErr.Render(ticket), wTicket) + "  " +
		styleHealthErr.Render("⚠ broken") + styleDim.Render(" — "+reason)
}
