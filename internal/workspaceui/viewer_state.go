package workspaceui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/config"
)

func (m *Model) reRenderWorkflowDetail() {
	yOff := m.viewer.viewport.YOffset
	content := renderWorkflowDetail(m.navigation.workflowName, m.data.workflows, m.data.allWorkers, filepath.Join(m.root, "stages"), m.data.features, m.navigation.workflowCursor, m.width-4)
	m.viewer.viewport.SetContent(content)
	m.viewer.viewport.SetYOffset(yOff)
}

// reRenderWorkflowDetailAndScroll re-renders the workflow detail and scrolls the
// viewport so the selected cursor row stays visible.
func (m *Model) reRenderWorkflowDetailAndScroll() {
	content := renderWorkflowDetail(m.navigation.workflowName, m.data.workflows, m.data.allWorkers, filepath.Join(m.root, "stages"), m.data.features, m.navigation.workflowCursor, m.width-4)
	m.viewer.viewport.SetContent(content)
	targetLine := workflowCursorLine(content)
	viewH := m.viewer.viewport.Height
	curY := m.viewer.viewport.YOffset
	if targetLine < curY {
		m.viewer.viewport.SetYOffset(targetLine)
	} else if targetLine >= curY+viewH {
		m.viewer.viewport.SetYOffset(targetLine - viewH + 1)
	}
}

func workflowCursorLine(content string) int {
	for i, line := range strings.Split(content, "\n") {
		if strings.Contains(line, "▶") {
			return i
		}
	}
	return 0
}

// stageViewerTitle builds the "stage · step N of M · advance · K workflows"
// title shown when a stage file is open in the viewer.
func stageViewerTitle(stageName, advance string, stepNum, total int, chains []workflowChain) string {
	wfCount := stageWorkflowCount(chains, stageName)
	wfWord := "workflows"
	if wfCount == 1 {
		wfWord = "workflow"
	}
	return fmt.Sprintf("%s · step %d of %d · %s · %d %s", workflowStageLabel(stageName, chains), stepNum, total, advance, wfCount, wfWord)
}

// loadViewerStage loads the stage at m.navigation.workflowCursor (in pipeline order) into
// the viewport for viewFile, rebuilding the "step N of M" title.
func (m *Model) loadViewerStage() {
	stageName, advance, stepNum, total := wfDetailSelectedStage(m.navigation.workflowName, m.navigation.workflowCursor, m.data.workflows)
	if stageName == "" {
		return
	}
	stagePath := filepath.Join(m.root, "stages", config.ResourcePath(stageName))
	m.viewer.render = fileRenderer(stagePath)
	m.viewer.viewport.SetContent(m.viewer.render(m.viewer.viewport.Width))
	m.viewer.viewport.SetYOffset(0)
	m.viewer.title = stageViewerTitle(stageName, advance, stepNum, total, m.data.workflows)
}

// loadViewerFile loads m.detail.files[m.detail.fileIndex] into the viewport for viewFile.
func (m *Model) loadViewerFile() {
	f := m.detail.files[m.detail.fileIndex]
	m.viewer.render = fileRenderer(f.path)
	m.viewer.viewport.SetContent(m.viewer.render(m.viewer.viewport.Width))
	m.viewer.viewport.SetYOffset(0)
	m.viewer.title = f.label
}

// reRenderDetail rebuilds the detail body into the viewport at the current
// width, preserving the scroll position. Called on resize and file-chip change.
func (m *Model) reRenderDetail() {
	if m.detail.feature == nil {
		return
	}
	off := m.viewer.viewport.YOffset
	m.viewer.viewport.SetContent(m.renderDetailBody())
	m.viewer.viewport.SetYOffset(off)
}

// reRenderViewerFile rebuilds the file viewer content at the current viewport
// width, preserving the scroll position. Called on window resize.
func (m *Model) reRenderViewerFile() {
	if m.viewer.render == nil {
		return
	}
	yOff := m.viewer.viewport.YOffset
	m.viewer.viewport.SetContent(m.viewer.render(m.viewer.viewport.Width))
	m.viewer.viewport.SetYOffset(yOff)
}

// ── Commands ──────────────────────────────────────────────────────
