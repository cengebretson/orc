package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	switch m.view {
	case viewDetail:
		return m.viewDetail()
	case viewFile:
		return m.viewFile()
	case viewWorkflowDetail:
		return m.viewWorkflowDetailPage()
	case viewCharacterSheet:
		if m.charSheetWorker != nil {
			return renderCharacterSheet(m, m.charSheetWorker)
		}
		return m.viewDashboard()
	default:
		return m.viewDashboard()
	}
}

// ── Dashboard view ────────────────────────────────────────────────

func (m Model) viewDashboard() string {
	outerW := m.width - 2

	// ── Column widths ────────────────────────────────────────────────
	const logoW = 30
	const rightBoxOuter = logoW + 4 // border (2) + 1-space padding each side (2)
	const logoGap = 1
	useLogo := m.width > rightBoxOuter+logoGap+44

	leftW := outerW
	if useLogo {
		leftW = outerW - rightBoxOuter - logoGap
	}
	leftInnerW := leftW - 2

	// ── Header stats ─────────────────────────────────────────────────
	since := time.Since(m.lastRefresh)
	ago := since.Round(time.Second)
	active, paused, broken := 0, 0, 0
	for _, f := range m.features {
		if f.s == nil {
			broken++
			continue
		}
		switch f.s.Status {
		case "active":
			active++
		case "paused":
			paused++
		}
	}
	orcLabel := styleHeader.Render("orc")
	if m.rainbowStep > 0 {
		idx := (rainbowSteps - m.rainbowStep) % len(rainbowPalette)
		c := lipgloss.Color(rainbowPalette[idx])
		orcLabel = lipgloss.NewStyle().Foreground(c).Bold(true).Render("orc")
	}
	headerTitle := orcLabel + styleDim.Render("  workspace orchestrator")
	statsLine := "  " +
		styleSubtext.Render(fmt.Sprintf("%d features", len(m.features))) +
		styleDim.Render("  ·  ") +
		styleHealthOK.Render(fmt.Sprintf("%d active", active)) +
		styleDim.Render("  ·  ") +
		styleStatusWaiting.Render(fmt.Sprintf("%d paused", paused))
	if broken > 0 {
		statsLine += styleDim.Render("  ·  ") +
			styleHealthErr.Render(fmt.Sprintf("⚠ %d broken", broken))
	}
	statsLine += stalenessStyle(since).Render(fmt.Sprintf("  ·  ↺ %s ago", ago))

	// ── Left column: header + sections ───────────────────────────────
	var left strings.Builder
	left.WriteString(drawBoxLabeled(headerTitle, []string{statsLine}, leftW) + "\n")

	healthFocused := m.focusedPane == "section" && m.sectionFocus == "health"
	healthContent := m.healthIssueLines(leftInnerW - 4)
	if len(healthContent) == 0 {
		healthContent = []string{styleHealthOK.Render("All checks passed")}
	}
	if healthFocused {
		healthContent = append(healthContent, "", styleDim.Render("enter to view full report"))
	}
	// Surface the issue count in the collapsed summary so problems are visible
	// even when the section is collapsed (the default).
	healthSummary := styleDim.Render(fmt.Sprintf("%d checks", len(m.healthItems)))
	if extra := m.healthSummaryExtra(); extra != "" {
		healthSummary += styleDim.Render("  ·  ") + extra
	}
	if m.artifactPolicy != "" {
		healthSummary += styleDim.Render("  ·  artifacts ") + artifactPolicyStyle(m.artifactPolicy).Render(m.artifactPolicy)
	}
	left.WriteString(m.sectionBox("health", "1", "Health",
		healthSummary, healthContent, leftW, healthFocused) + "\n")

	wfFocused := m.focusedPane == "section" && m.sectionFocus == "workflows"
	var wfContent []string
	if wfFocused {
		wfContent = renderGroupedWorkflowList(m.workflowGroups, m.sectionCursor)
	} else {
		wfContent = renderWorkflowChainGroups(m.workflows, leftInnerW-4)
	}
	left.WriteString(m.sectionBox("workflows", "2", "Workflows",
		styleDim.Render(fmt.Sprintf("%d", len(m.workflows))),
		wfContent, leftW, wfFocused) + "\n")

	wkFocused := m.focusedPane == "section" && m.sectionFocus == "workers"
	var wkContent []string
	if wkFocused {
		wkContent = renderGroupedWorkerList(m.workerGroups, m.sectionCursor)
	} else {
		wkContent = renderWorkerGroups(m.workerGroups, leftInnerW-4)
		if len(wkContent) == 0 {
			wkContent = renderNameList(leftInnerW-4, m.workerNames)
		}
	}
	left.WriteString(m.sectionBox("workers", "3", "Workers",
		styleDim.Render(fmt.Sprintf("%d", len(m.workerNames))),
		wkContent, leftW, wkFocused) + "\n")

	rtFocused := m.focusedPane == "section" && m.sectionFocus == "routes"
	var rtContent []string
	if rtFocused {
		rtContent = renderNavigableList(m.sectionItems["routes"], m.sectionCursor)
	} else {
		rtContent = renderRepoList(m.repos, leftInnerW-4)
	}
	left.WriteString(m.sectionBox("routes", "4", "Routes",
		styleDim.Render(fmt.Sprintf("%d repos", len(m.repos))),
		rtContent, leftW, rtFocused))

	// ── Top block: left column + right box (logo + quote) ───────────
	var topBlock string
	if useLogo {
		leftStr := left.String()
		leftHeight := lipgloss.Height(leftStr)

		logoColor := lipgloss.Color(activeTheme.Palette.Surface1)
		if m.rainbowStep > 0 {
			// offset by half the palette so logo and header title use different colors
			idx := (rainbowSteps - m.rainbowStep + len(rainbowPalette)/2) % len(rainbowPalette)
			logoColor = lipgloss.Color(rainbowPalette[idx])
		}
		logoStyle := lipgloss.NewStyle().Foreground(logoColor)
		quoteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(activeTheme.Palette.Overlay0)).Italic(true)

		var rightLines []string
		for _, l := range strings.Split(logo, "\n") {
			rightLines = append(rightLines, " "+logoStyle.Render(l))
		}
		rightLines = append(rightLines, "")
		if m.quote != "" {
			for _, l := range strings.Split(wrapText(m.quote, logoW), "\n") {
				centered := lipgloss.PlaceHorizontal(logoW, lipgloss.Center, quoteStyle.Render(l))
				rightLines = append(rightLines, " "+centered)
			}
		}
		targetLines := leftHeight - 2
		if minContent := len(rightLines); targetLines < minContent {
			targetLines = minContent
		}
		for len(rightLines) < targetLines {
			rightLines = append(rightLines, "")
		}
		rightLines = rightLines[:targetLines]

		rightBox := drawBoxLabeledWith("", rightLines, rightBoxOuter, activeTheme.Palette.Surface1)
		topBlock = "\n" + lipgloss.JoinHorizontal(lipgloss.Top, leftStr, strings.Repeat(" ", logoGap), rightBox) + "\n"
	} else {
		topBlock = "\n" + left.String() + "\n"
	}

	// ── Features box (full width, height-capped with scrolling) ──────
	// Remaining height = terminal - topBlock lines - help bar(1) - box borders(2) - table header(2)
	topLines := strings.Count(topBlock, "\n")
	maxDataRows := m.height - topLines - 1 - 4
	if maxDataRows < 1 {
		maxDataRows = 1
	}

	archiveToggle := styleDim.Render("  [a] show archived")
	if m.showArchived {
		archiveToggle = styleDim.Render("  [a] hide archived")
	}

	allRows := m.visibleFeatures()
	total := len(allRows)

	// Scroll window: keep cursor in view.
	offset := 0
	if m.cursor >= maxDataRows {
		offset = m.cursor - maxDataRows + 1
	}
	if offset+maxDataRows > total {
		offset = total - maxDataRows
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + maxDataRows
	if end > total {
		end = total
	}
	visibleRows := allRows[offset:end]

	var featuresTitle string
	if m.searching {
		query := m.search.Value()
		matchCount := len(m.visibleFeatures())
		var matchHint string
		if query == "" {
			matchHint = styleDim.Render("  type to filter  esc cancel")
		} else {
			noun := "matches"
			if matchCount == 1 {
				noun = "match"
			}
			matchHint = styleDim.Render(fmt.Sprintf("  %d %s  esc cancel", matchCount, noun))
		}
		featuresTitle = styleSection.Render("Features") + "  " + m.search.View() + matchHint
	} else if m.search.Value() != "" {
		noun := "matches"
		if total == 1 {
			noun = "match"
		}
		featuresTitle = styleSection.Render("Features") +
			styleDim.Render("  /") + " " + styleStatusWaiting.Render(m.search.Value()) +
			styleDim.Render(fmt.Sprintf("  %d %s  esc clear", total, noun))
	} else {
		featuresTitle = styleSection.Render("Features") + archiveToggle + styleDim.Render("  [/] search")
		if total > maxDataRows {
			featuresTitle += styleDim.Render(fmt.Sprintf("  %d–%d of %d", offset+1, end, total))
		}
	}

	var tableLines []string
	if total == 0 {
		tableLines = []string{"  " + styleDim.Render("No features found. Run orc work <ticket> to start one.")}
	} else {
		tableLines = strings.Split(m.renderTable(visibleRows, outerW-2, m.cursor-offset), "\n")
	}
	featuresBorderColor := activeTheme.Palette.Surface1
	if m.focusedPane == "features" {
		featuresBorderColor = activeTheme.Palette.Mauve
	}

	var b strings.Builder
	b.WriteString(topBlock)
	b.WriteString(drawBoxLabeledWith(featuresTitle, tableLines, outerW, featuresBorderColor) + "\n")

	// ── Help bar ─────────────────────────────────────────────────────
	if !m.searching {
		var helpItems []string
		helpItems = append(helpItems,
			helpItem("↑↓", "navigate"),
			helpItem("enter", "open"),
			helpItem("tab", "focus sections"),
			helpItem("t", "attach"),
			helpItem("1-4", "expand/collapse"),
			helpItem("r", "refresh"),
			helpItem("q", "quit"),
		)
		b.WriteString(styleHelp.Render(" " + strings.Join(helpItems, "  ")))
	}

	return b.String()
}

