package workspaceui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/ui"
	"github.com/charmbracelet/lipgloss"
)

func repositoriesForDisplay(root string, repos []config.Repo) []config.Repo {
	display := append([]config.Repo(nil), repos...)
	for index := range display {
		display[index].Path = repositoryDisplayPath(root, display[index].Path)
	}
	return display
}

func repositoryDisplayPath(root, configured string) string {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return ""
	}
	if root == "" {
		return filepath.Clean(configured)
	}
	workspaceRoot, err := filepath.Abs(root)
	if err != nil {
		workspaceRoot = filepath.Clean(root)
	}
	resolved := configured
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(workspaceRoot, resolved)
	}
	resolved = filepath.Clean(resolved)
	if resolved == filepath.Clean(workspaceRoot) {
		return "workspace root"
	}
	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(home)
		if relative, relErr := filepath.Rel(home, resolved); relErr == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return filepath.Join("~", relative)
		}
	}
	return resolved
}

func renderRepoList(repos []config.Repo, maxW int) []string {
	if len(repos) == 0 {
		return []string{styleDim.Render("No repos configured. Edit orc.yaml to add repos.")}
	}
	var lines []string
	for _, r := range repos {
		name := repoAccentStyle(r.Name).Render(r.Name)
		sep := styleDivider.Render("  —  ")
		purpose := styleDim.Render(r.Purpose)
		var badges []string
		if r.WorktreeSetup != "" {
			badges = append(badges, styleHealthOK.Render("setup"))
		}
		if len(r.AgentHints) > 0 {
			badges = append(badges, styleStatusWaiting.Render("hints"))
		}
		badgeText := ""
		if len(badges) > 0 {
			badgeText = styleDim.Render("  ") + strings.Join(badges, styleDim.Render(" "))
		}
		line := name + badgeText + sep + purpose
		if lipgloss.Width(line) > maxW {
			purpose = styleDim.Render(ui.Truncate(r.Purpose, maxW-lipgloss.Width(name+badgeText+sep)))
			line = name + badgeText + sep + purpose
		}
		lines = append(lines, line)
		if r.Path != "" {
			lines = append(lines, styleDim.Render("  ↳ "+ui.Truncate(r.Path, max(1, maxW-4))))
		}
	}
	return lines
}

func renderRepoRoutingOverview(repos []config.Repo, routes []config.RepoRoute, maxW int) []string {
	lines := renderRepoList(repos, maxW)
	if len(repos) == 0 {
		return lines
	}
	lines = append(lines, styleSection.Render("OPTIONAL ROUTING"))
	if len(routes) == 0 {
		return append(lines, styleDim.Render(ui.Truncate("No metadata routes · use repo purpose and hints", maxW)))
	}
	for _, route := range routes {
		lines = append(lines, renderRepoRouteLine(route, maxW))
	}
	return lines
}

func renderRepoRouteLine(route config.RepoRoute, maxW int) string {
	signals := routeSignalText(route)
	targets := strings.Join(route.Repos, ", ")
	plain := signals + "  →  " + targets
	plain = ui.Truncate(plain, maxW)
	if before, after, ok := strings.Cut(plain, "  →  "); ok {
		return styleSubtext.Render(before) + styleDivider.Render("  →  ") + styleHealthOK.Render(after)
	}
	return styleSubtext.Render(plain)
}

func routeSignalText(route config.RepoRoute) string {
	parts := make([]string, 0, len(route.Labels)+len(route.Components))
	for _, label := range route.Labels {
		parts = append(parts, "label:"+label)
	}
	for _, component := range route.Components {
		parts = append(parts, "component:"+component)
	}
	if len(parts) == 0 {
		return "no signals"
	}
	return strings.Join(parts, " · ")
}

type repoWorkSummary struct {
	active  int
	paused  int
	tickets []string
}

