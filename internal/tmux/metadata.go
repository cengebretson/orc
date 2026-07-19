package tmux

import (
	"fmt"
	"strings"
)

// SetWindowMetadata stamps Orc identity onto a tmux window using user options.
// Empty values are omitted so partial metadata can still be recorded safely.
func SetWindowMetadata(session, window string, metadata WindowMetadata) error {
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
func SetPaneMetadata(pane string, metadata WindowMetadata) error {
	options := []struct {
		name  string
		value string
	}{
		{"@orc_agent", "1"},
		{"@orc_ticket", metadata.Ticket},
		{"@orc_stage", metadata.Stage},
		{"@orc_worker", metadata.Worker},
		{"@orc_engine", metadata.Engine},
		{"@orc_provider_engine", metadata.Engine},
		{"@orc_provider_session", metadata.ProviderSessionID},
		{"@orc_feature_dir", metadata.FeatureDir},
	}
	for _, option := range options {
		if option.value == "" {
			if option.name == "@orc_provider_engine" || option.name == "@orc_provider_session" {
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

// WindowAttention returns the supported tmux-attention state for a window.
// Unknown, cleared, and unreadable values are treated as no live overlay.
func WindowAttention(session, window string) string {
	value, err := WindowOption(session, window, "@agent_attention")
	if err != nil {
		return ""
	}
	switch strings.ToLower(value) {
	case AttentionInput, AttentionBlocked, AttentionReview, AttentionDone:
		return strings.ToLower(value)
	default:
		return ""
	}
}
