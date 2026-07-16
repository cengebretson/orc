package watch

import (
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderRailIsCompact(t *testing.T) {
	m := Model{
		width:  28,
		height: 20,
		rows: []row{
			{ticket: "PROJ-123", stage: "develop", worker: "bob", status: "paused", tmuxState: "live", next: "fix tests"},
			{ticket: "PROJ-124", stage: "review", worker: "ada", status: "active", tmuxState: "live"},
		},
	}

	view := m.renderRail()
	for _, want := range []string{"ORC", "SESSIONS", "PROJ-123", "PROJ-124", "DETAIL", "blocked", "develop", "bob", "Blocker", "fix tests"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderRail() missing %q in:\n%s", want, view)
		}
	}
	for _, tooWordy := range []string{"WORKERS", "Bob (Developer)", "Ticket", "Next"} {
		if strings.Contains(view, tooWordy) {
			t.Fatalf("renderRail() should not render %q in compact view:\n%s", tooWordy, view)
		}
	}
}

func TestRenderWideUsesSelectionIndicator(t *testing.T) {
	m := Model{
		width:  80,
		height: 20,
		cursor: 1,
		rows: []row{
			{ticket: "PROJ-123", stage: "develop", worker: "bob", status: "paused", tmuxState: "live"},
			{
				ticket:    "PROJ-124",
				stage:     "review",
				worker:    "ada",
				status:    "active",
				tmuxState: "live",
				history: []historyRow{
					{at: "2026-06-20T10:00:00Z", stage: "develop", worker: "bob", result: "implemented watch rail"},
					{at: "2026-06-21T11:00:00Z", stage: "review", worker: "ada", result: "requested changes"},
				},
			},
		},
	}

	view := m.renderWide()
	if !strings.Contains(view, "> PROJ-124") {
		t.Fatalf("renderWide() should mark selected work row with >:\n%s", view)
	}
	if !strings.Contains(view, "Worker") || !strings.Contains(view, "ada") {
		t.Fatalf("renderWide() should include worker in the session list:\n%s", view)
	}
	if strings.Contains(view, "WORKERS") {
		t.Fatalf("renderWide() should not render a separate worker list:\n%s", view)
	}
	for _, notWant := range []string{"HISTORY", "implemented watch rail", "requested changes"} {
		if strings.Contains(view, notWant) {
			t.Fatalf("renderWide() should keep history on the expanded details page, found %q in:\n%s", notWant, view)
		}
	}
}

func TestPreviewWrapsNextAction(t *testing.T) {
	m := Model{
		width:   24,
		height:  20,
		preview: true,
		rows: []row{
			{
				ticket: "PROJ-123",
				stage:  "develop",
				worker: "bob",
				status: "active",
				next:   "fix the failing password reset tests and hand off to review",
				history: []historyRow{
					{at: "2026-06-20T10:00:00Z", stage: "develop", worker: "bob", result: "implemented watch rail"},
					{at: "2026-06-21T11:00:00Z", stage: "review", worker: "ada", result: "requested changes"},
				},
			},
		},
	}

	view := m.previewContent()
	if !strings.Contains(view, "Next") {
		t.Fatalf("previewContent() missing Next section:\n%s", view)
	}
	if !strings.Contains(view, "password reset") {
		t.Fatalf("previewContent() missing wrapped next action:\n%s", view)
	}
	for _, want := range []string{"History", "implemented watch rail", "requested changes"} {
		if !strings.Contains(view, want) {
			t.Fatalf("previewContent() missing history text %q in:\n%s", want, view)
		}
	}
}

func TestRailDetailWrapsNextAction(t *testing.T) {
	view := renderRailDetail(row{
		stage:     "develop",
		worker:    "bob",
		status:    "active",
		tmuxState: "live",
		next:      "fix the failing password reset tests and hand off to review",
	}, 18)

	for _, want := range []string{"Next", "fix the failing", "password reset", "hand off", "review"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderRailDetail() missing wrapped text %q in:\n%s", want, view)
		}
	}
	if strings.Contains(view, "fix the failing password reset tests and hand off to review") {
		t.Fatalf("renderRailDetail() should wrap, not leave next action on one line:\n%s", view)
	}
}

