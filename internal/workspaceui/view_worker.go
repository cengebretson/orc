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
		outerW := width
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
		detailsBox := drawBoxLabeledWith(
			styleHeader.Render(workerName),
			lines,
			outerW,
			activeTheme.Palette.Mauve,
		)

		activeFeatureRows := activeWorkerFeatureRows(w.ID, features)
		label := styleSection.Render(fmt.Sprintf("Active Features (%d)", len(activeFeatureRows)))
		activeRows := renderActiveFeatureRows(activeFeatureRows, outerW-2)
		activeBox := drawBoxLabeled(label, activeRows, outerW)
		if width >= 100 {
			const gapW = 2
			leftW := (outerW - gapW) / 2
			rightW := outerW - gapW - leftW
			detailsBox = drawBoxLabeledWith(styleHeader.Render(workerName), lines, leftW, activeTheme.Palette.Mauve)
			activeRows = renderActiveFeatureRows(activeFeatureRows, rightW-2)
			activeBox = drawBoxLabeled(label, activeRows, rightW)
			detailsBox, activeBox = equalizeBoxHeights(detailsBox, activeBox)
			sb.WriteString(joinColumns(detailsBox, activeBox, "  ") + "\n")
		} else {
			sb.WriteString(detailsBox + "\n")
			sb.WriteString(activeBox + "\n")
		}
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

type activeFeatureRow struct {
	ticket string
	stage  string
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
		rows = append(rows, activeFeatureRow{
			ticket: row.s.Ticket,
			stage:  workflowLabel + "/" + stageLabel,
		})
	}
	return rows
}

func renderActiveFeatureRows(rows []activeFeatureRow, width int) []string {
	const maxVisibleActiveFeatures = 5
	if width < 20 {
		width = 20
	}
	ticketW := min(14, max(8, width/3))
	stageW := width - ticketW - 2
	if stageW < 1 {
		stageW = 1
	}
	visibleRows := rows
	if len(visibleRows) > maxVisibleActiveFeatures {
		visibleRows = visibleRows[:maxVisibleActiveFeatures]
	}
	lines := make([]string, 0, len(visibleRows)+1)
	for _, row := range visibleRows {
		ticket := styleSubtext.Render(ui.PadRight(ui.Truncate(row.ticket, ticketW), ticketW))
		stage := styleDim.Render(ui.Truncate(row.stage, stageW))
		lines = append(lines, ticket+"  "+stage)
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
