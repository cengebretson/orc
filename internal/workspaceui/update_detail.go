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
		if m.fileIdx < len(m.detailFiles)-1 {
			m.fileIdx++
			m.reRenderDetail() // refresh the selected file chip
		}
	case key.Matches(msg, keys.cycleBackward, keys.previous):
		if m.fileIdx > 0 {
			m.fileIdx--
			m.reRenderDetail()
		}
	case key.Matches(msg, keys.attach):
		if m.detail.s.Runtime.Tmux != nil && m.detail.tmuxLive {
			return m, attachTmux(m.detail.s.Runtime.Tmux.Session, m.detail.s.Stage.Name)
		}
	case key.Matches(msg, keys.open):
		if m.fileIdx < len(m.detailFiles) {
			f := m.detailFiles[m.fileIdx]
			m.detailScroll = m.viewport.YOffset // restore on return from the file viewer
			m.openViewer(fileRenderer(f.path), f.label, m.detail.s.Ticket, viewDetail)
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
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
		if m.wfDetailCursor > 0 {
			m.wfDetailCursor--
			m.reRenderWorkflowDetailAndScroll()
		}
	case key.Matches(msg, keys.next):
		if m.wfDetailCursor < wfDetailTotal(m.wfDetailName, m.workflows)-1 {
			m.wfDetailCursor++
			m.reRenderWorkflowDetailAndScroll()
		}
	case key.Matches(msg, keys.up):
		m.viewport.ScrollUp(1)
	case key.Matches(msg, keys.down):
		m.viewport.ScrollDown(1)
	case key.Matches(msg, keys.pageUp):
		m.viewport.PageUp()
	case key.Matches(msg, keys.pageDown):
		m.viewport.PageDown()
	case key.Matches(msg, keys.halfPageUp):
		m.viewport.HalfPageUp()
	case key.Matches(msg, keys.halfPageDown):
		m.viewport.HalfPageDown()
	case key.Matches(msg, keys.top):
		m.viewport.GotoTop()
	case key.Matches(msg, keys.bottom):
		m.viewport.GotoBottom()
	case key.Matches(msg, keys.open):
		stageName, advance, stepNum, total := wfDetailSelectedStage(m.wfDetailName, m.wfDetailCursor, m.workflows)
		if stageName != "" {
			stagePath := filepath.Join(m.root, "stages", config.ResourcePath(stageName))
			title := stageViewerTitle(stageName, advance, stepNum, total, m.workflows)
			m.openViewer(fileRenderer(stagePath), title, m.wfDetailName, viewWorkflowDetail)
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleFileKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case key.Matches(msg, keys.back):
		switch m.viewerReturn {
		case viewWorkflowDetail:
			m.reRenderWorkflowDetailAndScroll()
		case viewDetail:
			// the viewport holds file content — rebuild the detail body and
			// restore the scroll position we left from.
			m.viewport.SetContent(m.renderDetailBody())
			m.viewport.SetYOffset(m.detailScroll)
		}
		m.view = m.viewerReturn
	case key.Matches(msg, keys.up):
		m.viewport.ScrollUp(1)
	case key.Matches(msg, keys.down):
		m.viewport.ScrollDown(1)
	case key.Matches(msg, keys.pageUp):
		m.viewport.PageUp()
	case key.Matches(msg, keys.pageDown):
		m.viewport.PageDown()
	case key.Matches(msg, keys.halfPageUp):
		m.viewport.HalfPageUp()
	case key.Matches(msg, keys.halfPageDown):
		m.viewport.HalfPageDown()
	case key.Matches(msg, keys.top):
		m.viewport.GotoTop()
	case key.Matches(msg, keys.bottom):
		m.viewport.GotoBottom()
	case key.Matches(msg, keys.previous):
		switch m.viewerReturn {
		case viewDetail:
			if m.fileIdx > 0 {
				m.fileIdx--
				m.loadViewerFile()
			}
		case viewWorkflowDetail:
			if m.wfDetailCursor > 0 {
				m.wfDetailCursor--
				m.loadViewerStage()
			}
		}
	case key.Matches(msg, keys.next):
		switch m.viewerReturn {
		case viewDetail:
			if m.fileIdx < len(m.detailFiles)-1 {
				m.fileIdx++
				m.loadViewerFile()
			}
		case viewWorkflowDetail:
			if m.wfDetailCursor < wfDetailTotal(m.wfDetailName, m.workflows)-1 {
				m.wfDetailCursor++
				m.loadViewerStage()
			}
		}
	case key.Matches(msg, keys.character):
		if m.charSheetWorker != nil {
			m.charSheetReturn = viewFile
			m.view = viewCharacterSheet
			return m, tea.ClearScreen
		}
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleCharacterSheetKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.quit):
		return m, tea.Quit
	case key.Matches(msg, keys.character, keys.back):
		m.view = m.charSheetReturn
		return m, tea.ClearScreen
	}
	return m, nil
}
