package workspaceui

import (
	"path/filepath"

	"github.com/cengebretson/orc/internal/config"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Search mode: route keys to textinput ─────────────────────
	if m.searching {
		switch {
		case key.Matches(msg, keys.cancel):
			m.searching = false
			m.search.Blur()
			m.search.SetValue("")
			m.cursor = 0
			return m, nil
		case key.Matches(msg, keys.confirm):
			m.searching = false
			m.search.Blur()
			m.cursor = 0
			return m, nil
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.cursor = 0
			return m, cmd
		}
	}

	// track last 3 keys for "orc" easter egg
	m.keyBuffer[0] = m.keyBuffer[1]
	m.keyBuffer[1] = m.keyBuffer[2]
	m.keyBuffer[2] = msg.String()
	if m.keyBuffer == [3]string{"o", "r", "c"} && m.rainbowStep == 0 {
		m.rainbowStep = rainbowSteps
		return m, rainbowTick()
	}

	if section, ok := sectionForShortcut(msg.String()); ok {
		m.toggleSection(section)
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case key.Matches(msg, keys.refresh):
		return m, loadData(m.root)
	case key.Matches(msg, keys.filter):
		if m.focusedPane == paneFeatures {
			m.searching = true
			m.search.Focus()
			return m, textinput.Blink
		}

	case key.Matches(msg, keys.cycleForward, keys.cycleBackward):
		m.cycleSectionFocus(key.Matches(msg, keys.cycleForward))

	case key.Matches(msg, keys.back):
		if m.search.Value() != "" {
			m.search.SetValue("")
			m.cursor = 0
		} else if m.focusedPane == paneSection {
			m.blurSectionFocus()
		}

	case key.Matches(msg, keys.up):
		if m.focusedPane == paneSection {
			if m.sectionCursor > 0 {
				m.sectionCursor--
			}
		} else {
			if m.cursor > 0 {
				m.cursor--
			}
		}

	case key.Matches(msg, keys.down):
		if m.focusedPane == paneSection {
			items := m.sectionItems[m.sectionFocus]
			if m.sectionCursor < len(items)-1 {
				m.sectionCursor++
			}
		} else {
			rows := m.visibleFeatures()
			if m.cursor < len(rows)-1 {
				m.cursor++
			}
		}

	case key.Matches(msg, keys.featurePageDown):
		if m.focusedPane != paneSection {
			m.moveFeatureCursor(m.featuresPageSize())
		}
	case key.Matches(msg, keys.featurePageUp):
		if m.focusedPane != paneSection {
			m.moveFeatureCursor(-m.featuresPageSize())
		}
	case key.Matches(msg, keys.top):
		if m.focusedPane != paneSection {
			m.cursor = 0
		}
	case key.Matches(msg, keys.bottom):
		if m.focusedPane != paneSection {
			m.cursor = len(m.visibleFeatures()) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}

	case key.Matches(msg, keys.archive):
		if m.focusedPane == paneFeatures {
			m.showArchived = !m.showArchived
			m.cursor = 0
		}

	case key.Matches(msg, keys.attach):
		if m.focusedPane == paneFeatures {
			rows := m.visibleFeatures()
			if m.cursor < len(rows) {
				row := rows[m.cursor]
				if row.s != nil && row.s.Runtime.Tmux != nil && row.tmuxLive {
					return m, attachTmux(row.s.Runtime.Tmux.Session, row.s.Stage.Name)
				}
			}
		}

	case key.Matches(msg, keys.character):
		if m.focusedPane == paneSection && m.sectionFocus == sectionWorkers {
			items := m.sectionItems[sectionWorkers]
			if m.sectionCursor < len(items) {
				m.charSheetWorker = workerForPath(items[m.sectionCursor].path, m.allWorkers)
				m.charSheetReturn = viewDashboard
				m.view = viewCharacterSheet
				return m, tea.ClearScreen
			}
		}

	case key.Matches(msg, keys.open):
		if m.focusedPane == paneSection {
			m.openSectionItem()
		} else {
			rows := m.visibleFeatures()
			if m.cursor < len(rows) {
				row := rows[m.cursor]
				if row.s == nil {
					m.openViewer(func(int) string { return renderBrokenFeature(row) },
						row.ticketID(), "broken", viewDashboard)
					return m, nil
				}
				m.detail = row
				m.detailFiles = buildFileList(m.detail.featureDir, m.detail.s)
				m.fileIdx = 0
				m.viewport = viewport.New(m.width-4, m.height-6)
				m.viewport.SetContent(m.renderDetailBody())
				m.detailScroll = 0
				m.view = viewDetail
			}
		}
	}

	return m, nil
}

