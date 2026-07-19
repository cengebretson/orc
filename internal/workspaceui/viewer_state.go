package workspaceui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/config"
)

func (m *Model) reRenderWorkflowDetail() {
	yOff := m.viewport.YOffset
	content := renderWorkflowDetail(m.wfDetailName, m.workflows, m.allWorkers, filepath.Join(m.root, "stages"), m.features, m.wfDetailCursor, m.width-4)
	m.viewport.SetContent(content)
	m.viewport.SetYOffset(yOff)
}

// reRenderWorkflowDetailAndScroll re-renders the workflow detail and scrolls the
// viewport so the selected cursor row stays visible.
func (m *Model) reRenderWorkflowDetailAndScroll() {
	content := renderWorkflowDetail(m.wfDetailName, m.workflows, m.allWorkers, filepath.Join(m.root, "stages"), m.features, m.wfDetailCursor, m.width-4)
	m.viewport.SetContent(content)
	targetLine := workflowCursorLine(content)
	viewH := m.viewport.Height
	curY := m.viewport.YOffset
	if targetLine < curY {
		m.viewport.SetYOffset(targetLine)
	} else if targetLine >= curY+viewH {
		m.viewport.SetYOffset(targetLine - viewH + 1)
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

// loadViewerStage loads the stage at m.wfDetailCursor (in pipeline order) into
// the viewport for viewFile, rebuilding the "step N of M" title.
func (m *Model) loadViewerStage() {
	stageName, advance, stepNum, total := wfDetailSelectedStage(m.wfDetailName, m.wfDetailCursor, m.workflows)
	if stageName == "" {
		return
	}
	stagePath := filepath.Join(m.root, "stages", config.ResourcePath(stageName))
	m.viewerRender = fileRenderer(stagePath)
	m.viewport.SetContent(m.viewerRender(m.viewport.Width))
	m.viewport.SetYOffset(0)
	m.viewerTitle = stageViewerTitle(stageName, advance, stepNum, total, m.workflows)
}

// loadViewerFile loads m.detailFiles[m.fileIdx] into the viewport for viewFile.
func (m *Model) loadViewerFile() {
	f := m.detailFiles[m.fileIdx]
	m.viewerRender = fileRenderer(f.path)
	m.viewport.SetContent(m.viewerRender(m.viewport.Width))
	m.viewport.SetYOffset(0)
	m.viewerTitle = f.label
}

// reRenderDetail rebuilds the detail body into the viewport at the current
// width, preserving the scroll position. Called on resize and file-chip change.
func (m *Model) reRenderDetail() {
	if m.detail == nil {
		return
	}
	off := m.viewport.YOffset
	m.viewport.SetContent(m.renderDetailBody())
	m.viewport.SetYOffset(off)
}

// reRenderViewerFile rebuilds the file viewer content at the current viewport
// width, preserving the scroll position. Called on window resize.
func (m *Model) reRenderViewerFile() {
	if m.viewerRender == nil {
		return
	}
	yOff := m.viewport.YOffset
	m.viewport.SetContent(m.viewerRender(m.viewport.Width))
	m.viewport.SetYOffset(yOff)
}

// ── Commands ──────────────────────────────────────────────────────
