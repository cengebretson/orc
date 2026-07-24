package workspaceui

import (
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) openViewer(render func(width int) string, title, context string, returnView viewState) {
	viewerWidth := max(1, m.width-4)
	view := viewport.New(viewerWidth, max(1, m.height-6))
	view.SetContent(render(viewerWidth))
	m.viewer = viewerState{
		viewport:   view,
		title:      title,
		context:    context,
		returnView: returnView,
		render:     render,
	}
	m.view = viewFile
}

func (m *Model) openHealthReport(returnView viewState) {
	checks := append([]doctor.Check(nil), m.data.healthItems...)
	render := func(w int) string { return renderHealthReport(checks, w) }
	if m.embedded && returnView == viewFile {
		render = func(w int) string { return renderHealthDetails(checks, w) }
	}
	m.openViewer(render,
		"doctor report", "Health", returnView)
	m.viewer.kind = viewerHealth
	m.viewer.viewport.Height = m.viewerHeight()
}

func (m *Model) openRepositoryReport(returnView viewState) {
	repos := repositoriesForDisplay(m.root, m.data.repos)
	routes := append([]config.RepoRoute(nil), m.data.routes...)
	features := append([]*featureRow(nil), m.data.features...)
	render := func(w int) string { return renderRoutingReport(repos, routes, features, w) }
	if m.embedded && returnView == viewFile {
		render = func(w int) string { return renderRepositoryDetails(repos, routes, features, w) }
	}
	m.openViewer(render, "map", "Repositories", returnView)
	m.viewer.kind = viewerRepositories
	m.viewer.viewport.Height = m.viewerHeight()
}

func (m *Model) openWorkerReport(returnView viewState) {
	items := m.navigation.items[sectionWorkers]
	if len(items) == 0 {
		m.openViewer(func(int) string { return styleDim.Render("No workers configured.") }, "none", "Workers", returnView)
		m.viewer.kind = viewerWorker
		m.viewer.viewport.Height = m.viewerHeight()
		return
	}
	item := items[m.selectedWorkerIndex()]
	m.effects.charSheetWorker = workerForPath(item.path, m.data.allWorkers)
	m.openViewer(workerRenderer(item.path, m.data.features), item.label, "Workers", returnView)
	m.viewer.kind = viewerWorker
	m.viewer.path = item.path
	m.viewer.viewport.Height = m.viewerHeight()
}

func (m Model) viewerHeight() int {
	if header := m.directDestinationHeader(); header != "" {
		// The header ends with the newline that starts the viewport, so their
		// rendered heights overlap by one terminal row.
		return max(1, m.height-lipgloss.Height(header)+1)
	}
	return max(1, m.height-6)
}

func (m Model) isDirectHealth() bool {
	return m.embedded && m.viewer.kind == viewerHealth && m.viewer.returnView == viewFile
}

func (m Model) isDirectRepositories() bool {
	return m.embedded && m.viewer.kind == viewerRepositories && m.viewer.returnView == viewFile
}

func (m Model) isDirectWorkers() bool {
	return m.embedded && m.viewer.kind == viewerWorker && m.viewer.returnView == viewFile
}

func (m Model) directDestinationHeader() string {
	switch {
	case m.isDirectHealth():
		return m.directHealthHeader()
	case m.isDirectRepositories():
		return m.directRepositoryHeader()
	case m.isDirectWorkers():
		return m.directWorkerHeader()
	default:
		return ""
	}
}

func (m Model) directHealthHeader() string {
	width := max(20, m.viewer.viewport.Width)
	return "\n" + renderPinnedHealthSummary(m.data.healthItems, width) + "\n"
}

func (m Model) directRepositoryHeader() string {
	width := max(20, m.viewer.viewport.Width)
	return "\n" + renderRepositoryMapSummary(m.data.repos, m.data.routes, width) + "\n"
}

func (m Model) directWorkerHeader() string {
	width := max(20, m.viewer.viewport.Width)
	return "\n" + m.operationalBanner(width) + "\n" +
		renderWorkerSelector(m.data.workerGroups, m.selectedWorkerIndex(), width) + "\n"
}

func (m Model) selectedWorkerIndex() int {
	items := m.navigation.items[sectionWorkers]
	if len(items) == 0 {
		return 0
	}
	return min(max(0, m.navigation.sectionCursor), len(items)-1)
}

func (m *Model) selectDirectWorker(delta int) {
	items := m.navigation.items[sectionWorkers]
	if len(items) == 0 {
		return
	}
	next := m.selectedWorkerIndex() + delta
	if next < 0 {
		next = 0
	}
	if next >= len(items) {
		next = len(items) - 1
	}
	if next == m.navigation.sectionCursor {
		return
	}
	m.navigation.sectionCursor = next
	m.navigation.sectionCursors[sectionWorkers] = next
	item := items[next]
	m.effects.charSheetWorker = workerForPath(item.path, m.data.allWorkers)
	m.viewer.path = item.path
	m.viewer.title = item.label
	m.viewer.render = workerRenderer(item.path, m.data.features)
	m.viewer.viewport.SetContent(m.viewer.render(m.viewer.viewport.Width))
	m.viewer.viewport.SetYOffset(0)
	m.viewer.viewport.Height = m.viewerHeight()
}

func (m *Model) refreshStructuredViewer() {
	if m.view != viewFile {
		return
	}
	switch m.viewer.kind {
	case viewerHealth:
		checks := append([]doctor.Check(nil), m.data.healthItems...)
		if m.isDirectHealth() {
			m.viewer.render = func(w int) string { return renderHealthDetails(checks, w) }
		} else {
			m.viewer.render = func(w int) string { return renderHealthReport(checks, w) }
		}
	case viewerRepositories:
		repos := repositoriesForDisplay(m.root, m.data.repos)
		routes := append([]config.RepoRoute(nil), m.data.routes...)
		features := append([]*featureRow(nil), m.data.features...)
		if m.isDirectRepositories() {
			m.viewer.render = func(w int) string { return renderRepositoryDetails(repos, routes, features, w) }
		} else {
			m.viewer.render = func(w int) string { return renderRoutingReport(repos, routes, features, w) }
		}
	case viewerWorker:
		if m.isDirectWorkers() {
			items := m.navigation.items[sectionWorkers]
			if len(items) == 0 {
				m.viewer.path = ""
				m.viewer.title = "none"
				m.effects.charSheetWorker = nil
				m.viewer.render = func(int) string { return styleDim.Render("No workers configured.") }
			} else {
				item := items[m.selectedWorkerIndex()]
				m.viewer.path = item.path
				m.viewer.title = item.label
				m.effects.charSheetWorker = workerForPath(item.path, m.data.allWorkers)
				m.viewer.render = workerRenderer(item.path, m.data.features)
			}
		} else {
			m.viewer.render = workerRenderer(m.viewer.path, m.data.features)
		}
	default:
		return
	}
	m.viewer.viewport.Height = m.viewerHeight()
	m.reRenderViewerFile()
}

// fileRenderer returns a width-aware renderer for a markdown file path.
func fileRenderer(path string) func(int) string {
	return func(w int) string {
		c, err := renderFile(path, w)
		if err != nil {
			return styleHealthErr.Render("could not read file: " + err.Error())
		}
		return c
	}
}

// workerRenderer returns a width-aware renderer for a worker .md file. The
// feature list is captured at open time to resolve the worker's active stories.
func workerRenderer(path string, features []*featureRow) func(int) string {
	return func(w int) string {
		c, err := renderWorkerFile(path, features, w)
		if err != nil {
			return styleHealthErr.Render("could not read: " + err.Error())
		}
		return c
	}
}
