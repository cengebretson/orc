package watch

import (
	"fmt"
	"hash/fnv"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/state"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const petAnimationInterval = 350 * time.Millisecond

// Mode selects the primary watch presentation. The rail remains the default;
// pet mode is an additive view over the same rows and interactions.
type Mode string

const (
	ModeRail Mode = "rail"
	ModePet  Mode = "pet"
)

// PetSize selects the creature sprite density without changing card metadata.
type PetSize string

const (
	PetSizeNormal PetSize = "normal"
	PetSizeMicro  PetSize = "micro"
)

// ParsePetSize validates a pet sprite size.
func ParsePetSize(value string) (PetSize, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(PetSizeNormal):
		return PetSizeNormal, nil
	case string(PetSizeMicro):
		return PetSizeMicro, nil
	default:
		return "", fmt.Errorf("unsupported pet size %q (use normal or micro)", value)
	}
}

// PetLayout controls whether pet cards respond to terminal width or stay in a
// single vertical column.
type PetLayout string

const (
	PetLayoutResponsive PetLayout = "responsive"
	PetLayoutColumn     PetLayout = "column"
)

// ParsePetLayout validates a pet card layout. Vertical is accepted as a
// readable alias for the canonical column value.
func ParsePetLayout(value string) (PetLayout, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(PetLayoutResponsive):
		return PetLayoutResponsive, nil
	case string(PetLayoutColumn), "vertical":
		return PetLayoutColumn, nil
	default:
		return "", fmt.Errorf("unsupported pet layout %q (use responsive or column)", value)
	}
}

func normalizePetSize(size PetSize) PetSize {
	if size == PetSizeMicro {
		return PetSizeMicro
	}
	return PetSizeNormal
}

func normalizePetLayout(layout PetLayout) PetLayout {
	if layout == PetLayoutColumn {
		return PetLayoutColumn
	}
	return PetLayoutResponsive
}

// ParseMode validates a watch presentation name.
func ParseMode(value string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(ModeRail):
		return ModeRail, nil
	case string(ModePet), "tamagotchi":
		return ModePet, nil
	default:
		return "", fmt.Errorf("unsupported watch view %q (use rail or pet)", value)
	}
}

type petTickMsg time.Time

func petTick() tea.Cmd {
	return tea.Tick(petAnimationInterval, func(t time.Time) tea.Msg {
		return petTickMsg(t)
	})
}

type petState string

const (
	petEgg     petState = "egg"
	petWorking petState = "working"
	petIdle    petState = "idle"
	petInput   petState = "input"
	petBlocked petState = "blocked"
	petOffline petState = "offline"
	petDone    petState = "done"
	petError   petState = "error"
)

func petStateFor(r row) petState {
	_, label := displayState(r)
	switch label {
	case "error":
		return petError
	case "blocked":
		return petBlocked
	case "input", "review":
		return petInput
	case "stopped":
		return petOffline
	case "pending", "ready":
		return petEgg
	case "done":
		return petDone
	case "active":
		switch strings.ToLower(r.liveState) {
		case "idle", "waiting", "sleeping":
			return petIdle
		default:
			return petWorking
		}
	default:
		return petIdle
	}
}

var petStateLabels = map[petState]string{
	petEgg:     "hatching",
	petWorking: "working",
	petIdle:    "sleeping",
	petInput:   "needs input",
	petBlocked: "blocked",
	petOffline: "offline",
	petDone:    "celebrating",
	petError:   "needs care",
}

func petStateLabel(r row) string {
	label := petStateLabels[petStateFor(r)]
	if r.context.Level == contextpressure.LevelRed {
		return label + " · exhausted"
	}
	if r.context.Level == contextpressure.LevelYellow {
		return label + " · tired"
	}
	return label
}

func petIdentity(r row) string {
	if r.providerID != "" {
		return r.providerID
	}
	return r.ticket + "|" + r.room
}

func petHash(r row) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(petIdentity(r)))
	return h.Sum32()
}

