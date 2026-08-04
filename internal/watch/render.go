package watch

import (
	"fmt"
	"strings"
	"time"

	terminalui "github.com/cengebretson/orc/internal/ui"
)

func (m Model) renderRail() string {
	if m.width > 0 && m.width <= 7 {
		return m.renderCollapsedRail()
	}
	width := max(12, m.width)
	inner := max(8, width-1)
	var b strings.Builder
	b.WriteString(renderWatchOverview(m.rows, inner, m.lastLoad, m.renderNow()))
	if m.ticket != "" {
		b.WriteString("\n")
		b.WriteString(accentStyle.Render(truncate(m.ticket, inner)))
	}
	b.WriteString("\n\n")
	if m.searching || m.searchBox.Value() != "" {
		b.WriteString(m.renderFilter(inner))
		b.WriteString("\n\n")
	}

	if m.loadErr != nil {
		b.WriteString(blockedStyle.Render("! load error"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(truncate(m.loadErr.Error(), inner)))
		return b.String()
	}
	if m.loadWarning != nil {
		b.WriteString(blockedStyle.Render("! parking warning"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(truncate(m.loadWarning.Error(), inner)))
		b.WriteString("\n\n")
	}

	if len(m.rows) == 0 {
		if m.parkedCount() == 0 {
			b.WriteString(mutedStyle.Render("No active work"))
		}
	} else {
		for i, r := range m.rows {
			if r.parked && (i == 0 || !m.rows[i-1].parked) {
				b.WriteString(mutedStyle.Render(fmt.Sprintf("  ▾ Parked (%d) · p to toggle", m.parkedCount())))
				b.WriteString("\n")
			}
			b.WriteString(renderRailRow(r, i == m.cursor, inner, m.renderNow(), m.uiFrame))
			if i != len(m.rows)-1 {
				b.WriteString("\n")
			}
		}
	}
	if count := m.parkedCount(); count > 0 && !m.parkedExpanded {
		if len(m.rows) > 0 {
			b.WriteString("\n")
		}
		marker := "▸"
		if m.parkedExpanded {
			marker = "▾"
		}
		b.WriteString(mutedStyle.Render(fmt.Sprintf("  %s Parked (%d) · p to toggle", marker, count)))
	}

	b.WriteString("\n\n")
	if r, ok := m.selectedWork(); ok {
		cardWidth := max(1, inner-4)
		contentWidth := max(8, cardWidth-2)
		b.WriteString(selectedCardStyle.Width(cardWidth).Render(renderRailDetailAt(r, contentWidth, m.renderNow())))
		b.WriteString("\n\n")
	}
	if m.message != "" {
		b.WriteString(mutedStyle.Render(truncate(m.message, inner)))
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderCollapsedRail() string {
	width := max(1, m.width)
	rowLimit := len(m.rows)
	if m.height > 1 {
		rowLimit = min(rowLimit, m.height-1)
	}
	lines := make([]string, 0, max(rowLimit+1, m.height))
	for i := 0; i < rowLimit; i++ {
		icon, label := displayState(m.rows[i])
		icon = animatedStateIcon(m.rows[i], icon, label, m.renderNow(), m.uiFrame)
		if m.rows[i].woken {
			icon, label = "↟", "woken"
		}
		lines = append(lines, centerRailCell(stateStyle(label).Render(icon), width))
	}
	if m.height > 0 {
		for len(lines) < m.height-1 {
			lines = append(lines, "")
		}
	}
	lines = append(lines, centerRailCell(accentStyle.Render("»"), width))
	return strings.Join(lines, "\n")
}

func centerRailCell(value string, width int) string {
	return strings.Repeat(" ", max(0, (width-1)/2)) + value
}

func renderRailRow(r row, selected bool, width int, now time.Time, frame int) string {
	icon, label := displayState(r)
	icon = animatedStateIcon(r, icon, label, now, frame)
	if r.woken {
		icon, label = "↟", "woken"
	}
	style := selectedRowStyle
	if now.Before(r.celebrateUntil) || r.demoCelebration {
		style = doneFlashStyle
	} else if now.Before(r.flashUntil) {
		style = transitionRowStyle
	}
	badge := lifecycleAgeBadge(r, now)
	suffix := ""
	if badge != "" {
		suffix = " " + badge
	}
	ticketWidth := max(1, width-4-len(suffix))
	if selected {
		content := "▌ " + icon + " " + truncate(r.ticket, ticketWidth) + suffix
		return style.Width(width).Render(content)
	}
	if now.Before(r.celebrateUntil) || r.demoCelebration || now.Before(r.flashUntil) {
		return style.Width(width).Render("  " + icon + " " + truncate(r.ticket, ticketWidth) + suffix)
	}
	return "  " + stateStyle(label).Render(icon) + " " + truncate(r.ticket, ticketWidth) + mutedStyle.Render(suffix)
}

const stuckLifecycleAge = 15 * time.Minute

func lifecycleAgeBadge(r row, now time.Time) string {
	if r.lifecycleSince.IsZero() || now.IsZero() || r.lifecycleSince.After(now) {
		return ""
	}
	age := now.Sub(r.lifecycleSince)
	value := humanDuration(age)
	threshold := r.stuckAfter
	if threshold <= 0 {
		threshold = stuckLifecycleAge
	}
	if r.liveState == "working" && age >= threshold {
		return "stuck " + value
	}
	return value
}

func (m Model) renderNow() time.Time {
	if !m.now.IsZero() {
		return m.now
	}
	if !m.lastLoad.IsZero() {
		return m.lastLoad
	}
	return time.Now()
}

func animatedStateIcon(r row, icon, label string, now time.Time, frame int) string {
	if now.Before(r.celebrateUntil) || r.demoCelebration {
		if frame%2 == 0 {
			return "✦"
		}
		return "★"
	}
	if label == "active" && r.tmuxState == "live" && frame%2 == 1 {
		return "◉"
	}
	return icon
}

func (m Model) renderWorkListWide(b *strings.Builder, width int) {
	ticketW, stageW, workerW, stateW, contextW, tmuxW := wideColumnWidths(width)

	b.WriteString(mutedStyle.Render("  " +
		terminalui.Cell("Ticket", ticketW) + "  " +
		terminalui.Cell("Stage", stageW) + "  " +
		terminalui.Cell("Worker", workerW) + "  " +
		terminalui.Cell("State", stateW) + "  " +
		terminalui.Cell("Context", contextW) + "  " +
		terminalui.Cell("Tmux", tmuxW)))
	b.WriteString("\n")
	for i, r := range m.rows {
		if r.parked && (i == 0 || !m.rows[i-1].parked) {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("  ▾ Parked (%d) · p to toggle", m.parkedCount())))
			b.WriteString("\n")
		}
		icon, label := displayState(r)
		icon = animatedStateIcon(r, icon, label, m.renderNow(), m.uiFrame)
		if r.woken {
			icon, label = "↟", "woken"
		}
		stateText := icon + " " + label
		prefix := "  "
		if i == m.cursor {
			prefix = "▌ "
		}
		linePrefix := terminalui.Cell(prefix, 2) +
			terminalui.Cell(r.ticket, ticketW) + "  " +
			terminalui.Cell(r.stage, stageW) + "  " +
			terminalui.Cell(r.worker, workerW) + "  " +
			terminalui.Cell(stateText, stateW) + "  "
		tmuxText := truncate(r.tmuxState, tmuxW)
		line := linePrefix + terminalui.Cell(r.context.Label(), contextW) + "  " + terminalui.Cell(tmuxText, tmuxW)
		flash := m.renderNow().Before(r.celebrateUntil) || r.demoCelebration || m.renderNow().Before(r.flashUntil)
		if i == m.cursor || flash {
			style := selectedRowStyle
			if m.renderNow().Before(r.celebrateUntil) || r.demoCelebration {
				style = doneFlashStyle
			} else if m.renderNow().Before(r.flashUntil) {
				style = transitionRowStyle
			}
			b.WriteString(style.Render(line))
		} else {
			b.WriteString(stateStyle(label).Render(linePrefix))
			b.WriteString(padStyledRight(renderContextPressure(r.context), contextW))
			b.WriteString(stateStyle(label).Render("  "))
			b.WriteString(padStyledRight(stateStyle(label).Render(tmuxText), tmuxW))
		}
		b.WriteString("\n")
	}
	if count := m.parkedCount(); count > 0 && !m.parkedExpanded {
		marker := "▸"
		if m.parkedExpanded {
			marker = "▾"
		}
		b.WriteString(mutedStyle.Render(fmt.Sprintf("  %s Parked (%d) · p to toggle", marker, count)))
		b.WriteString("\n")
	}
}

func wideColumnWidths(width int) (ticketW, stageW, workerW, stateW, contextW, tmuxW int) {
	stateW, contextW = 10, 8
	if width < 68 {
		stateW, contextW = 8, 7
	}
	// Prefix and the five two-column gaps consume 12 columns. Everything else
	// is distributed so the table expands exactly with the terminal.
	available := max(29, width-12-stateW-contextW)
	tmuxW = max(4, available/7)
	remaining := max(25, available-tmuxW)
	ticketW = max(9, remaining*34/100)
	stageW = max(8, remaining*33/100)
	workerW = max(8, remaining-ticketW-stageW)
	return
}

func (m Model) renderWide() string {
	width := max(40, m.width)
	var b strings.Builder
	b.WriteString(renderWatchOverview(m.rows, width, m.lastLoad, m.renderNow()))
	b.WriteString("\n\n")
	if m.searching || m.searchBox.Value() != "" {
		b.WriteString(m.renderFilter(width))
		b.WriteString("\n\n")
	}

	if m.loadErr != nil {
		b.WriteString(blockedStyle.Render("load error: "))
		b.WriteString(m.loadErr.Error())
		return b.String()
	}
	if m.loadWarning != nil {
		b.WriteString(blockedStyle.Render("parking warning: "))
		b.WriteString(m.loadWarning.Error())
		b.WriteString("\n\n")
	}
	if len(m.rows) == 0 {
		b.WriteString(mutedStyle.Render("No active work found."))
		b.WriteString("\n\n")
	} else {
		b.WriteString(sectionStyle.Render("SESSIONS"))
		b.WriteString("\n")
		m.renderWorkListWide(&b, width)
		b.WriteString("\n")
	}
	if r, ok := m.selectedWork(); ok {
		panelWidth := max(1, width-2)
		contentWidth := max(8, panelWidth-2)
		b.WriteString(selectedCardStyle.Width(panelWidth).Render(renderRailDetailAt(r, contentWidth, m.renderNow())))
		b.WriteString("\n\n")
	}

	if m.message != "" {
		b.WriteString(mutedStyle.Render(truncate(m.message, width)))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderFilter(width int) string {
	if m.searching {
		return truncate(m.searchBox.View(), width)
	}
	return mutedStyle.Render(truncate(fmt.Sprintf("/ %s  (%d/%d)", m.searchBox.Value(), len(m.rows), len(m.allRows)), width))
}

func (m Model) renderPromptAction() string {
	width := max(24, m.width)
	inner := min(72, max(20, width-4))
	var content strings.Builder
	content.WriteString(selectedStyle.Render("SEND TO " + truncate(m.promptRow.ticket, max(1, inner-8))))
	content.WriteString("\n\n")
	if m.prompting {
		content.WriteString(truncate(m.promptBox.View(), inner))
		content.WriteString("\n\n")
		content.WriteString(mutedStyle.Render("enter review · esc cancel"))
	} else {
		content.WriteString(wrap(m.promptBox.Value(), inner))
		content.WriteString("\n\n")
		content.WriteString(blockedStyle.Render("Send this prompt? y / n"))
	}
	if m.message != "" {
		content.WriteString("\n\n")
		content.WriteString(mutedStyle.Render(truncate(m.message, inner)))
	}
	var b strings.Builder
	b.WriteString(watchHeaderStyle.Width(inner + 4).Render(" ORC WATCH · PROMPT"))
	b.WriteString("\n\n")
	b.WriteString(selectedCardStyle.Width(inner).Render(content.String()))
	return b.String()
}

func (m Model) renderPreview() string {
	content := m.viewport.View()
	if strings.TrimSpace(content) == "" {
		content = m.previewContent()
	}
	return content
}

func (m Model) renderHelp() string {
	width := max(24, m.width)
	inner := min(48, max(20, width-4))
	var content strings.Builder
	sections := HelpSections()
	for sectionIndex, section := range sections {
		if sectionIndex > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(selectedStyle.Render(strings.TrimPrefix(section.Title, "LIVE · ")))
		for _, entry := range section.Entries {
			content.WriteString("\n")
			content.WriteString(truncate(terminalui.PadRight(entry.Keys, 18)+entry.Description, inner))
		}
	}
	var b strings.Builder
	b.WriteString(watchHeaderStyle.Width(inner + 4).Render(" ORC WATCH · HELP"))
	b.WriteString("\n\n")
	b.WriteString(selectedCardStyle.Width(inner).Render(content.String()))
	return b.String()
}
