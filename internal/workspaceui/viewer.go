package workspaceui

import (
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/charmbracelet/bubbles/viewport"
)

func (m *Model) openViewer(render func(width int) string, title, context string, returnView viewState) {
	viewerWidth := max(1, m.width-4)
	m.viewport = viewport.New(viewerWidth, max(1, m.height-6))
	m.viewport.SetContent(render(viewerWidth))
	m.viewerTitle = title
	m.viewerContext = context
	m.viewerReturn = returnView
	m.viewerRender = render
	m.viewerKind = viewerNone
	m.viewerPath = ""
	m.view = viewFile
}

func (m *Model) refreshStructuredViewer() {
	if m.view != viewFile {
		return
	}
	switch m.viewerKind {
	case viewerHealth:
		checks := append([]doctor.Check(nil), m.healthItems...)
		m.viewerRender = func(w int) string { return renderHealthReport(checks, w) }
	case viewerRepositories:
		repos := append([]config.Repo(nil), m.repos...)
		routes := append([]config.RepoRoute(nil), m.routes...)
		features := append([]*featureRow(nil), m.features...)
		m.viewerRender = func(w int) string { return renderRoutingReport(repos, routes, features, w) }
	case viewerWorker:
		m.viewerRender = workerRenderer(m.viewerPath, m.features)
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