func summarizeRepoWork(features []*featureRow) map[string]repoWorkSummary {
	type workAccumulator struct {
		active  int
		paused  int
		tickets map[string]struct{}
	}
	work := make(map[string]*workAccumulator)
	for _, feature := range features {
		if feature == nil || feature.s == nil || (feature.s.Status != "active" && feature.s.Status != "paused") {
			continue
		}
		for repoName := range feature.s.Repos {
			entry := work[repoName]
			if entry == nil {
				entry = &workAccumulator{tickets: make(map[string]struct{})}
				work[repoName] = entry
			}
			if feature.s.Status == "active" {
				entry.active++
			} else {
				entry.paused++
			}
			if feature.s.Ticket != "" {
				entry.tickets[feature.s.Ticket] = struct{}{}
			}
		}
	}

	summaries := make(map[string]repoWorkSummary, len(work))
	for repoName, entry := range work {
		tickets := make([]string, 0, len(entry.tickets))
		for ticket := range entry.tickets {
			tickets = append(tickets, ticket)
		}
		sort.Strings(tickets)
		summaries[repoName] = repoWorkSummary{active: entry.active, paused: entry.paused, tickets: tickets}
	}
	return summaries
}

func padInspectorLines(lines []string) []string {
	padded := make([]string, len(lines))
	for i, line := range lines {
		padded[i] = "  " + line
	}
	return padded
}

func renderRepositoryMapSummary(repos []config.Repo, routes []config.RepoRoute, width int) string {
	outerW := max(20, width)
	summary := []string{styleSubtext.Render(fmt.Sprintf("%d repositories  ·  %d optional metadata routes", len(repos), len(routes)))}
	for _, text := range []string{
		"ticket context  →  exact label/component match  →  selected repo set",
		"no match  →  purpose + agent hints     ambiguous  →  pause for input",
	} {
		for _, line := range strings.Split(ui.Wrap(text, max(8, outerW-6)), "\n") {
			summary = append(summary, styleDim.Render(line))
		}
	}
	return drawBoxLabeledWith(styleHeader.Render("Repository map"), padInspectorLines(summary), outerW, activeTheme.Palette.Mauve)
}

func renderRepositoryDetails(repos []config.Repo, routes []config.RepoRoute, features []*featureRow, width int) string {
	outerW := max(20, width)
	work := summarizeRepoWork(features)
	sections := make([]string, 0, len(repos)+len(routes))

	if len(repos) == 0 {
		sections = append(sections, drawBoxLabeled(styleSection.Render("Repositories"), padInspectorLines([]string{styleDim.Render("No repositories configured.")}), outerW))
	} else {
		for i := 0; i < len(repos); {
			if outerW >= 90 && i+1 < len(repos) {
				const gap = 2
				leftW := (outerW - gap) / 2
				rightW := outerW - gap - leftW
				left := renderRepositoryInspectorCard(repos[i], work[repos[i].Name], leftW)
				right := renderRepositoryInspectorCard(repos[i+1], work[repos[i+1].Name], rightW)
				left, right = equalizeBoxHeights(left, right)
				sections = append(sections, joinColumns(left, right, strings.Repeat(" ", gap)))
				i += 2
				continue
			}
			sections = append(sections, renderRepositoryInspectorCard(repos[i], work[repos[i].Name], outerW))
			i++
		}
	}

	if len(routes) == 0 {
		contentW := max(1, outerW-6)
		lines := []string{
			styleDim.Render("No deterministic metadata routes configured."),
			styleSubtext.Render(ui.Truncate("task scope  →  repo purpose + agent hints", contentW)),
			styleStatusWaiting.Render(ui.Truncate("ambiguous  →  pause for input", contentW)),
		}
		sections = append(sections, drawBoxLabeled(styleSection.Render("Optional routing"), padInspectorLines(lines), outerW))
	} else {
		for i := 0; i < len(routes); {
			if outerW >= 90 && i+1 < len(routes) {
				const gap = 2
				leftW := (outerW - gap) / 2
				rightW := outerW - gap - leftW
				left := renderRouteInspectorCard(routes[i], i+1, leftW)
				right := renderRouteInspectorCard(routes[i+1], i+2, rightW)
				left, right = equalizeBoxHeights(left, right)
				sections = append(sections, joinColumns(left, right, strings.Repeat(" ", gap)))
				i += 2
				continue
			}
			sections = append(sections, renderRouteInspectorCard(routes[i], i+1, outerW))
			i++
		}
	}
	return strings.Join(sections, "\n")
}

