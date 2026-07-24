package dashboard

import (
	"math/rand"
	"runtime/debug"
	"strings"
	"time"

	terminalui "github.com/cengebretson/orc/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultLegacyQuote = "Mmm... human burgers!"

type orcTickMsg struct{}

func orcTick() tea.Cmd {
	return tea.Tick(terminalui.RainbowInterval, func(time.Time) tea.Msg { return orcTickMsg{} })
}

func chooseLegacyQuote(quotes []string) string {
	if len(quotes) == 0 {
		return defaultLegacyQuote
	}
	return quotes[rand.Intn(len(quotes))]
}

func chooseNextLegacyQuote(quotes []string, current string) string {
	if len(quotes) < 2 {
		return chooseLegacyQuote(quotes)
	}
	next := quotes[rand.Intn(len(quotes)-1)]
	if next == current {
		next = quotes[len(quotes)-1]
	}
	return next
}

func resolveBuildMetadata(version, buildDate, revision string) (string, string, string) {
	if strings.TrimSpace(version) == "" {
		version = "dev"
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, buildDate, revision
	}
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if revision == "" {
				revision = setting.Value
			}
		case "vcs.time":
			if buildDate == "" {
				if parsed, err := time.Parse(time.RFC3339, setting.Value); err == nil {
					buildDate = parsed.UTC().Format(time.DateOnly)
				}
			}
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if len(revision) > 8 {
		revision = revision[:8]
	}
	if modified && revision != "" {
		revision += "+dirty"
	}
	return version, buildDate, revision
}

func (m Model) renderOrc() string {
	width := max(1, m.width)
	height := max(1, m.height-1)
	innerWidth := max(1, width-2)

	logoLines := strings.Split(terminalui.Logo(), "\n")
	quote := terminalui.Wrap("“"+m.quote+"”", min(44, innerWidth))
	quoteLines := strings.Split(quote, "\n")
	metadata := m.orcMetadata(innerWidth)
	metadataLines := strings.Split(metadata, "\n")

	logoBudget := max(1, height-len(quoteLines)-len(metadataLines)-3)
	if len(logoLines) > logoBudget {
		start := (len(logoLines) - logoBudget) / 2
		logoLines = logoLines[start : start+logoBudget]
	}
	activeLogoStyle := logoStyle
	if m.orcAnimationStep > 0 {
		activeLogoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(terminalui.RainbowColor(m.orcAnimationStep, 6)))
	}
	logo := activeLogoStyle.Render(strings.Join(logoLines, "\n"))
	contentWidth := min(innerWidth, max(lipgloss.Width(logo), lipgloss.Width(quote)))
	contentWidth = max(contentWidth, lipgloss.Width(metadata))
	logo = lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, logo)
	quote = lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, quoteStyle.Render(quote))
	metadata = lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, navStyle.Render(metadata))
	content := logo
	if quote != "" && lipgloss.Height(logo) < height {
		content += "\n" + quote
	}
	if metadata != "" && lipgloss.Height(content)+2 <= height {
		content += "\n\n" + metadata
	}

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) orcMetadata(width int) string {
	build := "orc " + strings.TrimSpace(m.version)
	if m.buildDate != "" {
		build += "  ·  built " + m.buildDate
	} else {
		build += "  ·  development build"
	}
	if m.revision != "" {
		build += "  ·  " + m.revision
	}
	lines := []string{terminalui.Truncate(build, width)}
	if workspace := strings.TrimSpace(m.root); workspace != "" {
		lines = append(lines, terminalui.Truncate("workspace  "+workspace, width))
	}
	return strings.Join(lines, "\n")
}
