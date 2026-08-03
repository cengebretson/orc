package watch

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/state"
	"github.com/charmbracelet/lipgloss"
)

func petPixelFramesFor(state petState) []pixelGrid {
	if frames := petPixelFrames[state]; len(frames) > 0 {
		return frames
	}
	return petPixelFrames[petError]
}

// renderPixelSprite draws a pixel grid two rows per terminal line using
// upper/lower half-block characters, so each cell carries its own
// foreground (top pixel) and background (bottom pixel) true color.
func renderPixelSprite(grid pixelGrid, hue petHue) string {
	rows := len(grid)
	lines := make([]string, 0, (rows+1)/2)
	for y := 0; y < rows; y += 2 {
		var b strings.Builder
		for x := range grid[y] {
			top := grid[y][x]
			bot := 0
			if y+1 < rows {
				bot = grid[y+1][x]
			}
			switch {
			case top == 0 && bot == 0:
				b.WriteString(" ")
			case top == 0:
				b.WriteString(lipgloss.NewStyle().Foreground(rgbColor(petPixelColor(bot, hue))).Render("▄"))
			case bot == 0:
				b.WriteString(lipgloss.NewStyle().Foreground(rgbColor(petPixelColor(top, hue))).Render("▀"))
			default:
				b.WriteString(lipgloss.NewStyle().
					Foreground(rgbColor(petPixelColor(top, hue))).
					Background(rgbColor(petPixelColor(bot, hue))).
					Render("▀"))
			}
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func renderPetSprite(r row, frame int, width int) string {
	frames := petPixelFramesFor(petStateFor(r))
	phase := int(petHash(r) % uint32(len(frames)))
	grid := frames[(frame+phase)%len(frames)]
	sprite := renderPixelSprite(grid, petHueFor(r))
	return lipgloss.NewStyle().
		Width(max(11, width)).
		Align(lipgloss.Center).
		Render(sprite)
}

func renderPetContext(r row, width int) string {
	if !r.context.Observed {
		return mutedStyle.Render("ctx —")
	}
	if !r.context.Available {
		return mutedStyle.Render("ctx n/a")
	}
	slots := min(10, max(5, width-10))
	filled := min(slots, int(r.context.Percent)*slots/100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", slots-filled)
	style := contextGreenStyle
	switch r.context.Level {
	case contextpressure.LevelYellow:
		style = contextYellowStyle
	case contextpressure.LevelRed:
		style = contextRedStyle
	}
	return mutedStyle.Render("ctx ") + style.Render(bar+" "+r.context.Label())
}

func renderPetCard(r row, selected bool, width, frame int) string {
	contentWidth := max(16, width-4)
	// lipgloss Width includes the card's horizontal padding, so reserve those
	// cells before truncating styled metadata to keep narrow cards single-line.
	lineWidth := max(12, contentWidth-3)
	name := r.ticket
	if selected {
		name = "▶ " + name
	}
	room := r.room
	if room == "" {
		room = "workspace"
	}
	meta := strings.Trim(strings.Join([]string{r.stage, r.worker}, " · "), " ·")
	provider := strings.Trim(strings.Join([]string{r.branch, r.engine, r.model}, " · "), " ·")

	lines := []string{
		mutedStyle.Render(truncate("⌂ "+room, lineWidth)),
		renderPetSprite(r, frame, contentWidth),
		selectedStyle.Render(truncate(name, lineWidth)),
		stateStyleForPet(r).Render(truncate(petStateLabel(r), lineWidth)),
	}
	if meta != "" {
		lines = append(lines, mutedStyle.Render(truncate(meta, lineWidth)))
	}
	if provider != "" {
		lines = append(lines, mutedStyle.Render(truncate(provider, lineWidth)))
	}
	lines = append(lines, renderPetContext(r, contentWidth))

	border := lipgloss.HiddenBorder()
	if selected {
		border = lipgloss.RoundedBorder()
	}
	style := lipgloss.NewStyle().
		Width(contentWidth).
		Padding(0, 1).
		Border(border).
		BorderForeground(petAccent(r, selected))
	if selected {
		style = style.Bold(true)
	}
	return style.Render(strings.Join(lines, "\n"))
}

func stateStyleForPet(r row) lipgloss.Style {
	switch petStateFor(r) {
	case petError, petBlocked:
		return blockedStyle
	case petInput:
		return inputStyle
	case petDone:
		return doneStyle
	case petEgg:
		return pendingStyle
	case petWorking:
		return activeStyle
	default:
		return mutedStyle
	}
}

func (m Model) renderPets() string {
	width := max(24, m.width)
	height := m.height
	layout := normalizePetLayout(m.petLayout)
	if height <= 0 {
		height = 24
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("ORC PETS"))
	if m.ticket != "" {
		b.WriteString(" " + selectedStyle.Render(m.ticket))
	}
	if layout != PetLayoutResponsive {
		b.WriteString(mutedStyle.Render("  " + string(layout)))
	}
	if !m.lastLoad.IsZero() {
		b.WriteString(mutedStyle.Render("  " + m.lastLoad.Format("15:04:05")))
	}
	b.WriteString("\n")
	if m.searching || m.searchBox.Value() != "" {
		b.WriteString(m.renderFilter(width))
		b.WriteString("\n")
	}
	if m.loadErr != nil {
		b.WriteString("\n" + blockedStyle.Render("load error: ") + m.loadErr.Error())
		return b.String()
	}
	if m.loadWarning != nil {
		b.WriteString("\n" + blockedStyle.Render("parking warning: ") + m.loadWarning.Error())
	}
	if len(m.rows) == 0 {
		b.WriteString("\n" + mutedStyle.Render("No creatures are awake."))
		b.WriteString("\n\n" + mutedStyle.Render("v rail  l layout  / filter  r refresh  q quit"))
		return b.String()
	}

	const (
		minCardWidth = 28
		maxColumns   = 4
		gap          = 1
	)
	cols := min(maxColumns, max(1, (width+gap)/(minCardWidth+gap)))
	if layout == PetLayoutColumn {
		cols = 1
	}
	cardWidth := max(20, (width-gap*(cols-1))/cols)
	cardHeight := 13
	rowsPerPage := 1
	if width >= 56 || layout == PetLayoutColumn {
		rowsPerPage = max(1, (height-6)/cardHeight)
	}
	perPage := max(1, cols*rowsPerPage)
	page := m.cursor / perPage
	start := page * perPage
	end := min(len(m.rows), start+perPage)

	var gridRows []string
	for i := start; i < end; i += cols {
		var cards []string
		for j := i; j < min(end, i+cols); j++ {
			if len(cards) > 0 {
				cards = append(cards, strings.Repeat(" ", gap))
			}
			cards = append(cards, renderPetCard(m.rows[j], j == m.cursor, cardWidth, m.petFrame))
		}
		gridRows = append(gridRows, lipgloss.JoinHorizontal(lipgloss.Top, cards...))
	}
	b.WriteString("\n")
	b.WriteString(lipgloss.JoinVertical(lipgloss.Left, gridRows...))
	b.WriteString("\n")
	if m.message != "" {
		b.WriteString(mutedStyle.Render(truncate(m.message, width)))
		b.WriteString("\n")
	}
	pageCount := (len(m.rows) + perPage - 1) / perPage
	footer := "v rail  l layout  / filter  j/k move  enter preview  a attach  i focus  q quit"
	if pageCount > 1 {
		footer += fmt.Sprintf("  page %d/%d", page+1, pageCount)
	}
	b.WriteString(mutedStyle.Render(truncate(footer, width)))
	return strings.TrimRight(b.String(), "\n")
}

func featureRoom(s *state.State) (string, string) {
	if s == nil || len(s.Repos) == 0 {
		return "workspace", ""
	}
	names := make([]string, 0, len(s.Repos))
	for name := range s.Repos {
		names = append(names, name)
	}
	sort.Strings(names)
	name := names[0]
	repo := s.Repos[name]
	room := name
	if repo.Worktree != "" {
		base := filepath.Base(filepath.Clean(repo.Worktree))
		if base != "." && base != string(filepath.Separator) && base != name {
			room += "/" + base
		}
	}
	return room, repo.Branch
}
