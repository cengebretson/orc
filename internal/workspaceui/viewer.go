package workspaceui

import (
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/charmbracelet/bubbles/viewport"
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

func (m *Model) refreshStructuredViewer() {
	if m.view != viewFile {
		return
	}
	switch m.viewer.kind {
	case viewerHealth:
		checks := append([]doctor.Check(nil), m.data.healthItems...)
		m.viewer.render = func(w int) string { return renderHealthReport(checks, w) }
	case viewerRepositories:
		repos := append([]config.Repo(nil), m.data.repos...)
		routes := append([]config.RepoRoute(nil), m.data.routes...)
		features := append([]*featureRow(nil), m.data.features...)
		m.viewer.render = func(w int) string { return renderRoutingReport(repos, routes, features, w) }
	case viewerWorker:
		m.viewer.render = workerRenderer(m.viewer.path, m.data.features)
	default:
		return
	}
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
