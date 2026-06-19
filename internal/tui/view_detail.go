package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/report"
	"github.com/cengebretson/orc/internal/ticketview"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// ── Detail view ───────────────────────────────────────────────────

// viewDetail renders the chrome — the title bar and help line — around the
// scrollable detail body, which lives in the viewport so long tickets stay
// usable on short terminals.
func (m Model) viewDetail() string {
	if m.detail == nil {
		return ""
	}
	outerW := m.width - 2
	var b strings.Builder
	b.WriteString("\n" + drawBox(styleDetailTitle.Render(" "+m.detail.s.Slug+" "), nil, outerW) + "\n")
	b.WriteString(m.viewport.View())
	help := strings.Join([]string{
		helpItem("tab/←→", "cycle files"),
		helpItem("↑↓/pgup/pgdn", "scroll"),
		helpItem("enter", "view file"),
		helpItem("t", "attach"),
		helpItem("esc", "back"),
		helpItem("q", "quit"),
	}, "  ")
	b.WriteString("\n" + styleHelp.Render(" "+help))
	return b.String()
}

// renderDetailBody renders the scrollable body of the detail view — the State,
// Repos, Timing, History, and Files boxes — for the viewport.
func (m Model) renderDetailBody() string {
	s := m.detail.s
	summary := ticketview.Build(m.root, m.detail.featureDir, s, ticketview.Options{
		TmuxAvailable: func() bool { return true },
		SessionExists: func(session string) bool {
			return s.Runtime.Tmux != nil && session == s.Runtime.Tmux.Session && m.detail.tmuxLive
		},
		AttachHint: func(session, window string) string {
			return "tmux attach -t " + session + ":" + window
		},
	})
	// The body renders inside the viewport (width m.width-4), so build the boxes
	// to that width — the title bar in viewDetail keeps the full m.width-2.
	outerW := m.width - 4
	innerW := outerW - 2
	var b strings.Builder

	// State fields
	var stateLines []string
	workerLabel := summary.WorkerName
	if workerLabel == "" {
		workerLabel = summary.WorkerID
	}
	workflowLabel := m.detail.workflowLabel
	if workflowLabel == "" {
		workflowLabel = summary.Workflow
	}
	stageLabel := m.detail.stageLabel
	if stageLabel == "" {
		stageLabel = summary.Stage
	}
	fields := []struct{ label, value string }{
		{" Ticket  ", s.Ticket},
		{" Status  ", statusStyle(s.Status).Render(statusIcon(s.Status) + " " + s.Status)},
		{" Workflow", workflowLabel},
		{" Stage   ", stageLabel + summary.StageLoopLabel},
		{" Worker  ", workerLabel},
	}
	for _, f := range fields {
		stateLines = append(stateLines, fmt.Sprintf("%s  %s",
			styleDetailLabel.Render(f.label), f.value))
	}
	if m.detail.hasIssues {
		stateLines = append(stateLines, fmt.Sprintf("%s  %s",
			styleDetailLabel.Render(" Issues  "),
			styleHealthWarn.Render("! no worker assigned for this stage — set worker: in orc.yaml or run `orc mark "+s.Ticket+" next --worker <id>`")))
	}
	if summary.TmuxConfigured {
		if summary.TmuxLive {
			hint := styleTmuxLive.Render(summary.TmuxAttachHint)
			stateLines = append(stateLines, fmt.Sprintf("%s  %s", styleDetailLabel.Render(" Session "), hint))
		} else {
			stateLines = append(stateLines, fmt.Sprintf("%s  %s",
				styleDetailLabel.Render(" Session "),
				styleTmuxDead.Render("not running — "+summary.TmuxRestart)))
		}
	}
	if summary.JIT != nil {
		jit := summary.JIT
		stateLines = append(stateLines,
			fmt.Sprintf("%s  %s", styleDetailLabel.Render(" JIT     "), styleStatusWaiting.Render(jit.Worker+" · "+truncate(jit.Task, innerW-20))),
			fmt.Sprintf("%s  %s", styleDetailLabel.Render("         "), styleDim.Render("started "+jit.StartedAt)),
		)
	}
	if summary.NextStage != "" {
		nextStage := workflowStageLabel(summary.NextStage, m.workflows)
		var nextVal string
		if summary.NextAdvance == "auto" {
			nextVal = styleHealthOK.Render("→ "+nextStage) + styleDim.Render("  auto")
		} else {
			nextVal = styleStatusWaiting.Render("→ "+nextStage) + styleDim.Render("  awaiting approval")
		}
		stateLines = append(stateLines, fmt.Sprintf("%s  %s",
			styleDetailLabel.Render(" Next    "), nextVal))
	} else if s.Stage.Name != "" {
		stateLines = append(stateLines, fmt.Sprintf("%s  %s",
			styleDetailLabel.Render(" Next    "), styleDim.Render("last stage")))
	}
	if s.Status == "paused" {
		stateLines = append(stateLines, fmt.Sprintf("%s  %s",
			styleDetailLabel.Render(" Paused  "), styleStatusWaiting.Render(truncate(summary.PausedReason, innerW-16))))
	}
	b.WriteString(drawBox(styleSection.Render(" State "), stateLines, outerW) + "\n")

	// Repos
	if len(s.Repos) > 0 {
		var repoLines []string
		repoNames := make([]string, 0, len(s.Repos))
		for name := range s.Repos {
			repoNames = append(repoNames, name)
		}
		sort.Strings(repoNames)
		for _, name := range repoNames {
			r := s.Repos[name]
			repoLines = append(repoLines, "  "+styleSubtext.Render(name))
			if r.Main != "" {
				repoLines = append(repoLines, fmt.Sprintf("    %s  %s", styleDetailLabel.Render("main    "), styleDim.Render(r.Main)))
			}
			if r.Worktree != "" {
				repoLines = append(repoLines, fmt.Sprintf("    %s  %s", styleDetailLabel.Render("worktree"), styleDim.Render(r.Worktree)))
				repoLines = append(repoLines, fmt.Sprintf("    %s  %s", styleDetailLabel.Render("branch  "), styleDim.Render(r.Branch)))
			}
		}
		b.WriteString(drawBox(styleSection.Render(" Repos "), repoLines, outerW) + "\n")
	}

	// Timing — per-stage durations derived from history
	if rep := report.Compute(s, time.Now()); len(rep.Stages) > 0 {
		var timingLines []string
		timingLines = append(timingLines, fmt.Sprintf(" %s  %-10s  %-10s  %s",
			styleDetailLabel.Render(padRight("stage", 18)),
			styleDetailLabel.Render("active"),
			styleDetailLabel.Render("wall"),
			styleDetailLabel.Render("visits")))
		for _, st := range rep.Stages {
			marker := ""
			if rep.Open && st.Stage == s.Stage.Name {
				marker = styleHealthOK.Render("  ← current")
			}
			timingLines = append(timingLines, fmt.Sprintf(" %s  %-10s  %-10s  %-6d%s",
				styleSubtext.Render(padRight(truncate(st.Stage, 18), 18)),
				report.Humanize(st.Active),
				styleDim.Render(report.Humanize(st.Wall)),
				st.Visits, marker))
		}
		totalLabel := "total"
		if rep.Open {
			totalLabel = "total so far"
		}
		timingLines = append(timingLines, fmt.Sprintf(" %s  %-10s  %s",
			styleDetailLabel.Render(padRight(totalLabel, 18)),
			report.Humanize(rep.Active),
			styleDim.Render(report.Humanize(rep.Wall))))
		b.WriteString(drawBox(styleSection.Render(" Timing "), timingLines, outerW) + "\n")
	}

	// History
	if len(s.History) > 0 {
		var histLines []string
		for _, h := range s.History {
			ts := h.At
			if len(ts) > 10 {
				ts = ts[:10]
			}
			histLines = append(histLines, fmt.Sprintf(" %s  %-20s  %-18s  %s",
				styleDim.Render(ts),
				styleSubtext.Render(truncate(h.Stage, 20)),
				styleDim.Render(truncate(h.Worker, 18)),
				styleSubtext.Render(truncate(h.Result, innerW-72)),
			))
		}
		b.WriteString(drawBox(styleSection.Render(" History "), histLines, outerW) + "\n")
	}

	// Files
	if len(m.detailFiles) > 0 {
		chips := " "
		for i, f := range m.detailFiles {
			exists := fileExists(f.path)
			var chip string
			if i == m.fileIdx {
				chip = styleFileSelected.Render(f.label)
			} else if exists {
				chip = styleFileOK.Render(f.label)
			} else {
				chip = styleFileMissing.Render(f.label)
			}
			chips += chip + " "
		}
		fileLines := []string{chips}
		if m.fileIdx < len(m.detailFiles) {
			f := m.detailFiles[m.fileIdx]
			if fileExists(f.path) {
				fileLines = append(fileLines, " "+styleDim.Render("enter to view "+f.label))
			} else {
				fileLines = append(fileLines, " "+styleDim.Render(f.label+" does not exist yet"))
			}
		}
		b.WriteString(drawBox(styleSection.Render(" Files "), fileLines, outerW) + "\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// ── File viewer ───────────────────────────────────────────────────

func (m Model) viewFile() string {
	outerW := m.width - 2
	var b strings.Builder
	title := styleDetailTitle.Render(" "+m.viewerContext) +
		styleDim.Render(" · ") +
		styleSubtext.Render(m.viewerTitle+" ")
	b.WriteString("\n" + drawBox(title, nil, outerW) + "\n")
	b.WriteString(m.viewport.View())
	helpItems := []string{
		helpItem("↑↓/pgup/pgdn", "scroll"),
	}
	switch m.viewerReturn {
	case viewDetail:
		helpItems = append(helpItems, helpItem("←→", "prev/next file"))
	case viewWorkflowDetail:
		helpItems = append(helpItems, helpItem("←→", "prev/next stage"))
	}
	helpItems = append(helpItems,
		helpItem("esc", "back"),
		helpItem("q", "quit"),
	)
	help := strings.Join(helpItems, "  ")
	b.WriteString("\n" + styleHelp.Render("  "+help))
	return b.String()
}

// ── Workflow detail view ──────────────────────────────────────────

func (m Model) viewWorkflowDetailPage() string {
	outerW := m.width - 2
	var b strings.Builder
	title := styleDetailTitle.Render(" Workflows") +
		styleDim.Render(" · ") +
		styleSubtext.Render(workflowLabel(m.wfDetailName, m.workflows)+" ")
	b.WriteString("\n" + drawBox(title, nil, outerW) + "\n")
	b.WriteString(m.viewport.View())
	help := strings.Join([]string{
		helpItem("↑↓/←→", "select stage"),
		helpItem("pgup/pgdn", "scroll"),
		helpItem("enter", "view stage"),
		helpItem("esc", "back"),
		helpItem("q", "quit"),
	}, "  ")
	b.WriteString("\n" + styleHelp.Render("  "+help))
	return b.String()
}

func wfDetailTotal(name string, chains []workflowChain) int {
	for _, c := range chains {
		if c.name == name {
			return len(c.steps) + len(c.repairSteps)
		}
	}
	return 0
}

func wfDetailSelectedStage(name string, idx int, chains []workflowChain) (stageName, advance string, stepNum, total int) {
	for _, c := range chains {
		if c.name != name {
			continue
		}
		total = len(c.steps) + len(c.repairSteps)
		if idx < len(c.steps) {
			s := c.steps[idx]
			return s.name, s.advance, idx + 1, total
		}
		ri := idx - len(c.steps)
		if ri < len(c.repairSteps) {
			rs := c.repairSteps[ri]
			return rs.name, rs.advance, idx + 1, total
		}
	}
	return "", "", 0, 0
}

func stageWorkflowCount(chains []workflowChain, stageName string) int {
	count := 0
	for _, c := range chains {
		for _, s := range c.steps {
			if s.name == stageName {
				count++
				break
			}
		}
		for _, rs := range c.repairSteps {
			if rs.name == stageName {
				count++
				break
			}
		}
	}
	return count
}

func renderFile(path string, width int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if width <= 0 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylesFromJSONBytes(activeTheme.Glamour),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return string(data), nil
	}
	out, err := r.Render(string(data))
	if err != nil {
		return string(data), nil
	}
	return out, nil
}

// renderWorkerFile renders a worker .md file as a frontmatter info box followed
// by the markdown body. width is the available viewport width.
func renderWorkerFile(path string, features []*featureRow, width int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	raw := string(data)
	body := raw

	// Split frontmatter from body.
	var w workers.Worker
	hasFM := false
	if strings.HasPrefix(strings.TrimSpace(raw), "---") {
		content := strings.TrimSpace(raw)[3:]
		if end := strings.Index(content, "\n---"); end != -1 {
			fm := strings.TrimSpace(content[:end])
			if e := yaml.Unmarshal([]byte(fm), &w); e == nil {
				hasFM = true
				rest := content[end+4:]
				body = strings.TrimSpace(rest)
			}
		}
	}

	var sb strings.Builder

	if hasFM {
		// Build info rows: label → value pairs.
		type row struct{ label, value string }
		var rows []row
		add := func(label, value string) {
			if value != "" {
				rows = append(rows, row{label, value})
			}
		}
		add("id", w.ID)
		add("engine", w.Engine)
		add("model", w.Model)
		argKeys := make([]string, 0, len(w.Args))
		for k := range w.Args {
			argKeys = append(argKeys, k)
		}
		sort.Strings(argKeys)
		for _, k := range argKeys {
			add(k, w.Args[k])
		}
		// Measure label column width.
		labelW := 0
		for _, r := range rows {
			if len(r.label) > labelW {
				labelW = len(r.label)
			}
		}

		// Render rows as styled lines.
		innerW := width - 4
		var lines []string
		for _, r := range rows {
			pad := strings.Repeat(" ", labelW-len(r.label))
			label := styleDetailLabel.Render(r.label+pad) + styleDim.Render("  ")
			val := styleDetailValue.Render(r.value)
			lines = append(lines, "  "+label+val)
		}

		workerName := w.Name
		if workerName == "" {
			workerName = w.ID
		}
		sb.WriteString(drawBoxLabeledWith(
			styleHeader.Render(workerName),
			lines,
			innerW,
			activeTheme.Palette.Mauve,
		))
		sb.WriteString("\n")

		// Active stories for this worker
		var activeRows []string
		for _, row := range features {
			if row.s == nil {
				continue
			}
			if row.s.Stage.Worker == w.ID && row.s.Status != "archived" {
				ticket := styleSubtext.Render(padRight(row.s.Ticket, 14))
				wf := row.s.Workflow
				if wf == "" {
					wf = "default"
				}
				workflowLabel := row.workflowLabel
				if workflowLabel == "" {
					workflowLabel = wf
				}
				stageLabel := row.stageLabel
				if stageLabel == "" {
					stageLabel = row.s.Stage.Name
				}
				stage := styleDim.Render(workflowLabel + "/" + stageLabel)
				activeRows = append(activeRows, "  "+ticket+"  "+stage)
			}
		}
		label := styleSection.Render(fmt.Sprintf(" Active Stories (%d) ", len(activeRows)))
		sb.WriteString(drawBox(label, activeRows, width) + "\n")
	}

	// Render markdown body.
	if body != "" {
		r, err := glamour.NewTermRenderer(
			glamour.WithStylesFromJSONBytes(activeTheme.Glamour),
			glamour.WithWordWrap(width),
		)
		if err == nil {
			if out, err := r.Render(body); err == nil {
				sb.WriteString(out)
			} else {
				sb.WriteString(body)
			}
		} else {
			sb.WriteString(body)
		}
	}

	return sb.String(), nil
}

// renderWorkflowFile renders a stage markdown file. width is the available viewport width.
// renderWorkflowDetail renders an inline detail view for a named workflow.
func renderWorkflowDetail(name string, chains []workflowChain, allWorkers []*workers.Worker, stagesDir string, features []*featureRow, selectedIdx int, width int) string {
	var chain *workflowChain
	for i := range chains {
		if chains[i].name == name {
			chain = &chains[i]
			break
		}
	}
	if chain == nil {
		return styleHealthErr.Render("workflow " + name + " not found")
	}

	// ticket count per stage for this workflow
	stageCounts := map[string]int{}
	for _, row := range features {
		if row.s == nil {
			continue
		}
		if row.workflow == name {
			stageName := row.stage
			if stageName == "" {
				stageName = row.s.Stage.Name
			}
			stageCounts[stageName]++
		}
	}

	workerLabel := func(id string) string {
		if id == "" {
			return styleDim.Render("—")
		}
		if w := workers.FindByID(allWorkers, id); w != nil {
			label := w.Name
			if label == "" {
				label = w.ID
			}
			if w.Engine != "" {
				label += styleDim.Render("  " + w.Engine)
			}
			return label
		}
		return styleDim.Render(id)
	}

	stageExists := func(stageName string) string {
		if _, err := os.Stat(filepath.Join(stagesDir, config.ResourcePath(stageName))); err == nil {
			return styleHealthOK.Render("✓")
		}
		return styleHealthErr.Render("✗")
	}

	innerW := width - 4
	var sb strings.Builder

	// Description — a one-line summary of what this workflow is for.
	if chain.description != "" {
		for _, l := range strings.Split(wrapText(chain.description, innerW), "\n") {
			sb.WriteString("  " + styleSubtext.Render(l) + "\n")
		}
		sb.WriteString("\n")
	}

	// Route chain visualization
	chainLines := renderRouteChain(chain.steps, chain.loops, innerW)
	routeLines := make([]string, 0, len(chainLines))
	for _, l := range chainLines {
		routeLines = append(routeLines, "  "+l)
	}
	sb.WriteString(drawBox(styleSection.Render(" Route "), routeLines, width) + "\n")

	// Stage table: ✓ | Stage | Worker (engine) | Advance | Active
	const (
		wCheck     = 1
		wStageName = 20
		wAdvance   = 10
		wActive    = 6
	)
	wWorker := innerW - wCheck - wStageName - wAdvance - wActive - 10
	if wWorker < 16 {
		wWorker = 16
	}
	header := "  " +
		padRight(styleTableHeader.Render(""), wCheck) + "  " +
		padRight(styleTableHeader.Render("Stage"), wStageName) + "  " +
		padRight(styleTableHeader.Render("Worker"), wWorker) + "  " +
		padRight(styleTableHeader.Render("Advance"), wAdvance) + "  " +
		styleTableHeader.Render("Active")
	divider := "  " + styleDivider.Render(strings.Repeat("─", innerW-2))

	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(activeTheme.Palette.Mauve))

	stageRows := func(steps []routeStep, baseIdx int) []string {
		var lines []string
		lines = append(lines, header, divider)
		for i, step := range steps {
			var advVal string
			if step.advance == "manual" {
				advVal = styleStatusWaiting.Render("● manual")
			} else {
				advVal = styleHealthOK.Render("auto")
			}
			count := stageCounts[step.name]
			var activeVal string
			if count > 0 {
				activeVal = styleSubtext.Render(fmt.Sprintf("%d", count))
			} else {
				activeVal = styleDim.Render("—")
			}
			cursor := "  "
			if baseIdx+i == selectedIdx {
				cursor = cursorStyle.Render("▶") + " "
			}
			lines = append(lines, cursor+
				padRight(stageExists(step.name), wCheck)+"  "+
				padRight(styleSubtext.Render(truncate(stepLabel(step), wStageName)), wStageName)+"  "+
				padRight(workerLabel(step.workerID), wWorker)+"  "+
				padRight(advVal, wAdvance)+"  "+
				activeVal)
		}
		return lines
	}

	sb.WriteString(drawBox(styleSection.Render(" Stages "), stageRows(chain.steps, 0), width) + "\n")

	if len(chain.repairSteps) > 0 {
		repairAsSteps := make([]routeStep, len(chain.repairSteps))
		for i, rs := range chain.repairSteps {
			repairAsSteps[i] = routeStep{name: rs.name, label: rs.label, advance: rs.advance, workerID: rs.workerID}
		}
		rawRows := stageRows(repairAsSteps, len(chain.steps))
		// rawRows is [header, divider, row0, row1, ...]
		// Interleave each annotation directly under its row.
		annotationIndent := "  " + strings.Repeat(" ", wCheck+wStageName+4)
		var repairLines []string
		repairLines = append(repairLines, rawRows[:2]...) // header + divider
		for i, rs := range chain.repairSteps {
			repairLines = append(repairLines, rawRows[2+i])
			detail := fmt.Sprintf("repairs %s", repairTargetLabel(rs))
			if rs.maxRetries > 0 {
				detail = fmt.Sprintf("repairs %s · max %d", repairTargetLabel(rs), rs.maxRetries)
			}
			repairLines = append(repairLines, annotationIndent+styleDim.Render(detail))
		}
		sb.WriteString(drawBox(styleSection.Render(" Loop Stages "), repairLines, width) + "\n")
	}

	return sb.String()
}
