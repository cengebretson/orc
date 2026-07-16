package main

import (
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/telemetry"
)

func TestLiveColumns(t *testing.T) {
	model, state, context, active := liveColumns(&telemetry.Live{
		Model: "gpt-5", State: "working", ContextUsed: 100000, ContextLimit: 200000,
		LastActive: time.Now().Add(-2 * time.Hour),
	})
	if model != "gpt-5" || state != "working" || context != "50%" || active != "2h" {
		t.Fatalf("columns = %q %q %q %q", model, state, context, active)
	}
}

func TestCompactTokens(t *testing.T) {
	for input, want := range map[uint64]string{42: "42", 1200: "1.2k", 1500000: "1.5M"} {
		if got := compactTokens(input); got != want {
			t.Errorf("compactTokens(%d) = %q, want %q", input, got, want)
		}
	}
}

func TestProviderResumeArgs(t *testing.T) {
	binary, args, err := telemetry.ResumeArgs("codex", "abc", "/work tree")
	if err != nil || binary != "codex" || len(args) != 4 || args[0] != "resume" || args[1] != "abc" || args[3] != "/work tree" {
		t.Fatalf("codex resume = %q %#v, %v", binary, args, err)
	}
	binary, args, err = telemetry.ResumeArgs("claude", "def", "/ignored")
	if err != nil || binary != "claude" || len(args) != 2 || args[1] != "def" {
		t.Fatalf("claude resume = %q %#v, %v", binary, args, err)
	}
}
