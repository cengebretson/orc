package watch

import "strings"

func (m Model) renderRail() string {
	width := max(12, m.width)
	inner := max(8, width-1)
	var b strings.Builder
	b.WriteString(titleStyle.Render("ORC"))
	if m.ticket != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(truncate(m.ticket, inner)))
	}
	b.WriteString("\n\n")

	if m.loadErr != nil {
		b.WriteString(blockedStyle.Render("! load error"))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(truncate(m.loadErr.Error(), inner)))
		return b.String()
	}

	b.WriteString(sectionStyle.Render("SESSIONS"))
	b.WriteString("\n")
	if len(m.rows) == 0 {
		b.WriteString(mutedStyle.Render("  none"))
	} else {
		for i, r := range m.rows {
			icon, label := displayState(r)
			prefix := "  "
			if i == m.cursor {
				prefix = "> "
			}
			state := stateStyle(label).Render(icon)
			line := prefix + state + " " + truncate(r.ticket, max(1, inner-4))
			if i == m.cursor {
				line = selectedStyle.Render(line)
			}
			b.WriteString(line)
			if i != len(m.rows)-1 {
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("\n\n")
	if r, ok := m.selectedWork(); ok {
		b.WriteString(sectionStyle.Render("DETAIL"))
		b.WriteString("\n")
		b.WriteString(renderRailDetail(r, inner))
		b.WriteString("\n\n")
	}
	if m.message != "" {
		b.WriteString(mutedStyle.Render(truncate(m.message, inner)))
		b.WriteString("\n")
	}

	b.WriteString(mutedStyle.Render("enter expand  a attach"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("i focus  q quit"))
	return b.String()
}

func (m Model) renderWorkListWide(b *strings.Builder, width int) {
	ticketW := min(18, max(10, width/4))
	stageW := min(18, max(10, width/5))
	workerW := min(18, max(8, width/5))
	stateW := 10

	b.WriteString(mutedStyle.Render(linef("%-2s%-*s  %-*s  %-*s  %-*s  %s",
		"",
		ticketW, "Ticket",
		stageW, "Stage",
		workerW, "Worker",
		stateW, "State",
		"Tmux",
	)))
	b.WriteString("\n")
	for i, r := range m.rows {
		icon, label := displayState(r)
		stateText := icon + " " + label
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}
		line := linef("%-2s%-*s  %-*s  %-*s  %-*s  %s",
			prefix,
			ticketW, truncate(r.ticket, ticketW),
			stageW, truncate(r.stage, stageW),
			workerW, truncate(r.worker, workerW),
			stateW, truncate(stateText, stateW),
			r.tmuxState,
		)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(stateStyle(label).Render(line))
		}
		b.WriteString("\n")
	}
}

func (m Model) renderWide() string {
	width := max(40, m.width)
	var b strings.Builder
	title := "ORC WATCH"
	if m.ticket != "" {
		title += " " + m.ticket
	}
	b.WriteString(titleStyle.Render(title))
	if !m.lastLoad.IsZero() {
		b.WriteString(mutedStyle.Render("  " + m.lastLoad.Format("15:04:05")))
	}
	b.WriteString("\n\n")

	if m.loadErr != nil {
		b.WriteString(blockedStyle.Render("load error: "))
		b.WriteString(m.loadErr.Error())
		return b.String()
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

	if m.message != "" {
		b.WriteString(mutedStyle.Render(truncate(m.message, width)))
		b.WriteString("\n")
	}
	b.WriteString(mutedStyle.Render("j/k move  enter preview  a attach  i focus  r refresh  q quit"))
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) renderPreview() string {
	content := m.viewport.View()
	if strings.TrimSpace(content) == "" {
		content = m.previewContent()
	}
	return content
}

func (m Model) previewContent() string {
	if r, ok := m.selectedWork(); ok {
		return m.workPreviewContent(r)
	}
	return mutedStyle.Render("No item selected.")
}

func renderRailDetail(r row, width int) string {
	icon, label := displayState(r)
	var b strings.Builder
	b.WriteString(stateStyle(label).Render(icon + " " + truncate(label, width-2)))
	if r.stage != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate(r.stage, max(1, width-2))))
	}
	if r.worker != "" && r.worker != "-" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate(r.worker, max(1, width-2))))
	}
	if r.tmuxState != "" && r.tmuxState != "-" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate("tmux "+r.tmuxState, max(1, width-2))))
	}
	if r.attention != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate("attention "+r.attention, max(1, width-2))))
	}
	if r.next != "" {
		b.WriteString("\n\n")
		b.WriteString(sectionStyle.Render(promptLabel(r)))
		b.WriteString("\n")
		b.WriteString(wrap(r.next, width))
	}
	return b.String()
}