// drawBox renders a plain rounded box (no title in border).

// healthIconStyle maps a check status to its glyph and color, shared by the
// dashboard health overview, the inline issue lines, and the full report.
func healthIconStyle(s doctor.Status) (string, lipgloss.Style) {
	switch s {
	case doctor.OK:
		return "✓", styleHealthOK
	case doctor.Warning:
		return "⚠", styleHealthWarn
	default:
		return "✗", styleHealthErr
	}
}

// renderHealthLines renders health items grouped by their Group field.
// Items with no group flow together on wrapped rows. Each new group gets a
// header line and its items indented on their own wrapped rows below it.
func (m Model) renderHealthLines(maxW int) []string {
	sep := styleDivider.Render("  ·  ")
	sepW := lipgloss.Width(sep)
	indent := "  "
	indentW := 2

	flushRow := func(rows *[]string, row string) {
		if row != "" {
			*rows = append(*rows, row)
		}
	}

	var rows []string
	row := ""
	rowW := 0
	currentGroup := ""

	for _, item := range m.healthItems {
		icon, s := healthIconStyle(item.Status)
		part := s.Render(icon + " " + strings.TrimSpace(item.Name))

		// group boundary — flush current row and emit header
		if item.Group != currentGroup {
			flushRow(&rows, row)
			row = ""
			rowW = 0
			currentGroup = item.Group
			if currentGroup != "" {
				rows = append(rows, styleDim.Render(currentGroup))
			}
		}

		prefix := ""
		prefixW := 0
		if currentGroup != "" {
			prefix = indent
			prefixW = indentW
		}

		pW := lipgloss.Width(part)
		if rowW > 0 && rowW+sepW+pW > maxW {
			flushRow(&rows, row)
			row = prefix
			rowW = prefixW
		}
		if rowW > prefixW {
			row += sep
			rowW += sepW
		} else if rowW == 0 {
			row = prefix
			rowW = prefixW
		}
		row += part
		rowW += pW
	}
	flushRow(&rows, row)
	return rows
}

