package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	workspaceevents "github.com/cengebretson/orc/internal/events"
)

func TestEventsIsPrimaryCommand(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"events"})
	if err != nil {
		t.Fatal(err)
	}
	if command != eventsCmd || command.Hidden || command.Deprecated != "" {
		t.Fatalf("events command = %#v, want primary visible command", command)
	}
	for _, name := range []string{"follow", "json", "interval"} {
		if command.Flags().Lookup(name) == nil {
			t.Errorf("events flag --%s is missing", name)
		}
	}
}

func TestEventEmitterWritesJSONL(t *testing.T) {
	event := workspaceevents.Event{
		Type:   workspaceevents.StageChanged,
		At:     time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		Ticket: "ORC-1",
	}
	var output bytes.Buffer
	if err := eventEmitter(&output, true)(event); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(output.String(), "\n") {
		t.Fatalf("JSON event is not newline-delimited: %q", output.String())
	}
	var decoded workspaceevents.Event
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Type != workspaceevents.StageChanged || decoded.Ticket != "ORC-1" {
		t.Fatalf("decoded event = %#v", decoded)
	}
}
