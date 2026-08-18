package workspaceui

import (
	"path/filepath"

	"github.com/cengebretson/orc/internal/config"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case key.Matches(msg, keys.back):
		m.view = viewDashboard
	case key.Matches(msg, keys.cycleForward, keys.next):
		if m.detail.fileIndex < len(m.detail.files)-1 {
			m.detail.fileIndex++
			m.reRenderDetail() // refresh the selected file chip
		}
	case key.Matches(msg, keys.cycleBackward, keys.previous):
		if m.detail.fileIndex > 0 {
			m.detail.fileIndex--
			m.reRenderDetail()
		}
	case key.Matches(msg, keys.attach):
		if m.detail.feature.tmuxLive {
			if target, ok := m.detail.feature.s.Runtime.MuxTarget(m.detail.feature.s.Stage.Name); ok {
				return m, attachMux(m.mux, target)
			}
		}
	case key.Matches(msg, keys.openLink):
		return m, featureLinkCommand(m.detail.feature, linkActionOpen)
	case key.Matches(msg, keys.copyLink):
		return m, featureLinkCommand(m.detail.feature, linkActionCopy)
	case key.Matches(msg, keys.checks):
		return m, featureLinkCommand(m.detail.feature, linkActionChecks)
	case key.Matches(msg, keys.open):
		if m.detail.fileIndex < len(m.detail.files) {
			f := m.detail.files[m.detail.fileIndex]
			m.detail.scroll = m.viewer.viewport.YOffset // restore on return from the file viewer
			m.openViewer(fileRenderer(f.path), f.label, m.detail.feature.s.Ticket, viewDetail)
		}
	default:
		var cmd tea.Cmd
		m.viewer.viewport, cmd = m.viewer.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleWorkflowDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case key.Matches(msg, keys.back):
		m.view = viewDashboard
	case key.Matches(msg, keys.previous):
		if m.navigation.workflowCursor > 0 {
			m.navigation.workflowCursor--
			m.reRenderWorkflowDetailAndScroll()
		}
	case key.Matches(msg, keys.next):
		if m.navigation.workflowCursor < wfDetailTotal(m.navigation.workflowName, m.data.workflows)-1 {
			m.navigation.workflowCursor++
			m.reRenderWorkflowDetailAndScroll()
		}
	case key.Matches(msg, keys.up):
		m.viewer.viewport.ScrollUp(1)
	case key.Matches(msg, keys.down):
		m.viewer.viewport.ScrollDown(1)
	case key.Matches(msg, keys.pageUp):
		m.viewer.viewport.PageUp()
	case key.Matches(msg, keys.pageDown):
		m.viewer.viewport.PageDown()
	case key.Matches(msg, keys.halfPageUp):
		m.viewer.viewport.HalfPageUp()
	case key.Matches(msg, keys.halfPageDown):
		m.viewer.viewport.HalfPageDown()
	case key.Matches(msg, keys.top):
		m.viewer.viewport.GotoTop()
	case key.Matches(msg, keys.bottom):
		m.viewer.viewport.GotoBottom()
	case key.Matches(msg, keys.open):
		stageName, advance, stepNum, total := wfDetailSelectedStage(m.navigation.workflowName, m.navigation.workflowCursor, m.data.workflows)
		if stageName != "" {
			stagePath := filepath.Join(m.root, "stages", config.ResourcePath(stageName))
			title := stageViewerTitle(stageName, advance, stepNum, total, m.data.workflows)
			m.openViewer(fileRenderer(stagePath), title, m.navigation.workflowName, viewWorkflowDetail)
		}
	default:
		var cmd tea.Cmd
		m.viewer.viewport, cmd = m.viewer.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleFileKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case m.isDirectWorkers() && key.Matches(msg, keys.up, keys.previous):
		m.selectDirectWorker(-1)
	case m.isDirectWorkers() && key.Matches(msg, keys.down, keys.next):
		m.selectDirectWorker(1)
	case key.Matches(msg, keys.back):
		switch m.viewer.returnView {
		case viewWorkflowDetail:
			m.reRenderWorkflowDetailAndScroll()
		case viewDetail:
			// the viewport holds file content — rebuild the detail body and
			// restore the scroll position we left from.
			m.viewer.viewport.SetContent(m.renderDetailBody())
			m.viewer.viewport.SetYOffset(m.detail.scroll)
		}
		m.view = m.viewer.returnView
	case key.Matches(msg, keys.up):
		m.viewer.viewport.ScrollUp(1)
	case key.Matches(msg, keys.down):
		m.viewer.viewport.ScrollDown(1)
	case key.Matches(msg, keys.pageUp):
		m.viewer.viewport.PageUp()
	case key.Matches(msg, keys.pageDown):
		m.viewer.viewport.PageDown()
	case key.Matches(msg, keys.halfPageUp):
		m.viewer.viewport.HalfPageUp()
	case key.Matches(msg, keys.halfPageDown):
		m.viewer.viewport.HalfPageDown()
	case key.Matches(msg, keys.top):
		m.viewer.viewport.GotoTop()
	case key.Matches(msg, keys.bottom):
		m.viewer.viewport.GotoBottom()
	case key.Matches(msg, keys.previous):
		switch m.viewer.returnView {
		case viewDetail:
			if m.detail.fileIndex > 0 {
				m.detail.fileIndex--
				m.loadViewerFile()
			}
		case viewWorkflowDetail:
			if m.navigation.workflowCursor > 0 {
				m.navigation.workflowCursor--
				m.loadViewerStage()
			}
		}
	case key.Matches(msg, keys.next):
		switch m.viewer.returnView {
		case viewDetail:
			if m.detail.fileIndex < len(m.detail.files)-1 {
				m.detail.fileIndex++
				m.loadViewerFile()
			}
		case viewWorkflowDetail:
			if m.navigation.workflowCursor < wfDetailTotal(m.navigation.workflowName, m.data.workflows)-1 {
				m.navigation.workflowCursor++
				m.loadViewerStage()
			}
		}
	case key.Matches(msg, keys.character):
		if m.effects.charSheetWorker != nil {
			m.effects.charSheetReturn = viewFile
			m.view = viewCharacterSheet
			return m, tea.ClearScreen
		}
	default:
		var cmd tea.Cmd
		m.viewer.viewport, cmd = m.viewer.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleCharacterSheetKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case key.Matches(msg, keys.character, keys.back):
		m.view = m.effects.charSheetReturn
		return m, tea.ClearScreen
	}
	return m, nil
}