// healthSummaryExtra renders a compact "N ✗  M ⚠" badge of non-OK checks for
// the collapsed Health summary, or "" when everything is healthy.
func (m Model) healthSummaryExtra() string {
	warns, fails := 0, 0
	for _, c := range m.healthItems {
		switch c.Status {
		case doctor.Warning:
			warns++
		case doctor.Fail:
			fails++
		}
	}
	var parts []string
	if fails > 0 {
		parts = append(parts, styleHealthErr.Render(fmt.Sprintf("%d ✗", fails)))
	}
	if warns > 0 {
		parts = append(parts, styleHealthWarn.Render(fmt.Sprintf("%d ⚠", warns)))
	}
	return strings.Join(parts, styleDim.Render("  "))
}

// healthIssueLines returns one explanatory line per non-OK check — icon, name,
// and the doctor detail — so the expanded dashboard explains problems instead of
// just flagging them. OK checks stay in the compact wrapped overview.
func (m Model) healthIssueLines(maxW int) []string {
	var lines []string
	for _, c := range m.healthItems {
		if c.Status == doctor.OK {
			continue
		}
		icon, st := healthIconStyle(c.Status)
		head := icon + " " + strings.TrimSpace(c.Name)
		line := st.Render(head)
		if c.Detail != "" {
			budget := maxW - lipgloss.Width(head) - 3
			if budget > 1 {
				line += styleDim.Render(" — " + truncate(c.Detail, budget))
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// renderHealthReport renders the full doctor report — every check with its
// group, status icon, name, and detail — for the Health drill-in viewer. This
// mirrors `orc doctor` output, styled with the active theme.
func renderHealthReport(checks []doctor.Check, width int) string {
	if len(checks) == 0 {
		return styleDim.Render("No health checks.")
	}
	if width < 20 {
		width = 20
	}
	nameW := 0
	for _, c := range checks {
		if w := lipgloss.Width(c.Name); w > nameW {
			nameW = w
		}
	}
	// Column where the detail text begins: "   " + icon + "  " + name + "  ".
	detailCol := 3 + 1 + 2 + nameW + 2
	detailW := width - detailCol
	if detailW < 8 {
		detailW = 8
	}

	var b strings.Builder
	currentGroup := "\x00" // sentinel so the first group prints even when ""
	for _, c := range checks {
		if c.Group != currentGroup {
			currentGroup = c.Group
			if currentGroup != "" {
				b.WriteString(" " + styleSection.Render(currentGroup) + "\n")
			}
		}
		icon, st := healthIconStyle(c.Status)
		prefix := "   " + st.Render(icon) + "  " + styleSubtext.Render(padRight(c.Name, nameW)) + "  "
		if c.Detail == "" {
			b.WriteString(strings.TrimRight(prefix, " ") + "\n")
			continue
		}
		// Wrap long details to fit the width; continuation lines align under
		// the detail column so the report re-flows cleanly on resize.
		wrapped := strings.Split(wrapText(c.Detail, detailW), "\n")
		b.WriteString(prefix + styleDim.Render(wrapped[0]) + "\n")
		indent := strings.Repeat(" ", detailCol)
		for _, cont := range wrapped[1:] {
			b.WriteString(indent + styleDim.Render(cont) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// sectionBox renders a collapsible labeled box.
// Collapsed: just the top+bottom border with title and summary in the border line.
// Expanded: full box with content.
func (m Model) sectionBox(key, keyStr, name, summary string, content []string, outerW int, focused bool) string {
	innerW := outerW - 2
	borderColor := activeTheme.Palette.Surface1
	if focused {
		borderColor = activeTheme.Palette.Mauve
	}
	bd := lipgloss.NewStyle().Foreground(lipgloss.Color(borderColor))
	title := styleDim.Render(keyStr) + " " + styleSection.Render(name)

	if !m.expanded[key] {
		label := " " + title
		if summary != "" {
			label += "  " + summary // caller styles the summary
		}
		label += " "
		labelW := lipgloss.Width(label)
		dashRight := innerW - 1 - labelW
		if dashRight < 0 {
			dashRight = 0
		}
		top := bd.Render("╭─") + label + bd.Render(strings.Repeat("─", dashRight)+"╮")
		bot := bd.Render("╰" + strings.Repeat("─", innerW) + "╯")
		return strings.Join([]string{top, bot}, "\n")
	}

	var indented []string
	for _, l := range content {
		indented = append(indented, "  "+l)
	}
	return drawBoxLabeledWith(title, indented, outerW, borderColor)
}

// renderNavigableList renders a list of section items with a cursor indicator.
func renderNavigableList(items []sectionItem, cursor int) []string {
	var lines []string
	for i, item := range items {
		label := labelWithDimID(item.label, item.id)
		if i == cursor {
			lines = append(lines, styleHealthOK.Render("▶")+"  "+styleSubtext.Render(label)+
				styleDim.Render("  enter to view"))
		} else {
			lines = append(lines, "   "+styleDim.Render(label))
		}
	}
	return lines
}

func renderWorkerGroups(groups []workerGroup, maxW int) []string {
	var lines []string
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		var names []string
		for _, item := range group.items {
			names = append(names, item.label)
		}
		for _, line := range renderNameList(maxW-2, names) {
			lines = append(lines, "  "+line)
		}
	}
	return lines
}

func renderWorkflowGroups(groups []workflowGroup, maxW int) []string {
	var lines []string
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		var names []string
		for _, item := range group.items {
			names = append(names, item.label)
		}
		for _, line := range renderNameList(maxW-2, names) {
			lines = append(lines, "  "+line)
		}
	}
	return lines
}

func renderWorkflowChainGroups(chains []workflowChain, maxW int) []string {
	grouped := map[string][]workflowChain{}
	for _, chain := range chains {
		grouped[workflowNamespace(chain.name)] = append(grouped[workflowNamespace(chain.name)], chain)
	}

	var lines []string
	for _, groupName := range sortedKeys(grouped) {
		lines = append(lines, styleDim.Render(groupName))
		for _, chain := range grouped[groupName] {
			if chain.name != "" {
				lines = append(lines, "  "+workflowChainHeading(chain)+":")
			}
			for _, line := range renderRouteChain(chain.steps, chain.loops, maxW-2) {
				lines = append(lines, "  "+line)
			}
			lines = append(lines, "")
		}
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func workflowChainHeading(c workflowChain) string {
	return styleDim.Render(labelWithDimID(chainLabel(c), c.name))
}

func renderGroupedWorkerList(groups []workerGroup, cursor int) []string {
	var lines []string
	itemIdx := 0
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		for _, item := range group.items {
			label := labelWithDimID(item.label, item.id)
			if itemIdx == cursor {
				lines = append(lines, styleHealthOK.Render("▶")+"  "+styleSubtext.Render(label)+
					styleDim.Render("  enter to view"))
			} else {
				lines = append(lines, "   "+styleDim.Render(label))
			}
			itemIdx++
		}
	}
	if len(lines) == 0 {
		return []string{styleDim.Render("No workers configured.")}
	}
	return lines
}

func renderGroupedWorkflowList(groups []workflowGroup, cursor int) []string {
	var lines []string
	itemIdx := 0
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		for _, item := range group.items {
			label := labelWithDimID(item.label, item.id)
			if itemIdx == cursor {
				lines = append(lines, styleHealthOK.Render("▶")+"  "+styleSubtext.Render(label)+
					styleDim.Render("  enter to view"))
			} else {
				lines = append(lines, "   "+styleDim.Render(label))
			}
			itemIdx++
		}
	}
	if len(lines) == 0 {
		return []string{styleDim.Render("No workflows configured.")}
	}
	return lines
}

// renderNameList wraps a list of names with · separators to fit maxW.
func renderNameList(maxW int, names []string) []string {
	sep := styleDivider.Render("  ·  ")
	sepW := lipgloss.Width(sep)

	var rows []string
	row := ""
	rowW := 0
	for _, name := range names {
		chip := styleSubtext.Render(name)
		chipW := lipgloss.Width(chip)
		if rowW > 0 && rowW+sepW+chipW > maxW {
			rows = append(rows, row)
			row = ""
			rowW = 0
		}
		if rowW > 0 {
			row += sep
			rowW += sepW
		}
		row += chip
		rowW += chipW
	}
	if row != "" {
		rows = append(rows, row)
	}
	return rows
}

// renderRepoList renders repos as "name — purpose" lines.
func renderRepoList(repos []config.Repo, maxW int) []string {
	if len(repos) == 0 {
		return []string{styleDim.Render("No repos configured. Edit orc.yaml to add repos.")}
	}
	var lines []string
	for _, r := range repos {
		name := styleSubtext.Render(r.Name)
		sep := styleDivider.Render("  —  ")
		purpose := styleDim.Render(r.Purpose)
		var badges []string
		if r.WorktreeSetup != "" {
			badges = append(badges, styleHealthOK.Render("setup"))
		}
		if len(r.AgentHints) > 0 {
			badges = append(badges, styleStatusWaiting.Render("hints"))
		}
		badgeText := ""
		if len(badges) > 0 {
			badgeText = styleDim.Render("  ") + strings.Join(badges, styleDim.Render(" "))
		}
		line := name + badgeText + sep + purpose
		if lipgloss.Width(line) > maxW {
			purpose = styleDim.Render(truncate(r.Purpose, maxW-lipgloss.Width(name+badgeText+sep)))
			line = name + badgeText + sep + purpose
		}
		lines = append(lines, line)
	}
	return lines
}

func artifactPolicyStyle(policy string) lipgloss.Style {
	if policy == "block" {
		return styleHealthWarn
	}
	return styleHealthOK
}

// renderRouteChain renders the workflow stage sequence with colored arrows and loop stage annotations.
func renderRouteChain(chain []routeStep, loops []repairLoop, maxW int) []string {
	if len(chain) == 0 {
		return nil
	}
	sep := styleDivider.Render("  ")
	sepW := lipgloss.Width(sep)

	// build index: workflow name → x-offset in rendered row
	chipOffsets := map[string]int{}

	var rows []string
	row := ""
	rowW := 0
	for i, step := range chain {
		chip := styleSubtext.Render(stepLabel(step))
		chipW := lipgloss.Width(chip)

		var arrow string
		var arrowW int
		if i < len(chain)-1 {
			if chain[i].advance == "manual" {
				arrow = sep + styleStatusWaiting.Render("→") + sep
			} else {
				arrow = sep + styleHealthOK.Render("→") + sep
			}
			arrowW = sepW*2 + 1
		}

		needed := chipW + arrowW
		if rowW > 0 && rowW+needed > maxW {
			rows = append(rows, row)
			row = ""
			rowW = 0
		}
		chipOffsets[step.name] = rowW
		row += chip
		rowW += chipW
		if arrow != "" {
			row += arrow
			rowW += arrowW
		}
	}
	if row != "" {
		rows = append(rows, row)
	}

	// loop stage annotations: ↺ name positioned under target chip
	if len(loops) > 0 {
		// group loops by target for layout
		type loopAnnotation struct {
			offset int
			label  string
		}
		var annotations []loopAnnotation
		for _, lp := range loops {
			offset, ok := chipOffsets[lp.target]
			if !ok {
				continue
			}
			loopLabel := lp.label
			if loopLabel == "" {
				loopLabel = lp.name
			}
			label := styleStatusWaiting.Render("↺ ") + styleSubtext.Render(loopLabel)
			annotations = append(annotations, loopAnnotation{offset: offset, label: label})
		}
		// sort by offset so we build the line left-to-right
		sort.Slice(annotations, func(i, j int) bool {
			return annotations[i].offset < annotations[j].offset
		})
		if len(annotations) > 0 {
			loopLine := ""
			loopW := 0
			for _, ann := range annotations {
				if ann.offset > loopW {
					loopLine += strings.Repeat(" ", ann.offset-loopW)
					loopW = ann.offset
				}
				connector := styleDivider.Render("└╴")
				full := connector + ann.label
				w := lipgloss.Width(full)
				loopLine += full
				loopW += w
			}
			rows = append(rows, loopLine)
		}
	}

	return rows
}

func (m Model) renderTable(rows []*featureRow, w int, selectedIdx int) string {
	const (
		wTicket = 12
		wStatus = 12
		wTmux   = 6
	)
	// fixed overhead: leading space + static columns + separators (5 × "  ")
	fixed := 1 + wTicket + wStatus + wTmux + 5*2
	flex := w - fixed
	if flex < 48 {
		flex = 48
	}
	wName := flex / 2
	remaining := flex - wName
	wWorkflow := remaining / 2
	wWorker := remaining - wWorkflow

	header := " " +
		padRight(styleTableHeader.Render("Ticket"), wTicket) + "  " +
		padRight(styleTableHeader.Render("Name"), wName) + "  " +
		padRight(styleTableHeader.Render("Status"), wStatus) + "  " +
		padRight(styleTableHeader.Render("Stage"), wWorkflow) + "  " +
		padRight(styleTableHeader.Render("Worker"), wWorker) + "  " +
		padRight(styleTableHeader.Render("Tmux"), wTmux)

	div := " " + styleDivider.Render(strings.Repeat("─", w-1))

	var lines []string
	lines = append(lines, header, div)

	for i, row := range rows {
		selected := i == selectedIdx

		if row.s == nil {
			lines = append(lines, brokenRow(row, w, wTicket, selected))
			continue
		}
		s := row.s

		icon := statusIcon(s.Status)
		name := strings.TrimPrefix(s.Slug, s.Ticket+"-")
		workflowLabel := row.workflowLabel
		if workflowLabel == "" {
			workflowLabel = row.workflow
		}
		stageLabel := row.stageLabel
		if stageLabel == "" {
			stageLabel = s.Stage.Name
		}
		stageCell := workflowLabel + " › " + stageLabel + row.stageLoopLabel
		if s.Runtime.JIT != nil {
			stageCell += " + jit"
		}

		plainWorker := row.workerName
		if plainWorker == "" {
			plainWorker = "—"
		}
		plainTmux := "-"
		if s.Runtime.Tmux != nil {
			if row.tmuxLive {
				plainTmux = "✓"
			} else {
				plainTmux = "✗"
			}
		}

		ticketCell := truncate(s.Ticket, wTicket)
		if row.hasIssues {
			ticketCell = truncate("! "+s.Ticket, wTicket)
		}

		if selected {
			// Plain unstyled text so styleRowSelected background covers the full row
			line := " " +
				padRight(ticketCell, wTicket) + "  " +
				padRight(truncate(name, wName), wName) + "  " +
				padRight(truncate(icon+" "+s.Status, wStatus), wStatus) + "  " +
				padRight(truncate(stageCell, wWorkflow), wWorkflow) + "  " +
				padRight(truncate(plainWorker, wWorker), wWorker) + "  " +
				padRight(plainTmux, wTmux)
			lines = append(lines, styleRowSelected.Width(w).Render(line))
		} else {
			ticketStyled := ticketCell
			if row.hasIssues {
				ticketStyled = styleHealthWarn.Render(ticketCell)
			}
			statusCell := statusStyle(s.Status).Render(icon + " " + s.Status)
			nameCell := styleDim.Render(truncate(name, wName))
			workerCell := styleDim.Render(truncate(plainWorker, wWorker))
			var tmuxCell string
			if s.Runtime.Tmux != nil {
				if row.tmuxLive {
					tmuxCell = styleTmuxLive.Render(plainTmux)
				} else {
					tmuxCell = styleTmuxDead.Render(plainTmux)
				}
			} else {
				tmuxCell = styleTmuxNone.Render(plainTmux)
			}
			line := " " +
				padRight(ticketStyled, wTicket) + "  " +
				padRight(nameCell, wName) + "  " +
				padRight(statusCell, wStatus) + "  " +
				padRight(truncate(stageCell, wWorkflow), wWorkflow) + "  " +
				padRight(workerCell, wWorker) + "  " +
				padRight(tmuxCell, wTmux)
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

// brokenRow renders a feature whose STATE.yaml could not be parsed: ticket from
// the directory name, a red "broken" status, and the parse error in the stage
// column. The "!" health marker flags it like any other issue.
func brokenRow(row *featureRow, w, wTicket int, selected bool) string {
	ticket := truncate(row.ticketID(), wTicket)
	reason := "unreadable STATE.yaml"
	if row.loadErr != nil {
		reason = row.loadErr.Error()
	}
	const marker = "⚠ broken — "
	// width left for the reason after the leading space, ticket col, separator,
	// and the marker
	reasonW := w - (1 + wTicket + 2 + lipgloss.Width(marker))
	if reasonW < 0 {
		reasonW = 0
	}
	reason = truncate(reason, reasonW)

	if selected {
		line := " " + padRight(ticket, wTicket) + "  " + marker + reason
		return styleRowSelected.Width(w).Render(line)
	}
	return " " +
		padRight(styleHealthErr.Render(ticket), wTicket) + "  " +
		styleHealthErr.Render("⚠ broken") + styleDim.Render(" — "+reason)
}
