package tmux

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/tmuxattention"
)

// SetWindowMetadata stamps Orc identity onto a tmux window using user options.
// Empty values are omitted so partial metadata can still be recorded safely.
func SetWindowMetadata(session, window string, metadata mux.Metadata) error {
	target := session + ":" + window
	options := []struct {
		name  string
		value string
	}{
		{"@orc_ticket", metadata.Ticket},
		{"@orc_stage", metadata.Stage},
		{"@orc_worker", metadata.Worker},
		{"@orc_engine", metadata.Engine},
		{"@orc_provider_engine", metadata.Engine},
		{"@orc_provider_session", metadata.ProviderSessionID},
		{"@orc_feature_dir", metadata.FeatureDir},
		{"@orc_feature_slug", metadata.FeatureSlug},
	}
	for _, option := range options {
		if option.value == "" {
			if option.name == "@orc_provider_engine" || option.name == "@orc_provider_session" {
				if err := newCommand("tmux", "set-option", "-w", "-u", "-t", target, option.name).Run(); err != nil {
					return fmt.Errorf("clear %s on %s: %w", option.name, target, err)
				}
			}
			continue
		}
		if err := newCommand("tmux", "set-option", "-w", "-t", target, option.name, option.value).Run(); err != nil {
			return fmt.Errorf("set %s on %s: %w", option.name, target, err)
		}
	}
	return nil
}

// SetPaneMetadata stamps the exact agent pane used by Orc.
func SetPaneMetadata(pane string, metadata mux.Metadata) error {
	options := []struct {
		name           string
		value          string
		clearWhenEmpty bool
	}{
		{name: "@orc_agent", value: "1"},
		{name: "@orc_agent_id", value: metadata.AgentID, clearWhenEmpty: true},
		{name: "@orc_agent_instance", value: metadata.AgentInstance, clearWhenEmpty: true},
		{name: "@orc_ticket", value: metadata.Ticket},
		{name: "@orc_stage", value: metadata.Stage},
		{name: "@orc_worker", value: metadata.Worker},
		{name: "@orc_engine", value: metadata.Engine},
		{name: "@orc_provider_engine", value: metadata.Engine, clearWhenEmpty: true},
		{name: "@orc_provider_session", value: metadata.ProviderSessionID, clearWhenEmpty: true},
		{name: "@orc_feature_dir", value: metadata.FeatureDir},
		{name: "@orc_feature_slug", value: metadata.FeatureSlug},
		{name: "@agent_pane_context_override", value: metadata.Ticket, clearWhenEmpty: true},
		{name: "@agent_pane_context_slug", value: tmuxattention.DisplaySlug(metadata.Ticket, metadata.FeatureSlug), clearWhenEmpty: true},
	}
	for _, option := range options {
		if option.value == "" {
			if option.clearWhenEmpty {
				if err := newCommand("tmux", "set-option", "-p", "-u", "-t", pane, option.name).Run(); err != nil {
					return fmt.Errorf("clear %s on pane %s: %w", option.name, pane, err)
				}
			}
			continue
		}
		if err := newCommand("tmux", "set-option", "-p", "-t", pane, option.name, option.value).Run(); err != nil {
			return fmt.Errorf("set %s on pane %s: %w", option.name, pane, err)
		}
	}
	return nil
}

// SetSessionEnvironment records live correlation metadata in tmux's session
// environment. Callers must also pass the value to an already-running pane
// process when they need the provider itself to inherit it.
func SetSessionEnvironment(session, name, value string) error {
	if !validEnvironmentName(name) {
		return fmt.Errorf("invalid tmux environment name %q", name)
	}
	if err := newCommand("tmux", "set-environment", "-t", session, name, value).Run(); err != nil {
		return fmt.Errorf("set tmux environment %s on %s: %w", name, session, err)
	}
	return nil
}

func validEnvironmentName(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		if index == 0 && char >= '0' && char <= '9' {
			return false
		}
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

// SessionEnvironment reads a value from a tmux session environment table.
func SessionEnvironment(session, name string) (string, error) {
	if !validEnvironmentName(name) {
		return "", fmt.Errorf("invalid tmux environment name %q", name)
	}
	out, err := newCommand("tmux", "show-environment", "-t", session, name).Output()
	if err != nil {
		return "", fmt.Errorf("read tmux environment %s on %s: %w", name, session, err)
	}
	line := strings.TrimSpace(string(out))
	_, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", fmt.Errorf("tmux environment %s on %s has invalid output", name, session)
	}
	return value, nil
}

// WindowOption reads a tmux user option from a specific window.
func WindowOption(session, window, option string) (string, error) {
	target := session + ":" + window
	out, err := newCommand("tmux", "show-options", "-w", "-qv", "-t", target, option).Output()
	if err != nil {
		return "", fmt.Errorf("read %s on %s: %w", option, target, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// normalizeAttention maps a raw option value to a supported attention state.
// Unknown and cleared values are treated as no live overlay rather than being
// passed through, so a display never has to recognize a state Orc does not
// define.
func normalizeAttention(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case mux.AttentionInput, mux.AttentionBlocked, mux.AttentionReview, mux.AttentionDone:
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

// WindowAttention returns the attention state for a window, rolled up across
// its panes.
//
// The marker is read per pane rather than per window because a window can host
// more than one agent — a jit task sent into a stage's session, or a split the
// user made — and a window-scoped read reports whichever wrote last, so a
// blocked agent could be hidden behind a done one. Panes inherit the window's
// value when they have none of their own, so setups that only ever set the
// window option keep working unchanged.
func WindowAttention(session, window string) string {
	state, _ := WindowAttentionSince(session, window)
	return state
}

// WindowAttentionSince returns the rolled-up attention state for a window and
// when it began, in epoch seconds. A zero time means no pane reported one.
func WindowAttentionSince(session, window string) (string, int64) {
	return mux.RollUpAttention(windowPanes(session, window))
}
