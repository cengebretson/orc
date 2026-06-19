package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 6
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
		interval := m.refreshInterval
		if interval == 0 {
			interval = defaultRefreshInterval
		}
		return m, tea.Batch(loadData(m.root), tickEvery(interval))

	case dataMsg:
		m.features = msg.features
		m.healthItems = msg.healthItems
		m.workerNames = msg.workerNames
		m.workerGroups = msg.workerGroups
		m.workflowGroups = msg.workflowGroups
		m.allWorkers = msg.allWorkers
		m.workflows = msg.workflows
		m.repos = msg.repos
		m.sectionItems = msg.sectionItems
		m.refreshInterval = msg.refreshInterval
		if m.quote == "" {
			m.quote = pickQuote(msg.quotes)
		}
		m.lastRefresh = time.Now()
		if rows := m.visibleFeatures(); m.cursor >= len(rows) && len(rows) > 0 {
			m.cursor = len(rows) - 1
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case rainbowTickMsg:
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

func (m Model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Search mode: route keys to textinput ─────────────────────
	if m.searching {
		switch msg.String() {
		case "esc":
			m.searching = false
			m.search.Blur()
			m.search.SetValue("")
			m.cursor = 0
			return m, nil
		case "enter":
			m.searching = false
			m.search.Blur()
			m.cursor = 0
			return m, nil
		default:
			var cmd tea.Cmd
			m.search, cmd = m.search.Update(msg)
			m.cursor = 0
			return m, cmd
		}
	}

	// track last 3 keys for "orc" easter egg
	m.keyBuffer[0] = m.keyBuffer[1]
	m.keyBuffer[1] = m.keyBuffer[2]
	m.keyBuffer[2] = msg.String()
	if m.keyBuffer == [3]string{"o", "r", "c"} && m.rainbowStep == 0 {
		m.rainbowStep = rainbowSteps
		return m, rainbowTick()
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "r":
		return m, loadData(m.root)
	case "/":
		if m.focusedPane == "features" {
			m.searching = true
			m.search.Focus()
			return m, textinput.Blink
		}

	case "tab", "shift+tab":
		m.cycleSectionFocus(msg.String() == "tab")

	case "esc", "b":
		if m.search.Value() != "" {
			m.search.SetValue("")
			m.cursor = 0
		} else if m.focusedPane == "section" {
			m.blurSectionFocus()
		}

	case "1":
		m.toggleSection("health")
	case "2":
		m.toggleSection("workflows")
	case "3":
		m.toggleSection("workers")
	case "4":
		m.toggleSection("routes")

	case "up", "k":
		if m.focusedPane == "section" {
			if m.sectionCursor > 0 {
				m.sectionCursor--
			}
		} else {
			if m.cursor > 0 {
				m.cursor--
			}
		}

	case "down", "j":
		if m.focusedPane == "section" {
			items := m.sectionItems[m.sectionFocus]
			if m.sectionCursor < len(items)-1 {
				m.sectionCursor++
			}
		} else {
			rows := m.visibleFeatures()
			if m.cursor < len(rows)-1 {
				m.cursor++
			}
		}

	case "pgdown", "ctrl+f":
		if m.focusedPane != "section" {
			m.moveFeatureCursor(m.featuresPageSize())
		}
	case "pgup", "ctrl+b":
		if m.focusedPane != "section" {
			m.moveFeatureCursor(-m.featuresPageSize())
		}
	case "g", "home":
		if m.focusedPane != "section" {
			m.cursor = 0
		}
	case "G", "end":
		if m.focusedPane != "section" {
			m.cursor = len(m.visibleFeatures()) - 1
			if m.cursor < 0 {
				m.cursor = 0
			}
		}

	case "a":
		if m.focusedPane == "features" {
			m.showArchived = !m.showArchived
			m.cursor = 0
		}

	case "t":
		if m.focusedPane == "features" {
			rows := m.visibleFeatures()
			if m.cursor < len(rows) {
				row := rows[m.cursor]
				if row.s != nil && row.s.Runtime.Tmux != nil && row.tmuxLive {
					return m, attachTmux(row.s.Runtime.Tmux.Session, row.s.Stage.Name)
				}
			}
		}

	case "!":
		if m.focusedPane == "section" && m.sectionFocus == "workers" {
			items := m.sectionItems["workers"]
			if m.sectionCursor < len(items) {
				m.charSheetWorker = workerForPath(items[m.sectionCursor].path, m.allWorkers)
				m.charSheetReturn = viewDashboard
				m.view = viewCharacterSheet
				return m, tea.ClearScreen
			}
		}

	case "enter":
		if m.focusedPane == "section" {
			m.openSectionItem()
		} else {
			rows := m.visibleFeatures()
			if m.cursor < len(rows) {
				row := rows[m.cursor]
				if row.s == nil {
					m.openViewer(func(int) string { return renderBrokenFeature(row) },
						row.ticketID(), "broken", viewDashboard)
					return m, nil
				}
				m.detail = row
				m.detailFiles = buildFileList(m.detail.featureDir, m.detail.s)
				m.fileIdx = 0
				m.viewport = viewport.New(m.width-4, m.height-6)
				m.viewport.SetContent(m.renderDetailBody())
				m.detailScroll = 0
				m.view = viewDetail
			}
		}
	}

	return m, nil
}

// featuresPageSize approximates one screenful of feature rows for pgup/pgdn.
// The exact visible-row count depends on which sections are expanded, but the
// render's scroll window keeps the cursor visible regardless, so an estimate is
// fine.
func (m Model) featuresPageSize() int {
	p := m.height - 8
	if p < 1 {
		p = 1
	}
	return p
}

// moveFeatureCursor shifts the features cursor by delta, clamped to the list.
func (m *Model) moveFeatureCursor(delta int) {
	n := len(m.visibleFeatures())
	m.cursor += delta
	if m.cursor > n-1 {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// cycleSectionFocus moves focus through the navigable sections with tab /
// shift+tab, wrapping back to the features pane at either end.
func (m *Model) cycleSectionFocus(forward bool) {
	navigable := m.navigableSections()
	if len(navigable) == 0 {
		return
	}
	if m.focusedPane != "section" {
		if forward {
			m.focusSection(navigable[0])
		} else {
			m.focusSection(navigable[len(navigable)-1])
		}
		return
	}
	idx := -1
	for i, k := range navigable {
		if k == m.sectionFocus {
			idx = i
			break
		}
	}
	next := idx - 1
	if forward {
		next = idx + 1
	}
	if next < 0 || next >= len(navigable) {
		m.blurSectionFocus()
	} else {
		m.focusSection(navigable[next])
	}
}

func (m *Model) focusSection(name string) {
	m.collapseAutoExpandedSection()
	m.focusedPane = "section"
	m.sectionFocus = name
	m.sectionCursor = 0
	if !m.expanded[name] {
		m.autoExpandedSection = name
	}
	m.expanded[name] = true
}

func (m *Model) blurSectionFocus() {
	m.collapseAutoExpandedSection()
	m.focusedPane = "features"
	m.sectionFocus = ""
	m.sectionCursor = 0
}

func (m *Model) collapseAutoExpandedSection() {
	if m.autoExpandedSection != "" {
		m.expanded[m.autoExpandedSection] = false
		m.autoExpandedSection = ""
	}
}

// toggleSection expands or collapses a dashboard section; collapsing the
// focused section returns focus to the features pane.
func (m *Model) toggleSection(name string) {
	wasExpanded := m.expanded[name]
	if m.autoExpandedSection == name {
		m.autoExpandedSection = ""
	}
	m.expanded[name] = !wasExpanded
	if wasExpanded && m.sectionFocus == name {
		m.blurSectionFocus()
	}
}

// openSectionItem opens the focused section's selected item: a worker file,
// a workflow detail page, or a plain file view.
func (m *Model) openSectionItem() {
	// Health has no list items — it drills straight into the full doctor report.
	if m.sectionFocus == "health" {
		checks := m.healthItems
		m.openViewer(func(w int) string { return renderHealthReport(checks, w) },
			"doctor report", "Health", viewDashboard)
		return
	}
	items := m.sectionItems[m.sectionFocus]
	if m.sectionCursor >= len(items) {
		return
	}
	f := items[m.sectionCursor]
	switch m.sectionFocus {
	case "workers":
		m.charSheetWorker = workerForPath(f.path, m.allWorkers)
		m.openViewer(workerRenderer(f.path, m.features), f.label, sectionLabel(m.sectionFocus), viewDashboard)
	case "workflows":
		workflowName := f.id
		if workflowName == "" {
			workflowName = f.label
		}
		m.wfDetailName = workflowName
		m.wfDetailCursor = 0
		content := renderWorkflowDetail(workflowName, m.workflows, m.allWorkers, filepath.Join(m.root, "stages"), m.features, 0, m.width-4)
		m.viewport = viewport.New(m.width-4, m.height-6)
		m.viewport.SetContent(content)
		m.view = viewWorkflowDetail
	default:
		m.openViewer(fileRenderer(f.path), f.label, sectionLabel(m.sectionFocus), viewDashboard)
	}
}

// openViewer switches to the file viewer, rendering content via render at the
// current width. render is retained so the viewer re-flows on resize.
func (m *Model) openViewer(render func(width int) string, title, context string, returnView viewState) {
	m.viewport = viewport.New(m.width-4, m.height-6)
	m.viewport.SetContent(render(m.width - 4))
	m.viewerTitle = title
	m.viewerContext = context
	m.viewerReturn = returnView
	m.viewerRender = render
	m.view = viewFile
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

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "b":
		m.view = viewDashboard
	case "tab", "right", "l":
		if m.fileIdx < len(m.detailFiles)-1 {
			m.fileIdx++
			m.reRenderDetail() // refresh the selected file chip
		}
	case "shift+tab", "left", "h":
		if m.fileIdx > 0 {
			m.fileIdx--
			m.reRenderDetail()
		}
	case "t":
		if m.detail.s.Runtime.Tmux != nil && m.detail.tmuxLive {
			return m, attachTmux(m.detail.s.Runtime.Tmux.Session, m.detail.s.Stage.Name)
		}
	case "enter":
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
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "b":
		m.view = viewDashboard
	case "up", "k", "left", "h":
		if m.wfDetailCursor > 0 {
			m.wfDetailCursor--
			m.reRenderWorkflowDetailAndScroll()
		}
	case "down", "j", "right", "l":
		if m.wfDetailCursor < wfDetailTotal(m.wfDetailName, m.workflows)-1 {
			m.wfDetailCursor++
			m.reRenderWorkflowDetailAndScroll()
		}
	case "enter":
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
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "b":
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
	case "left", "h":
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
	case "right", "l":
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
	case "!":
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
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "!", "esc", "b":
		m.view = m.charSheetReturn
		return m, tea.ClearScreen
	}
	return m, nil
}

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

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func attachTmux(session, window string) tea.Cmd {
	return tea.ExecProcess(
		newTmuxCmd(session, window),
		func(err error) tea.Msg { return nil },
	)
}

func newTmuxCmd(session, window string) *exec.Cmd {
	target := session + ":" + window
	if os.Getenv("TMUX") != "" {
		return exec.Command("tmux", "switch-client", "-t", target)
	}
	return exec.Command("tmux", "attach-session", "-t", target)
}
