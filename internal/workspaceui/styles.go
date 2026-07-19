package workspaceui

import (
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

// Theme holds the palette and glamour style for a dashboard theme.
type Theme = terminalui.Theme

var activeTheme Theme

// LoadTheme loads a theme by name from the embedded themes directory.
// Falls back to catppuccin-mocha if name is empty or not found.
func LoadTheme(name string) error {
	theme, err := terminalui.LoadTheme(name)
	if err != nil {
		return err
	}
	SetTheme(theme)
	return nil
}

func SetTheme(theme Theme) {
	activeTheme = theme
	initStyles()
}

func init() {
	if err := LoadTheme(""); err != nil {
		panic("failed to load default theme: " + err.Error())
	}
}

var logo = terminalui.Logo()

// Style vars — initialized by initStyles(), called from LoadTheme.
var (
	styleDim     lipgloss.Style
	styleSubtext lipgloss.Style

	styleHeader  lipgloss.Style
	styleSection lipgloss.Style

	styleTableHeader lipgloss.Style
	styleRowSelected lipgloss.Style

	styleStatusInProgress lipgloss.Style
	styleStatusWaiting    lipgloss.Style
	styleStatusArchived   lipgloss.Style
	styleStatusPending    lipgloss.Style
	styleStatusReady      lipgloss.Style

	styleHealthOK   lipgloss.Style
	styleHealthWarn lipgloss.Style
	styleHealthErr  lipgloss.Style

	styleDetailLabel lipgloss.Style
	styleDetailValue lipgloss.Style
	styleDetailTitle lipgloss.Style

	styleFileOK       lipgloss.Style
	styleFileMissing  lipgloss.Style
	styleFileSelected lipgloss.Style

	styleHelp    lipgloss.Style
	styleHelpKey lipgloss.Style

	styleTmuxLive lipgloss.Style
	styleTmuxDead lipgloss.Style
	styleTmuxNone lipgloss.Style

	styleDivider lipgloss.Style
)

func initStyles() {
	p := activeTheme.Palette

	styleDim = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0))
	styleSubtext = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Subtext0))

	styleHeader = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve)).Bold(true)
	styleSection = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Lavender)).Bold(true)

	styleTableHeader = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Subtext0)).Bold(true)
	styleRowSelected = lipgloss.NewStyle().Background(lipgloss.Color(p.Surface0)).Foreground(lipgloss.Color(p.Text))

	styleStatusInProgress = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve))
	styleStatusWaiting = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	styleStatusArchived = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0))
	styleStatusPending = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Peach))
	styleStatusReady = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Sky))

	styleHealthOK = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green))
	styleHealthWarn = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	styleHealthErr = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))

	styleDetailLabel = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Subtext0))
	styleDetailValue = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Text))
	styleDetailTitle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve)).Bold(true)

	styleFileOK = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green)).Background(lipgloss.Color(p.Surface0)).Padding(0, 1)
	styleFileMissing = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0)).Background(lipgloss.Color(p.Surface0)).Padding(0, 1)
	styleFileSelected = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Base)).Background(lipgloss.Color(p.Mauve)).Padding(0, 1)

	styleHelp = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0))
	styleHelpKey = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Subtext1))

	styleTmuxLive = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green))
	styleTmuxDead = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))
	styleTmuxNone = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0))

	styleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Surface1))
}

func statusStyle(status string) lipgloss.Style {
	switch status {
	case "active":
		return styleStatusInProgress
	case "paused":
		return styleStatusWaiting
	case "ready":
		return styleStatusReady
	case "done":
		return styleStatusArchived
	case "archived":
		return styleStatusArchived
	case "pending":
		return styleStatusPending
	default:
		return styleSubtext
	}
}

func statusIcon(status string) string {
	switch status {
	case "active":
		return "▶"
	case "paused":
		return "◐"
	case "ready":
		return "▷"
	case "done":
		return "✓"
	case "archived":
		return "✓"
	case "pending":
		return "○"
	default:
		return "·"
	}
}

func stalenessStyle(age time.Duration) lipgloss.Style {
	p := activeTheme.Palette
	switch {
	case age > 45*time.Second:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))
	case age > 15*time.Second:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	default:
		return styleDim
	}
}

func renderContextPressure(pressure contextpressure.Pressure) string {
	style := styleDim
	switch pressure.Level {
	case contextpressure.LevelGreen:
		style = styleHealthOK
	case contextpressure.LevelYellow:
		style = styleHealthWarn
	case contextpressure.LevelRed:
		style = styleHealthErr
	}
	return style.Render(pressure.Label())
}

func helpItem(key, desc string) string {
	return styleHelpKey.Render(key) + styleDim.Render(" "+desc)
}
