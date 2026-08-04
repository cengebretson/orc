package workspaceui

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/cengebretson/orc/internal/workers"
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
	contextStyles   terminalui.LevelStyles

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
	contextStyles = terminalui.LevelStyles{
		Unknown: styleDim, Green: styleHealthOK,
		Yellow: styleHealthWarn, Red: styleHealthErr,
	}

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

// workerAccentPalette returns the theme colors set aside for worker identity —
// deliberately excluding Mauve, Yellow, Green, Red, Sky, and Peach, which are
// already claimed by selection, status, and health semantics elsewhere in the
// dashboard, so a worker's color is never mistaken for a status color.
func workerAccentPalette() []string {
	p := activeTheme.Palette
	return []string{p.Blue, p.Sapphire, p.Teal, p.Maroon, p.Pink, p.Flamingo}
}

// workerColorAssignments maps a worker ID to its assigned palette index,
// populated by assignWorkerAccentColors whenever the worker list loads. A
// package-level cache (consistent with activeTheme above) rather than a
// Model field, since Bubble Tea's single-goroutine update loop already makes
// package-level UI state safe here.
var workerColorAssignments = map[string]int{}

// assignWorkerAccentColors gives each known worker a stable palette slot,
// assigned in sorted-ID order so the same workspace always produces the same
// assignment and, for typical worker counts, no two workers share a color —
// something a plain hash can't guarantee (fixed workers can still collide by
// chance). Unrecognized worker IDs (e.g. stale state.yaml data) fall back to
// a hash in workerAccentColor below.
func assignWorkerAccentColors(allWorkers []*workers.Worker) {
	ids := make([]string, 0, len(allWorkers))
	for _, w := range allWorkers {
		if w.ID != "" {
			ids = append(ids, w.ID)
		}
	}
	sort.Strings(ids)
	assignments := make(map[string]int, len(ids))
	for i, id := range ids {
		assignments[id] = i % len(workerAccentPalette())
	}
	workerColorAssignments = assignments
}

// workerAccentColor returns a stable color for the given worker ID: the
// assigned palette slot from assignWorkerAccentColors when known, or a hash
// of the ID as a fallback so an unrecognized worker still gets a consistent
// (if potentially collision-prone) color rather than falling back to gray.
func workerAccentColor(workerID string) string {
	colors := workerAccentPalette()
	if workerID == "" || len(colors) == 0 {
		return activeTheme.Palette.Overlay0
	}
	if idx, ok := workerColorAssignments[workerID]; ok {
		return colors[idx]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(workerID))
	return colors[int(h.Sum32())%len(colors)]
}

// workerAccentStyle is workerAccentColor wrapped as a lipgloss.Style for
// direct use in Render calls.
func workerAccentStyle(workerID string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(workerAccentColor(workerID)))
}

// repoColorAssignments mirrors workerColorAssignments for repository names.
var repoColorAssignments = map[string]int{}

// assignRepoAccentColors gives each configured repo a stable palette slot in
// sorted-name order, the same collision-avoidance approach used for workers.
// Repos and workers share the same palette but assign independently — they
// never appear in the same cell, so a coincidental color match between a
// repo and a worker isn't visually confusing.
func assignRepoAccentColors(repos []config.Repo) {
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		if r.Name != "" {
			names = append(names, r.Name)
		}
	}
	sort.Strings(names)
	assignments := make(map[string]int, len(names))
	for i, name := range names {
		assignments[name] = i % len(workerAccentPalette())
	}
	repoColorAssignments = assignments
}

// repoAccentColor returns a stable color for the given repo name: the
// assigned palette slot from assignRepoAccentColors when known, or a hash of
// the name as a fallback.
func repoAccentColor(name string) string {
	colors := workerAccentPalette()
	if name == "" || len(colors) == 0 {
		return activeTheme.Palette.Overlay0
	}
	if idx, ok := repoColorAssignments[name]; ok {
		return colors[idx]
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return colors[int(h.Sum32())%len(colors)]
}

// repoAccentStyle is repoAccentColor wrapped as a lipgloss.Style for direct
// use in Render calls.
func repoAccentStyle(name string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(repoAccentColor(name)))
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

// refreshCountdownWidth is the fill-bar width used by refreshCountdown.
const refreshCountdownWidth = 6

// refreshCountdown renders a small fill bar plus seconds remaining, showing
// progress toward the next full data refresh (age/interval) instead of a
// static "elapsed since last refresh" timestamp. Past the interval (a
// refresh is overdue) the bar shows full and the countdown clamps to 0s
// rather than going negative.
func refreshCountdown(age, interval time.Duration) string {
	if interval <= 0 {
		interval = defaultRefreshInterval
	}
	frac := float64(age) / float64(interval)
	frac = max(0, min(frac, 1))
	filled := int(frac * float64(refreshCountdownWidth))
	bar := strings.Repeat("▓", filled) + strings.Repeat("░", refreshCountdownWidth-filled)
	remaining := interval - age
	if remaining < 0 {
		remaining = 0
	}
	return fmt.Sprintf("%s %ds", bar, int(remaining.Round(time.Second).Seconds()))
}

func renderContextPressure(pressure contextpressure.Pressure) string {
	return contextStyles.For(pressure).Render(pressure.Label())
}

// renderContextSparkline renders the trend line for a feature's recent
// context-pressure samples, colored the same as its current level, followed
// by the current percentage. Returns "" when there's no history yet (a
// feature just picked up telemetry, or has none) so callers can fall back to
// the plain percentage.
func renderContextSparkline(history []uint64, pressure contextpressure.Pressure) string {
	if len(history) < 2 {
		return ""
	}
	style := contextStyles.For(pressure)
	return style.Render(terminalui.Sparkline(history, 0)) + " " + style.Render(pressure.Label())
}

// hexBlend linearly interpolates between two "#rrggbb" colors at t in [0,1]
// (0 returns a, 1 returns b).
func hexBlend(a, b string, t float64) string {
	parse := func(hex string) (r, g, bl int) {
		_, _ = fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &bl)
		return
	}
	ar, ag, ab := parse(a)
	br, bg, bb := parse(b)
	lerp := func(x, y int) int { return int(float64(x) + t*float64(y-x)) }
	return fmt.Sprintf("#%02x%02x%02x", lerp(ar, br), lerp(ag, bg), lerp(ab, bb))
}

// breathStyle returns the "NEEDS YOU" banner style for the given breathPhase,
// gently blending its foreground between the steady red and a dimmer shade
// over a slow dim-brighten-dim cycle. The blend is capped well short of full
// range so it reads as an ambient pulse rather than a flash.
func breathStyle(phase int) lipgloss.Style {
	const maxBlend = 0.4
	half := breathSteps / 2
	step := phase % breathSteps
	// Triangle wave: 0 -> half -> 0 across one full cycle.
	amount := step
	if step > half {
		amount = breathSteps - step
	}
	t := maxBlend * float64(amount) / float64(half)
	color := hexBlend(activeTheme.Palette.Red, activeTheme.Palette.Base, t)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true)
}

func helpItem(key, desc string) string {
	return styleHelpKey.Render(key) + styleDim.Render(" "+desc)
}
