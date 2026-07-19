package watch

import (
	"fmt"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func (m Model) previewContent() string {
	if r, ok := m.selectedWork(); ok {
		return m.workPreviewContent(r)
	}
	return mutedStyle.Render("No item selected.")
}

func renderRailDetail(r row, width int) string {
	return renderRailDetailAt(r, width, time.Time{})
}

func renderRailDetailAt(r row, width int, now time.Time) string {
	icon, label := displayState(r)
	var b strings.Builder
	b.WriteString(stateBadge(label).Render(icon + " " + strings.ToUpper(truncate(label, width-2))))
	for _, location := range compactDetailLocations(r) {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate(location, max(1, width-2))))
	}
	if r.worker != "" && r.worker != "-" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate(r.worker, max(1, width-2))))
	}
	if r.tmuxState != "" && r.tmuxState != "-" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate("tmux "+r.tmuxState, max(1, width-2))))
	}
	if r.attention != "" {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("  " + truncate("attention "+r.attention, max(1, width-2))))
	}
	if r.context.Observed {
		b.WriteString("\n")
		b.WriteString("  " + renderContextVisual(r, max(3, min(8, width-10))))
	}
	if age := activityAge(r, now); age != "" {
		b.WriteString("\n")
		prefix := "updated "
		if !r.lastActive.IsZero() {
			prefix = "active "
		}
		b.WriteString(mutedStyle.Render("  " + prefix + age))
	}
	if len(r.workflowSteps) > 0 {
		b.WriteString("\n\n")
		label := "FLOW"
		if r.workflowLabel != "" {
			label += " · " + r.workflowLabel
		}
		b.WriteString(sectionStyle.Render(truncate(label, width)))
		b.WriteString("\n")
		b.WriteString(renderWorkflowFlow(r, width))
	}
	if r.next != "" {
		b.WriteString("\n\n")
		b.WriteString(sectionStyle.Render(promptLabel(r)))
		b.WriteString("\n")
		b.WriteString(wrap(r.next, width))
	}
	return b.String()
}

func stateBadge(label string) lipgloss.Style {
	return stateStyle(label).Background(lipgloss.Color("#313244")).Bold(true).Padding(0, 1)
}

func watchSummary(rows []row) string {
	running, paused, needs := watchCounts(rows)
	parts := []string{fmt.Sprintf("%d running", running), fmt.Sprintf("%d paused", paused)}
	if needs > 0 {
		parts = append(parts, fmt.Sprintf("%d needs you", needs))
	}
	return strings.Join(parts, " · ")
}

func watchCounts(rows []row) (running, paused, needs int) {
	for _, r := range rows {
		if r.tmuxState == "live" {
			running++
		}
		if r.status == "paused" {
			paused++
		}
		if attentionNeeded(r) {
			needs++
		}
	}
	return running, paused, needs
}

func renderWatchStatusLines(rows []row, width int) []string {
	running, paused, needs := watchCounts(rows)
	runningText := fmt.Sprintf("● %d RUNNING", running)
	pausedText := fmt.Sprintf("◐ %d PAUSED", paused)
	needsText := fmt.Sprintf("! %d NEEDS YOU", needs)
	runningView := activeStyle.Bold(true).Render(runningText)
	pausedView := pendingStyle.Bold(true).Render(pausedText)
	needsView := blockedStyle.Bold(true).Render(needsText)

	all := runningView + "  " + pausedView
	if needs > 0 {
		all += "  " + needsView
	}
	if lipgloss.Width(all) <= width {
		return []string{all}
	}
	primary := runningView + "  " + pausedView
	if lipgloss.Width(primary) <= width {
		lines := []string{primary}
		if needs > 0 {
			lines = append(lines, needsView)
		}
		return lines
	}

	compact := activeStyle.Bold(true).Render(fmt.Sprintf("● %d", running)) + "  " +
		pendingStyle.Bold(true).Render(fmt.Sprintf("◐ %d", paused))
	lines := []string{compact}
	if needs > 0 {
		lines = append(lines, blockedStyle.Bold(true).Render(fmt.Sprintf("! %d", needs)))
	}
	return lines
}

