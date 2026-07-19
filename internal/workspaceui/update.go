package workspaceui

import (
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = max(1, msg.Width-4)
		m.viewport.Height = max(1, msg.Height-6)
		// viewport-backed views hold content pre-rendered at the old width;
		// rebuild it (the dashboard and detail views render from m.width live)
		switch m.view {
		case viewFile:
			m.reRenderViewerFile()
		case viewDetail:
			m.reRenderDetail()
		case viewWorkflowDetail:
			m.reRenderWorkflowDetail()
		case viewCharacterSheet:
			return m, tea.ClearScreen
		}
		return m, nil

	case tickMsg:
		if m.inactive || msg.epoch != m.epoch {
			return m, nil
		}
		interval := m.refreshInterval
		if interval == 0 {
			interval = defaultRefreshInterval
		}
		return m, tea.Batch(loadData(m.root), tickEvery(interval, m.epoch))

	case dataMsg:
		m.lastRefresh = time.Now()
		if msg.err != nil {
			m.loadErr = msg.err
			m.healthItems = []doctor.Check{{Group: "workspace", Name: config.Filename, Status: doctor.Fail, Detail: msg.err.Error()}}
			m.refreshStructuredViewer()
			return m, nil
		}
		m.loadErr = nil
		m.features = msg.features
		m.healthItems = msg.healthItems
		m.artifactPolicy = msg.artifactPolicy
		m.workerNames = msg.workerNames
		m.workerGroups = msg.workerGroups
		m.workflowGroups = msg.workflowGroups
		m.allWorkers = msg.allWorkers
		m.workflows = msg.workflows
		m.repos = msg.repos
		m.routes = msg.routes
		m.sectionItems = msg.sectionItems
		m.refreshInterval = msg.refreshInterval
		if m.quote == "" {
			m.quote = pickQuote(msg.quotes)
		}
		if rows := m.visibleFeatures(); m.cursor >= len(rows) && len(rows) > 0 {
			m.cursor = len(rows) - 1
		}
		m.refreshStructuredViewer()
		if m.view == viewWorkflowDetail {
			m.reRenderWorkflowDetail()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case rainbowTickMsg:
		if m.inactive {
			return m, nil
		}
		if m.rainbowStep > 0 {
			m.rainbowStep--
			if m.rainbowStep > 0 {
				return m, rainbowTick()
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.view {
	case viewDashboard:
		return m.handleDashboardKey(msg)
	case viewDetail:
		return m.handleDetailKey(msg)
	case viewWorkflowDetail:
		return m.handleWorkflowDetailKey(msg)
	case viewFile:
		return m.handleFileKey(msg)
	case viewCharacterSheet:
		return m.handleCharacterSheetKey(msg)
	}
	return m, nil
}