func renderRoutingReport(repos []config.Repo, routes []config.RepoRoute, features []*featureRow, width int) string {
	outerW := max(20, width)
	work := summarizeRepoWork(features)
	var sections []string
	summary := []string{styleSubtext.Render(fmt.Sprintf("%d repositories  ·  %d optional metadata routes", len(repos), len(routes)))}
	for _, text := range []string{
		"ticket context  →  exact label/component match  →  selected repo set",
		"no match  →  purpose + agent hints     ambiguous  →  pause for input",
	} {
		for _, line := range strings.Split(ui.Wrap(text, max(8, outerW-6)), "\n") {
			summary = append(summary, styleDim.Render(line))
		}
	}
	sections = append(sections, drawBoxLabeledWith(styleHeader.Render("Repository map"), padInspectorLines(summary), outerW, activeTheme.Palette.Mauve))

	if len(repos) == 0 {
		sections = append(sections, drawBoxLabeled(styleSection.Render("Repositories"), padInspectorLines([]string{styleDim.Render("No repositories configured.")}), outerW))
	} else {
		for i := 0; i < len(repos); {
			if outerW >= 90 && i+1 < len(repos) {
				const gap = 2
				leftW := (outerW - gap) / 2
				rightW := outerW - gap - leftW
				left := renderRepositoryInspectorCard(repos[i], work[repos[i].Name], leftW)
				right := renderRepositoryInspectorCard(repos[i+1], work[repos[i+1].Name], rightW)
				left, right = equalizeBoxHeights(left, right)
				sections = append(sections, joinColumns(left, right, strings.Repeat(" ", gap)))
				i += 2
				continue
			}
			sections = append(sections, renderRepositoryInspectorCard(repos[i], work[repos[i].Name], outerW))
			i++
		}
	}

	if len(routes) == 0 {
		contentW := max(1, outerW-6)
		var lines []string
		for _, item := range []struct {
			text  string
			style lipgloss.Style
		}{
			{"No deterministic metadata routes configured.", styleDim},
			{"task scope  →  repo purpose + agent hints", styleSubtext},
			{"ambiguous  →  pause for input", styleStatusWaiting},
		} {
			for _, line := range strings.Split(ui.Wrap(item.text, contentW), "\n") {
				lines = append(lines, item.style.Render(line))
			}
		}
		sections = append(sections, drawBoxLabeled(styleSection.Render("Optional routing"), padInspectorLines(lines), outerW))
	} else {
		for i, route := range routes {
			sections = append(sections, renderRouteInspectorCard(route, i+1, outerW))
		}
	}
	return strings.Join(sections, "\n")
}

func renderRepositoryInspectorCard(repo config.Repo, work repoWorkSummary, width int) string {
	contentW := max(8, width-15)
	lines := labeledInspectorLines("path", repo.Path, contentW, styleSubtext)
	if repo.Purpose != "" {
		lines = append(lines, labeledInspectorLines("purpose", repo.Purpose, contentW, styleDim)...)
	}
	total := work.active + work.paused
	featureLabel := "features"
	if total == 1 {
		featureLabel = "feature"
	}
	workText := fmt.Sprintf("%d %s · %d active · %d paused", total, featureLabel, work.active, work.paused)
	lines = append(lines, labeledInspectorLines("work", workText, contentW, styleDim)...)
	if len(work.tickets) > 0 {
		lines = append(lines, labeledInspectorLines("tickets", strings.Join(work.tickets, ", "), contentW, styleStatusWaiting)...)
	}
	if repo.WorktreeSetup != "" {
		lines = append(lines, labeledInspectorLines("setup", "configured", contentW, styleHealthOK)...)
	}
	if len(repo.AgentHints) > 0 {
		lines = append(lines, labeledInspectorLines("hints", fmt.Sprintf("%d configured", len(repo.AgentHints)), contentW, styleStatusWaiting)...)
	}
	accent := repoAccentColor(repo.Name)
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(accent)).Bold(true).Render(repo.Name)
	return drawBoxLabeledWith(title, padInspectorLines(lines), width, accent)
}