func TestRailDetailUsesIndentedMetaAndPromptHeader(t *testing.T) {
	view := renderRailDetail(row{
		stage:     "default:pr-repair",
		worker:    "Bob (Developer)",
		status:    "paused",
		tmuxState: "live",
		next:      "Need product decision on refresh token TTL",
	}, 32)
	lines := strings.Split(view, "\n")

	wantOrder := []string{
		"! blocked",
		"  default:pr-repair",
		"  Bob (Developer)",
		"  tmux live",
		"",
		"Blocker",
		"Need product decision on refresh",
	}
	if len(lines) < len(wantOrder) {
		t.Fatalf("renderRailDetail() has too few lines:\n%s", view)
	}
	for i, want := range wantOrder {
		if lines[i] != want {
			t.Fatalf("line %d = %q, want %q in:\n%s", i, lines[i], want, view)
		}
	}
}

func TestPromptLabelUsesBlockerForPaused(t *testing.T) {
	blocked := renderRailDetail(row{status: "paused", next: "need product decision"}, 24)
	if !strings.Contains(blocked, "Blocker") || strings.Contains(blocked, "Next") {
		t.Fatalf("paused rail detail should label prompt as Blocker:\n%s", blocked)
	}

	active := renderRailDetail(row{status: "active", next: "continue implementation"}, 24)
	if !strings.Contains(active, "Next") || strings.Contains(active, "Blocker") {
		t.Fatalf("active rail detail should label prompt as Next:\n%s", active)
	}
}

func TestRenderHistoryShowsMostRecentEntries(t *testing.T) {
	rows := []historyRow{
		{at: "2026-06-17T10:00:00Z", stage: "one", worker: "a", result: "oldest"},
		{at: "2026-06-18T10:00:00Z", stage: "two", worker: "b", result: "older"},
		{at: "2026-06-19T10:00:00Z", stage: "three", worker: "c", result: "newest"},
	}

	view := renderHistory(rows, 72, 2)
	if strings.Contains(view, "oldest") {
		t.Fatalf("renderHistory() should omit entries past the limit:\n%s", view)
	}
	for _, want := range []string{"2026-06-18", "older", "2026-06-19", "newest"} {
		if !strings.Contains(view, want) {
			t.Fatalf("renderHistory() missing %q in:\n%s", want, view)
		}
	}
}

func TestDisplayStateUsesDurableStatusFirst(t *testing.T) {
	tests := []struct {
		name      string
		row       row
		wantIcon  string
		wantLabel string
	}{
		{
			name:      "paused is blocked even with live tmux",
			row:       row{status: "paused", tmuxState: "live"},
			wantIcon:  "!",
			wantLabel: "blocked",
		},
		{
			name:      "done is done even with stopped tmux",
			row:       row{status: "done", tmuxState: "stopped"},
			wantIcon:  "✓",
			wantLabel: "done",
		},
		{
			name:      "pending is pending",
			row:       row{status: "pending", tmuxState: "live"},
			wantIcon:  "○",
			wantLabel: "pending",
		},
		{
			name:      "ready is ready",
			row:       row{status: "ready", tmuxState: "live"},
			wantIcon:  "▶",
			wantLabel: "ready",
		},
		{
			name:      "active without tmux is stopped",
			row:       row{status: "active", tmuxState: "stopped"},
			wantIcon:  "x",
			wantLabel: "stopped",
		},
		{
			name:      "active with live tmux is active",
			row:       row{status: "active", tmuxState: "live"},
			wantIcon:  "●",
			wantLabel: "active",
		},
		{
			name:      "active input attention needs input",
			row:       row{status: "active", tmuxState: "live", attention: "input"},
			wantIcon:  "!",
			wantLabel: "input",
		},
		{
			name:      "active review attention needs review",
			row:       row{status: "active", tmuxState: "live", attention: "review"},
			wantIcon:  "◆",
			wantLabel: "review",
		},
		{
			name:      "durable paused overrides input attention",
			row:       row{status: "paused", tmuxState: "live", attention: "input"},
			wantIcon:  "!",
			wantLabel: "blocked",
		},
		{
			name:      "stopped overrides attention",
			row:       row{status: "active", tmuxState: "stopped", attention: "input"},
			wantIcon:  "x",
			wantLabel: "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			icon, label := displayState(tt.row)
			if icon != tt.wantIcon || label != tt.wantLabel {
				t.Fatalf("displayState() = %q/%q, want %q/%q", icon, label, tt.wantIcon, tt.wantLabel)
			}
		})
	}
}

