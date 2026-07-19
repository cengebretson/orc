package workspaceui

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

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
				line += styleDim.Render(" — " + ui.Truncate(c.Detail, budget))
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// renderHealthReport renders the full doctor report as a summary followed by
// one labeled panel per check group.
func renderHealthReport(checks []doctor.Check, width int) string {
	if len(checks) == 0 {
		return drawBoxLabeled(styleSection.Render("Health"), []string{styleDim.Render("No health checks.")}, max(20, width))
	}
	outerW := max(20, width)
	type healthGroup struct {
		name   string
		checks []doctor.Check
	}
	groups := make([]healthGroup, 0)
	groupIndex := map[string]int{}
	for _, c := range checks {
		name := strings.TrimSpace(c.Group)
		if name == "" {
			name = "general"
		}
		idx, ok := groupIndex[name]
		if !ok {
			idx = len(groups)
			groupIndex[name] = idx
			groups = append(groups, healthGroup{name: name})
		}
		groups[idx].checks = append(groups[idx].checks, c)
	}

	okCount, warnCount, failCount := healthReportCounts(checks)
	stats := []string{
		styleHealthOK.Render(fmt.Sprintf("✓ %d passing", okCount)),
		styleHealthWarn.Render("⚠ " + healthCountLabel(warnCount, "warning", "warnings")),
		styleHealthErr.Render(fmt.Sprintf("✗ %d failing", failCount)),
	}
	statsLine := strings.Join(stats, styleDim.Render("  ·  "))
	summaryLines := []string{statsLine}
	if lipgloss.Width(statsLine) > outerW-4 {
		summaryLines = stats
	}
	sections := []string{drawBoxLabeledWith(styleHeader.Render("Health summary"), summaryLines, outerW, healthReportBorderColor(warnCount, failCount))}

	for _, group := range groups {
		groupOK, groupWarn, groupFail := healthReportCounts(group.checks)
		title := group.name + "  ·  " + healthCountLabel(len(group.checks), "check", "checks")
		if groupFail > 0 {
			title += fmt.Sprintf("  ·  %d failing", groupFail)
		}
		if groupWarn > 0 {
			title += "  ·  " + healthCountLabel(groupWarn, "warning", "warnings")
		}
		if groupFail == 0 && groupWarn == 0 {
			title += fmt.Sprintf("  ·  %d passing", groupOK)
		}
		title = ui.Truncate(title, max(1, outerW-6))
		lines := renderHealthGroupChecks(group.checks, outerW-4)
		sections = append(sections, drawBoxLabeledWith(styleSection.Render(title), lines, outerW, healthReportBorderColor(groupWarn, groupFail)))
	}
	return strings.Join(sections, "\n")
}

func healthCountLabel(count int, singular, plural string) string {
	label := plural
	if count == 1 {
		label = singular
	}
	return fmt.Sprintf("%d %s", count, label)
}

func healthReportCounts(checks []doctor.Check) (okCount, warnCount, failCount int) {
	for _, c := range checks {
		switch c.Status {
		case doctor.OK:
			okCount++
		case doctor.Warning:
			warnCount++
		default:
			failCount++
		}
	}
	return
}

func healthReportBorderColor(warnings, failures int) string {
	if failures > 0 {
		return activeTheme.Palette.Red
	}
	if warnings > 0 {
		return activeTheme.Palette.Yellow
	}
	return activeTheme.Palette.Green
}

func renderHealthGroupChecks(checks []doctor.Check, contentW int) []string {
	contentW = max(8, contentW)
	var lines []string
	for _, check := range checks {
		icon, statusStyle := healthIconStyle(check.Status)
		headPlain := icon + " " + strings.TrimSpace(check.Name)
		head := statusStyle.Render(icon) + " " + styleSubtext.Render(strings.TrimSpace(check.Name))
		detail := strings.TrimSpace(check.Detail)
		if detail == "" {
			lines = append(lines, "  "+head)
			continue
		}
		inlineW := contentW - lipgloss.Width(headPlain) - 5
		if inlineW >= 8 && lipgloss.Width(detail) <= inlineW {
			lines = append(lines, "  "+head+styleDim.Render("  —  "+detail))
			continue
		}
		lines = append(lines, "  "+head)
		for _, wrapped := range strings.Split(ui.Wrap(detail, max(8, contentW-3)), "\n") {
			lines = append(lines, styleDim.Render("     "+wrapped))
		}
	}
	return lines
}

// sectionBox renders a collapsible labeled box.
// Collapsed: just the top+bottom border with title and summary in the border line.
// Expanded: full box with content.
