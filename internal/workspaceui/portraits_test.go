package workspaceui

import (
	"os"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/workers"
)

// The pixel portrait must fit the portrait panel's content area. If it is
// rendered wider, lipgloss hard-wraps each placeholder row into an extra
// mostly-blank line and the image shows horizontal gap stripes.
func TestCharacterSheetPortraitRowsNotWrapped(t *testing.T) {
	t.Setenv("ORC_PORTRAIT", "kitty")
	t.Setenv("TMUX", "")
	portraitCache = portraitCacheEntry{}

	// renderPortraitKitty writes the image transmission to stdout; silence it.
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer devnull.Close() //nolint:errcheck
	orig := os.Stdout
	os.Stdout = devnull
	defer func() { os.Stdout = orig }()

	m := Model{width: 120, height: 40}
	w := &workers.Worker{ID: "dev-bob", Name: "Bob", Engine: "claude"} // → WARRIOR
	if _, ok := portraitImages[bardClass(w)]; !ok {
		t.Fatal("no embedded PNG for class")
	}

	out := renderCharacterSheet(m, w)

	var counts []int
	for _, line := range strings.Split(out, "\n") {
		if n := strings.Count(line, string(kittyPlaceholder)); n > 0 {
			counts = append(counts, n)
		}
	}
	if len(counts) == 0 {
		t.Fatal("no placeholder rows in character sheet output")
	}
	for i, n := range counts {
		if n != counts[0] {
			t.Errorf("placeholder row %d has %d cells, want %d — portrait row was wrapped", i, n, counts[0])
		}
	}
}
