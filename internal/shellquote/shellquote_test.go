package shellquote

import (
	"os/exec"
	"strings"
	"testing"
)

// Quote's output has to depend only on its input. agenthooks matches freshly
// built hook commands against commands written into Codex and Claude config by
// earlier Orc versions, so a value that quotes differently depending on its
// content would stop Orc recognizing hooks it installed itself.
func TestQuoteAlwaysQuotes(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"", "''"},
		{"plain", "'plain'"},
		{"/usr/local/bin/orc", "'/usr/local/bin/orc'"},
		{"with space", "'with space'"},
		{"it's", `'it'"'"'s'`},
	} {
		if got := Quote(test.in); got != test.want {
			t.Errorf("Quote(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// Word leaves values alone when they already survive as one shell word, so
// generated command lines stay readable.
func TestWordQuotesOnlyWhenNeeded(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"", "''"},
		{"plain", "plain"},
		{"--workspace", "--workspace"},
		{"/usr/local/bin/orc", "/usr/local/bin/orc"},
		{"with space", "'with space'"},
		{"it's", `'it'"'"'s'`},
		{"a;b", "'a;b'"},
		{"$HOME", "'$HOME'"},
	} {
		if got := Word(test.in); got != test.want {
			t.Errorf("Word(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// Whatever the two functions produce, a shell has to read it back as exactly
// the original string — that is the whole point of quoting it.
func TestQuotedValuesRoundTripThroughBash(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	values := []string{
		"plain", "with space", "it's", `a"b`, "a;b|c&d", "$HOME", "`whoami`",
		"tab\there", "new\nline", "(paren)", "{brace}", "back\\slash", "!bang",
	}
	for _, quote := range []struct {
		name string
		fn   func(string) string
	}{{"Quote", Quote}, {"Word", Word}} {
		for _, value := range values {
			out, err := exec.Command("bash", "-c", "printf %s "+quote.fn(value)).Output()
			if err != nil {
				t.Errorf("%s(%q): bash rejected %s: %v", quote.name, value, quote.fn(value), err)
				continue
			}
			if string(out) != value {
				t.Errorf("%s(%q) round-tripped as %q", quote.name, value, string(out))
			}
		}
	}
}

// A quoted value is one word even when it contains separators, so a command
// line built from several of them keeps its argument boundaries.
func TestQuotedValuesStayOneArgument(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not installed")
	}
	script := "printf '%s\\n' " + Word("/tmp/orc bin/orc") + " " + Quote("a b c")
	out, err := exec.Command("bash", "-c", script).Output()
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	got := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	want := []string{"/tmp/orc bin/orc", "a b c"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("arguments = %q, want %q", got, want)
	}
}
