package watch

import (
	"strings"

	"github.com/cengebretson/orc/internal/searchmatch"
	terminalui "github.com/cengebretson/orc/internal/ui"
)

func (m *Model) refreshPreview() {
	if !m.preview {
		return
	}
	m.viewport.SetContent(m.previewContent())
}

func (m Model) itemCount() int {
	return len(m.rows)
}

func (m *Model) applyFilter(resetCursor bool) {
	if m.allRows == nil {
		return
	}
	query := m.searchBox.Value()
	rows := make([]row, 0, len(m.allRows))
	for _, candidate := range m.allRows {
		if searchmatch.Match(query, candidate.search...) {
			rows = append(rows, candidate)
		}
	}
	m.rows = rows
	if resetCursor {
		m.cursor = 0
	}
}

func (m Model) selectedWork() (row, bool) {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func truncate(s string, width int) string {
	return terminalui.Truncate(strings.TrimSpace(s), width)
}
