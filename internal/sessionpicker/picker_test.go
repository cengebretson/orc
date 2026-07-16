package sessionpicker

import (
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/telemetry"
	tea "github.com/charmbracelet/bubbletea"
)

func testCandidates() []Candidate {
	return []Candidate{
		{Live: telemetry.Live{Engine: "codex", ProviderSessionID: "codex-123456789", Model: "gpt-5", CWD: "/work/orc", ContextUsed: 50, ContextLimit: 100}, Branch: "feature/search"},
		{Live: telemetry.Live{Engine: "claude", ProviderSessionID: "claude-456", Model: "opus", CWD: "/work/los", State: "idle"}, Branch: "develop"},
	}
}

func pickerModel(t *testing.T, value tea.Model) Model {
	t.Helper()
	model, ok := value.(Model)
	if !ok {
		t.Fatalf("model = %T, want sessionpicker.Model", value)
	}
	return model
}

func TestModelFiltersAndSelects(t *testing.T) {
	m := New(testCandidates())
	for _, char := range "claude develop" {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{char}})
		m = pickerModel(t, updated)
	}
	if len(m.visible) != 1 || m.visible[0].Live.ProviderSessionID != "claude-456" {
		t.Fatalf("visible = %#v", m.visible)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = pickerModel(t, updated)
	if !m.hasSelection || m.selected.Live.ProviderSessionID != "claude-456" {
		t.Fatalf("selection = %#v", m.selected)
	}
	if cmd == nil {
		t.Fatal("enter should quit")
	}
}

func TestModelNavigatesAndCancels(t *testing.T) {
	m := New(testCandidates())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = pickerModel(t, updated)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = pickerModel(t, updated)
	if !m.cancelled || cmd == nil {
		t.Fatalf("cancelled=%v cmd=%v", m.cancelled, cmd)
	}
}

func TestViewShowsResumeMetadata(t *testing.T) {
	m := New(testCandidates()[:1])
	m.width = 120
	m.height = 20
	m.now = time.Now()
	view := m.View()
	for _, want := range []string{"codex", "gpt-5", "ctx 50%", "feature/search", "/work/orc"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
}
