package workspaceui

import (
	"embed"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/ui"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/lipgloss"
)

//go:embed assets/portraits/*.txt
var portraitsFS embed.FS

//go:embed assets/portraits/png
var portraitImagesFS embed.FS

// bardClass maps a worker to a Bard's Tale character class based on its id/name.
func bardClass(w *workers.Worker) string {
	combined := strings.ToLower(w.ID + " " + w.Name)
	switch {
	case containsAny(combined, "review", "ninja", "rogue", "shadow", "spy", "audit", "sec", "pentest"):
		return "ROGUE"
	case containsAny(combined, "qa", "test", "quality", "ranger", "hunter", "scout"):
		return "RANGER"
	case containsAny(combined, "doc", "write", "spec", "bard", "scribe", "analyst", "content"):
		return "BARD"
	case containsAny(combined, "dev", "implement", "build", "engineer", "warrior", "code", "fix"):
		return "WARRIOR"
	case containsAny(combined, "refactor", "clean", "lint", "tidy", "monk", "meditat"):
		return "MONK"
	case containsAny(combined, "arch", "design", "wizard", "ml", "ai", "research", "data", "model"):
		return "WIZARD"
	case containsAny(combined, "ops", "devops", "sre", "infra", "heal", "support", "priest"):
		return "PRIEST"
	case containsAny(combined, "perf", "load", "stress", "chaos", "brute", "barbarian"):
		return "BARBARIAN"
	default:
		return "ADVENTURER"
	}
}

func containsAny(s string, terms ...string) bool {
	for _, t := range terms {
		if strings.Contains(s, t) {
			return true
		}
	}
	return false
}

// workerStats generates deterministic D&D-style stats from the worker ID.
// Returns [STR, DEX, INT, WIS, VIT] each in the range 3–18.
func workerStats(id string) [5]int {
	h := fnv.New64a()
	h.Write([]byte(id))
	v := h.Sum64()
	var out [5]int
	for i := range out {
		out[i] = int((v>>(uint(i)*11))&0xF) + 3
	}
	return out
}

// portraits holds retro ASCII art for each class, loaded from assets/portraits/.
var portraits = func() map[string][]string {
	classes := []string{"WARRIOR", "RANGER", "BARD", "ROGUE", "MONK", "WIZARD", "PRIEST", "BARBARIAN", "ADVENTURER"}
	m := make(map[string][]string, len(classes))
	for _, class := range classes {
		data, err := portraitsFS.ReadFile("assets/portraits/" + strings.ToLower(class) + ".txt")
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		m[class] = lines
	}
	return m
}()

// portraitImages holds PNG data for each class, loaded from assets/portraits/png/.
var portraitImages = func() map[string][]byte {
	classes := []string{"WARRIOR", "RANGER", "BARD", "ROGUE", "MONK", "WIZARD", "PRIEST", "BARBARIAN", "ADVENTURER"}
	m := make(map[string][]byte, len(classes))
	for _, class := range classes {
		data, err := portraitImagesFS.ReadFile("assets/portraits/png/" + strings.ToLower(class) + ".png")
		if err != nil {
			continue
		}
		m[class] = data
	}
	return m
}()

// engineWeapon maps an engine name to a Bard's Tale-style weapon name.
func engineWeapon(engine string) string {
	switch strings.ToLower(engine) {
	case "claude":
		return "Arcane Tome"
	case "codex":
		return "Codex Blade"
	case "cursor":
		return "Cursor Staff"
	case "gemini":
		return "Gemini Orb"
	default:
		return "Unknown Relic"
	}
}

// modelArmor maps a model name to a fantasy armor type.
// Heavier models = heavier armor.
func modelArmor(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return "Plate Armor"
	case strings.Contains(m, "sonnet"):
		return "Chain Mail"
	case strings.Contains(m, "haiku"):
		return "Leather Armor"
	case strings.Contains(m, "gpt-4"):
		return "Battle Plate"
	case strings.Contains(m, "gpt-3"):
		return "Ring Mail"
	case strings.Contains(m, "gemini-pro") || strings.Contains(m, "ultra"):
		return "Scale Mail"
	case strings.Contains(m, "gemini"):
		return "Studded Leather"
	case model == "":
		return ""
	default:
		return "Traveler's Cloak"
	}
}

