// Package dashboard composes Orc's live watch and workspace explorer into one
// Bubble Tea application while preserving watch's standalone narrow layout.
package dashboard

import (
	"strings"

	"github.com/cengebretson/orc/internal/config"
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/cengebretson/orc/internal/watch"
	"github.com/cengebretson/orc/internal/workspaceui"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Section is a top-level dashboard destination.
type Section int

const (
	SectionLive Section = iota
	SectionWorkspace
)

// Options configures the shared dashboard shell.
type Options struct {
	Start    Section
	Adaptive bool
	Watch    watch.Options
}

// Model owns the shared shell and delegates section behavior to the existing
// Live and Workspace models. Both children stay loaded so switching is instant.
type Model struct {
	section   Section
	adaptive  bool
	help      bool
	width     int
	height    int
	live      watch.Model
	workspace workspaceui.Model
}

var (
	brandStyle    lipgloss.Style
	navStyle      lipgloss.Style
	selectedStyle lipgloss.Style
	cardStyle     lipgloss.Style
	keyStyle      lipgloss.Style
)

func init() {
	setTheme(terminalui.DefaultTheme())
}

func setTheme(theme terminalui.Theme) {
	p := theme.Palette
	brandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Text)).Bold(true)
	navStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve)).Bold(true)
	cardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(p.Mauve)).Padding(0, 1)
	keyStyle = selectedStyle
}

// New constructs a dashboard without starting the terminal program.
func New(root string, opts Options) (Model, error) {
	if cfg, err := config.Load(root); err == nil {
		if theme, themeErr := terminalui.LoadTheme(cfg.Settings.Theme); themeErr == nil {
			workspaceui.SetTheme(theme)
			watch.SetTheme(theme)
			setTheme(theme)
		}
	}
	live, err := watch.New(root, opts.Watch)
	if err != nil {
		return Model{}, err
	}
	workspace := workspaceui.NewEmbedded(root)
	live = live.SetActive(opts.Start == SectionLive)
	workspace = workspace.SetActive(opts.Start == SectionWorkspace)
	return Model{
		section:   opts.Start,
		adaptive:  opts.Adaptive,
		live:      live,
		workspace: workspace,
	}, nil
}

// Run opens the shared dashboard in the terminal alternate screen.
func Run(root string, opts Options) error {
	m, err := New(root, opts)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.live.Init(), m.workspace.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		var sectionCmd tea.Cmd
		if m.adaptive && !m.shellVisible() && m.section != SectionLive {
			sectionCmd = m.switchSection(SectionLive)
			m.help = false
		}
		updated, sizeCmd := m.updateBoth(m.childSizeMsg())
		return updated, tea.Batch(sectionCmd, sizeCmd)
	case tea.KeyMsg:
		if m.help {
			switch {
			case key.Matches(msg, keys.quit):
				return m, tea.Quit
			case key.Matches(msg, keys.help, keys.back):
				m.help = false
			}
			return m, nil
		}
		if m.shellVisible() && m.canSwitchSection() {
			switch {
			case key.Matches(msg, keys.help):
				m.help = true
				return m, nil
			case key.Matches(msg, keys.live):
				return m, m.switchSection(SectionLive)
			case key.Matches(msg, keys.workspace):
				return m, m.switchSection(SectionWorkspace)
			}
		}
		return m.updateActive(msg)
	case tea.MouseMsg:
		return m.updateActive(msg)
	default:
		return m.updateBoth(msg)
	}
}

func (m *Model) switchSection(section Section) tea.Cmd {
	if m.section == section {
		return nil
	}
	m.live = m.live.SetActive(false)
	m.workspace = m.workspace.SetActive(false)
	m.section = section
	if section == SectionWorkspace {
		m.workspace = m.workspace.SetActive(true)
		return m.workspace.Init()
	}
	m.live = m.live.SetActive(true)
	return m.live.Init()
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if !m.shellVisible() {
		return m.live.View()
	}
	if m.help {
		return m.renderHelp()
	}
	body := m.live.View()
	if m.section == SectionWorkspace {
		body = m.workspace.View()
	}
	return m.renderHeader() + "\n" + body
}

func (m Model) shellVisible() bool {
	return !m.adaptive || m.width >= 56
}

func (m Model) canSwitchSection() bool {
	if m.section == SectionWorkspace {
		return m.workspace.CanSwitchSection()
	}
	return m.live.CanSwitchSection()
}

func (m Model) childSizeMsg() tea.WindowSizeMsg {
	height := m.height
	if m.shellVisible() {
		height = max(1, height-1)
	}
	return tea.WindowSizeMsg{Width: m.width, Height: height}
}

func (m Model) updateActive(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.section == SectionWorkspace {
		updated, cmd := m.workspace.Update(msg)
		if next, ok := updated.(workspaceui.Model); ok {
			m.workspace = next
		}
		return m, cmd
	}
	updated, cmd := m.live.Update(msg)
	if next, ok := updated.(watch.Model); ok {
		m.live = next
	}
	return m, cmd
}

func (m Model) updateBoth(msg tea.Msg) (tea.Model, tea.Cmd) {
	live, liveCmd := m.live.Update(msg)
	workspace, workspaceCmd := m.workspace.Update(msg)
	if next, ok := live.(watch.Model); ok {
		m.live = next
	}
	if next, ok := workspace.(workspaceui.Model); ok {
		m.workspace = next
	}
	return m, tea.Batch(liveCmd, workspaceCmd)
}

func (m Model) renderHeader() string {
	live := navStyle.Render("LIVE")
	workspace := navStyle.Render("WORKSPACE")
	if m.section == SectionLive {
		live = selectedStyle.Render("[ LIVE ]")
	} else {
		workspace = selectedStyle.Render("[ WORKSPACE ]")
	}
	return terminalui.Fit(brandStyle.Render(" ORC")+"  "+live+"  "+workspace, m.width)
}

func (m Model) renderHelp() string {
	width := max(16, m.width)
	inner := min(52, max(12, width-6))
	sections := []terminalui.HelpSection{dashboardHelpSection()}
	sections = append(sections, watch.HelpSections()...)
	sections = append(sections, workspaceui.HelpSections()...)
	var lines []string
	for sectionIndex, section := range sections {
		if sectionIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, keyStyle.Render(section.Title))
		for _, entry := range section.Entries {
			line := terminalui.PadRight(entry.Keys, 20) + entry.Description
			lines = append(lines, terminalui.Truncate(line, inner))
		}
	}
	content := strings.Join(lines, "\n")
	return brandStyle.Render(" ORC DASHBOARD · HELP") + "\n\n" + cardStyle.Width(inner).Render(content)
}
