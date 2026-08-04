package watch

import (
	"strings"
	"time"
)

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
