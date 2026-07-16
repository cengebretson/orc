// Package searchmatch provides the shared presentation-only matching used by
// Orc's interactive session and workflow views.
package searchmatch

import "strings"

// Match reports whether every whitespace-delimited query term appears in the
// combined fields. Matching is case-insensitive and deliberately simple so the
// same query behaves identically in each interactive view.
func Match(query string, fields ...string) bool {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join(fields, " "))
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
