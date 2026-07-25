package workspaceui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/ui"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

func renderWorkerSelector(groups []workerGroup, cursor, width int) string {
	outerW := max(20, width)
	total := 0
	for _, group := range groups {
		total += len(group.items)
	}
	var lines []string
	itemIndex := 0
	for _, group := range groups {
		if len(group.items) == 0 {
			continue
		}
		lines = append(lines, styleDim.Render(group.name))
		for _, item := range group.items {
			label := coloredWorkerLabel(item.label, item.id)
			if itemIndex == cursor {
				lines = append(lines, "  "+styleHealthOK.Render("▶")+"  "+label)
			} else {
				lines = append(lines, "     "+label)
			}
			itemIndex++
		}
	}
	if total == 0 {
		lines = append(lines, styleDim.Render("No workers configured."))
	}
	return drawBoxLabeledWith(styleHeader.Render("Workers"), padInspectorLines(lines), outerW, activeTheme.Palette.Mauve)
}

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
		activeFeatureRows := activeWorkerFeatureRows(w.ID, features)

		// Build info rows: label → value pairs. Role and active-feature count
		// are pulled up here from the markdown body and the Active Features
		// panel so the card is a complete at-a-glance summary on its own.
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
		add("role", extractMarkdownSection(body, "Role"))
		argKeys := make([]string, 0, len(w.Args))
		for k := range w.Args {
			argKeys = append(argKeys, k)
		}
		sort.Strings(argKeys)
		for _, k := range argKeys {
			add(k, w.Args[k])
		}
		add("active", fmt.Sprintf("%d feature(s)", len(activeFeatureRows)))

		// Measure label column width.
		labelW := 0
		for _, r := range rows {
			if len(r.label) > labelW {
				labelW = len(r.label)
			}
		}

		// Render rows as styled lines, wrapping long values (like role) under
		// the label column instead of letting them run off the box edge.
		outerW := width
		valueW := max(10, outerW-2-labelW-4)
		var lines []string
		for _, r := range rows {
			pad := strings.Repeat(" ", labelW-len(r.label))
			label := styleDetailLabel.Render(r.label+pad) + styleDim.Render("  ")
			wrapped := strings.Split(ui.Wrap(r.value, valueW), "\n")
			for i, wline := range wrapped {
				if i == 0 {
					lines = append(lines, "  "+label+styleDetailValue.Render(wline))
					continue
				}
				lines = append(lines, "  "+strings.Repeat(" ", labelW+2)+styleDetailValue.Render(wline))
			}
		}

		workerName := w.Name
		if workerName == "" {
			workerName = w.ID
		}
		accent := workerAccentColor(w.ID)
		accentTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true).Render(workerName)
		detailsBox := drawBoxLabeledWith(accentTitle, lines, outerW, accent)
		sb.WriteString(detailsBox + "\n")

		activeLabel := styleSection.Render(fmt.Sprintf("Active Features (%d)", len(activeFeatureRows)))
		activeRows := padInspectorLines(renderActiveFeatureRows(activeFeatureRows, outerW-6))
		sb.WriteString(drawBoxLabeled(activeLabel, activeRows, outerW) + "\n")
	}

	// Render markdown body, wrapped in a panel to match the boxed layout
	// above instead of flowing as unboxed text.
	if body != "" {
		innerW := max(20, width-4)
		r, err := glamour.NewTermRenderer(
			glamour.WithStylesFromJSONBytes(activeTheme.Glamour),
			glamour.WithWordWrap(innerW),
		)
		var mdLines []string
		if err == nil {
			if out, err := r.Render(body); err == nil {
				mdLines = strings.Split(strings.TrimRight(out, "\n"), "\n")
			} else {
				mdLines = strings.Split(body, "\n")
			}
		} else {
			mdLines = strings.Split(body, "\n")
		}
		sb.WriteString(drawBox(styleSection.Render(" Documentation "), mdLines, width))
	}

	return sb.String(), nil
}

