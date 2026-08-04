package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed themes/*.json assets/logo.txt
var themeFS embed.FS

type Palette struct {
	Crust    string `json:"crust"`
	Mantle   string `json:"mantle"`
	Base     string `json:"base"`
	Surface0 string `json:"surface0"`
	Surface1 string `json:"surface1"`
	Surface2 string `json:"surface2"`
	Overlay0 string `json:"overlay0"`
	Overlay1 string `json:"overlay1"`
	Subtext0 string `json:"subtext0"`
	Subtext1 string `json:"subtext1"`
	Text     string `json:"text"`
	Lavender string `json:"lavender"`
	Blue     string `json:"blue"`
	Sapphire string `json:"sapphire"`
	Sky      string `json:"sky"`
	Teal     string `json:"teal"`
	Green    string `json:"green"`
	Yellow   string `json:"yellow"`
	Peach    string `json:"peach"`
	Maroon   string `json:"maroon"`
	Red      string `json:"red"`
	Mauve    string `json:"mauve"`
	Pink     string `json:"pink"`
	Flamingo string `json:"flamingo"`
}

type Theme struct {
	Palette Palette         `json:"palette"`
	Glamour json.RawMessage `json:"glamour"`
}

func LoadTheme(name string) (Theme, error) {
	if name == "" {
		name = "catppuccin-mocha"
	}
	data, err := themeFS.ReadFile("themes/" + name + ".json")
	if err != nil {
		return Theme{}, fmt.Errorf("theme %q not found", name)
	}
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return Theme{}, fmt.Errorf("parsing theme %q: %w", name, err)
	}
	return theme, nil
}

func DefaultTheme() Theme {
	theme, err := LoadTheme("")
	if err != nil {
		panic("failed to load default theme: " + err.Error())
	}
	return theme
}

func Logo() string {
	data, err := themeFS.ReadFile("assets/logo.txt")
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(data), "\n")
}

// ThemeNames lists the embedded themes, sorted. A theme is selected by name in
// orc.yaml and an unknown name falls back silently, so callers need a way to
// tell a user what the valid names actually are.
func ThemeNames() []string {
	entries, err := themeFS.ReadDir("themes")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, strings.TrimSuffix(entry.Name(), ".json"))
	}
	sort.Strings(names)
	return names
}
