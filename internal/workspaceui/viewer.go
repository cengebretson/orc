package workspaceui

import (
	"fmt"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) openViewer(render func(width int) string, title, context string, returnView viewState) {
	// Set view/viewer state before measuring height: viewerHeight consults it
	// (via directDestinationHeader) to size the viewport around whatever
	// pinned header applies, instead of overflowing past it.
	m.view = viewFile
	m.viewer = viewerState{title: title, context: context, returnView: returnView, render: render}
	viewerWidth := max(1, m.width-4)
	view := viewport.New(viewerWidth, m.viewerHeight())
	view.SetContent(render(viewerWidth))
	m.viewer.viewport = view
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

// directDestinationHeader returns the pinned banner (and any destination-
// specific summary) shown above the scrollable viewport for the current
// view, or "" when none applies (non-embedded mode, or a view with no
// pinned header). The four isDirectX cases below cover Health, Repositories,
// and Workers opened straight from the tab bar, each with its own summary
// box; the remaining cases cover every other embedded scrollable page —
// Workflows (direct or drilled-in), the ticket detail page, and the generic
// file/report viewer — with the shared banner above their existing title.
func (m Model) directDestinationHeader() string {
	switch {
	case m.isDirectHealth():
		return m.directHealthHeader()
	case m.isDirectRepositories():
		return m.directRepositoryHeader()
	case m.isDirectWorkers():
		return m.directWorkerHeader()
	case m.embedded && m.view == viewWorkflowDetail:
		return m.directWorkflowHeader()
	case m.embedded && m.view == viewDetail:
		return m.detailPinnedHeader()
	case m.embedded && m.view == viewFile:
		return m.genericFileHeader()
	default:
		return ""
	}
}

func (m Model) directHealthHeader() string {
	width := max(20, m.viewer.viewport.Width)
	return "\n" + m.operationalBanner(width) + "\n" +
		renderPinnedHealthSummary(m.data.healthItems, width) + "\n"
}

func (m Model) directRepositoryHeader() string {
	width := max(20, m.viewer.viewport.Width)
	return "\n" + m.operationalBanner(width) + "\n" +
		renderRepositoryMapSummary(m.data.repos, m.data.routes, width) + "\n"
}

func (m Model) directWorkerHeader() string {
	width := max(20, m.viewer.viewport.Width)
	return "\n" + m.operationalBanner(width) + "\n" +
		renderWorkerSelector(m.data.workerGroups, m.selectedWorkerIndex(), width) + "\n"
}

// directWorkflowHeader renders the pinned banner and workflow title shown
// above the route chain, whether the Workflows tab was opened directly or
// drilled into from the section list — both cases show the same title, so
// there's no need to distinguish them here. Building it here (rather than
// inline in viewWorkflowDetailPage) lets viewerHeight measure its exact line
// count so the viewport underneath is never over- or under-sized.
func (m Model) directWorkflowHeader() string {
	outerW := max(4, m.width-2)
	title := styleDetailTitle.Render(" Workflows") +
		styleDim.Render(" · ") +
		styleSubtext.Render(workflowDisplayWithID(m.navigation.workflowName, m.data.workflows)) +
		styleDim.Render(fmt.Sprintf("  ·  %.0f%% ", m.viewer.viewport.ScrollPercent()*100))
	return "\n" + m.operationalBanner(outerW) + "\n" + drawBox(title, nil, outerW) + "\n"
}

// detailPinnedHeader renders the shared banner and ticket title shown above
// the scrollable ticket detail body (State, Repos, Timing, History, Files).
func (m Model) detailPinnedHeader() string {
	if m.detail.feature == nil {
		return ""
	}
	outerW := max(4, m.width-2)
	title := styleDetailTitle.Render(" " + m.detail.feature.s.Slug + " ")
	return "\n" + m.operationalBanner(outerW) + "\n" + drawBox(title, nil, outerW) + "\n"
}

// genericFileHeader renders the shared banner above a plain file or report
// viewer opened by drilling into a section item — a stage file, a ticket's
// linked document, a broken feature's state file — as opposed to the
// destination-specific direct headers above.
func (m Model) genericFileHeader() string {
	outerW := max(4, m.width-2)
	title := styleDetailTitle.Render(" "+m.viewer.context) +
		styleDim.Render(" · ") +
		styleSubtext.Render(m.viewer.title) +
		styleDim.Render(fmt.Sprintf("  ·  %.0f%% ", m.viewer.viewport.ScrollPercent()*100))
	return "\n" + m.operationalBanner(outerW) + "\n" + drawBox(title, nil, outerW) + "\n"
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
