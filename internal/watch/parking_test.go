package watch

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/parking"
)

func TestScopedParkingDisplayRetainsFullPolicySnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	policy := parking.Policy{AutoPark: []string{"paused"}, WakeOn: []string{"stage_change"}}
	now := time.Now().UTC()
	rows := []row{{ticket: "ORC-1", status: "paused", stageName: "develop"}, {ticket: "ORC-2", status: "paused", stageName: "develop"}}
	observations := []parking.Observation{{Ticket: "ORC-1", Status: "paused", Stage: "develop"}, {Ticket: "ORC-2", Status: "paused", Stage: "develop"}}
	if err := applyParkingToRows(rows, path, "/workspace", policy, observations, now); err != nil {
		t.Fatal(err)
	}
	if scoped := filterRowsByTicket(rows, "orc-1"); len(scoped) != 1 || scoped[0].ticket != "ORC-1" {
		t.Fatalf("scoped rows = %#v", scoped)
	}

	rows = []row{{ticket: "ORC-1", status: "paused", stageName: "develop"}, {ticket: "ORC-2", status: "paused", stageName: "review"}}
	observations[1].Stage = "review"
	if err := applyParkingToRows(rows, path, "/workspace", policy, observations, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if !rows[1].woken || rows[1].wakeReason != "stage_change" {
		t.Fatalf("second row = %#v, want stage-change wake", rows[1])
	}
}

func TestParkingWarningKeepsRowsVisible(t *testing.T) {
	m := Model{width: 80}
	updated, _ := m.Update(dataMsg{rows: []row{{ticket: "ORC-8", name: "Visible"}}, warning: errors.New("state unavailable")})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("updated model type = %T", updated)
	}
	if len(got.rows) != 1 || got.loadErr != nil || got.loadWarning == nil {
		t.Fatalf("model rows=%d loadErr=%v warning=%v", len(got.rows), got.loadErr, got.loadWarning)
	}
	view := got.renderRail()
	if !strings.Contains(view, "ORC-8") || !strings.Contains(view, "parking warning") {
		t.Fatalf("rendered warning did not preserve row:\n%s", view)
	}
}

func TestParkingIgnoresPresentationOnlyAttention(t *testing.T) {
	// Inference, Orc's own launch reset, and anything Orc did not register --
	// "claude" and "codex" are what the tmux-attention CLI records when an agent
	// hook sets a marker, and its --source is free text, so any tool or person
	// can write one. None of these may wake parked work.
	for _, source := range []string{"screen", "title", "launch", "claude", "codex", "orc", ""} {
		if got := parkingAttention(row{attention: "blocked", attentionSource: source}); got != "" {
			t.Errorf("source %q attention = %q, want empty for parking", source, got)
		}
	}
	// Registration: the agent reported its own state through a channel Orc
	// verifies.
	for _, source := range []string{"hook", "native"} {
		if got := parkingAttention(row{attention: "blocked", attentionSource: source}); got != "blocked" {
			t.Errorf("source %q attention = %q, want blocked", source, got)
		}
	}
}