func renderWatchOverview(rows []row, width int, lastLoad, now time.Time) string {
	outerW := max(12, width)
	innerW := outerW - 2
	title := titleStyle.Render("orc") + mutedStyle.Render("  workspace orchestrator")
	if innerW < 29 {
		title = titleStyle.Render("orc") + mutedStyle.Render("  workspace")
	}
	if innerW < 17 {
		title = titleStyle.Render("orc")
	}

	statusLines := renderWatchStatusLines(rows, max(8, innerW-2))
	refresh := mutedStyle.Render(watchRefreshAge(lastLoad, now))
	lines := make([]string, 0, len(statusLines)+1)
	for _, line := range statusLines {
		lines = append(lines, " "+line)
	}
	last := len(lines) - 1
	combined := lines[last] + mutedStyle.Render("  ·  ") + refresh
	if lipgloss.Width(combined) <= innerW {
		lines[last] = combined
	} else {
		lines = append(lines, " "+refresh)
	}
	return renderWatchLabeledBox(title, lines, outerW)
}

func watchRefreshAge(lastLoad, now time.Time) string {
	if lastLoad.IsZero() {
		return "↺ loading"
	}
	if now.IsZero() {
		now = time.Now()
	}
	age := now.Sub(lastLoad)
	if age < 0 {
		age = 0
	}
	return fmt.Sprintf("↺ %s ago", age.Round(time.Second))
}

