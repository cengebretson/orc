package workspaceui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/artifactcheck"
	"github.com/cengebretson/orc/internal/report"
	"github.com/cengebretson/orc/internal/ticketview"
	"github.com/charmbracelet/lipgloss"
)

// ── Detail view ───────────────────────────────────────────────────

// viewDetail renders the chrome — the title bar and help line — around the
// scrollable detail body, which lives in the viewport so long tickets stay
// usable on short terminals.
func (m Model) viewDetail() string {
	if m.detail == nil {
		return ""
	}
	outerW := max(4, m.width-2)
	var b strings.Builder
	b.WriteString("\n" + drawBox(styleDetailTitle.Render(" "+m.detail.s.Slug+" "), nil, outerW) + "\n")
	b.WriteString(m.viewport.View())
	if !m.embedded {
		help := strings.Join([]string{
			combinedBindingHelp("cycle files", keys.cycleForward, keys.cycleBackward, keys.previous, keys.next),
			combinedBindingHelp("scroll", keys.up, keys.down, keys.pageUp, keys.pageDown),
			bindingHelp(keys.open),
			bindingHelp(keys.attach),
			bindingHelp(keys.back),
			bindingHelp(keys.quit),
		}, "  ")
		b.WriteString("\n" + styleHelp.Render(" "+help))
	}
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
	outerW := max(4, m.width-4)
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
	workflowValue := labelWithDimID(workflowLabel, summary.Workflow)
	stageLabel := m.detail.stageLabel
	if stageLabel == "" {
		stageLabel = summary.Stage
	}
	stageValue := labelWithDimID(stageLabel+summary.StageLoopLabel, summary.Stage)
	workerValue := labelWithDimID(workerLabel, summary.WorkerID)
	fields := []struct{ label, value string }{
		{" Ticket  ", s.Ticket},
		{" Status  ", statusStyle(s.Status).Render(statusIcon(s.Status) + " " + s.Status)},
		{" Workflow", workflowValue},
		{" Stage   ", stageValue},
		{" Worker  ", workerValue},
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
	if m.detail.context.Observed {
		stateLines = append(stateLines, fmt.Sprintf("%s  %s",
			styleDetailLabel.Render(" Context "), renderContextPressure(m.detail.context)))
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

	if len(m.detail.requiredArtifacts) > 0 {
		b.WriteString(drawBox(styleSection.Render(" Required Artifacts "),
			artifactStatusLines(m.detail.featureDir, artifactcheck.TemplateDir(m.root), m.detail.requiredArtifacts, innerW), outerW) + "\n")
	}

	// Timing — per-stage durations derived from history
	if rep := report.Compute(s, time.Now()); len(rep.Stages) > 0 {
		const (
			minTimingStage = 18
			wActive        = 10
			wWall          = 10
			wVisits        = 6
		)
		wTimingStage := minTimingStage
		for _, st := range rep.Stages {
			wTimingStage = max(wTimingStage, lipgloss.Width(st.Stage))
		}
		totalLabel := "total"
		if rep.Open {
			totalLabel = "total so far"
		}
		wTimingStage = max(wTimingStage, lipgloss.Width(totalLabel))
		maxTimingStage := innerW - wActive - wWall - wVisits - 7
		if maxTimingStage < minTimingStage {
			maxTimingStage = minTimingStage
		}
		if wTimingStage > maxTimingStage {
			wTimingStage = maxTimingStage
		}

		var timingLines []string
		timingLines = append(timingLines, fmt.Sprintf(" %s  %s  %s  %s",
			padRight(styleDetailLabel.Render("stage"), wTimingStage),
			padRight(styleDetailLabel.Render("active"), wActive),
			padRight(styleDetailLabel.Render("elapsed"), wWall),
			padRight(styleDetailLabel.Render("visits"), wVisits)))
		for _, st := range rep.Stages {
			marker := ""
			if rep.Open && st.Stage == s.Stage.Name {
				marker = styleHealthOK.Render("  ← current")
			}
			timingLines = append(timingLines, fmt.Sprintf(" %s  %s  %s  %s%s",
				padRight(styleSubtext.Render(truncate(st.Stage, wTimingStage)), wTimingStage),
				padRight(report.Humanize(st.Active), wActive),
				padRight(styleDim.Render(report.Humanize(st.Wall)), wWall),
				padRight(fmt.Sprintf("%d", st.Visits), wVisits),
				marker))
		}
		timingLines = append(timingLines, fmt.Sprintf(" %s  %s  %s",
			padRight(styleDetailLabel.Render(totalLabel), wTimingStage),
			padRight(report.Humanize(rep.Active), wActive),
			styleDim.Render(report.Humanize(rep.Wall))))
		b.WriteString(drawBox(styleSection.Render(" Timing "), timingLines, outerW) + "\n")
	}

	// History
	if len(s.History) > 0 {
		const (
			wAt       = 10
			minStage  = 20
			minWorker = 18
			minResult = 24
		)
		wStage := minStage
		wWorker := minWorker
		for _, h := range s.History {
			wStage = max(wStage, lipgloss.Width(h.Stage))
			wWorker = max(wWorker, lipgloss.Width(h.Worker))
		}
		available := innerW - wAt - minResult - 7
		if available < minStage+minWorker {
			available = minStage + minWorker
		}
		if wStage+wWorker > available {
			over := wStage + wWorker - available
			if wWorker > minWorker {
				shrink := min(over, wWorker-minWorker)
				wWorker -= shrink
				over -= shrink
			}
			if over > 0 && wStage > minStage {
				wStage -= min(over, wStage-minStage)
			}
		}
		wResult := innerW - wAt - wStage - wWorker - 7
		var histLines []string
		histLines = append(histLines, fmt.Sprintf(" %s  %s  %s  %s",
			padRight(styleDetailLabel.Render("date"), wAt),
			padRight(styleDetailLabel.Render("stage"), wStage),
			padRight(styleDetailLabel.Render("worker"), wWorker),
			styleDetailLabel.Render("result"),
		))
		for _, h := range s.History {
			ts := h.At
			if len(ts) > 10 {
				ts = ts[:10]
			}
			histLines = append(histLines, fmt.Sprintf(" %s  %s  %s  %s",
				padRight(styleDim.Render(truncate(ts, wAt)), wAt),
				padRight(styleSubtext.Render(truncate(h.Stage, wStage)), wStage),
				padRight(styleDim.Render(truncate(h.Worker, wWorker)), wWorker),
				styleSubtext.Render(truncate(h.Result, wResult)),
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

func artifactStatusLines(featureDir, templateDir string, artifacts []string, maxW int) []string {
	issues := artifactcheck.Check(featureDir, templateDir, artifacts)
	issueByPath := map[string]artifactcheck.Issue{}
	for _, issue := range issues {
		issueByPath[issue.Path] = issue
	}
	var lines []string
	for _, artifact := range artifacts {
		if issue, found := issueByPath[artifact]; found {
			lines = append(lines, " "+styleHealthWarn.Render("!")+" "+styleFileMissing.Render(truncate(issue.Detail(), maxW-4)))
		} else {
			lines = append(lines, " "+styleHealthOK.Render("✓")+" "+styleFileOK.Render(truncate(artifact, maxW-4)))
		}
	}
	return lines
}

func labelWithDimID(label, id string) string {
	if id == "" || id == label {
		return label
	}
	return label + styleDim.Render(" ("+id+")")
}

func joinColumns(left, right, gap string) string {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")
	leftW := 0
	for _, line := range leftLines {
		leftW = max(leftW, lipgloss.Width(line))
	}
	n := max(len(leftLines), len(rightLines))
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		leftLine := ""
		if i < len(leftLines) {
			leftLine = leftLines[i]
		}
		rightLine := ""
		if i < len(rightLines) {
			rightLine = rightLines[i]
		}
		out = append(out, padRight(leftLine, leftW)+gap+rightLine)
	}
	return strings.Join(out, "\n")
}

// ── File viewer ───────────────────────────────────────────────────