// featuresPageSize approximates one screenful of feature rows for pgup/pgdn.
// The exact visible-row count depends on which sections are expanded, but the
// render's scroll window keeps the cursor visible regardless, so an estimate is
// fine.
func (m Model) featuresPageSize() int {
	p := m.height - 8
	if p < 1 {
		p = 1
	}
	return p
}

// moveFeatureCursor shifts the features cursor by delta, clamped to the list.
func (m *Model) moveFeatureCursor(delta int) {
	n := len(m.visibleFeatures())
	m.cursor += delta
	if m.cursor > n-1 {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// cycleSectionFocus moves focus through the navigable sections with tab /
// shift+tab, wrapping back to the features pane at either end.
func (m *Model) cycleSectionFocus(forward bool) {
	navigable := m.navigableSections()
	if len(navigable) == 0 {
		return
	}
	if m.focusedPane != paneSection {
		if forward {
			m.focusSection(navigable[0])
		} else {
			m.focusSection(navigable[len(navigable)-1])
		}
		return
	}
	idx := -1
	for i, k := range navigable {
		if k == m.sectionFocus {
			idx = i
			break
		}
	}
	next := idx - 1
	if forward {
		next = idx + 1
	}
	if next < 0 || next >= len(navigable) {
		m.blurSectionFocus()
	} else {
		m.focusSection(navigable[next])
	}
}

func (m *Model) focusSection(name sectionID) {
	m.collapseAutoExpandedSection()
	m.focusedPane = paneSection
	m.sectionFocus = name
	m.sectionCursor = 0
	if !m.expanded[name] {
		m.autoExpandedSection = name
	}
	m.expanded[name] = true
}

func (m *Model) blurSectionFocus() {
	m.collapseAutoExpandedSection()
	m.focusedPane = paneFeatures
	m.sectionFocus = sectionNone
	m.sectionCursor = 0
}

func (m *Model) collapseAutoExpandedSection() {
	if m.autoExpandedSection != sectionNone {
		m.expanded[m.autoExpandedSection] = false
		m.autoExpandedSection = sectionNone
	}
}

// toggleSection expands or collapses a dashboard section; collapsing the
// focused section returns focus to the features pane.
func (m *Model) toggleSection(name sectionID) {
	wasExpanded := m.expanded[name]
	if m.autoExpandedSection == name {
		m.autoExpandedSection = sectionNone
	}
	m.expanded[name] = !wasExpanded
	if wasExpanded && m.sectionFocus == name {
		m.blurSectionFocus()
	}
}

// openSectionItem opens the focused section's selected item: a worker file,
// a workflow detail page, or a plain file view.
func (m *Model) openSectionItem() {
	// Health has no list items — it drills straight into the full doctor report.
	if m.sectionFocus == sectionHealth {
		checks := m.healthItems
		m.openViewer(func(w int) string { return renderHealthReport(checks, w) },
			"doctor report", "Health", viewDashboard)
		m.viewerKind = viewerHealth
		return
	}
	items := m.sectionItems[m.sectionFocus]
	if m.sectionCursor >= len(items) {
		return
	}
	f := items[m.sectionCursor]
	switch m.sectionFocus {
	case sectionWorkers:
		m.charSheetWorker = workerForPath(f.path, m.allWorkers)
		m.openViewer(workerRenderer(f.path, m.features), f.label, sectionLabel(m.sectionFocus), viewDashboard)
		m.viewerKind = viewerWorker
		m.viewerPath = f.path
	case sectionWorkflows:
		workflowName := f.id
		if workflowName == "" {
			workflowName = f.label
		}
		m.wfDetailName = workflowName
		m.wfDetailCursor = 0
		content := renderWorkflowDetail(workflowName, m.workflows, m.allWorkers, filepath.Join(m.root, "stages"), m.features, 0, m.width-4)
		m.viewport = viewport.New(m.width-4, m.height-6)
		m.viewport.SetContent(content)
		m.view = viewWorkflowDetail
	case sectionRepositories:
		repos := append([]config.Repo(nil), m.repos...)
		routes := append([]config.RepoRoute(nil), m.routes...)
		features := append([]*featureRow(nil), m.features...)
		m.openViewer(func(w int) string { return renderRoutingReport(repos, routes, features, w) },
			"map", "Repositories", viewDashboard)
		m.viewerKind = viewerRepositories
	default:
		m.openViewer(fileRenderer(f.path), f.label, sectionLabel(m.sectionFocus), viewDashboard)
	}
}

// openViewer switches to the file viewer, rendering content via render at the
// current width. render is retained so the viewer re-flows on resize.
