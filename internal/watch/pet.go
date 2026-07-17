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

// petHue is a deterministic identity color: a body tone plus its shadow,
// picked per-agent so the same ticket looks the same little orc across
// refreshes regardless of mood.
type petHue struct {
	main   [3]int
	shadow [3]int
}

var petHues = []petHue{
	{main: [3]int{95, 150, 80}, shadow: [3]int{65, 115, 58}},    // moss
	{main: [3]int{80, 165, 140}, shadow: [3]int{55, 130, 108}},  // jade
	{main: [3]int{110, 158, 70}, shadow: [3]int{78, 122, 48}},   // goblin green
	{main: [3]int{88, 172, 120}, shadow: [3]int{60, 138, 90}},   // fern
	{main: [3]int{100, 190, 150}, shadow: [3]int{70, 155, 118}}, // lime-teal
	{main: [3]int{70, 150, 165}, shadow: [3]int{48, 118, 132}},  // frost teal
}

func petHueFor(r row) petHue {
	return petHues[int(petHash(r))%len(petHues)]
}

func rgbColor(c [3]int) lipgloss.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2]))
}

func petColor(r row) lipgloss.Color {
	return rgbColor(petHueFor(r).main)
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

// pixelGrid is a rows x cols grid of palette indices. 0 is transparent.
// Index 1 resolves to the agent's body hue, 2 to its shadow, 22 to the body
// hue again (egg speckles hint at the color the creature will hatch into);
// every other index is a fixed feature color shared across all agents.
type pixelGrid [][]int

var petFeatureColors = map[int][3]int{
	4:  {230, 230, 225}, // eye white
	5:  {25, 25, 20},    // pupil / mouth dark
	6:  {140, 90, 70},   // ear inner
	7:  {245, 240, 215}, // tusk
	8:  {190, 90, 80},   // scar / cheek
	9:  {255, 210, 70},  // sparkle
	10: {230, 110, 60},  // alert orange
	11: {190, 210, 255}, // zzz pale blue
	12: {200, 200, 210}, // pause-bar grey
	13: {180, 60, 50},   // mohawk
	14: {220, 90, 90},   // bandage patch
	20: {255, 248, 225}, // egg shell
	21: {225, 205, 175}, // egg shell shadow
	30: {150, 165, 155}, // offline body (dims regardless of identity)
	31: {110, 125, 118}, // offline shadow
}

func petPixelColor(index int, hue petHue) [3]int {
	switch index {
	case 1, 22:
		return hue.main
	case 2:
		return hue.shadow
	default:
		return petFeatureColors[index]
	}
}

// Rugged little-orc sprites: flared ears, tusks, and a mohawk tuft, rendered
// two pixel rows per terminal line with half-block characters so each cell
// carries its own true-color foreground and background.
var petPixelFrames = map[petState][]pixelGrid{
	petEgg: {
		{
			{0, 0, 0, 20, 20, 20, 0, 0, 0, 0},
			{0, 0, 20, 20, 20, 20, 20, 0, 0, 0},
			{0, 20, 20, 20, 20, 20, 20, 20, 0, 0},
			{0, 20, 20, 22, 20, 20, 22, 20, 0, 0},
			{0, 20, 20, 20, 20, 20, 20, 20, 0, 0},
			{0, 20, 20, 20, 20, 20, 20, 20, 0, 0},
			{0, 20, 22, 20, 20, 20, 20, 22, 0, 0},
			{0, 20, 20, 21, 21, 21, 20, 20, 0, 0},
			{0, 0, 20, 21, 21, 20, 0, 0, 0, 0},
			{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
		{
			{0, 0, 0, 0, 20, 20, 20, 0, 0, 0},
			{0, 0, 0, 20, 20, 20, 20, 20, 0, 0},
			{0, 0, 20, 20, 20, 20, 20, 20, 20, 0},
			{0, 0, 20, 20, 22, 20, 20, 22, 20, 0},
			{0, 0, 20, 20, 20, 20, 20, 20, 20, 0},
			{0, 0, 20, 20, 20, 20, 20, 20, 20, 0},
			{0, 0, 20, 22, 20, 20, 20, 20, 22, 0},
			{0, 0, 20, 20, 21, 21, 21, 20, 20, 0},
			{0, 0, 0, 20, 21, 21, 20, 0, 0, 0},
			{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	},
	petWorking: {
		{
			{9, 0, 13, 13, 1, 1, 13, 13, 0, 9},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{6, 6, 1, 1, 1, 1, 1, 1, 6, 6},
			{6, 1, 1, 1, 1, 1, 1, 1, 1, 6},
			{0, 1, 4, 5, 1, 1, 5, 4, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{0, 8, 1, 1, 1, 1, 1, 1, 8, 0},
			{7, 1, 1, 5, 5, 5, 5, 1, 1, 7},
			{7, 0, 1, 1, 1, 1, 1, 1, 0, 7},
			{0, 0, 2, 0, 0, 0, 0, 2, 0, 0},
		},
		{
			{0, 9, 13, 0, 1, 1, 0, 13, 9, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{6, 6, 1, 1, 1, 1, 1, 1, 6, 6},
			{6, 1, 1, 1, 1, 1, 1, 1, 1, 6},
			{0, 1, 5, 5, 1, 1, 5, 5, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{0, 8, 1, 1, 1, 1, 1, 1, 8, 0},
			{7, 1, 1, 5, 5, 5, 5, 1, 1, 7},
			{7, 0, 1, 1, 1, 1, 1, 1, 0, 7},
			{0, 0, 2, 0, 0, 0, 0, 2, 0, 0},
		},
	},
	petIdle: {
		{
			{11, 0, 13, 13, 1, 1, 13, 13, 0, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{6, 6, 1, 1, 1, 1, 1, 1, 6, 6},
			{6, 1, 1, 1, 1, 1, 1, 1, 1, 6},
			{0, 1, 2, 2, 1, 1, 2, 2, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{7, 1, 1, 5, 1, 1, 5, 1, 1, 7},
			{7, 0, 1, 1, 1, 1, 1, 1, 0, 7},
			{0, 0, 2, 0, 0, 0, 0, 2, 0, 0},
		},
	},
	// Input grows two rows taller than the other moods: a "?" needs its own
	// dedicated space above the head to read clearly, and this is the one
	// state meant to be the most visually aggressive in the grid.
	petInput: {
		{
			{0, 0, 10, 10, 10, 0, 0, 0, 0, 0},
			{0, 0, 0, 0, 0, 10, 0, 0, 0, 0},
			{0, 0, 0, 0, 10, 10, 0, 0, 0, 0},
			{0, 0, 0, 0, 0, 10, 0, 0, 0, 0},
			{6, 6, 1, 1, 1, 1, 1, 1, 6, 6},
			{6, 1, 1, 1, 1, 1, 1, 1, 1, 6},
			{0, 1, 4, 5, 1, 1, 5, 4, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{0, 8, 1, 1, 1, 1, 1, 1, 8, 0},
			{7, 1, 1, 5, 10, 10, 5, 1, 1, 7},
			{7, 0, 1, 1, 1, 1, 1, 1, 0, 7},
			{0, 0, 2, 0, 0, 0, 0, 2, 0, 0},
		},
		{
			{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			{0, 0, 10, 10, 10, 0, 0, 0, 0, 0},
			{0, 0, 0, 0, 0, 10, 0, 0, 0, 0},
			{0, 0, 0, 0, 10, 10, 0, 0, 0, 0},
			{6, 6, 1, 1, 1, 1, 1, 1, 6, 6},
			{6, 1, 1, 1, 1, 1, 1, 1, 1, 6},
			{0, 1, 4, 5, 1, 1, 5, 4, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{0, 8, 1, 1, 1, 1, 1, 1, 8, 0},
			{7, 1, 1, 10, 5, 5, 10, 1, 1, 7},
			{7, 0, 1, 1, 1, 1, 1, 1, 0, 7},
			{0, 0, 2, 0, 0, 0, 0, 2, 0, 0},
		},
	},
	petBlocked: {
		{
			{0, 12, 13, 13, 1, 1, 13, 13, 12, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{6, 6, 1, 1, 1, 1, 1, 1, 6, 6},
			{6, 1, 1, 1, 1, 1, 1, 1, 1, 6},
			{0, 1, 12, 12, 1, 1, 12, 12, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{7, 1, 1, 5, 1, 1, 5, 1, 1, 7},
			{7, 0, 1, 1, 1, 1, 1, 1, 0, 7},
			{0, 0, 2, 0, 0, 0, 0, 2, 0, 0},
		},
	},
	// Done is two columns wider than the other moods to give raised arms
	// somewhere to go: down at the sides in frame 1, thrust overhead in
	// frame 2, so celebrating reads as a fist-pump rather than a static grin.
	petDone: {
		{
			{0, 9, 13, 13, 0, 1, 1, 0, 13, 13, 9, 0},
			{0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0},
			{0, 6, 6, 1, 1, 1, 1, 1, 1, 6, 6, 0},
			{0, 6, 1, 1, 1, 1, 1, 1, 1, 1, 6, 0},
			{1, 0, 1, 5, 4, 1, 1, 4, 5, 1, 0, 1},
			{1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1},
			{1, 0, 8, 1, 1, 1, 1, 1, 1, 8, 0, 1},
			{5, 7, 5, 5, 5, 5, 5, 5, 5, 5, 7, 5},
			{0, 7, 0, 1, 1, 1, 1, 1, 1, 0, 7, 0},
			{0, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 0},
		},
		{
			{5, 9, 13, 13, 0, 1, 1, 0, 13, 13, 9, 5},
			{1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1},
			{1, 6, 6, 1, 1, 1, 1, 1, 1, 6, 6, 1},
			{0, 6, 1, 1, 1, 1, 1, 1, 1, 1, 6, 0},
			{0, 0, 1, 5, 4, 1, 1, 4, 5, 1, 0, 0},
			{0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0},
			{0, 0, 8, 1, 1, 1, 1, 1, 1, 8, 0, 0},
			{0, 7, 5, 5, 5, 5, 5, 5, 5, 5, 7, 0},
			{0, 7, 0, 1, 1, 1, 1, 1, 1, 0, 7, 0},
			{0, 0, 0, 2, 0, 0, 0, 0, 2, 0, 0, 0},
		},
	},
	petError: {
		{
			{0, 0, 14, 14, 1, 1, 0, 0, 0, 0},
			{0, 1, 14, 14, 1, 1, 1, 1, 0, 0},
			{6, 6, 1, 1, 1, 1, 1, 1, 6, 6},
			{6, 1, 1, 1, 1, 1, 1, 1, 1, 6},
			{0, 1, 5, 5, 1, 1, 5, 5, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{0, 1, 1, 1, 1, 1, 1, 1, 1, 0},
			{7, 1, 1, 5, 8, 8, 5, 1, 1, 7},
			{7, 0, 1, 1, 1, 1, 1, 1, 0, 7},
			{0, 0, 2, 0, 0, 0, 0, 2, 0, 0},
		},
	},
	petOffline: {
		{
			{0, 0, 0, 30, 30, 30, 30, 0, 0, 0},
			{0, 0, 30, 30, 30, 30, 30, 30, 0, 0},
			{0, 30, 30, 30, 30, 30, 30, 30, 30, 0},
			{0, 30, 30, 30, 30, 30, 30, 30, 30, 0},
			{0, 30, 31, 31, 30, 30, 31, 31, 30, 0},
			{0, 30, 30, 30, 30, 30, 30, 30, 30, 0},
			{0, 30, 30, 30, 30, 30, 30, 30, 30, 0},
			{0, 30, 30, 31, 30, 30, 31, 30, 30, 0},
			{0, 0, 30, 30, 30, 30, 30, 30, 0, 0},
			{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	},
}

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
		mutedStyle.Render(truncatePet("⌂ "+room, lineWidth)),
		renderPetSprite(r, frame, contentWidth),
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
