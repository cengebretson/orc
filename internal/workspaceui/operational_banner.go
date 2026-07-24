package workspaceui

import (
	"fmt"
	"time"

	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

func (m Model) operationalBanner(width int) string {
	lastLiveRefresh := m.lifecycle.lastLiveRefresh
	if lastLiveRefresh.IsZero() {
		lastLiveRefresh = m.lifecycle.lastRefresh
	}
	overview := workspaceOverviewFor(m.data.features, lastLiveRefresh, time.Now())
	ago := overview.refreshAge.Round(time.Second)
	orcLabel := styleHeader.Render("orc")
	if m.effects.rainbowStep > 0 {
		c := lipgloss.Color(terminalui.RainbowColor(m.effects.rainbowStep, 0))
		orcLabel = lipgloss.NewStyle().Foreground(c).Bold(true).Render("orc")
	}
	headerTitle := orcLabel + styleDim.Render("  workspace orchestrator")
	statsLine := "  " +
		styleSubtext.Render(fmt.Sprintf("%d FEATURES", overview.features)) +
		styleDim.Render("  ·  ") +
		styleStatusReady.Bold(true).Render(fmt.Sprintf("● %d RUNNING", overview.running)) +
		styleDim.Render("  ·  ") +
		styleStatusWaiting.Bold(true).Render(fmt.Sprintf("◐ %d PAUSED", overview.paused)) +
		styleDim.Render("  ·  ") +
		styleHealthErr.Bold(true).Render(fmt.Sprintf("! %d NEEDS YOU", overview.needs))
	if overview.broken > 0 {
		statsLine += styleDim.Render("  ·  ") +
			styleHealthErr.Render(fmt.Sprintf("⚠ %d broken", overview.broken))
	}
	statsLine += stalenessStyle(overview.refreshAge).Render(fmt.Sprintf("  ·  ↺ %s ago", ago))
	return drawBoxLabeled(headerTitle, []string{statsLine}, width)
}
