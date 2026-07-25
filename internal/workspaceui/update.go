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
		m.viewer.viewport.Width = max(1, msg.Width-4)
		m.viewer.viewport.Height = m.viewerHeight()
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
		if m.lifecycle.inactive || msg.epoch != m.lifecycle.epoch {
			return m, nil
		}
		interval := m.lifecycle.refreshInterval
		if interval == 0 {
			interval = defaultRefreshInterval
		}
		return m, tea.Batch(loadData(m.root), tickEvery(interval, m.lifecycle.epoch))

	case liveTickMsg:
		if m.lifecycle.inactive || msg.epoch != m.lifecycle.epoch {
			return m, nil
		}
		return m, tea.Batch(
			loadLiveData(m.root, m.data.config, m.data.allWorkers),
			liveTickEvery(defaultLiveRefreshInterval, m.lifecycle.epoch),
		)

	case dataMsg:
		m.lifecycle.lastRefresh = time.Now()
		m.lifecycle.lastLiveRefresh = m.lifecycle.lastRefresh
		if msg.err != nil {
			m.lifecycle.loadErr = msg.err
			m.data.healthItems = []doctor.Check{{Group: "workspace", Name: config.Filename, Status: doctor.Fail, Detail: msg.err.Error()}}
			m.refreshStructuredViewer()
			return m, nil
		}
		m.lifecycle.loadErr = nil
		cfg := msg.config
		if cfg == nil {
			cfg = m.data.config
		}
		m.data = workspaceData{
			config:         cfg,
			features:       msg.features,
			healthItems:    msg.healthItems,
			artifactPolicy: msg.artifactPolicy,
			workerNames:    msg.workerNames,
			workerGroups:   msg.workerGroups,
			workflowGroups: msg.workflowGroups,
			allWorkers:     msg.allWorkers,
			workflows:      msg.workflows,
			repos:          msg.repos,
			routes:         msg.routes,
		}
		assignWorkerAccentColors(msg.allWorkers)
		m.navigation.items = msg.sectionItems
		m.lifecycle.refreshInterval = msg.refreshInterval
		if m.effects.quote == "" {
			m.effects.quote = pickQuote(msg.quotes)
		}
		if rows := m.visibleFeatures(); m.navigation.featureCursor >= len(rows) && len(rows) > 0 {
			m.navigation.featureCursor = len(rows) - 1
		}
		m.refreshStructuredViewer()
		if m.view == viewWorkflowDetail {
			if !workflowKnown(m.navigation.workflowName, m.data.workflows) && len(m.data.workflows) > 0 {
				m.navigation.workflowName = m.data.workflows[0].name
				m.navigation.workflowCursor = 0
			}
			m.reRenderWorkflowDetail()
		}
		return m, nil

	case liveDataMsg:
		if msg.err != nil || msg.features == nil {
			return m, nil
		}
		m.lifecycle.lastLiveRefresh = time.Now()
		m.data.features = mergeLiveFeatures(m.data.features, msg.features)
		if rows := m.visibleFeatures(); m.navigation.featureCursor >= len(rows) && len(rows) > 0 {
			m.navigation.featureCursor = len(rows) - 1
		}
		if m.view == viewDetail {
			m.reRenderDetail()
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case rainbowTickMsg:
		if m.lifecycle.inactive {
			return m, nil
		}
		if m.effects.rainbowStep > 0 {
			m.effects.rainbowStep--
			if m.effects.rainbowStep > 0 {
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
