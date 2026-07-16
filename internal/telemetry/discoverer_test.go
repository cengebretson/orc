package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testSessionOne = "12345678-1234-1234-1234-123456789abc"
	testSessionTwo = "87654321-4321-4321-4321-cba987654321"
)

func TestDiscovererReadsOnlyCodexAppendAfterInitialSnapshot(t *testing.T) {
	home, path := writeCodexTranscript(t, codexMeta(testSessionOne, "/repo")+
		codexTurn("old", "high")+codexEvent("task_started"))
	discoverer := testDiscoverer(1024 * 1024)

	got := discoverCodexForTest(t, discoverer, home)
	if len(got) != 1 || got[0].State != "working" || got[0].Model != "old" {
		t.Fatalf("initial sessions = %#v", got)
	}
	initialRead := discoverer.lastRead

	appended := codexTurn("new", "medium") + codexEvent("task_complete")
	appendFile(t, path, appended)
	got = discoverCodexForTest(t, discoverer, home)
	if len(got) != 1 || got[0].State != "idle" || got[0].Model != "new" || got[0].Effort != "medium" {
		t.Fatalf("updated sessions = %#v", got)
	}
	if discoverer.lastRead != int64(len(appended)) {
		t.Fatalf("append read = %d, initial read = %d, append size = %d", discoverer.lastRead, initialRead, len(appended))
	}
}

