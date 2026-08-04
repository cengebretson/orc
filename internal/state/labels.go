package state

import (
	"fmt"
	"sort"
	"strings"
)

// Labels are durable, arbitrary key=value tags on a feature. Orc assigns no
// meaning to any key: they exist so a workspace can carry its own vocabulary
// (area, priority, owner) without Orc growing a field for each one.
//
// They are workflow-neutral. Nothing in a transition reads them, so a label can
// never change what a stage does — only which features a view shows.

// SetLabel records or replaces one label. An empty value is rejected rather
// than treated as a delete, so removing a label is always explicit.
func SetLabel(featureDir, key, value string) error {
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	if err := validateLabelKey(key); err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("label %q needs a value — use `orc label <ticket> --remove %s` to delete it", key, key)
	}
	return Update(featureDir, func(s *State) error {
		if s.Labels == nil {
			s.Labels = map[string]string{}
		}
		s.Labels[key] = value
		return nil
	})
}

// RemoveLabel deletes one label. Removing a label that was never set is an
// error: silently succeeding would hide a typo in the key.
func RemoveLabel(featureDir, key string) error {
	key = strings.TrimSpace(key)
	if err := validateLabelKey(key); err != nil {
		return err
	}
	return Update(featureDir, func(s *State) error {
		if _, ok := s.Labels[key]; !ok {
			return fmt.Errorf("%s has no label %q", s.Ticket, key)
		}
		delete(s.Labels, key)
		if len(s.Labels) == 0 {
			s.Labels = nil
		}
		return nil
	})
}

func validateLabelKey(key string) error {
	if key == "" {
		return fmt.Errorf("label key is required")
	}
	if strings.ContainsAny(key, "= \t\n") {
		return fmt.Errorf("label key %q cannot contain '=' or whitespace", key)
	}
	return nil
}

// ParseLabel splits a key=value argument.
func ParseLabel(raw string) (key, value string, err error) {
	key, value, found := strings.Cut(raw, "=")
	if !found {
		return "", "", fmt.Errorf("label %q must be key=value", raw)
	}
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	if err := validateLabelKey(key); err != nil {
		return "", "", err
	}
	if value == "" {
		return "", "", fmt.Errorf("label %q must be key=value", raw)
	}
	return key, value, nil
}

// LabelPairs renders labels as sorted "key=value" strings, so every view lists
// them in the same order regardless of map iteration.
func (s *State) LabelPairs() []string {
	if s == nil || len(s.Labels) == 0 {
		return nil
	}
	pairs := make([]string, 0, len(s.Labels))
	for key, value := range s.Labels {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}

// LabelSelector is a parsed --label filter. A selector with no value matches
// any feature carrying the key at all, which is what makes `--label owner`
// useful for "anything assigned".
type LabelSelector struct {
	Key      string
	Value    string
	HasValue bool
}

// ParseSelector parses a --label argument as either "key" or "key=value".
func ParseSelector(raw string) (LabelSelector, error) {
	raw = strings.TrimSpace(raw)
	key, value, found := strings.Cut(raw, "=")
	key, value = strings.TrimSpace(key), strings.TrimSpace(value)
	if err := validateLabelKey(key); err != nil {
		return LabelSelector{}, err
	}
	if !found {
		return LabelSelector{Key: key}, nil
	}
	if value == "" {
		return LabelSelector{}, fmt.Errorf("label selector %q must be key or key=value", raw)
	}
	return LabelSelector{Key: key, Value: value, HasValue: true}, nil
}

// Matches reports whether a feature's labels satisfy the selector. Comparison
// is case-insensitive to match how the interactive filters already behave.
func (sel LabelSelector) Matches(labels map[string]string) bool {
	for key, value := range labels {
		if !strings.EqualFold(key, sel.Key) {
			continue
		}
		return !sel.HasValue || strings.EqualFold(value, sel.Value)
	}
	return false
}

// MatchesAll reports whether labels satisfy every selector. Multiple selectors
// intersect: `--label area=api --label priority=high` means both, which is the
// only reading that lets a filter narrow.
func MatchesAll(labels map[string]string, selectors []LabelSelector) bool {
	for _, selector := range selectors {
		if !selector.Matches(labels) {
			return false
		}
	}
	return true
}
