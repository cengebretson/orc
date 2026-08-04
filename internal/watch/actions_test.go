package watch

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/mux/muxtest"
	tea "github.com/charmbracelet/bubbletea"
)

type watchPromptBackend struct {
	*muxtest.Fake
	prompt func(mux.Target, string, bool, mux.AgentControlOptions) (mux.AgentControlResult, error)
}

func (b *watchPromptBackend) StateAgent(target mux.Target) (mux.AgentControlResult, error) {
	return mux.AgentControlResult{Backend: b.Name(), Target: target, Lifecycle: mux.LifecycleIdle}, nil
}

func (b *watchPromptBackend) WaitAgent(target mux.Target, _ mux.AgentControlOptions) (mux.AgentControlResult, error) {
	return b.StateAgent(target)
}

func (b *watchPromptBackend) PromptAgent(target mux.Target, text string, wait bool, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
	return b.prompt(target, text, wait, options)
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

func TestWatchPromptRequiresReviewAndExplicitConfirmation(t *testing.T) {
	var gotTarget mux.Target
	var gotText string
	backend := &watchPromptBackend{
		Fake: &muxtest.Fake{NameFunc: func() string { return "tmux" }},
		prompt: func(target mux.Target, text string, wait bool, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
			if !wait || options.Context == nil || options.Timeout != watchPromptTimeout || len(options.Until) != 5 {
				t.Fatalf("wait=%v options=%#v", wait, options)
			}
			gotTarget, gotText = target, text
			return mux.AgentControlResult{Backend: "tmux", Target: target, Lifecycle: mux.LifecycleIdle}, nil
		},
	}
	m, err := New(t.TempDir(), Options{Mux: backend})
	if err != nil {
		t.Fatal(err)
	}
	m.width = 48
	m.rows = []row{{
		ticket: "PROJ-123", backend: "tmux", session: "orc", window: "develop", pane: "%7",
		agentID: "agent-1", agentInstance: "instance-1", tmuxState: "live",
	}}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = watchModel(t, updated)
	if !m.prompting || m.confirming || cmd == nil || m.CanSwitchSection() {
		t.Fatalf("prompt start = prompting %v confirming %v cmd %v", m.prompting, m.confirming, cmd)
	}
	for _, r := range "please review" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = watchModel(t, updated)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = watchModel(t, updated)
	if m.prompting || !m.confirming || !strings.Contains(m.View(), "Send this prompt? y / n") {
		t.Fatalf("confirmation state = prompting %v confirming %v\n%s", m.prompting, m.confirming, m.View())
	}

	// Enter alone is intentionally not confirmation.
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = watchModel(t, updated)
	if !m.confirming || cmd != nil {
		t.Fatal("enter sent prompt without explicit y confirmation")
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = watchModel(t, updated)
	if m.confirming || cmd == nil || m.message != "sending prompt to PROJ-123" {
		t.Fatalf("confirmed state = %#v", m)
	}
	updated, _ = m.Update(cmd())
	m = watchModel(t, updated)
	if m.message != "prompt sent to PROJ-123 · idle" {
		t.Fatalf("message = %q", m.message)
	}
	if gotText != "please review" || gotTarget != (mux.Target{
		Backend: "tmux", Workspace: "orc", Tab: "develop", Pane: "%7", AgentID: "agent-1", AgentInstance: "instance-1",
	}) {
		t.Fatalf("prompt = %q / %#v", gotText, gotTarget)
	}
}

func TestWatchPromptRejectsMissingExactIdentityAndCanCancel(t *testing.T) {
	backend := &watchPromptBackend{
		Fake: &muxtest.Fake{NameFunc: func() string { return "tmux" }},
		prompt: func(mux.Target, string, bool, mux.AgentControlOptions) (mux.AgentControlResult, error) {
			t.Fatal("unexpected prompt")
			return mux.AgentControlResult{}, nil
		},
	}
	m, err := New(t.TempDir(), Options{Mux: backend})
	if err != nil {
		t.Fatal(err)
	}
	m.rows = []row{{ticket: "OLD-1", backend: "tmux", session: "orc", window: "develop", pane: "%7", tmuxState: "live"}}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = watchModel(t, updated)
	if m.prompting || m.message != "no exact agent target for OLD-1" {
		t.Fatalf("missing identity state = %#v", m)
	}

	m.rows[0].agentID, m.rows[0].agentInstance = "agent-1", "instance-1"
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = watchModel(t, updated)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = watchModel(t, updated)
	if m.prompting || m.confirming || m.promptBox.Value() != "" {
		t.Fatalf("cancelled state = %#v", m)
	}
}
