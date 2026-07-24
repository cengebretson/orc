// Package dashboard composes Orc's live watch and workspace explorer into one
// Bubble Tea application while preserving watch's standalone narrow layout.
package dashboard

import (
	"fmt"
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
	SectionFeatures
	SectionWorkflows
	SectionWorkers
	SectionRepositories
	SectionHealth
	SectionOrc
)

var sections = []Section{
	SectionFeatures,
	SectionWorkflows,
	SectionWorkers,
	SectionRepositories,
	SectionHealth,
}

// Options configures the shared dashboard shell.
type Options struct {
	Start     Section
	Adaptive  bool
	Version   string
	BuildDate string
	Revision  string
	Watch     watch.Options
}

// Model owns the shared shell, delegates dashboard tabs to Workspace, and keeps
// Live available as the adaptive standalone watch view.
type Model struct {
	section          Section
	wideSection      Section
	adaptive         bool
	watchOnly        bool
	help             bool
	width            int
	height           int
	quote            string
	quotes           []string
	root             string
	version          string
	buildDate        string
	revision         string
	orcAnimationStep int
	live             watch.Model
	workspace        workspaceui.Model
}

var (
	brandStyle    lipgloss.Style
	navStyle      lipgloss.Style
	selectedStyle lipgloss.Style
	warningStyle  lipgloss.Style
	logoStyle     lipgloss.Style
	quoteStyle    lipgloss.Style
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
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow)).Bold(true)
	logoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Surface1))
	quoteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0)).Italic(true)
	cardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(p.Mauve)).Padding(0, 1)
	keyStyle = selectedStyle
}

// New constructs a dashboard without starting the terminal program.
func New(root string, opts Options) (Model, error) {
	quotes := []string{defaultLegacyQuote}
	version, buildDate, revision := resolveBuildMetadata(opts.Version, opts.BuildDate, opts.Revision)
	if cfg, err := config.Load(root); err == nil {
		if theme, themeErr := terminalui.LoadTheme(cfg.Settings.Theme); themeErr == nil {
			workspaceui.SetTheme(theme)
			watch.SetTheme(theme)
			setTheme(theme)
		}
		if len(cfg.Settings.Quotes) > 0 {
			quotes = append([]string(nil), cfg.Settings.Quotes...)
		}
	}
	live, err := watch.New(root, opts.Watch)
	if err != nil {
		return Model{}, err
	}
	workspace := workspaceui.NewEmbedded(root).SetDestination(workspaceDestination(opts.Start))
	live = live.SetActive(opts.Start == SectionLive)
	workspace = workspace.SetActive(opts.Start != SectionLive && opts.Start != SectionOrc)
	return Model{
		section:     opts.Start,
		wideSection: initialWideSection(opts.Start),
		adaptive:    opts.Adaptive,
		watchOnly:   opts.Adaptive && opts.Start == SectionLive,
		quote:       chooseLegacyQuote(quotes),
		quotes:      quotes,
		root:        root,
		version:     version,
		buildDate:   buildDate,
		revision:    revision,
		live:        live,
		workspace:   workspace,
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
		if m.adaptive && !m.watchOnly && m.width < 56 && m.section != SectionLive {
			m.wideSection = m.section
			sectionCmd = m.switchSection(SectionLive)
			m.help = false
		} else if m.adaptive && !m.watchOnly && m.width >= 56 && m.section == SectionLive {
			sectionCmd = m.switchSection(initialWideSection(m.wideSection))
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
			shortcutSection, hasShortcut := sectionForShortcut(msg.String())
			switch {
			case key.Matches(msg, keys.help):
				m.help = true
				return m, nil
			case key.Matches(msg, keys.previous):
				return m, m.switchSection(m.adjacentSection(-1))
			case key.Matches(msg, keys.next):
				return m, m.switchSection(m.adjacentSection(1))
			case hasShortcut:
				return m, m.switchSection(shortcutSection)
			}
		}
		return m.updateActive(msg)
	case orcTickMsg:
		if m.section != SectionOrc || m.orcAnimationStep <= 0 {
			return m, nil
		}
		m.orcAnimationStep--
		if m.orcAnimationStep > 0 {
			return m, orcTick()
		}
		return m, nil
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && m.canSwitchSection() {
			if section, ok := m.navSectionAt(msg.X, msg.Y); ok {
				return m, m.switchSection(section)
			}
		}
		return m.updateActive(msg)
	default:
		return m.updateBoth(msg)
	}
}

func (m *Model) switchSection(section Section) tea.Cmd {
	if m.section == section {
		return nil
	}
	workspaceWasActive := m.workspace.IsActive()
	m.section = section
	switch section {
	case SectionOrc:
		m.live = m.live.SetActive(false)
		m.workspace = m.workspace.SetActive(false)
		m.quote = chooseNextLegacyQuote(m.quotes, m.quote)
		m.orcAnimationStep = terminalui.RainbowSteps
		return orcTick()
	case SectionLive:
		m.orcAnimationStep = 0
		m.workspace = m.workspace.SetActive(false)
		m.live = m.live.SetActive(true)
		return m.live.Init()
	default:
		m.orcAnimationStep = 0
		m.wideSection = section
		m.workspace = m.workspace.SetDestination(workspaceDestination(section))
		m.live = m.live.SetActive(false)
		m.workspace = m.workspace.SetActive(true)
		if workspaceWasActive {
			return nil
		}
		return m.workspace.Init()
	}
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
	if m.section == SectionOrc {
		body = m.renderOrc()
	} else if m.section != SectionLive {
		body = m.workspace.View()
	}
	return m.renderHeader() + "\n" + body
}