func TestSortRowsPrioritizesAttention(t *testing.T) {
	rows := []row{
		{ticket: "ACTIVE", status: "active", tmuxState: "live"},
		{ticket: "DONE", status: "done", tmuxState: "stopped"},
		{ticket: "REVIEW", status: "active", tmuxState: "live", attention: "review"},
		{ticket: "STOPPED", status: "active", tmuxState: "stopped"},
		{ticket: "INPUT", status: "active", tmuxState: "live", attention: "input"},
		{ticket: "BLOCKED", status: "paused", tmuxState: "live"},
	}

	sortRows(rows)
	got := make([]string, 0, len(rows))
	for _, r := range rows {
		got = append(got, r.ticket)
	}
	want := []string{"BLOCKED", "INPUT", "REVIEW", "STOPPED", "ACTIVE", "DONE"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sortRows() = %v, want %v", got, want)
	}
}

func TestAttachSelectedValidatesTarget(t *testing.T) {
	tests := []struct {
		name    string
		model   Model
		message string
	}{
		{
			name:    "no selection",
			model:   Model{},
			message: "no session selected",
		},
		{
			name:    "missing target",
			model:   Model{rows: []row{{ticket: "PROJ-123", status: "active"}}},
			message: "no tmux target for PROJ-123",
		},
		{
			name: "stopped",
			model: Model{rows: []row{{
				ticket:    "PROJ-123",
				session:   "PROJ-123",
				window:    "develop",
				status:    "active",
				tmuxState: "stopped",
			}}},
			message: "tmux session stopped for PROJ-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, message := tt.model.attachSelected()
			if cmd != nil {
				t.Fatalf("attachSelected() cmd = %#v, want nil", cmd)
			}
			if message != tt.message {
				t.Fatalf("attachSelected() message = %q, want %q", message, tt.message)
			}
		})
	}
}

func TestAttachSelectedBuildsCommand(t *testing.T) {
	orig := newAttachCmd
	defer func() { newAttachCmd = orig }()
	var gotSession, gotWindow string
	newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
		gotSession = session
		gotWindow = window
		return exec.Command("true"), nil
	}

	m := Model{rows: []row{{
		ticket:    "PROJ-123",
		session:   "PROJ-123",
		window:    "develop",
		status:    "active",
		tmuxState: "live",
	}}}
	cmd, message := m.attachSelected()
	if cmd == nil {
		t.Fatal("attachSelected() cmd = nil, want command")
	}
	if message != "attaching PROJ-123:develop" {
		t.Fatalf("attachSelected() message = %q", message)
	}
	if gotSession != "PROJ-123" || gotWindow != "develop" {
		t.Fatalf("newAttachCmd called with %q/%q", gotSession, gotWindow)
	}
}

func TestWatchUpdateAttachSetsMessage(t *testing.T) {
	orig := newAttachCmd
	defer func() { newAttachCmd = orig }()
	newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
		return exec.Command("true"), nil
	}

	m := Model{rows: []row{{
		ticket:    "PROJ-123",
		session:   "PROJ-123",
		window:    "develop",
		status:    "active",
		tmuxState: "live",
	}}}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if got.message != "attaching PROJ-123:develop" {
		t.Fatalf("message = %q", got.message)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want attach command")
	}
}

func TestWatchUpdateFocusesNextAttentionSession(t *testing.T) {
	orig := newAttachCmd
	defer func() { newAttachCmd = orig }()
	var gotSession, gotWindow string
	newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
		gotSession = session
		gotWindow = window
		return exec.Command("true"), nil
	}

	m := Model{
		cursor: 0,
		rows: []row{
			{ticket: "ACTIVE", session: "ACTIVE", window: "develop", status: "active", tmuxState: "live"},
			{ticket: "REVIEW", session: "REVIEW", window: "code-review", status: "active", tmuxState: "live", attention: "review"},
		},
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}
	if got.message != "attaching REVIEW:code-review" {
		t.Fatalf("message = %q", got.message)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want focus attach command")
	}
	if gotSession != "REVIEW" || gotWindow != "code-review" {
		t.Fatalf("newAttachCmd called with %q/%q", gotSession, gotWindow)
	}
}
