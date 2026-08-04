package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// parseClaudeLine is what the incremental discoverer applies to every Claude
// transcript record it reads. It must lift metadata out and leave the prompt
// and response bodies behind — Orc never retains conversation content.
func TestParseClaudeLineExtractsMetadataOnly(t *testing.T) {
	lines := []string{
		`{"type":"user","timestamp":"2026-07-13T10:00:00Z","cwd":"/repo","message":{"content":"secret prompt"}}`,
		`{"type":"assistant","timestamp":"2026-07-13T10:01:00Z","message":{"model":"claude-opus","content":"secret response","usage":{"input_tokens":10,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":4}}}`,
	}

	var live Live
	for _, line := range lines {
		parseClaudeLine([]byte(line), &live)
	}
	if live.CWD != "/repo" || live.Model != "claude-opus" || live.ContextUsed != 64 {
		t.Fatalf("live = %#v", live)
	}
	if want := time.Date(2026, 7, 13, 10, 1, 0, 0, time.UTC); !live.LastActive.Equal(want) {
		t.Fatalf("LastActive = %v, want %v", live.LastActive, want)
	}
	if contains(live, "secret") {
		t.Fatalf("parsed telemetry retained conversation content: %#v", live)
	}
}

// contains reports whether any string field of live holds the given substring,
// so the metadata-only guarantee is checked against the whole value rather
// than the handful of fields a test happens to name.
func contains(live Live, substring string) bool {
	for _, field := range []string{
		live.Engine, live.ProviderSessionID, live.Model, live.Effort,
		live.State, live.CWD, live.Ticket,
	} {
		if strings.Contains(field, substring) {
			return true
		}
	}
	return false
}

func TestParseCodexLineExtractsLifecycleAndUsage(t *testing.T) {
	lines := []string{
		`{"timestamp":"2026-07-13T10:00:00Z","type":"session_meta","payload":{"id":"abc","cwd":"/repo"}}`,
		`{"timestamp":"2026-07-13T10:01:00Z","type":"turn_context","payload":{"model":"gpt-5","effort":"high"}}`,
		`{"timestamp":"2026-07-13T10:02:00Z","type":"event_msg","payload":{"type":"task_started"}}`,
		`{"timestamp":"2026-07-13T10:03:00Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":1234},"last_token_usage":{"total_tokens":321},"model_context_window":200000}}}`,
	}

	live := Live{State: "idle"}
	working := false
	for _, line := range lines {
		parseCodexLine([]byte(line), &live, &working)
	}
	if live.ProviderSessionID != "abc" || live.CWD != "/repo" || live.Model != "gpt-5" || live.Effort != "high" {
		t.Fatalf("live = %#v", live)
	}
	if !working || live.ContextUsed != 321 || live.ContextLimit != 200000 {
		t.Fatalf("lifecycle/usage = working=%v %#v", working, live)
	}
}

func TestDiscoverClaudeDeduplicatesLiveAndHistoricalSession(t *testing.T) {
	const sessionID = "12345678-1234-1234-1234-123456789abc"
	home := t.TempDir()
	projectDir := filepath.Join(home, ".claude", "projects", "repo")
	processDir := filepath.Join(home, ".claude", "sessions")
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(processDir, 0o700); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(projectDir, sessionID+".jsonl")
	if err := os.WriteFile(transcript, []byte("{\"type\":\"assistant\",\"cwd\":\"/repo\",\"message\":{\"model\":\"opus\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	process := fmt.Sprintf("{\"pid\":%d,\"sessionId\":%q,\"cwd\":\"/repo\",\"status\":\"active\"}", os.Getpid(), sessionID)
	if err := os.WriteFile(filepath.Join(processDir, "current.json"), []byte(process), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := NewDiscoverer().DiscoverClaude(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ProviderSessionID != sessionID || got[0].State != "active" || got[0].Model != "opus" {
		t.Fatalf("sessions = %#v", got)
	}
}