func promptLabel(r row) string {
	_, label := displayState(r)
	if label == "blocked" {
		return "Blocker"
	}
	return "Next"
}

func renderHistory(rows []historyRow, width, limit int) string {
	if len(rows) == 0 {
		return mutedStyle.Render("none")
	}
	start := 0
	if limit > 0 && len(rows) > limit {
		start = len(rows) - limit
	}
	if width < 48 {
		return renderNarrowHistory(rows[start:], width)
	}
	dateW := 10
	stageW := min(18, max(10, width/5))
	workerW := min(14, max(8, width/6))
	resultW := max(8, width-dateW-stageW-workerW-8)
	var b strings.Builder
	for i, h := range rows[start:] {
		at := h.at
		if len(at) > dateW {
			at = at[:dateW]
		}
		line := linef("%-*s  %-*s  %-*s  %s",
			dateW, truncate(at, dateW),
			stageW, truncate(h.stage, stageW),
			workerW, truncate(h.worker, workerW),
			truncate(h.result, resultW),
		)
		b.WriteString(mutedStyle.Render(line))
		if i != len(rows[start:])-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderNarrowHistory(rows []historyRow, width int) string {
	var b strings.Builder
	for i, h := range rows {
		at := h.at
		if len(at) > 10 {
			at = at[:10]
		}
		meta := strings.TrimSpace(strings.Join([]string{at, h.stage, h.worker}, " "))
		b.WriteString(mutedStyle.Render(truncate(meta, width)))
		if h.result != "" {
			b.WriteString("\n")
			b.WriteString(wrap(h.result, width))
		}
		if i != len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) workPreviewContent(r row) string {
	icon, label := displayState(r)
	width := max(20, m.width)
	bodyW := max(12, width-2)

	var b strings.Builder
	b.WriteString(titleStyle.Render("ORC"))
	b.WriteString("\n\n")
	b.WriteString(selectedStyle.Render(truncate(r.ticket, bodyW)))
	b.WriteString("\n")
	b.WriteString(truncate(r.stage, bodyW))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render(truncate(r.worker, bodyW)))
	b.WriteString("\n\n")
	b.WriteString(stateStyle(label).Render(icon + " " + label))
	if r.tmuxState != "" && r.tmuxState != "-" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("tmux " + r.tmuxState))
	}
	if r.attention != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("attention " + r.attention))
	}
	if r.loadErr != nil {
		b.WriteString("\n\n")
		b.WriteString(blockedStyle.Render("Error"))
		b.WriteString("\n")
		b.WriteString(wrap(r.loadErr.Error(), bodyW))
	} else if r.next != "" {
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render(promptLabel(r)))
		b.WriteString("\n")
		b.WriteString(wrap(r.next, bodyW))
	}
	if len(r.history) > 0 {
		b.WriteString("\n\n")
		b.WriteString(sectionStyle.Render("History"))
		b.WriteString("\n")
		b.WriteString(renderHistory(r.history, bodyW, 8))
	}
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("esc back  q quit"))
	return b.String()
}

func wrap(s string, width int) string {
	width = max(8, width)
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	lines = append(lines, line)
	return strings.Join(lines, "\n")
}