var petPalette = []lipgloss.Color{
	"#a6e3a1", // moss
	"#94e2d5", // jade
	"#9ece6a", // goblin green
	"#8bd49c", // fern
	"#c3e88d", // lime
	"#7dcfff", // frost teal
}

func petColor(r row) lipgloss.Color {
	return petPalette[int(petHash(r))%len(petPalette)]
}

func petAccent(r row, selected bool) lipgloss.Color {
	switch petStateFor(r) {
	case petError, petBlocked:
		return lipgloss.Color("#f38ba8")
	case petInput:
		return lipgloss.Color("#fab387")
	case petDone:
		return lipgloss.Color("#a6e3a1")
	case petOffline:
		return lipgloss.Color("#6c7086")
	}
	switch r.context.Level {
	case contextpressure.LevelRed:
		return lipgloss.Color("#f38ba8")
	case contextpressure.LevelYellow:
		return lipgloss.Color("#f9e2af")
	}
	if selected {
		return lipgloss.Color("#cba6f7")
	}
	return petColor(r)
}

var petFrameSets = map[petState][][]string{
	petEgg: {
		{"    ▄▄     ", "  ▄████▄   ", " ████████  ", "  ▀████▀   ", "    ▀▀     "},
		{"     ▄▄    ", "   ▄████▄  ", "  ███▀████ ", "   ▀████▀  ", "     ▀▀    "},
	},
	petWorking: {
		{"⚒ ◢▄   ▄◣ ✦", "▟█████████▙", "█  ◕   ◕  █", "█  ▴ ▿ ▴  █", " ▀█▀   ▀█▀ "},
		{"✦ ◢▄   ▄◣ ⚒", "▟█████████▙", "█  ◕   ◕  █", "█  ▴ ▿ ▴  █", "  ▀█▀ ▀█▀  "},
	},
	petIdle: {
		{"zZ ◢▄   ▄◣  ", "▟█████████▙", "█  ─   ─  █", "█  ▴ ─ ▴  █", " ▀█▀   ▀█▀ "},
	},
	petInput: {
		{"!! ◢▄   ▄◣ !", "▟█████████▙", "█  ◉   ◉  █", "█  ▴ △ ▴  █", " ▀█▀   ▀█▀ "},
		{"!  ◢▄   ▄◣!!", "▟█████████▙", "█  ◉   ◉  █", "█  ▴ △ ▴  █", "  ▀█▀ ▀█▀  "},
	},
	petBlocked: {
		{".. ◢▄   ▄◣ .", "▟█████████▙", "█  ×   ×  █", "█  ▴ ─ ▴  █", " ▀█▀   ▀█▀ "},
	},
	petOffline: {
		{"  ◢▄   ▄◣  ", " ▟███████▙ ", " █  · ·  █ ", " █ ▴ ─ ▴ █ ", "  ▀█▀ ▀█▀  "},
	},
	petDone: {
		{"✦ ◢▄ ⚔ ▄◣ ✦", "▟█████████▙", "█  ^   ^  █", "█  ▴ ▿ ▴  █", " ▀█▀   ▀█▀ "},
		{"  ◢▄ ✦ ▄◣  ", "▟█████████▙", "█  ^   ^  █", "█  ▴ ▿ ▴  █", "  ▀█▀ ▀█▀  "},
	},
	petError: {
		{"+  ◢▄   ▄◣ +", "▟█████████▙", "█  x   x  █", "█  ▴ ~ ▴  █", " ▀█▀   ▀█▀ "},
	},
}

