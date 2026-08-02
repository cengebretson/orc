// Package notify runs an optional user-configured command after Orc workflow
// transitions. Notifications are best-effort at the CLI boundary; this package
// reports execution failures so callers can warn without rolling back state.
package notify

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
)

const defaultTimeout = 5 * time.Second

var environmentNames = []string{
	"ORC_TICKET",
	"ORC_SLUG",
	"ORC_EVENT",
	"ORC_STAGE",
	"ORC_WORKFLOW",
}

// Event is the workflow context exposed to a notification command.
type Event struct {
	Ticket   string
	Slug     string
	Name     string
	Stage    string
	Workflow string
	WorkDir  string
}

// Send runs the configured command when event is enabled. Empty commands and
// disabled events are successful no-ops.
func Send(settings config.NotifySettings, event Event) error {
	return send(settings, event, defaultTimeout)
}

func send(settings config.NotifySettings, event Event, timeout time.Duration) error {
	if strings.TrimSpace(settings.Command) == "" || !enabled(settings.On, event.Name) {
		return nil
	}

	command := expand(settings.Command, event)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	if event.WorkDir != "" {
		cmd.Dir = event.WorkDir
	}
	cmd.Env = eventEnvironment(os.Environ(), event)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("notification command timed out after %s", timeout)
	}
	if err != nil {
		message := strings.TrimSpace(output.String())
		if message == "" {
			return fmt.Errorf("notification command: %w", err)
		}
		return fmt.Errorf("notification command: %s: %w", message, err)
	}
	return nil
}

func enabled(events []string, event string) bool {
	event = strings.ToLower(strings.TrimSpace(event))
	for _, configured := range events {
		switch strings.ToLower(strings.TrimSpace(configured)) {
		case "all":
			return true
		case event:
			return event != ""
		}
	}
	return false
}

func expand(command string, event Event) string {
	return strings.NewReplacer(
		"{{ticket}}", event.Ticket,
		"{{slug}}", event.Slug,
		"{{event}}", event.Name,
		"{{stage}}", event.Stage,
		"{{workflow}}", event.Workflow,
	).Replace(command)
}

func eventEnvironment(base []string, event Event) []string {
	blocked := make(map[string]bool, len(environmentNames))
	for _, name := range environmentNames {
		blocked[name] = true
	}
	env := make([]string, 0, len(base)+len(environmentNames))
	for _, value := range base {
		name, _, _ := strings.Cut(value, "=")
		if !blocked[name] {
			env = append(env, value)
		}
	}
	return append(env,
		"ORC_TICKET="+event.Ticket,
		"ORC_SLUG="+event.Slug,
		"ORC_EVENT="+event.Name,
		"ORC_STAGE="+event.Stage,
		"ORC_WORKFLOW="+event.Workflow,
	)
}
