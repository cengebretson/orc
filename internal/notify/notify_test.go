package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/config"
)

func TestSendExpandsTemplatesAndExportsEnvironment(t *testing.T) {
	dir := t.TempDir()
	output := filepath.Join(dir, "notification.txt")
	settings := config.NotifySettings{
		On:      []string{"complete"},
		Command: `printf '%s\n%s\n' '{{ticket}} {{slug}} {{event}} {{stage}} {{workflow}}' "$ORC_TICKET $ORC_SLUG $ORC_EVENT $ORC_STAGE $ORC_WORKFLOW" > ` + shellQuote(output),
	}
	event := Event{Ticket: "ORC-9", Slug: "orc-9", Name: "complete", Stage: "review", Workflow: "default"}
	if err := Send(settings, event); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	want := "ORC-9 orc-9 complete review default\nORC-9 orc-9 complete review default\n"
	if string(got) != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestSendDisabledAndEmptyCommandsAreNoOps(t *testing.T) {
	for _, settings := range []config.NotifySettings{
		{On: []string{"blocked"}, Command: "exit 7"},
		{On: []string{"all"}},
	} {
		if err := Send(settings, Event{Name: "complete"}); err != nil {
			t.Fatalf("Send(%#v): %v", settings, err)
		}
	}
}

func TestSendAllEnablesEvent(t *testing.T) {
	if err := Send(config.NotifySettings{On: []string{"ALL"}, Command: "exit 0"}, Event{Name: "blocked"}); err != nil {
		t.Fatal(err)
	}
}

func TestSendReturnsCommandFailure(t *testing.T) {
	err := Send(config.NotifySettings{On: []string{"complete"}, Command: "echo broken >&2; exit 7"}, Event{Name: "complete"})
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("error = %v, want command output", err)
	}
}

func TestSendTimesOut(t *testing.T) {
	err := send(config.NotifySettings{On: []string{"complete"}, Command: "sleep 1"}, Event{Name: "complete"}, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
