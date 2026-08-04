package watch

import (
	"fmt"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/charmbracelet/lipgloss"
)

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
	if r.lifecycleSince.After(latest) {
		latest = r.lifecycleSince
	}
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
