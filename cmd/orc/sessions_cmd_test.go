package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/sessionpicker"
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

func TestBuildResumeCandidatesCachesBranchLookup(t *testing.T) {
	discovered := []telemetry.Live{
		{Engine: "codex", ProviderSessionID: "one", CWD: "/work/orc"},
		{Engine: "codex", ProviderSessionID: "two", CWD: "/work/orc"},
		{Engine: "claude", ProviderSessionID: "three", CWD: "/work/claude"},
		{Engine: "other", ProviderSessionID: "ignored", CWD: "/work/other"},
	}
	calls := 0
	candidates := buildResumeCandidates(discovered, "codex", func(cwd string) string {
		calls++
		return "feature/search"
	})
	if len(candidates) != 2 || calls != 1 {
		t.Fatalf("candidates=%d branch calls=%d, want 2/1", len(candidates), calls)
	}
	for _, candidate := range candidates {
		if candidate.Branch != "feature/search" {
			t.Errorf("branch = %q", candidate.Branch)
		}
	}
}

func TestRunSessionResumeWithoutIDUsesPicker(t *testing.T) {
	originalDiscover := discoverResumeSessions
	originalSelect := selectResumeSession
	originalBranch := lookupResumeBranch
	originalDry := sessionsResumeDry
	originalForce := sessionsResumeForce
	originalEngine := sessionsResumeEngine
	originalCWD := sessionsResumeCWD
	defer func() {
		discoverResumeSessions = originalDiscover
		selectResumeSession = originalSelect
		lookupResumeBranch = originalBranch
		sessionsResumeDry = originalDry
		sessionsResumeForce = originalForce
		sessionsResumeEngine = originalEngine
		sessionsResumeCWD = originalCWD
	}()

	cwd := t.TempDir()
	discoverResumeSessions = func(string) ([]telemetry.Live, error) {
		return []telemetry.Live{{Engine: "codex", ProviderSessionID: "picked-session", Model: "gpt-5", CWD: cwd}}, nil
	}
	lookupResumeBranch = func(string) string { return "feature/picker" }
	selectResumeSession = func(candidates []sessionpicker.Candidate) (sessionpicker.Candidate, error) {
		if len(candidates) != 1 || candidates[0].Branch != "feature/picker" {
			return sessionpicker.Candidate{}, fmt.Errorf("unexpected candidates: %#v", candidates)
		}
		return candidates[0], nil
	}
	sessionsResumeDry = true
	sessionsResumeForce = false
	sessionsResumeEngine = ""
	sessionsResumeCWD = ""

	out, err := captureStdout(func() error { return runSessionResume(nil, nil) })
	if err != nil {
		t.Fatalf("runSessionResume: %v", err)
	}
	for _, want := range []string{"cwd: " + cwd, "codex", "picked-session"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