func renderWatchLabeledBox(title string, content []string, outerW int) string {
	innerW := max(1, outerW-2)
	border := mutedStyle
	label := " " + title + " "
	dashRight := max(0, innerW-1-lipgloss.Width(label))
	lines := []string{
		border.Render("╭─") + label + border.Render(strings.Repeat("─", dashRight)+"╮"),
	}
	for _, line := range content {
		lines = append(lines, border.Render("│")+line+strings.Repeat(" ", max(0, innerW-lipgloss.Width(line)))+border.Render("│"))
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", innerW)+"╯"))
	return strings.Join(lines, "\n")
}

func compactDetailLocations(r row) []string {
	locations := make([]string, 0, 2)
	if r.stage != "" {
		locations = append(locations, r.stage)
	}
	if r.room != "" && r.room != "workspace" {
		locations = append(locations, r.room)
	}
	return locations
}

func renderContextMeter(pressure contextpressure.Pressure, cells int) string {
	if !pressure.Observed || !pressure.Available {
		return "ctx " + renderContextPressure(pressure)
	}
	cells = max(1, cells)
	filled := int(pressure.Percent * uint64(cells) / 100)
	if pressure.Percent > 0 && filled == 0 {
		filled = 1
	}
	filled = min(cells, filled)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", cells-filled)
	return "ctx " + stateForPressure(pressure).Render(bar+" "+pressure.Label())
}

func renderContextVisual(r row, cells int) string {
	if !r.context.Observed || !r.context.Available || len(r.contextTrend) < 2 {
		return renderContextMeter(r.context, cells)
	}
	return "ctx " + stateForPressure(r.context).Render(renderContextSparkline(r.contextTrend, cells)+" "+r.context.Label())
}

func renderContextSparkline(samples []uint64, cells int) string {
	const levels = "▁▂▃▄▅▆▇█"
	if len(samples) == 0 {
		return ""
	}
	if cells > 0 && len(samples) > cells {
		samples = samples[len(samples)-cells:]
	}
	var b strings.Builder
	for _, value := range samples {
		if value > 100 {
			value = 100
		}
		idx := int(value * 7 / 100)
		b.WriteRune([]rune(levels)[idx])
	}
	return b.String()
}

func renderWorkflowFlow(r row, width int) string {
	if len(r.workflowSteps) == 0 {
		return mutedStyle.Render("workflow unavailable")
	}
	current := -1
	for i, step := range r.workflowSteps {
		if step.name == r.stageName || (r.stageName == "" && step.name == r.stage) {
			current = i
			break
		}
	}
	parts := make([]string, 0, len(r.workflowSteps))
	for i, step := range r.workflowSteps {
		label := step.label
		if label == "" {
			label = step.name
		}
		switch {
		case r.status == "done" || (current >= 0 && i < current):
			parts = append(parts, doneStyle.Render("✓ "+label))
		case i == current:
			parts = append(parts, selectedStyle.Render("● "+label))
		default:
			parts = append(parts, mutedStyle.Render("○ "+label))
		}
	}
	return wrapStyledParts(parts, " → ", width)
}

func workflowPosition(r row) (current, total int) {
	total = len(r.workflowSteps)
	if r.status == "done" && total > 0 {
		return total, total
	}
	for i, step := range r.workflowSteps {
		if step.name == r.stageName || (r.stageName == "" && step.name == r.stage) {
			return i + 1, total
		}
	}
	return 0, total
}

func nextTransition(r row) string {
	current, total := workflowPosition(r)
	if current == 0 || current >= total || r.status == "done" {
		return ""
	}
	step := r.workflowSteps[current-1]
	next := r.workflowSteps[current]
	label := next.label
	if label == "" {
		label = next.name
	}
	advance := "approval required"
	switch step.advance {
	case "auto":
		advance = "automatic"
	case "loop":
		advance = "returns to workflow"
	}
	return "next  " + label + " · " + advance
}

func currentStageTiming(r row, now time.Time) string {
	if now.IsZero() || r.stageName == "" {
		return ""
	}
	visits := 0
	var entered time.Time
	for _, h := range r.history {
		if h.stage != r.stageName {
			continue
		}
		visits++
		if parsed, err := time.Parse(time.RFC3339, h.at); err == nil && parsed.After(entered) {
			entered = parsed
		}
	}
	if entered.IsZero() || entered.After(now) {
		return ""
	}
	visitLabel := "visit"
	if visits != 1 {
		visitLabel = "visits"
	}
	return humanDuration(now.Sub(entered)) + " in stage · " + fmt.Sprintf("%d %s", visits, visitLabel)
}

func humanDuration(duration time.Duration) string {
	switch {
	case duration < time.Minute:
		return "now"
	case duration < time.Hour:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	default:
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}

func wrapStyledParts(parts []string, separator string, width int) string {
	var lines []string
	line := ""
	for _, part := range parts {
		candidate := part
		if line != "" {
			candidate = line + mutedStyle.Render(separator) + part
		}
		if line != "" && lipgloss.Width(candidate) > width {
			lines = append(lines, line)
			line = part
		} else {
			line = candidate
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func stateForPressure(pressure contextpressure.Pressure) lipgloss.Style {
	switch pressure.Level {
	case contextpressure.LevelGreen:
		return contextGreenStyle
	case contextpressure.LevelYellow:
		return contextYellowStyle
	case contextpressure.LevelRed:
		return contextRedStyle
	default:
		return mutedStyle
	}
}

func activityAge(r row, now time.Time) string {
	if now.IsZero() {
		return ""
	}
	latest := r.lastActive
	if latest.IsZero() && len(r.history) > 0 {
		parsed, err := time.Parse(time.RFC3339, r.history[len(r.history)-1].at)
		if err == nil {
			latest = parsed
		}
	}
	if latest.IsZero() || latest.After(now) {
		return ""
	}
	age := now.Sub(latest)
	switch {
	case age < time.Minute:
		return "now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}

func promptLabel(r row) string {
	_, label := displayState(r)
	if label == "blocked" {
		return "Blocker"
	}
	return "Next"
}

func renderHistory(rows []historyRow, width, limit int) string {
	if len(rows) == 0 {
		return mutedStyle.Render("none")
	}
	start := 0
	if limit > 0 && len(rows) > limit {
		start = len(rows) - limit
	}
	if width < 48 {
		return renderNarrowHistory(rows[start:], width)
	}
	dateW := 10
	stageW := min(18, max(10, width/5))
	workerW := min(14, max(8, width/6))
	resultW := max(8, width-dateW-stageW-workerW-8)
	var b strings.Builder
	for i, h := range rows[start:] {
		at := h.at
		if len(at) > dateW {
			at = at[:dateW]
		}
		line := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
			dateW, truncate(at, dateW),
			stageW, truncate(h.stage, stageW),
			workerW, truncate(h.worker, workerW),
			truncate(h.result, resultW),
		)
		b.WriteString(mutedStyle.Render(line))
		if i != len(rows[start:])-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderTimelineHistory(rows []historyRow, width, limit int) string {
	if len(rows) == 0 {
		return mutedStyle.Render("none")
	}
	start := 0
	if limit > 0 && len(rows) > limit {
		start = len(rows) - limit
	}
	rows = rows[start:]
	var b strings.Builder
	for i, h := range rows {
		at := h.at
		if parsed, err := time.Parse(time.RFC3339, h.at); err == nil {
			at = parsed.Format("Jan 02 15:04")
		}
		meta := joinNonEmpty(" · ", at, h.stage, h.worker)
		b.WriteString(accentStyle.Render("● "))
		b.WriteString(truncate(meta, max(1, width-2)))
		if h.result != "" {
			for _, line := range strings.Split(wrap(h.result, max(8, width-2)), "\n") {
				b.WriteString("\n")
				b.WriteString(mutedStyle.Render("│ "))
				b.WriteString(line)
			}
		}
		if i != len(rows)-1 {
			b.WriteString("\n")
			b.WriteString(mutedStyle.Render("│"))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderNarrowHistory(rows []historyRow, width int) string {
	var b strings.Builder
	for i, h := range rows {
		at := h.at
		if len(at) > 10 {
			at = at[:10]
		}
		meta := strings.TrimSpace(strings.Join([]string{at, h.stage, h.worker}, " "))
		b.WriteString(mutedStyle.Render(truncate(meta, width)))
		if h.result != "" {
			b.WriteString("\n")
			b.WriteString(wrap(h.result, width))
		}
		if i != len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) workPreviewContent(r row) string {
	icon, label := displayState(r)
	width := max(20, m.width)
	panelW := max(12, width-2)
	contentW := max(8, panelW-4)
	now := m.renderNow()

	var b strings.Builder
	var stateLines []string
	if now.Before(r.celebrateUntil) || r.demoCelebration {
		stateLines = append(stateLines, doneFlashStyle.Render(" ✦ COMPLETE ✦ "))
	} else {
		stateLines = append(stateLines, stateBadge(label).Render(icon+" "+strings.ToUpper(label)))
	}
	if r.stage != "" {
		stateLines = append(stateLines, "stage     "+r.stage)
	}
	if r.room != "" && r.room != "workspace" {
		stateLines = append(stateLines, "repo      "+r.room)
	}
	if r.worker != "" && r.worker != "-" {
		stateLines = append(stateLines, "worker    "+r.worker)
	}
	if r.branch != "" {
		stateLines = append(stateLines, "branch    "+r.branch)
	}
	provider := joinNonEmpty(" · ", r.engine, r.model)
	if provider != "" {
		stateLines = append(stateLines, "provider  "+provider)
	}
	if r.tmuxState != "" && r.tmuxState != "-" {
		stateLines = append(stateLines, "tmux      "+r.tmuxState)
		if target := tmuxTargetLabel(r); target != "" {
			stateLines = append(stateLines, "target    "+target)
		}
	}
	if r.attention != "" {
		stateLines = append(stateLines, "attention "+r.attention)
	}
	if r.liveState != "" {
		stateLines = append(stateLines, "live      "+r.liveState)
	}
	if r.context.Observed {
		stateLines = append(stateLines, "context   "+strings.TrimPrefix(renderContextVisual(r, min(10, contentW-12)), "ctx "))
	}
	if age := activityAge(r, now); age != "" {
		stateLines = append(stateLines, "activity  "+age)
	}
	if timing := currentStageTiming(r, now); timing != "" {
		stateLines = append(stateLines, "timing    "+timing)
	}
	detailTitle := featureDetailTitle(r)
	b.WriteString(renderDetailCard(detailTitle, strings.Join(stateLines, "\n"), panelW))

	if len(r.workflowSteps) > 0 {
		workflowTitle := "WORKFLOW"
		if r.workflowLabel != "" {
			workflowTitle += " · " + r.workflowLabel
		}
		if current, total := workflowPosition(r); current > 0 && total > 0 {
			workflowTitle += fmt.Sprintf(" · %d/%d", current, total)
		}
		workflowContent := renderWorkflowFlow(r, contentW)
		if next := nextTransition(r); next != "" {
			workflowContent += "\n\n" + mutedStyle.Render(next)
		}
		b.WriteString("\n\n")
		b.WriteString(renderDetailCard(workflowTitle, workflowContent, panelW))
	}

	if r.loadErr != nil {
		b.WriteString("\n\n")
		b.WriteString(renderDetailCard("ERROR", wrap(r.loadErr.Error(), contentW), panelW))
	} else if r.next != "" {
		b.WriteString("\n\n")
		b.WriteString(renderDetailCard(strings.ToUpper(promptLabel(r)), wrap(r.next, contentW), panelW))
	}
	if len(r.history) > 0 {
		b.WriteString("\n\n")
		historyTitle := fmt.Sprintf("HISTORY · %d EVENTS", len(r.history))
		b.WriteString(renderDetailCard(historyTitle, renderTimelineHistory(r.history, contentW, 12), panelW))
	}
	return b.String()
}

func featureDetailTitle(r row) string {
	name := strings.TrimSpace(r.name)
	ticket := strings.TrimSpace(r.ticket)
	prefix := ticket + "-"
	if ticket != "" && len(name) > len(prefix) && strings.EqualFold(name[:len(prefix)], prefix) {
		name = name[len(prefix):]
	}
	if name == "" {
		return ticket
	}
	if ticket == "" || strings.EqualFold(name, ticket) {
		return name
	}
	return name + " · " + ticket
}

func renderDetailCard(title, content string, width int) string {
	outerW := max(12, width)
	innerW := outerW - 2
	contentW := max(1, innerW-2)
	title = truncate(title, max(1, innerW-3))
	label := " " + sectionStyle.Render(title) + " "
	dashRight := max(0, innerW-1-lipgloss.Width(label))
	border := accentStyle
	lines := []string{
		border.Render("╭─") + label + border.Render(strings.Repeat("─", dashRight)+"╮"),
	}
	for _, line := range strings.Split(content, "\n") {
		line = ansi.Truncate(line, contentW, "…")
		lines = append(lines, border.Render("│")+" "+line+strings.Repeat(" ", max(0, contentW-lipgloss.Width(line)))+" "+border.Render("│"))
	}
	lines = append(lines, border.Render("╰"+strings.Repeat("─", innerW)+"╯"))
	return strings.Join(lines, "\n")
}

func joinNonEmpty(separator string, values ...string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, separator)
}

func tmuxTargetLabel(r row) string {
	target := r.session
	if r.window != "" {
		if target != "" {
			target += ":"
		}
		target += r.window
	}
	if r.pane != "" {
		if target != "" {
			target += " · "
		}
		target += r.pane
	}
	return target
}

func wrap(s string, width int) string {
	return terminalui.Wrap(s, max(8, width))
}
