package ui

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestThemeNamesListsEmbeddedThemes(t *testing.T) {
	names := ThemeNames()
	for _, want := range []string{"catppuccin-mocha", "terminal"} {
		if !slices.Contains(names, want) {
			t.Fatalf("ThemeNames() = %v, missing %q", names, want)
		}
	}
	if !slices.IsSorted(names) {
		t.Fatalf("ThemeNames() = %v, want sorted", names)
	}
}

// The terminal theme exists so colours come from the user's own palette rather
// than a baked-in one. Its accent roles must therefore be ANSI slot indices,
// which the terminal maps, not hex values, which override it.
func TestTerminalThemeUsesAnsiSlotsNotHex(t *testing.T) {
	theme, err := LoadTheme("terminal")
	if err != nil {
		t.Fatal(err)
	}
	for role, got := range map[string]string{
		"red": theme.Palette.Red, "green": theme.Palette.Green,
		"yellow": theme.Palette.Yellow, "blue": theme.Palette.Blue,
		"mauve": theme.Palette.Mauve, "surface0": theme.Palette.Surface0,
	} {
		if got == "" || strings.HasPrefix(got, "#") {
			t.Errorf("%s = %q, want an ANSI slot index", role, got)
		}
	}
}

// Text is deliberately unset: lipgloss emits no escape sequence for an empty
// colour, so the terminal's own default foreground shows through. Pinning it to
// a slot would make the rail unreadable on a terminal whose background is the
// other polarity.
func TestTerminalThemeLeavesForegroundToTheTerminal(t *testing.T) {
	theme, err := LoadTheme("terminal")
	if err != nil {
		t.Fatal(err)
	}
	for role, got := range map[string]string{
		"text": theme.Palette.Text, "subtext0": theme.Palette.Subtext0, "subtext1": theme.Palette.Subtext1,
	} {
		if got != "" {
			t.Errorf("%s = %q, want empty", role, got)
		}
	}
	if out := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Palette.Text)).Render("x"); out != "x" {
		t.Fatalf("rendering with an empty colour emitted %q, want bare text", out)
	}
}

func TestTerminalThemeCarriesGlamourStyles(t *testing.T) {
	theme, err := LoadTheme("terminal")
	if err != nil {
		t.Fatal(err)
	}
	if len(theme.Glamour) == 0 {
		t.Fatal("glamour styles missing — markdown views would fall back to defaults")
	}
	// Matching bare "#" would trip over heading prefixes like "## ", so look for
	// hex colour literals specifically.
	if hex := regexp.MustCompile(`#[0-9a-fA-F]{6}`).FindString(string(theme.Glamour)); hex != "" {
		t.Fatalf("glamour still carries hex colour %s, defeating the point of the theme", hex)
	}
}

func TestLoadThemeRejectsUnknownName(t *testing.T) {
	if _, err := LoadTheme("no-such-theme"); err == nil {
		t.Fatal("LoadTheme should reject an unknown name")
	}
}
