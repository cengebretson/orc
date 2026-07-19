package workspaceui

import (
	"fmt"
	"strings"
	"time"

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
	overview := workspaceOverviewFor(m.features, m.lastRefresh, time.Now())
	ago := overview.refreshAge.Round(time.Second)
	orcLabel := styleHeader.Render("orc")
	if m.rainbowStep > 0 {
		idx := (rainbowSteps - m.rainbowStep) % len(rainbowPalette)
		c := lipgloss.Color(rainbowPalette[idx])
		orcLabel = lipgloss.NewStyle().Foreground(c).Bold(true).Render("orc")
	}
	headerTitle := orcLabel + styleDim.Render("  workspace orchestrator")
	statsLine := "  " +
		styleSubtext.Render(fmt.Sprintf("%d features", overview.features)) +
		styleDim.Render("  ·  ") +
		styleHealthOK.Render(fmt.Sprintf("%d active", overview.active)) +
		styleDim.Render("  ·  ") +
		styleStatusWaiting.Render(fmt.Sprintf("%d paused", overview.paused))
	if overview.broken > 0 {
		statsLine += styleDim.Render("  ·  ") +
			styleHealthErr.Render(fmt.Sprintf("⚠ %d broken", overview.broken))
	}
	statsLine += stalenessStyle(overview.refreshAge).Render(fmt.Sprintf("  ·  ↺ %s ago", ago))

	// ── Left column: header + sections ───────────────────────────────
	var left strings.Builder
	left.WriteString(drawBoxLabeled(headerTitle, []string{statsLine}, leftW) + "\n")

	healthSpec := sectionSpecFor(sectionHealth)
	healthFocused := m.focusedPane == paneSection && m.sectionFocus == sectionHealth
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
	left.WriteString(m.sectionBox(healthSpec,
		healthSummary, healthContent, leftW, healthFocused) + "\n")

	wfSpec := sectionSpecFor(sectionWorkflows)
	wfFocused := m.focusedPane == paneSection && m.sectionFocus == sectionWorkflows
	var wfContent []string
	if wfFocused {
		wfContent = renderGroupedWorkflowList(m.workflowGroups, m.sectionCursor)
	} else {
		wfContent = renderWorkflowChainGroups(m.workflows, leftInnerW-4)
	}
	left.WriteString(m.sectionBox(wfSpec,
		styleDim.Render(fmt.Sprintf("%d", len(m.workflows))),
		wfContent, leftW, wfFocused) + "\n")

	wkSpec := sectionSpecFor(sectionWorkers)
	wkFocused := m.focusedPane == paneSection && m.sectionFocus == sectionWorkers
	var wkContent []string
	if wkFocused {
		wkContent = renderGroupedWorkerList(m.workerGroups, m.sectionCursor)
	} else {
		wkContent = renderWorkerGroups(m.workerGroups, leftInnerW-4)
		if len(wkContent) == 0 {
			wkContent = renderNameList(leftInnerW-4, m.workerNames)
		}
	}
	left.WriteString(m.sectionBox(wkSpec,
		styleDim.Render(fmt.Sprintf("%d", len(m.workerNames))),
		wkContent, leftW, wkFocused) + "\n")

	repoSpec := sectionSpecFor(sectionRepositories)
	rtFocused := m.focusedPane == paneSection && m.sectionFocus == sectionRepositories
	rtContent := renderRepoRoutingOverview(m.repos, m.routes, leftInnerW-4)
	if rtFocused {
		rtContent = append(rtContent, "", styleDim.Render("enter to inspect repositories"))
	}
	left.WriteString(m.sectionBox(repoSpec,
		styleDim.Render(fmt.Sprintf("%d repos  ·  %d routes", len(m.repos), len(m.routes))),
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
	if m.focusedPane == paneFeatures {
		featuresBorderColor = activeTheme.Palette.Mauve
	}

	var b strings.Builder
	b.WriteString(topBlock)
	b.WriteString(drawBoxLabeledWith(featuresTitle, tableLines, outerW, featuresBorderColor) + "\n")

	// ── Help bar ─────────────────────────────────────────────────────
	if !m.searching && !m.embedded {
		var helpItems []string
		helpItems = append(helpItems,
			combinedBindingHelp("navigate", keys.up, keys.down),
			bindingHelp(keys.open),
			bindingHelp(keys.cycleForward),
			bindingHelp(keys.attach),
			helpItem("1-4", "expand/collapse"),
			bindingHelp(keys.refresh),
			bindingHelp(keys.quit),
		)
		b.WriteString(styleHelp.Render(" " + strings.Join(helpItems, "  ")))
	}

	return b.String()
}

// drawBox renders a plain rounded box (no title in border).

// healthIconStyle maps a check status to its glyph and color, shared by the
// dashboard health overview, the inline issue lines, and the full report.
