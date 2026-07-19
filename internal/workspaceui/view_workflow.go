package workspaceui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/ui"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/lipgloss"
)

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
		for _, l := range strings.Split(ui.Wrap(chain.description, innerW), "\n") {
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
		ui.PadRight(styleTableHeader.Render(""), wCheck) + "  " +
		ui.PadRight(styleTableHeader.Render("Stage"), wStageName) + "  " +
		ui.PadRight(styleTableHeader.Render("Worker"), wWorker) + "  " +
		ui.PadRight(styleTableHeader.Render("Advance"), wAdvance) + "  " +
		styleTableHeader.Render("Active")
	divider := "  " + styleDivider.Render(strings.Repeat("─", innerW-2))

	cursorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(activeTheme.Palette.Mauve))

	stageRowLines := func(step routeStep, absoluteIdx int) []string {
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
		if absoluteIdx == selectedIdx {
			cursor = cursorStyle.Render("▶") + " "
		}
		lines := []string{cursor +
			ui.PadRight(stageExists(step.name), wCheck) + "  " +
			ui.PadRight(styleSubtext.Render(ui.Truncate(stepLabel(step), wStageName)), wStageName) + "  " +
			ui.PadRight(workerLabel(step.workerID), wWorker) + "  " +
			ui.PadRight(advVal, wAdvance) + "  " +
			activeVal}
		if len(step.requiredArtifacts) > 0 {
			lines = append(lines, "  "+strings.Repeat(" ", wCheck+wStageName+4)+styleDim.Render("artifacts: "+strings.Join(step.requiredArtifacts, ", ")))
		}
		return lines
	}

	stageRows := func(steps []routeStep, baseIdx int) []string {
		var lines []string
		lines = append(lines, header, divider)
		for i, step := range steps {
			lines = append(lines, stageRowLines(step, baseIdx+i)...)
		}
		return lines
	}

	sb.WriteString(drawBox(styleSection.Render(" Stages "), stageRows(chain.steps, 0), width) + "\n")

	if len(chain.repairSteps) > 0 {
		repairAsSteps := make([]routeStep, len(chain.repairSteps))
		for i, rs := range chain.repairSteps {
			repairAsSteps[i] = routeStep{name: rs.name, label: rs.label, advance: rs.advance, workerID: rs.workerID, requiredArtifacts: rs.requiredArtifacts}
		}
		// Interleave each annotation directly under its row.
		annotationIndent := "  " + strings.Repeat(" ", wCheck+wStageName+4)
		repairLines := []string{header, divider}
		for i, rs := range chain.repairSteps {
			repairLines = append(repairLines, stageRowLines(repairAsSteps[i], len(chain.steps)+i)...)
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
