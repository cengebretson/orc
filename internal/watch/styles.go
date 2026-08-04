package watch

import (
	"github.com/cengebretson/orc/internal/contextpressure"
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle         lipgloss.Style
	mutedStyle         lipgloss.Style
	sectionStyle       lipgloss.Style
	activeStyle        lipgloss.Style
	blockedStyle       lipgloss.Style
	inputStyle         lipgloss.Style
	reviewStyle        lipgloss.Style
	doneStyle          lipgloss.Style
	pendingStyle       lipgloss.Style
	selectedStyle      lipgloss.Style
	accentStyle        lipgloss.Style
	watchHeaderStyle   lipgloss.Style
	selectedRowStyle   lipgloss.Style
	transitionRowStyle lipgloss.Style
	doneFlashStyle     lipgloss.Style
	selectedCardStyle  lipgloss.Style
	contextGreenStyle  lipgloss.Style
	contextYellowStyle lipgloss.Style
	contextRedStyle    lipgloss.Style
	contextStyles      terminalui.LevelStyles
)

func init() {
	SetTheme(terminalui.DefaultTheme())
}

// SetTheme applies the shared Orc palette to the Live presentation.
func SetTheme(theme terminalui.Theme) {
	p := theme.Palette
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.Green))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0))
	sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Subtext1)).Bold(true)
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Blue))
	blockedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))
	inputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Peach))
	reviewStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve))
	doneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve)).Bold(true)
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve))
	watchHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Base)).Background(lipgloss.Color(p.Mauve)).Bold(true)
	selectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Text)).Background(lipgloss.Color(p.Surface0)).Bold(true)
	transitionRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Base)).Background(lipgloss.Color(p.Sky)).Bold(true)
	doneFlashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Base)).Background(lipgloss.Color(p.Green)).Bold(true)
	selectedCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(p.Mauve)).Padding(0, 1)
	contextGreenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green))
	contextYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	contextRedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))
	contextStyles = terminalui.LevelStyles{
		Unknown: mutedStyle, Green: contextGreenStyle,
		Yellow: contextYellowStyle, Red: contextRedStyle,
	}
}

func stateStyle(label string) lipgloss.Style {
	switch label {
	case "blocked", "error", "stopped":
		return blockedStyle
	case "input":
		return inputStyle
	case "review":
		return reviewStyle
	case "done":
		return doneStyle
	case "pending", "ready":
		return pendingStyle
	case "active":
		return activeStyle
	default:
		return mutedStyle
	}
}

func renderContextPressure(pressure contextpressure.Pressure) string {
	return contextStyles.For(pressure).Render(pressure.Label())
}