func labeledInspectorLines(label, value string, width int, valueStyle lipgloss.Style) []string {
	wrapped := strings.Split(ui.Wrap(value, width), "\n")
	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		lineLabel := "         "
		if i == 0 {
			lineLabel = fmt.Sprintf("%-9s", label)
		}
		lines = append(lines, styleDetailLabel.Render(lineLabel)+valueStyle.Render(line))
	}
	return lines
}

func renderRouteInspectorCard(route config.RepoRoute, number, width int) string {
	contentW := max(1, width-6)
	var lines []string
	for _, line := range strings.Split(ui.Wrap(routeSignalText(route), contentW), "\n") {
		lines = append(lines, styleSubtext.Render(line))
	}
	lines = append(lines,
		styleDim.Render("│  exact match"),
		styleDim.Render("▼"),
	)
	lines = append(lines, labeledInspectorLines("selects", strings.Join(route.Repos, ", "), max(1, width-15), styleHealthOK)...)
	title := styleSection.Render(fmt.Sprintf("Optional route %d", number)) + styleDim.Render("  exact metadata")
	return drawBoxLabeledWith(title, padInspectorLines(lines), width, activeTheme.Palette.Teal)
}

func artifactPolicyStyle(policy string) lipgloss.Style {
	if policy == "block" {
		return styleHealthWarn
	}
	return styleHealthOK
}

// renderRouteChain renders the workflow stage sequence with colored arrows and loop stage annotations.
func renderRouteChain(chain []routeStep, loops []repairLoop, maxW int) []string {
	if len(chain) == 0 {
		return nil
	}
	sep := styleDivider.Render("  ")
	sepW := lipgloss.Width(sep)

	// build index: workflow name → x-offset in rendered row
	chipOffsets := map[string]int{}

	var rows []string
	row := ""
	rowW := 0
	for i, step := range chain {
		chip := styleSubtext.Render(stepLabel(step))
		chipW := lipgloss.Width(chip)

		var arrow string
		var arrowW int
		if i < len(chain)-1 {
			if chain[i].advance == "manual" {
				arrow = sep + styleStatusWaiting.Render("→") + sep
			} else {
				arrow = sep + styleHealthOK.Render("→") + sep
			}
			arrowW = sepW*2 + 1
		}

		needed := chipW + arrowW
		if rowW > 0 && rowW+needed > maxW {
			rows = append(rows, row)
			row = ""
			rowW = 0
		}
		chipOffsets[step.name] = rowW
		row += chip
		rowW += chipW
		if arrow != "" {
			row += arrow
			rowW += arrowW
		}
	}
	if row != "" {
		rows = append(rows, row)
	}

	// loop stage annotations: ↺ name positioned under target chip
	if len(loops) > 0 {
		// group loops by target for layout
		type loopAnnotation struct {
			offset int
			label  string
		}
		var annotations []loopAnnotation
		for _, lp := range loops {
			offset, ok := chipOffsets[lp.target]
			if !ok {
				continue
			}
			loopLabel := lp.label
			if loopLabel == "" {
				loopLabel = lp.name
			}
			label := styleStatusWaiting.Render("↺ ") + styleSubtext.Render(loopLabel)
			annotations = append(annotations, loopAnnotation{offset: offset, label: label})
		}
		// sort by offset so we build the line left-to-right
		sort.Slice(annotations, func(i, j int) bool {
			return annotations[i].offset < annotations[j].offset
		})
		if len(annotations) > 0 {
			loopLine := ""
			loopW := 0
			for _, ann := range annotations {
				if ann.offset > loopW {
					loopLine += strings.Repeat(" ", ann.offset-loopW)
					loopW = ann.offset
				}
				connector := styleDivider.Render("└╴")
				full := connector + ann.label
				w := lipgloss.Width(full)
				loopLine += full
				loopW += w
			}
			rows = append(rows, loopLine)
		}
	}

	return rows
}
