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
	orcLabel := styleHeader.Render("orc")
	if m.effects.rainbowStep > 0 {
		c := lipgloss.Color(terminalui.RainbowColor(m.effects.rainbowStep, 0))
		orcLabel = lipgloss.NewStyle().Foreground(c).Bold(true).Render("orc")
	}
	headerTitle := orcLabel + styleDim.Render("  workspace orchestrator")
	needsYouStyle := styleHealthErr.Bold(true)
	if overview.needs > 0 {
		needsYouStyle = breathStyle(m.effects.breathPhase)
	}
	statsLine := "  " +
		styleSubtext.Render(fmt.Sprintf("%d FEATURES", overview.features)) +
		styleDim.Render("  ·  ") +
		styleStatusReady.Bold(true).Render(fmt.Sprintf("● %d RUNNING", overview.running)) +
		styleDim.Render("  ·  ") +
		styleStatusWaiting.Bold(true).Render(fmt.Sprintf("◐ %d PAUSED", overview.paused)) +
		styleDim.Render("  ·  ") +
		needsYouStyle.Render(fmt.Sprintf("! %d NEEDS YOU", overview.needs))
	if overview.broken > 0 {
		statsLine += styleDim.Render("  ·  ") +
			styleHealthErr.Render(fmt.Sprintf("⚠ %d broken", overview.broken))
	}
	interval := m.lifecycle.refreshInterval
	if interval <= 0 {
		interval = defaultRefreshInterval
	}
	fullAge := time.Since(m.lifecycle.lastRefresh)
	if m.lifecycle.lastRefresh.IsZero() {
		fullAge = 0
	}
	statsLine += stalenessStyle(overview.refreshAge).Render(fmt.Sprintf("  ·  ↺ %s", refreshCountdown(fullAge, interval)))
	return drawBoxLabeled(headerTitle, []string{statsLine}, width)
}
