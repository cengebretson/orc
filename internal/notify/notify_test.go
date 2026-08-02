package notify

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/mux/muxtest"
)

type nativeBackend struct {
	*muxtest.Fake
	notification mux.Notification
}

func (b *nativeBackend) ShowNotification(notification mux.Notification) error {
	b.notification = notification
	return nil
}

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

func TestSendNativeMapsTransitionToBackendNotification(t *testing.T) {
	tests := []struct {
		event Event
		sound string
	}{
		{event: Event{Ticket: "ORC-9", Name: "blocked", Stage: "review", Workflow: "default"}, sound: "request"},
		{event: Event{Ticket: "ORC-9", Name: "complete", Stage: "review", Workflow: "default"}, sound: "done"},
	}
	for _, tt := range tests {
		t.Run(tt.event.Name, func(t *testing.T) {
			backend := &nativeBackend{Fake: &muxtest.Fake{}}
			if err := SendNative(backend, tt.event); err != nil {
				t.Fatal(err)
			}
			want := mux.Notification{
				Title: "Orc · ORC-9 " + tt.event.Name,
				Body:  "Stage: review · Workflow: default",
				Sound: tt.sound,
			}
			if backend.notification != want {
				t.Fatalf("notification = %#v, want %#v", backend.notification, want)
			}
		})
	}
}

func TestSendNativeIgnoresUnsupportedBackendAndEvent(t *testing.T) {
	if err := SendNative(&muxtest.Fake{}, Event{Name: "blocked"}); err != nil {
		t.Fatal(err)
	}
	backend := &nativeBackend{Fake: &muxtest.Fake{}}
	if err := SendNative(backend, Event{Name: "error"}); err != nil {
		t.Fatal(err)
	}
	if backend.notification != (mux.Notification{}) {
		t.Fatalf("unexpected notification: %#v", backend.notification)
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