// extractMarkdownSection returns the trimmed, single-line text of the named
// "## <heading>" section in a worker markdown body, or "" if the heading
// isn't present. Used to surface a one-line role summary in the info card
// without duplicating the full body.
func extractMarkdownSection(body, heading string) string {
	marker := "## " + heading
	idx := strings.Index(body, marker)
	if idx < 0 {
		return ""
	}
	rest := body[idx+len(marker):]
	if next := strings.Index(rest, "\n##"); next >= 0 {
		rest = rest[:next]
	}
	return strings.Join(strings.Fields(rest), " ")
}

type activeFeatureRow struct {
	ticket  string
	stage   string
	status  string // pre-styled icon + label, e.g. "◆ review"
	context string // pre-styled context-pressure label, or a dim "-"
}

func activeWorkerFeatureRows(workerID string, features []*featureRow) []activeFeatureRow {
	var rows []activeFeatureRow
	for _, row := range features {
		if row.s == nil {
			continue
		}
		if row.s.Stage.Worker != workerID || row.s.Status == "archived" {
			continue
		}
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
		icon, label := featureDisplayState(row)
		rows = append(rows, activeFeatureRow{
			ticket:  row.s.Ticket,
			stage:   workflowLabel + "/" + stageLabel,
			status:  featureStateStyle(row).Render(icon + " " + label),
			context: renderContextPressure(row.context),
		})
	}
	return rows
}

func renderActiveFeatureRows(rows []activeFeatureRow, width int) []string {
	if len(rows) == 0 {
		return []string{styleDim.Render("No active features.")}
	}
	const maxVisibleActiveFeatures = 8
	if width < 30 {
		width = 30
	}
	const (
		statusW  = 12
		contextW = 8
	)
	ticketW := min(14, max(8, width/5))
	stageW := width - ticketW - statusW - contextW - 6
	if stageW < 10 {
		stageW = 10
	}
	header := styleTableHeader.Render(ui.PadRight("Ticket", ticketW)) + "  " +
		styleTableHeader.Render(ui.PadRight("Stage", stageW)) + "  " +
		styleTableHeader.Render(ui.PadRight("Status", statusW)) + "  " +
		styleTableHeader.Render("Context")
	visibleRows := rows
	if len(visibleRows) > maxVisibleActiveFeatures {
		visibleRows = visibleRows[:maxVisibleActiveFeatures]
	}
	lines := make([]string, 0, len(visibleRows)+2)
	lines = append(lines, header)
	for _, row := range visibleRows {
		ticket := styleSubtext.Render(ui.PadRight(ui.Truncate(row.ticket, ticketW), ticketW))
		stage := styleDim.Render(ui.PadRight(ui.Truncate(row.stage, stageW), stageW))
		status := ui.PadRight(row.status, statusW)
		lines = append(lines, ticket+"  "+stage+"  "+status+"  "+row.context)
	}
	if hidden := len(rows) - len(visibleRows); hidden > 0 {
		lines = append(lines, styleDim.Render(fmt.Sprintf("+%d more", hidden)))
	}
	return lines
}

func equalizeBoxHeights(left, right string) (string, string) {
	leftLines := strings.Split(strings.TrimRight(left, "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right, "\n"), "\n")
	if len(leftLines) == len(rightLines) {
		return left, right
	}
	if len(leftLines) < len(rightLines) {
		return padBoxHeight(leftLines, len(rightLines)), right
	}
	return left, padBoxHeight(rightLines, len(leftLines))
}

func padBoxHeight(lines []string, target int) string {
	if len(lines) < 2 {
		return strings.Join(lines, "\n")
	}
	bottom := lines[len(lines)-1]
	body := lines[:len(lines)-1]
	innerW := lipgloss.Width(bottom) - 2
	if innerW < 0 {
		innerW = 0
	}
	for len(body)+1 < target {
		body = append(body, "│"+strings.Repeat(" ", innerW)+"│")
	}
	body = append(body, bottom)
	return strings.Join(body, "\n")
}

// renderWorkflowFile renders a stage markdown file. width is the available viewport width.
// renderWorkflowDetail renders an inline detail view for a named workflow.