func (m Model) shellVisible() bool {
	if m.watchOnly {
		return false
	}
	return !m.adaptive || m.width >= 56
}

func (m Model) canSwitchSection() bool {
	if m.section == SectionOrc {
		return true
	}
	if m.section != SectionLive {
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
	if m.section != SectionLive {
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
	items := make([]string, 0, len(sections))
	for _, section := range sections {
		items = append(items, renderNavigationItem(section, m.workspace.HealthIssueCount(), section == m.section))
	}
	brand := brandStyle.Render(" 👹 ORC")
	if m.section == SectionOrc {
		style := selectedStyle
		if m.orcAnimationStep > 0 {
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color(terminalui.RainbowColor(m.orcAnimationStep, 0))).
				Bold(true)
		}
		brand = style.Render("[ 👹 ORC ]")
	}
	full := brand + "  " + strings.Join(items, "  ")
	if lipgloss.Width(full) <= m.width {
		return full
	}
	if m.section == SectionOrc {
		return terminalui.Fit(brand, m.width)
	}
	compact := brand + "  " + navStyle.Render("‹") + " " +
		renderNavigationItem(m.section, m.workspace.HealthIssueCount(), true) + " " + navStyle.Render("›")
	return terminalui.Fit(compact, m.width)
}

// navRegion is a clickable column range in the header row, mapping a mouse
// X position to the tab it landed on.
type navRegion struct {
	section    Section
	start, end int // [start, end)
}

// navRegions mirrors renderHeader's layout to compute each tab's clickable
// column range. It returns nil when the header isn't shown, or when it has
// collapsed to its compact ‹ selected › form for a narrow terminal — that
// form has no per-tab targets, so clicks there fall through to the active
// view unchanged.
func (m Model) navRegions() []navRegion {
	if !m.shellVisible() || m.help {
		return nil
	}
	brandText := " 👹 ORC"
	if m.section == SectionOrc {
		brandText = "[ 👹 ORC ]"
	}
	items := make([]string, len(sections))
	for i, section := range sections {
		items[i] = renderNavigationItem(section, m.workspace.HealthIssueCount(), section == m.section)
	}
	full := brandText + "  " + strings.Join(items, "  ")
	if lipgloss.Width(full) > m.width {
		return nil
	}
	regions := make([]navRegion, len(sections))
	col := lipgloss.Width(brandText) + 2
	for i, section := range sections {
		w := lipgloss.Width(items[i])
		regions[i] = navRegion{section: section, start: col, end: col + w}
		col += w + 2
	}
	return regions
}

// navSectionAt reports which tab, if any, occupies column x on the header
// row (row 0).
func (m Model) navSectionAt(x, y int) (Section, bool) {
	if y != 0 {
		return SectionLive, false
	}
	for _, r := range m.navRegions() {
		if x >= r.start && x < r.end {
			return r.section, true
		}
	}
	return SectionLive, false
}

func initialWideSection(section Section) Section {
	if section == SectionLive {
		return SectionFeatures
	}
	return section
}

func navigationLabel(section Section, healthIssues int) string {
	label := sectionLabel(section)
	if section == SectionHealth && healthIssues > 0 {
		return fmt.Sprintf("%s ⚠ %d", label, healthIssues)
	}
	return label
}

func renderNavigationItem(section Section, healthIssues int, selected bool) string {
	style := navStyle
	if selected {
		style = selectedStyle
	}
	label := style.Render(sectionLabel(section))
	if section == SectionHealth && healthIssues > 0 {
		label += " " + warningStyle.Render(fmt.Sprintf("⚠ %d", healthIssues))
	}
	if selected {
		return style.Render("[ ") + label + style.Render(" ]")
	}
	return label
}

func (m Model) adjacentSection(delta int) Section {
	for index, section := range sections {
		if section == m.section {
			return sections[(index+delta+len(sections))%len(sections)]
		}
	}
	return SectionFeatures
}

func sectionForShortcut(shortcut string) (Section, bool) {
	if shortcut == "0" {
		return SectionOrc, true
	}
	if len(shortcut) != 1 || shortcut[0] < '1' || shortcut[0] > '5' {
		return SectionLive, false
	}
	return sections[int(shortcut[0]-'1')], true
}

func sectionLabel(section Section) string {
	switch section {
	case SectionLive:
		return "LIVE"
	case SectionFeatures:
		return "LIVE"
	case SectionWorkflows:
		return "WORKFLOWS"
	case SectionWorkers:
		return "WORKERS"
	case SectionRepositories:
		return "REPOSITORIES"
	case SectionHealth:
		return "HEALTH"
	default:
		return ""
	}
}

func workspaceDestination(section Section) workspaceui.Destination {
	switch section {
	case SectionWorkflows:
		return workspaceui.DestinationWorkflows
	case SectionWorkers:
		return workspaceui.DestinationWorkers
	case SectionRepositories:
		return workspaceui.DestinationRepositories
	case SectionHealth:
		return workspaceui.DestinationHealth
	default:
		return workspaceui.DestinationFeatures
	}
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
