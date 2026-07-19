package workspaceui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestDetailViewScrollsAndRestoresOnReturn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TICKET.md"), []byte("# ZZZUNIQUE file body"), 0644); err != nil {
		t.Fatal(err)
	}
	row := testRow("STORY-1", "active", "develop")
	row.featureDir = dir
	for i := 0; i < 30; i++ { // long history → taller than the viewport
		row.s.History = append(row.s.History, state.HistoryEntry{
			At: "2026-06-01T09:00:00Z", Stage: "develop", Worker: "bob", Result: "iteration",
		})
	}

	m := New("")
	m.width, m.height = 100, 20
	m.data.features = []*featureRow{row}

	m, _ = press(t, m, "enter")
	if m.view != viewDetail {
		t.Fatalf("view = %v, want viewDetail", m.view)
	}
	if body := ansi.Strip(m.viewer.viewport.View()); !strings.Contains(body, "State") {
		t.Fatalf("detail viewport missing body:\n%s", body)
	}

	before := m.viewer.viewport.YOffset
	m, _ = press(t, m, "down", "down", "down")
	if m.viewer.viewport.YOffset <= before {
		t.Fatalf("down should scroll the body, offset %d -> %d", before, m.viewer.viewport.YOffset)
	}
	scrolled := m.viewer.viewport.YOffset

	// open the selected file, then return — body restored, scroll preserved
	m, _ = press(t, m, "enter")
	if m.view != viewFile {
		t.Fatalf("enter should open the file viewer, got %v", m.view)
	}
	m, _ = press(t, m, "esc")
	if m.view != viewDetail {
		t.Fatalf("esc should return to detail, got %v", m.view)
	}
	back := ansi.Strip(m.viewer.viewport.View())
	if strings.Contains(back, "ZZZUNIQUE") {
		t.Errorf("file content still shown after returning to detail:\n%s", back)
	}
	if !strings.Contains(back, "STORY-1") {
		t.Errorf("detail body not restored after returning from file:\n%s", back)
	}
	if m.viewer.viewport.YOffset != scrolled {
		t.Errorf("scroll not restored: want %d, got %d", scrolled, m.viewer.viewport.YOffset)
	}
}

func TestUpdateWindowSizeKeepsDetailBody(t *testing.T) {
	row := testRow("STORY-1", "active", "develop")
	row.featureDir = t.TempDir()
	m := New("")
	m.width, m.height = 120, 40
	m.data.features = []*featureRow{row}
	m, _ = press(t, m, "enter")

	tm, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	got := asModel(t, tm)
	if got.view != viewDetail {
		t.Fatalf("view = %v after resize, want viewDetail", got.view)
	}
	if body := ansi.Strip(got.viewer.viewport.View()); !strings.Contains(body, "State") {
		t.Errorf("detail body lost after resize:\n%s", body)
	}
}