var microPetFrameSets = map[petState][][]string{
	petEgg: {
		{"   ▄▄    ", " ▄████▄  ", "  ▀██▀   "},
		{"    ▄▄   ", "  ▄████▄ ", "   ▀██▀  "},
	},
	petWorking: {
		{"⚒◢▄   ▄◣✦", "█  ◕▿◕  █", " ▀█   █▀ "},
		{"✦◢▄   ▄◣⚒", "█  ◕▿◕  █", "  ▀█ █▀  "},
	},
	petIdle: {
		{"z◢▄   ▄◣Z", "█  ── ─  █", " ▀█   █▀ "},
	},
	petInput: {
		{"!◢▄   ▄◣!", "█  ◉△◉  █", " ▀█   █▀ "},
		{"!!◢▄ ▄◣!!", "█  ◉△◉  █", "  ▀█ █▀  "},
	},
	petBlocked: {
		{".◢▄   ▄◣.", "█  ×─×  █", " ▀█   █▀ "},
	},
	petOffline: {
		{" ◢▄   ▄◣ ", "█  ·─·  █", "  ▀█ █▀  "},
	},
	petDone: {
		{"✦◢▄ ⚔ ▄◣✦", "█  ^▿^  █", " ▀█   █▀ "},
		{" ◢▄ ✦ ▄◣ ", "█  ^▿^  █", "  ▀█ █▀  "},
	},
	petError: {
		{"+◢▄   ▄◣+", "█  x~x  █", " ▀█   █▀ "},
	},
}

func petFrames(state petState, size PetSize) [][]string {
	frameSets := petFrameSets
	if normalizePetSize(size) == PetSizeMicro {
		frameSets = microPetFrameSets
	}
	if frames := frameSets[state]; len(frames) > 0 {
		return frames
	}
	return frameSets[petError]
}

func renderPetSprite(r row, frame int, width int, size PetSize) string {
	frames := petFrames(petStateFor(r), size)
	phase := int(petHash(r) % uint32(len(frames)))
	sprite := strings.Join(frames[(frame+phase)%len(frames)], "\n")
	return lipgloss.NewStyle().
		Foreground(petColor(r)).
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

func renderPetCard(r row, selected bool, width, frame int, size PetSize) string {
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
		mutedStyle.Render(truncatePet("⌂ "+room, lineWidth)),
		renderPetSprite(r, frame, contentWidth, size),
		selectedStyle.Render(truncatePet(name, lineWidth)),
		stateStyleForPet(r).Render(truncatePet(petStateLabel(r), lineWidth)),
	}
	if meta != "" {
		lines = append(lines, mutedStyle.Render(truncatePet(meta, lineWidth)))
	}
	if provider != "" {
		lines = append(lines, mutedStyle.Render(truncatePet(provider, lineWidth)))
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
	size := normalizePetSize(m.petSize)
	layout := normalizePetLayout(m.petLayout)
	if height <= 0 {
		height = 24
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("ORC PETS"))
	if m.ticket != "" {
		b.WriteString(" " + selectedStyle.Render(m.ticket))
	}
	var modes []string
	if size != PetSizeNormal {
		modes = append(modes, string(size))
	}
	if layout != PetLayoutResponsive {
		modes = append(modes, string(layout))
	}
	if len(modes) > 0 {
		b.WriteString(mutedStyle.Render("  " + strings.Join(modes, " · ")))
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
	if len(m.rows) == 0 {
		b.WriteString("\n" + mutedStyle.Render("No creatures are awake."))
		b.WriteString("\n\n" + mutedStyle.Render("v rail  s size  l layout  / filter  r refresh  q quit"))
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
	if size == PetSizeMicro {
		cardHeight = 11
	}
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
			cards = append(cards, renderPetCard(m.rows[j], j == m.cursor, cardWidth, m.petFrame, size))
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
	footer := "v rail  s size  l layout  / filter  j/k move  enter preview  a attach  i focus  q quit"
	if pageCount > 1 {
		footer += fmt.Sprintf("  page %d/%d", page+1, pageCount)
	}
	b.WriteString(mutedStyle.Render(truncate(footer, width)))
	return strings.TrimRight(b.String(), "\n")
}

func truncatePet(value string, width int) string {
	value = strings.TrimSpace(value)
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	var out []rune
	for _, candidate := range value {
		if lipgloss.Width(string(append(out, candidate))+"…") > width {
			break
		}
		out = append(out, candidate)
	}
	return string(out) + "…"
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
