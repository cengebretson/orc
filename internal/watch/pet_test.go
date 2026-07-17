package watch

import (
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/state"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestParseMode(t *testing.T) {
	for input, want := range map[string]Mode{
		"":           ModeRail,
		"rail":       ModeRail,
		"pet":        ModePet,
		"TAMAGOTCHI": ModePet,
	} {
		got, err := ParseMode(input)
		if err != nil || got != want {
			t.Fatalf("ParseMode(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ParseMode("office"); err == nil {
		t.Fatal("ParseMode(office) should fail")
	}
}

func TestParsePetPresentationOptions(t *testing.T) {
	for input, want := range map[string]PetSize{
		"":       PetSizeNormal,
		"normal": PetSizeNormal,
		"MICRO":  PetSizeMicro,
	} {
		got, err := ParsePetSize(input)
		if err != nil || got != want {
			t.Fatalf("ParsePetSize(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ParsePetSize("tiny"); err == nil {
		t.Fatal("ParsePetSize(tiny) should fail")
	}

	for input, want := range map[string]PetLayout{
		"":           PetLayoutResponsive,
		"responsive": PetLayoutResponsive,
		"COLUMN":     PetLayoutColumn,
		"vertical":   PetLayoutColumn,
	} {
		got, err := ParsePetLayout(input)
		if err != nil || got != want {
			t.Fatalf("ParsePetLayout(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := ParsePetLayout("masonry"); err == nil {
		t.Fatal("ParsePetLayout(masonry) should fail")
	}
}

func TestPetStateUsesDurableAndLiveSignals(t *testing.T) {
	tests := []struct {
		name string
		row  row
		want petState
	}{
		{name: "pending egg", row: row{status: "pending"}, want: petEgg},
		{name: "ready egg", row: row{status: "ready"}, want: petEgg},
		{name: "working", row: row{status: "active", tmuxState: "live", liveState: "working"}, want: petWorking},
		{name: "idle", row: row{status: "active", tmuxState: "live", liveState: "idle"}, want: petIdle},
		{name: "input", row: row{status: "active", tmuxState: "live", attention: "input"}, want: petInput},
		{name: "review", row: row{status: "active", tmuxState: "live", attention: "review"}, want: petInput},
		{name: "paused", row: row{status: "paused", tmuxState: "live"}, want: petBlocked},
		{name: "stopped", row: row{status: "active", tmuxState: "stopped"}, want: petOffline},
		{name: "done", row: row{status: "done"}, want: petDone},
		{name: "error", row: row{status: "error", loadErr: errFixture{}}, want: petError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := petStateFor(tt.row); got != tt.want {
				t.Fatalf("petStateFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

type errFixture struct{}

func (errFixture) Error() string { return "broken state" }

func TestWatchTogglesPetViewWithoutChangingPreview(t *testing.T) {
	m := Model{
		mode:   ModeRail,
		width:  80,
		height: 24,
		rows: []row{{
			ticket: "ORC-123", status: "active", tmuxState: "live", liveState: "working", room: "orc/main",
		}},
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = watchModel(t, updated)
	if m.mode != ModePet || !m.petTicking || cmd == nil {
		t.Fatalf("pet toggle = mode %q ticking %v cmd %#v", m.mode, m.petTicking, cmd)
	}
	if view := m.View(); !strings.Contains(view, "ORC PETS") || !strings.Contains(view, "ORC-123") {
		t.Fatalf("pet View() missing pet content:\n%s", view)
	}

	updated, cmd = m.Update(petTickMsg(time.Now()))
	m = watchModel(t, updated)
	if m.petFrame != 1 || cmd == nil {
		t.Fatalf("pet tick = frame %d cmd %#v", m.petFrame, cmd)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = watchModel(t, updated)
	if m.mode != ModeRail {
		t.Fatalf("second toggle mode = %q, want rail", m.mode)
	}
	updated, cmd = m.Update(petTickMsg(time.Now()))
	m = watchModel(t, updated)
	if m.petTicking || cmd != nil {
		t.Fatalf("rail should stop pet timer, ticking=%v cmd=%#v", m.petTicking, cmd)
	}

	m.preview = true
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	m = watchModel(t, updated)
	if m.mode != ModeRail || !m.preview {
		t.Fatalf("preview v should be inert, mode=%q preview=%v", m.mode, m.preview)
	}
}

func TestWatchTogglesPetSizeAndLayout(t *testing.T) {
	m := Model{mode: ModePet, width: 100, height: 24}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = watchModel(t, updated)
	if m.petSize != PetSizeMicro {
		t.Fatalf("s toggle size = %q, want micro", m.petSize)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = watchModel(t, updated)
	if m.petLayout != PetLayoutColumn {
		t.Fatalf("l toggle layout = %q, want column", m.petLayout)
	}

	m.preview = true
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = watchModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = watchModel(t, updated)
	if m.petSize != PetSizeMicro || m.petLayout != PetLayoutColumn {
		t.Fatalf("preview toggles should be inert, size=%q layout=%q", m.petSize, m.petLayout)
	}
}

func TestRenderPetsNarrowShowsSelectedLittleOrc(t *testing.T) {
	m := Model{
		mode:   ModePet,
		width:  32,
		height: 24,
		cursor: 1,
		rows: []row{
			{ticket: "ORC-ONE", status: "active", tmuxState: "live", room: "orc/one"},
			{ticket: "ORC-TWO", status: "active", tmuxState: "live", attention: "input", room: "orc/two"},
		},
	}
	view := m.renderPets()
	for _, want := range []string{"ORC PETS", "ORC-TWO", "needs input", "◢", "▴", "v rail"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderPets() missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "ORC-ONE") {
		t.Fatalf("narrow pet view should show only the selected creature:\n%s", view)
	}
	if strings.Contains(view, "\n│ (D") {
		t.Fatalf("narrow pet metadata should stay on one line:\n%s", view)
	}
}

func TestRenderPetsWideKeepsCoreControlsAndContext(t *testing.T) {
	m := Model{
		mode:   ModePet,
		width:  100,
		height: 24,
		rows: []row{
			{
				ticket: "ORC-ONE", status: "active", tmuxState: "live", liveState: "working",
				room: "orc/main", stage: "develop", worker: "builder", engine: "codex", model: "gpt-5",
				context: contextpressure.Evaluate(75, 100, contextpressure.DefaultThresholds()),
			},
			{ticket: "ORC-TWO", status: "paused", tmuxState: "live", room: "orc/review"},
		},
	}
	view := m.renderPets()
	for _, want := range []string{"ORC-ONE", "ORC-TWO", "orc/main", "develop · builder", "codex · gpt-5", "75%", "tired", "/ filter", "a attach", "i focus"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderPets() missing %q:\n%s", want, view)
		}
	}
}

func TestRenderPetsOnlySelectedCardHasVisibleBorder(t *testing.T) {
	m := Model{
		mode:   ModePet,
		width:  100,
		height: 24,
		cursor: 1,
		rows: []row{
			{ticket: "ORC-ONE", status: "active", tmuxState: "live"},
			{ticket: "ORC-TWO", status: "active", tmuxState: "live"},
			{ticket: "ORC-THREE", status: "active", tmuxState: "live"},
		},
	}
	view := m.renderPets()
	if got := strings.Count(view, "╭"); got != 1 {
		t.Fatalf("visible pet borders = %d; want exactly one around the selected pet:\n%s", got, view)
	}
	if !strings.Contains(view, "▶ ORC-TWO") {
		t.Fatalf("selected pet marker missing:\n%s", view)
	}
	if got := lipgloss.Width(view); got > m.width {
		t.Fatalf("focus-only border grid width = %d; want <= %d:\n%s", got, m.width, view)
	}
}

func TestRenderPetsWideFitsThreeLittleOrcs(t *testing.T) {
	m := Model{
		mode:   ModePet,
		width:  100,
		height: 24,
		rows: []row{
			{ticket: "ORC-ONE", status: "active", tmuxState: "live"},
			{ticket: "ORC-TWO", status: "active", tmuxState: "live"},
			{ticket: "ORC-THREE", status: "active", tmuxState: "live"},
			{ticket: "ORC-FOUR", status: "active", tmuxState: "live"},
		},
	}
	view := m.renderPets()
	for _, want := range []string{"ORC-ONE", "ORC-TWO", "ORC-THREE", "page 1/2"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderPets() missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "ORC-FOUR") {
		t.Fatalf("100-column first page should fit exactly three little orcs:\n%s", view)
	}
	if got := lipgloss.Width(view); got > m.width {
		t.Fatalf("compact pet grid width = %d; want <= %d:\n%s", got, m.width, view)
	}
}

func TestRenderPetsMicroColumnUsesVerticalSpace(t *testing.T) {
	m := Model{
		mode:      ModePet,
		petSize:   PetSizeMicro,
		petLayout: PetLayoutColumn,
		width:     100,
		height:    50,
		rows: []row{
			{ticket: "ORC-ONE", status: "active", tmuxState: "live"},
			{ticket: "ORC-TWO", status: "active", tmuxState: "live"},
			{ticket: "ORC-THREE", status: "active", tmuxState: "live"},
			{ticket: "ORC-FOUR", status: "active", tmuxState: "live"},
		},
	}
	view := m.renderPets()
	for _, want := range []string{"micro · column", "ORC-ONE", "ORC-TWO", "ORC-THREE", "ORC-FOUR", "s size", "l layout"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderPets() missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "page 1/") {
		t.Fatalf("50-row micro column should fit four little orcs without paging:\n%s", view)
	}
	if got := lipgloss.Width(view); got > m.width {
		t.Fatalf("micro column width = %d; want <= %d:\n%s", got, m.width, view)
	}
}

func TestPetFramesStayCompact(t *testing.T) {
	tests := []struct {
		name      string
		frameSets map[petState][][]string
		height    int
	}{
		{name: "normal", frameSets: petFrameSets, height: 5},
		{name: "micro", frameSets: microPetFrameSets, height: 3},
	}
	for _, tt := range tests {
		for state, frames := range tt.frameSets {
			for frameIndex, frame := range frames {
				if len(frame) != tt.height {
					t.Errorf("%s %s frame %d height = %d; want %d", tt.name, state, frameIndex, len(frame), tt.height)
				}
				for lineIndex, line := range frame {
					if width := lipgloss.Width(line); width > 13 {
						t.Errorf("%s %s frame %d line %d width = %d; want <= 13", tt.name, state, frameIndex, lineIndex, width)
					}
				}
			}
		}
	}
}

func TestPetAnimationIsDeterministicByState(t *testing.T) {
	working := row{ticket: "ORC-ONE", status: "active", tmuxState: "live", liveState: "working"}
	if renderPetSprite(working, 0, 20, PetSizeNormal) == renderPetSprite(working, 1, 20, PetSizeNormal) {
		t.Fatal("working orc should animate between frames")
	}
	idle := row{ticket: "ORC-TWO", status: "active", tmuxState: "live", liveState: "idle"}
	if renderPetSprite(idle, 0, 20, PetSizeMicro) != renderPetSprite(idle, 1, 20, PetSizeMicro) {
		t.Fatal("sleeping orc should remain still")
	}
}

func TestFeatureRoomUsesStablePrimaryRepository(t *testing.T) {
	room, branch := featureRoom(&state.State{Repos: map[string]state.Repo{
		"zeta":  {Worktree: "/work/zeta/feature-z", Branch: "feature/z"},
		"alpha": {Worktree: "/work/alpha/feature-a", Branch: "feature/a"},
	}})
	if room != "alpha/feature-a" || branch != "feature/a" {
		t.Fatalf("featureRoom() = %q/%q, want alpha/feature-a and feature/a", room, branch)
	}
}
