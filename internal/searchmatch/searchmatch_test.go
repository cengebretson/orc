package searchmatch

import "testing"

func TestMatch(t *testing.T) {
	fields := []string{"FLYWL-123", "credit-check", "code-review", "Ada", "los-app", "feature/flywl-123", "review"}
	for _, query := range []string{"", "flywl", "ADA", "los-app review", "feature code-review"} {
		if !Match(query, fields...) {
			t.Errorf("Match(%q) = false, want true", query)
		}
	}
	for _, query := range []string{"blocked", "los-app develop", "FLYWL missing"} {
		if Match(query, fields...) {
			t.Errorf("Match(%q) = true, want false", query)
		}
	}
}
