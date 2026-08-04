// Package shellquote renders values as single shell words.
//
// Orc builds shell command lines in several places — tmux send-keys scripts,
// Herdr task cells, and the agent lifecycle hook commands written into Codex
// and Claude configuration. Those callers do not all want the same thing, so
// this package offers two contracts rather than one blended function.
package shellquote

import "strings"

// metacharacters are the bytes that stop a value from being a single shell
// word on its own.
const metacharacters = " \t\n\"'\\$`!;|&<>(){}"

// Quote always wraps value in single quotes, escaping any embedded single
// quotes.
//
// Use it when the output is compared against a string built earlier — the
// agent hook installer matches freshly built commands against commands written
// into config by earlier Orc versions, so the result has to depend only on the
// input, never on whether the value happens to contain a metacharacter.
func Quote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

// Word quotes value only when it would not otherwise survive as one shell
// word, so ordinary paths and flags stay readable.
//
// Use it when building a command line a human may read, or when the value is
// interpolated into a template that supplies its own quoting. Do not use it
// where the output is later matched by prefix: whether a given value comes
// back quoted depends on its content, so the same input rendered by different
// call sites still matches, but a value that gains a metacharacter changes
// shape. Prefer [Quote] for anything round-tripped.
func Word(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, metacharacters) {
		return value
	}
	return Quote(value)
}
