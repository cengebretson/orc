package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadClaudeJSONLExtractsMetadataOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	content := "{\"type\":\"user\",\"timestamp\":\"2026-07-13T10:00:00Z\",\"cwd\":\"/repo\",\"message\":{\"content\":\"secret prompt\"}}\n" +
		"{\"type\":\"assistant\",\"timestamp\":\"2026-07-13T10:01:00Z\",\"message\":{\"model\":\"claude-opus\",\"content\":\"secret response\",\"usage\":{\"input_tokens\":10,\"cache_creation_input_tokens\":20,\"cache_read_input_tokens\":30,\"output_tokens\":4}}}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	var live Live
	readClaudeJSONL(path, &live)
	if live.CWD != "/repo" || live.Model != "claude-opus" || live.ContextUsed != 64 {
		t.Fatalf("live = %#v", live)
	}
	if want := time.Date(2026, 7, 13, 10, 1, 0, 0, time.UTC); !live.LastActive.Equal(want) {
		t.Fatalf("LastActive = %v, want %v", live.LastActive, want)
	}
}

func TestReadCodexJSONLExtractsLifecycleAndUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	content := "{\"timestamp\":\"2026-07-13T10:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"abc\",\"cwd\":\"/repo\"}}\n" +
		"{\"timestamp\":\"2026-07-13T10:01:00Z\",\"type\":\"turn_context\",\"payload\":{\"model\":\"gpt-5\",\"effort\":\"high\"}}\n" +
		"{\"timestamp\":\"2026-07-13T10:02:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\"}}\n" +
		"{\"timestamp\":\"2026-07-13T10:03:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"token_count\",\"info\":{\"total_token_usage\":{\"total_tokens\":1234},\"last_token_usage\":{\"total_tokens\":321},\"model_context_window\":200000}}}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	live := Live{State: "idle"}
	readCodexJSONL(path, &live)
	if live.ProviderSessionID != "abc" || live.CWD != "/repo" || live.Model != "gpt-5" || live.Effort != "high" {
		t.Fatalf("live = %#v", live)
	}
	if live.State != "working" || live.ContextUsed != 321 || live.ContextLimit != 200000 {
		t.Fatalf("lifecycle/usage = %#v", live)
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

	got, err := DiscoverClaude(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ProviderSessionID != sessionID || got[0].State != "active" || got[0].Model != "opus" {
		t.Fatalf("sessions = %#v", got)
	}
}
