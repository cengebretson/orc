package watch

import (
	"fmt"
	"strings"
	"time"
)

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
		line := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
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

func renderTimelineHistory(rows []historyRow, width, limit int) string {
	if len(rows) == 0 {
		return mutedStyle.Render("none")
	}
	start := 0
	if limit > 0 && len(rows) > limit {
		start = len(rows) - limit
	}
	rows = rows[start:]
	var b strings.Builder
	for i, h := range rows {
		at := h.at
		if parsed, err := time.Parse(time.RFC3339, h.at); err == nil {
			at = parsed.Format("Jan 02 15:04")
		}
		meta := joinNonEmpty(" · ", at, h.stage, h.worker)
		b.WriteString(accentStyle.Render("● "))
		b.WriteString(truncate(meta, max(1, width-2)))
		if h.result != "" {
			for _, line := range strings.Split(wrap(h.result, max(8, width-2)), "\n") {
				b.WriteString("\n")
				b.WriteString(mutedStyle.Render("│ "))
				b.WriteString(line)
			}
		}
		if i != len(rows)-1 {
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render("│"))
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