// workerDescription extracts the first non-heading paragraph from a worker .md file.
func workerDescription(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	body := string(data)
	// strip frontmatter
	if strings.HasPrefix(strings.TrimSpace(body), "---") {
		content := strings.TrimSpace(body)[3:]
		if end := strings.Index(content, "\n---"); end != -1 {
			body = strings.TrimSpace(content[end+4:])
		}
	}
	var para []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
			if len(para) > 0 {
				break
			}
			continue
		}
		if line == "" {
			if len(para) > 0 {
				break
			}
			continue
		}
		para = append(para, line)
	}
	return strings.Join(para, " ")
}

// workerForPath resolves the *workers.Worker for a sectionItem path by matching
// the file path stored on each worker at load time. Falls back to a stub.
func workerForPath(path string, allWorkers []*workers.Worker) *workers.Worker {
	for _, w := range allWorkers {
		if w.FilePath == path {
			return w
		}
	}
	id := strings.TrimSuffix(filepath.Base(path), ".md")
	return &workers.Worker{ID: id, Name: id}
}

// renderCharacterSheet renders the Bard's Tale easter egg full-screen for a worker.
func renderCharacterSheet(m Model, w *workers.Worker) string {
	p := activeTheme.Palette
	class := bardClass(w)
	stats := workerStats(w.ID)
	statNames := [5]string{"STR", "DEX", "INT", "WIS", "VIT"}
	statIcons := [5]string{"", "", "", "", "♥"}

	yellow := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow)).Bold(true)
	mauve := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve)).Bold(true)
	green := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green))
	peach := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Peach))
	red := lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))

	displayName := w.Name
	if displayName == "" {
		displayName = w.ID
	}

	// ── quest tally for derived stats ────────────────────────────────
	var activeQ, pausedQ, doneQ int
	type questEntry struct{ f *featureRow }
	var quests []questEntry
	for _, f := range m.features {
		if f.s == nil || (f.workerName != displayName && f.workerName != w.ID) {
			continue
		}
		quests = append(quests, questEntry{f})
		switch f.s.Status {
		case "active":
			activeQ++
		case "paused":
			pausedQ++
		case "done", "archived":
			doneQ++
		}
	}

	// ── derived RPG stats ────────────────────────────────────────────
	xp := doneQ*500 + activeQ*100
	level := xp/300 + 1
	if level > 20 {
		level = 20
	}
	maxHP := stats[4] * 5
	currentHP := maxHP - pausedQ*8
	if currentHP < 1 {
		currentHP = 1
	}
	ac := max(0, 10-(stats[1]-10)/2)

	hpStyle := green
	switch {
	case currentHP < maxHP/2:
		hpStyle = red
	case currentHP < maxHP:
		hpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	}

	// ── panel widths ─────────────────────────────────────────────────
	// outer box: Padding(1,3)=6h + border=2 → 8 total overhead
	// panel:     Padding(0,2)=4h + border=2 → 6 total overhead
	const panelOverhead = 6
	availW := m.width - 8 // total outer width for a row of panels

	// portrait panel takes ~1/3; info panel gets the rest (no border, just padding)
	portraitPanelW := availW/3 - panelOverhead
	if portraitPanelW < 12 {
		portraitPanelW = 12
	}
	// info has no border: overhead is just padding (0,2) = 4
	infoPanelW := availW - (portraitPanelW + panelOverhead) - 4
	if infoPanelW < 30 {
		infoPanelW = 30
	}
	mkPlain := func(w int) lipgloss.Style {
		return lipgloss.NewStyle().Padding(0, 2).Width(w)
	}

	dot := styleDim.Render("  ·  ")
	rpgLine := yellow.Render(fmt.Sprintf("LVL %2d", level)) +
		dot + hpStyle.Render(fmt.Sprintf("HP %d/%d", currentHP, maxHP)) +
		dot + peach.Render(fmt.Sprintf("XP %d", xp)) +
		dot + styleSubtext.Render(fmt.Sprintf("AC %d", ac))

	desc := workerDescription(w.FilePath)
	const barW = 14

	// ── portrait panel (right) ───────────────────────────────────────
	// The image must fit the panel's content area (width minus Padding(0,2)),
	// or lipgloss hard-wraps each pixel row into an extra mostly-blank line.
	portraitCols := portraitPanelW - 4
	var artBlock string
	if imgData, ok := portraitImages[class]; ok {
		portraitH := pngAspectHeight(imgData, portraitCols) / 2
		if portraitH < 4 {
			portraitH = 4
		}
		artBlock = renderPortrait(class, imgData, portraitCols, portraitH)
	}
	if artBlock == "" {
		art := portraits[class]
		if art == nil {
			art = portraits["ADVENTURER"]
		}
		artBlock = styleDim.Render(strings.Join(art, "\n"))
	}
	portraitPanel := mkPlain(portraitPanelW).Render(artBlock)

	// ── info panel (left): stats + equipment ─────────────────────────
	infoLines := []string{
		yellow.Render(class),
		mauve.Render(displayName),
		rpgLine,
	}
	if desc != "" {
		infoLines = append(infoLines, styleDim.Render(ui.Truncate(desc, infoPanelW)))
	}
	infoLines = append(infoLines, "")
	for i, v := range stats {
		filled := v * barW / 18
		bar := green.Render(strings.Repeat("▰", filled)) + styleDim.Render(strings.Repeat("▱", barW-filled))
		infoLines = append(infoLines, fmt.Sprintf("%s  %s  %s  %2d",
			peach.Render(statIcons[i]),
			peach.Render(statNames[i]),
			bar, v))
	}

	weapon := engineWeapon(w.Engine)
	armor := modelArmor(w.Model)
	equipLines := []string{
		"",
		styleSection.Render("Equipment"),
		fmt.Sprintf("  %s  %s %s", peach.Render("⚔"), styleDetailValue.Render(weapon), styleDim.Render("("+w.Engine+")")),
	}
	if armor != "" {
		equipLines = append(equipLines, fmt.Sprintf("  %s  %s", peach.Render(""), styleDetailValue.Render(armor)))
	}
	if len(w.Args) > 0 {
		keys := make([]string, 0, len(w.Args))
		for k := range w.Args {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			equipLines = append(equipLines, fmt.Sprintf("  %s  %s %s",
				peach.Render("✦"),
				styleDetailValue.Render(fmt.Sprintf("%s of %s", strings.ToUpper(k[:1])+strings.ReplaceAll(strings.ReplaceAll(k[1:], "-", " "), "_", " "), w.Args[k])),
				styleDim.Render("("+k+"="+w.Args[k]+")"),
			))
		}
	}

	infoContent := strings.Join(infoLines, "\n") + "\n" + strings.Join(equipLines, "\n")
	infoPanel := mkPlain(infoPanelW).Render(infoContent)

	// ── quest log — borderless, below info ───────────────────────────
	questLines := []string{"", styleSection.Render("Quest Log")}
	if len(quests) == 0 {
		questLines = append(questLines, styleDim.Render("  (no quests)"))
	}
	for _, q := range quests {
		f := q.f
		st := statusStyle(f.s.Status)
		slug := strings.TrimPrefix(f.s.Slug, f.s.Ticket+"-")
		questLines = append(questLines,
			fmt.Sprintf("  %s  %s  %s",
				st.Render(statusIcon(f.s.Status)),
				styleDetailLabel.Render(f.s.Ticket),
				st.Render(f.s.Status),
			),
			fmt.Sprintf("     %s", styleDim.Render(ui.Truncate(slug, infoPanelW-6))),
		)
	}
	questPanel := mkPlain(infoPanelW).Render(strings.Join(questLines, "\n"))
	leftColumn := lipgloss.JoinVertical(lipgloss.Left, infoPanel, questPanel)

	// ── assemble ─────────────────────────────────────────────────────
	dismiss := styleDim.Render("[!] or [esc] to dismiss")
	// At narrow widths both panels hit their minimums and the horizontal join
	// overflows. Stack vertically instead (portrait panel drops below info).
	const minTwoColumn = 60
	var content string
	if availW >= minTwoColumn {
		content = lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, portraitPanel) + "\n" + dismiss
	} else {
		content = lipgloss.JoinVertical(lipgloss.Left, leftColumn, portraitPanel) + "\n" + dismiss
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, content)
}