func TestDiscovererReadsOnlyClaudeAppendAfterInitialSnapshot(t *testing.T) {
	home := t.TempDir()
	directory := filepath.Join(home, ".claude", "projects", "repo")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, testSessionOne+".jsonl")
	initial := claudeAssistant("old", 10)
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	discoverer := testDiscoverer(1024 * 1024)

	got, err := discoverer.DiscoverClaude(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Model != "old" || got[0].ContextUsed != 10 {
		t.Fatalf("initial sessions = %#v", got)
	}
	initialRead := discoverer.lastRead

	appended := claudeAssistant("new", 25)
	appendFile(t, path, appended)
	got, err = discoverer.DiscoverClaude(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Model != "new" || got[0].ContextUsed != 25 {
		t.Fatalf("updated sessions = %#v", got)
	}
	if discoverer.lastRead != int64(len(appended)) {
		t.Fatalf("append read = %d, initial read = %d, append size = %d", discoverer.lastRead, initialRead, len(appended))
	}
}

func TestDiscovererRetriesPartialRecordAfterAppend(t *testing.T) {
	home, path := writeCodexTranscript(t, codexMeta(testSessionOne, "/repo")+codexTurn("old", "high"))
	discoverer := testDiscoverer(1024 * 1024)
	discoverCodexForTest(t, discoverer, home)

	record := codexTurn("new", "low")
	split := len(record) / 2
	appendFile(t, path, record[:split])
	got := discoverCodexForTest(t, discoverer, home)
	if got[0].Model != "old" {
		t.Fatalf("partial record changed metadata: %#v", got[0])
	}

	appendFile(t, path, record[split:])
	got = discoverCodexForTest(t, discoverer, home)
	if got[0].Model != "new" || got[0].Effort != "low" {
		t.Fatalf("completed record was not applied: %#v", got[0])
	}
}

func TestDiscovererResetsAfterTruncate(t *testing.T) {
	home, path := writeCodexTranscript(t, codexMeta(testSessionOne, "/first/repository")+
		codexTurn("first-model-with-a-long-name", "high"))
	discoverer := testDiscoverer(1024 * 1024)
	discoverCodexForTest(t, discoverer, home)

	content := codexMeta(testSessionTwo, "/new") + codexTurn("new", "low")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	got := discoverCodexForTest(t, discoverer, home)
	if got[0].ProviderSessionID != testSessionTwo || got[0].CWD != "/new" || got[0].Model != "new" {
		t.Fatalf("truncated transcript = %#v", got[0])
	}
}

func TestDiscovererResetsSameSizeRewriteAfterModification(t *testing.T) {
	first := codexMeta(testSessionOne, "/one") + codexTurn("model-one", "high")
	second := codexMeta(testSessionTwo, "/two") + codexTurn("model-two", "high")
	if len(first) != len(second) {
		t.Fatalf("test records differ in size: %d != %d", len(first), len(second))
	}
	home, path := writeCodexTranscript(t, first)
	discoverer := testDiscoverer(1024 * 1024)
	discoverCodexForTest(t, discoverer, home)

	if err := os.WriteFile(path, []byte(second), 0o600); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	got := discoverCodexForTest(t, discoverer, home)
	if got[0].ProviderSessionID != testSessionTwo || got[0].CWD != "/two" || got[0].Model != "model-two" {
		t.Fatalf("rewritten transcript = %#v", got[0])
	}
}

func TestDiscovererInvalidatesMalformedAppend(t *testing.T) {
	home, path := writeCodexTranscript(t, codexMeta(testSessionOne, "/repo")+codexTurn("old", "high"))
	discoverer := testDiscoverer(1024 * 1024)
	discoverCodexForTest(t, discoverer, home)

	appendFile(t, path, "{not-json}\n"+codexTurn("new", "low"))
	first := discoverCodexForTest(t, discoverer, home)
	if first[0].Model != "old" {
		t.Fatalf("malformed refresh should retain safe snapshot: %#v", first[0])
	}
	second := discoverCodexForTest(t, discoverer, home)
	if second[0].Model != "new" || second[0].Effort != "low" {
		t.Fatalf("cache was not rebuilt after malformed append: %#v", second[0])
	}
}

func TestDiscovererUsesHeadAndTailWithinRefreshBudget(t *testing.T) {
	secretBody := strings.Repeat("secret prompt body", 2000)
	content := codexMeta(testSessionOne, "/repo") +
		fmt.Sprintf("{\"type\":\"response_item\",\"payload\":{\"body\":%q}}\n", secretBody) +
		codexTurn("latest", "high")
	home, _ := writeCodexTranscript(t, content)
	discoverer := newDiscoverer(discoverConfig{
		maxRefreshBytes: 2048,
		maxRefreshTime:  time.Second,
		headBytes:       512,
		tailBytes:       1024,
		now:             time.Now,
	})

	got := discoverCodexForTest(t, discoverer, home)
	if len(got) != 1 || got[0].ProviderSessionID != testSessionOne || got[0].Model != "latest" {
		t.Fatalf("head/tail summary = %#v", got)
	}
	if discoverer.lastRead > 2048 || discoverer.lastRead >= int64(len(content)) {
		t.Fatalf("read %d bytes from %d-byte history", discoverer.lastRead, len(content))
	}
	for _, snapshot := range discoverer.cache {
		if strings.Contains(fmt.Sprintf("%#v", snapshot), "secret prompt body") {
			t.Fatal("cached snapshot retained prompt content")
		}
	}
}

func TestDiscovererHonorsRefreshDeadline(t *testing.T) {
	content := codexMeta(testSessionOne, "/repo") + strings.Repeat(codexTurn("model", "high"), 1000)
	home, _ := writeCodexTranscript(t, content)
	clock := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	now := func() time.Time {
		clock = clock.Add(time.Millisecond)
		return clock
	}
	discoverer := newDiscoverer(discoverConfig{
		maxRefreshBytes: int64(len(content)),
		maxRefreshTime:  3 * time.Millisecond,
		headBytes:       int64(len(content)),
		tailBytes:       1024,
		now:             now,
	})

	discoverCodexForTest(t, discoverer, home)
	if discoverer.lastRead >= int64(len(content)) {
		t.Fatalf("deadline read entire %d-byte transcript", len(content))
	}
}

func TestDiscovererSharesRefreshBudgetAcrossProviders(t *testing.T) {
	home, _ := writeCodexTranscript(t, codexMeta(testSessionOne, "/repo")+
		strings.Repeat(codexTurn("model", "high"), 20))
	claudeDir := filepath.Join(home, ".claude", "projects", "repo")
	if err := os.MkdirAll(claudeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	claudePath := filepath.Join(claudeDir, testSessionTwo+".jsonl")
	if err := os.WriteFile(claudePath, []byte(strings.Repeat(claudeAssistant("opus", 10), 20)), 0o600); err != nil {
		t.Fatal(err)
	}
	discoverer := newDiscoverer(discoverConfig{
		maxRefreshBytes: 1024,
		maxRefreshTime:  time.Second,
		headBytes:       1024,
		tailBytes:       1024,
		now:             time.Now,
	})

	got, err := discoverer.Discover(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("sessions = %#v", got)
	}
	if discoverer.lastRead > 1024 {
		t.Fatalf("combined refresh read %d bytes", discoverer.lastRead)
	}
}

func testDiscoverer(maxBytes int64) *Discoverer {
	return newDiscoverer(discoverConfig{
		maxRefreshBytes: maxBytes,
		maxRefreshTime:  time.Second,
		headBytes:       maxBytes,
		tailBytes:       maxBytes,
		now:             time.Now,
	})
}

func writeCodexTranscript(t *testing.T, content string) (string, string) {
	t.Helper()
	home := t.TempDir()
	directory := filepath.Join(home, ".codex", "sessions", "2026", "07", "13")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "rollout-2026-07-13T00-00-00-"+testSessionOne+".jsonl")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return home, path
}

func appendFile(t *testing.T, path string, content string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close() //nolint:errcheck
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func discoverCodexForTest(t *testing.T, discoverer *Discoverer, home string) []Live {
	t.Helper()
	got, err := discoverer.DiscoverCodex(home)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func codexMeta(id string, cwd string) string {
	return fmt.Sprintf("{\"timestamp\":\"2026-07-13T10:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":%q,\"cwd\":%q}}\n", id, cwd)
}

func codexTurn(model string, effort string) string {
	return fmt.Sprintf("{\"timestamp\":\"2026-07-13T10:01:00Z\",\"type\":\"turn_context\",\"payload\":{\"model\":%q,\"effort\":%q}}\n", model, effort)
}

func codexEvent(kind string) string {
	return fmt.Sprintf("{\"timestamp\":\"2026-07-13T10:02:00Z\",\"type\":\"event_msg\",\"payload\":{\"type\":%q}}\n", kind)
}

func claudeAssistant(model string, tokens uint64) string {
	return fmt.Sprintf("{\"type\":\"assistant\",\"timestamp\":\"2026-07-13T10:00:00Z\",\"message\":{\"model\":%q,\"usage\":{\"input_tokens\":%d}}}\n", model, tokens)
}
