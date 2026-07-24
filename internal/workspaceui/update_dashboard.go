package workspaceui

import (
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Search mode: route keys to textinput ─────────────────────
	if m.filter.active {
		switch {
		case key.Matches(msg, keys.cancel):
			m.filter.active = false
			m.filter.input.Blur()
			m.filter.input.SetValue("")
			m.navigation.featureCursor = 0
			return m, nil
		case key.Matches(msg, keys.confirm):
			m.filter.active = false
			m.filter.input.Blur()
			m.navigation.featureCursor = 0
			return m, nil
		default:
			var cmd tea.Cmd
			m.filter.input, cmd = m.filter.input.Update(msg)
			m.navigation.featureCursor = 0
			return m, cmd
		}
	}

	// track last 3 keys for "orc" easter egg
	m.effects.keyBuffer[0] = m.effects.keyBuffer[1]
	m.effects.keyBuffer[1] = m.effects.keyBuffer[2]
	m.effects.keyBuffer[2] = msg.String()
	if m.effects.keyBuffer == [3]string{"o", "r", "c"} && m.effects.rainbowStep == 0 {
		m.effects.rainbowStep = rainbowSteps
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
		if m.navigation.pane == paneFeatures {
			m.filter.active = true
			m.filter.input.Focus()
			return m, textinput.Blink
		}

	case key.Matches(msg, keys.cycleForward, keys.cycleBackward):
		m.cycleSectionFocus(key.Matches(msg, keys.cycleForward))

	case key.Matches(msg, keys.back):
		if m.filter.input.Value() != "" {
			m.filter.input.SetValue("")
			m.navigation.featureCursor = 0
		} else if m.navigation.pane == paneSection {
			m.blurSectionFocus()
		}

	case key.Matches(msg, keys.up):
		if m.navigation.pane == paneSection {
			if m.navigation.sectionCursor > 0 {
				m.navigation.sectionCursor--
			}
		} else {
			if m.navigation.featureCursor > 0 {
				m.navigation.featureCursor--
			}
		}

	case key.Matches(msg, keys.down):
		if m.navigation.pane == paneSection {
			items := m.navigation.items[m.navigation.section]
			if m.navigation.sectionCursor < len(items)-1 {
				m.navigation.sectionCursor++
			}
		} else {
			rows := m.visibleFeatures()
			if m.navigation.featureCursor < len(rows)-1 {
				m.navigation.featureCursor++
			}
		}

	case key.Matches(msg, keys.featurePageDown):
		if m.navigation.pane != paneSection {
			m.moveFeatureCursor(m.featuresPageSize())
		}
	case key.Matches(msg, keys.featurePageUp):
		if m.navigation.pane != paneSection {
			m.moveFeatureCursor(-m.featuresPageSize())
		}
	case key.Matches(msg, keys.top):
		if m.navigation.pane != paneSection {
			m.navigation.featureCursor = 0
		}
	case key.Matches(msg, keys.bottom):
		if m.navigation.pane != paneSection {
			m.navigation.featureCursor = len(m.visibleFeatures()) - 1
			if m.navigation.featureCursor < 0 {
				m.navigation.featureCursor = 0
			}
		}

	case key.Matches(msg, keys.archive):
		if m.navigation.pane == paneFeatures {
			m.navigation.showArchived = !m.navigation.showArchived
			m.navigation.featureCursor = 0
		}

	case key.Matches(msg, keys.attach):
		if m.navigation.pane == paneFeatures {
			rows := m.visibleFeatures()
			if m.navigation.featureCursor < len(rows) {
				row := rows[m.navigation.featureCursor]
				if row.s != nil && row.s.Runtime.Tmux != nil && row.tmuxLive {
					return m, attachTmux(row.s.Runtime.Tmux.Session, row.s.Stage.Name)
				}
			}
		}

	case key.Matches(msg, keys.character):
		if m.navigation.pane == paneSection && m.navigation.section == sectionWorkers {
			items := m.navigation.items[sectionWorkers]
			if m.navigation.sectionCursor < len(items) {
				m.effects.charSheetWorker = workerForPath(items[m.navigation.sectionCursor].path, m.data.allWorkers)
				m.effects.charSheetReturn = viewDashboard
				m.view = viewCharacterSheet
				return m, tea.ClearScreen
			}
		}

	case key.Matches(msg, keys.open):
		if m.navigation.pane == paneSection {
			m.openSectionItem()
		} else {
			rows := m.visibleFeatures()
			if m.navigation.featureCursor < len(rows) {
				row := rows[m.navigation.featureCursor]
				if row.s == nil {
					m.openViewer(func(int) string { return renderBrokenFeature(row) },
						row.ticketID(), "broken", viewDashboard)
					return m, nil
				}
				m.detail = detailState{
					feature: row,
					files:   buildFileList(row.featureDir, row.s),
				}
				// Set view before measuring height: viewerHeight consults
				// m.view (via directDestinationHeader) to size the viewport
				// around the pinned banner+title header above it.
				m.view = viewDetail
				m.viewer.viewport = viewport.New(max(1, m.width-4), m.viewerHeight())
				m.viewer.viewport.SetContent(m.renderDetailBody())
				m.detail.scroll = 0
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
	m.navigation.featureCursor += delta
	if m.navigation.featureCursor > n-1 {
		m.navigation.featureCursor = n - 1
	}
	if m.navigation.featureCursor < 0 {
		m.navigation.featureCursor = 0
	}
}

// cycleSectionFocus moves focus through the navigable sections with tab /
// shift+tab, wrapping back to the features pane at either end.
func (m *Model) cycleSectionFocus(forward bool) {
	navigable := m.navigableSections()
	if len(navigable) == 0 {
		return
	}
	if m.navigation.pane != paneSection {
		if forward {
			m.focusSection(navigable[0])
		} else {
			m.focusSection(navigable[len(navigable)-1])
		}
		return
	}
	idx := -1
	for i, k := range navigable {
		if k == m.navigation.section {
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
	if m.navigation.sectionCursors == nil {
		m.navigation.sectionCursors = map[sectionID]int{}
	}
	if m.navigation.pane == paneSection {
		m.navigation.sectionCursors[m.navigation.section] = m.navigation.sectionCursor
	}
	m.collapseAutoExpandedSection()
	m.navigation.pane = paneSection
	m.navigation.section = name
	m.navigation.sectionCursor = m.navigation.sectionCursors[name]
	if !m.navigation.expanded[name] {
		m.navigation.autoExpanded = name
	}
	m.navigation.expanded[name] = true
}

func (m *Model) blurSectionFocus() {
	if m.navigation.sectionCursors == nil {
		m.navigation.sectionCursors = map[sectionID]int{}
	}
	if m.navigation.pane == paneSection {
		m.navigation.sectionCursors[m.navigation.section] = m.navigation.sectionCursor
	}
	m.collapseAutoExpandedSection()
	m.navigation.pane = paneFeatures
	m.navigation.section = sectionNone
	m.navigation.sectionCursor = 0
}

func (m *Model) collapseAutoExpandedSection() {
	if m.navigation.autoExpanded != sectionNone {
		m.navigation.expanded[m.navigation.autoExpanded] = false
		m.navigation.autoExpanded = sectionNone
	}
}

// toggleSection expands or collapses a dashboard section; collapsing the
// focused section returns focus to the features pane.
func (m *Model) toggleSection(name sectionID) {
	wasExpanded := m.navigation.expanded[name]
	if m.navigation.autoExpanded == name {
		m.navigation.autoExpanded = sectionNone
	}
	m.navigation.expanded[name] = !wasExpanded
	if wasExpanded && m.navigation.section == name {
		m.blurSectionFocus()
	}
}

// openSectionItem opens the focused section's selected item: a worker file,
// a workflow detail page, or a plain file view.
func (m *Model) openSectionItem() {
	// Health has no list items — it drills straight into the full doctor report.
	if m.navigation.section == sectionHealth {
		m.openHealthReport(viewDashboard)
		return
	}
	items := m.navigation.items[m.navigation.section]
	if m.navigation.sectionCursor >= len(items) {
		return
	}
	f := items[m.navigation.sectionCursor]
	switch m.navigation.section {
	case sectionWorkers:
		m.effects.charSheetWorker = workerForPath(f.path, m.data.allWorkers)
		m.openViewer(workerRenderer(f.path, m.data.features), f.label, sectionLabel(m.navigation.section), viewDashboard)
		m.viewer.kind = viewerWorker
		m.viewer.path = f.path
	case sectionWorkflows:
		workflowName := f.id
		if workflowName == "" {
			workflowName = f.label
		}
		m.enterWorkflowDetail(workflowName, 0, viewDashboard)
	case sectionRepositories:
		m.openRepositoryReport(viewDashboard)
	default:
		m.openViewer(fileRenderer(f.path), f.label, sectionLabel(m.navigation.section), viewDashboard)
	}
}

// enterWorkflowDetail opens the full-height route chain and stage table for
// the named workflow at the given stage cursor, scrolling the selected stage
// into view. returnView only affects "back": viewDashboard when drilling in
// from the section list returns there, while the self-referential
// viewWorkflowDetail used when opened directly from the tab bar means there
// is no drill-in parent to return to. Both cases render the same pinned
// header.
func (m *Model) enterWorkflowDetail(name string, cursor int, returnView viewState) {
	m.navigation.workflowName = name
	m.navigation.workflowCursor = cursor
	// Set view before measuring height: viewerHeight consults m.view (via
	// directDestinationHeader) to size the viewport around the pinned
	// banner+title header instead of overflowing past it.
	m.view = viewWorkflowDetail
	m.viewer = viewerState{returnView: returnView}
	width, height := max(1, m.width-4), m.viewerHeight()
	content := renderWorkflowDetail(name, m.data.workflows, m.data.allWorkers, filepath.Join(m.root, "stages"), m.data.features, cursor, width)
	view := viewport.New(width, height)
	view.SetContent(content)
	if line := workflowCursorLine(content); line > 0 {
		view.SetYOffset(max(0, line-height/2))
	}
	m.viewer.viewport = view
}

// openDefaultWorkflowDetail opens the workflow detail view directly, as the
// other Workspace destinations do, instead of requiring a drill-in from the
// collapsed section list. It defaults to the previously viewed workflow and
// stage when still valid, otherwise the first available workflow at its top
// stage, so returning to the tab preserves where the user left off.
func (m *Model) openDefaultWorkflowDetail() {
	name := m.navigation.workflowName
	cursor := m.navigation.workflowCursor
	if !workflowKnown(name, m.data.workflows) {
		cursor = 0
		if len(m.data.workflows) > 0 {
			name = m.data.workflows[0].name
		}
	}
	m.enterWorkflowDetail(name, cursor, viewWorkflowDetail)
}

// openViewer switches to the file viewer, rendering content via render at the
// current width. render is retained so the viewer re-flows on resize.
