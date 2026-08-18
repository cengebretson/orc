package workspaceui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
)

func TestParseFeatureLinks(t *testing.T) {
	links := parseFeatureLinks(`# Ticket

## Links

- Ticket: https://jira.example.com/browse/ORC-9
- PR: [PR 42](https://github.com/example/orc/pull/42)
`)
	if links.Ticket != "https://jira.example.com/browse/ORC-9" {
		t.Errorf("ticket URL = %q", links.Ticket)
	}
	if links.PR != "https://github.com/example/orc/pull/42" {
		t.Errorf("PR URL = %q", links.PR)
	}
}

func TestParseFeatureLinksIgnoresPlaceholdersAndUnrelatedURLs(t *testing.T) {
	links := parseFeatureLinks(`Description: https://example.com/not-a-link-field
- Ticket: <!-- URL -->
- PR: not-open-yet
`)
	if links != (featureLinks{}) {
		t.Fatalf("links = %#v, want empty", links)
	}
}

func TestPreferredFeatureLinkUsesPRThenTicket(t *testing.T) {
	links := featureLinks{Ticket: "https://tickets.example/ORC-9", PR: "https://github.com/example/orc/pull/42"}
	if target, label, err := links.preferred(); err != nil || target != links.PR || label != "PR" {
		t.Fatalf("preferred = %q, %q, %v", target, label, err)
	}
	links.PR = ""
	if target, label, err := links.preferred(); err != nil || target != links.Ticket || label != "ticket" {
		t.Fatalf("ticket fallback = %q, %q, %v", target, label, err)
	}
}

func TestFeatureLinkCommandsUseTicketMetadata(t *testing.T) {
	featureDir := t.TempDir()
	content := "## Links\n\n- Ticket: https://tickets.example/ORC-9\n- PR: https://github.com/example/orc/pull/42\n"
	if err := os.WriteFile(filepath.Join(featureDir, ticketFilename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	feature := &featureRow{s: &state.State{Ticket: "ORC-9"}, featureDir: featureDir}

	originalOpen := openExternalURL
	originalCopy := writeClipboard
	originalChecks := fetchPRChecks
	t.Cleanup(func() {
		openExternalURL = originalOpen
		writeClipboard = originalCopy
		fetchPRChecks = originalChecks
	})

	var opened, copied, checked string
	openExternalURL = func(target string) error { opened = target; return nil }
	writeClipboard = func(target string) error { copied = target; return nil }
	fetchPRChecks = func(target string) (string, error) { checked = target; return "checks pass", nil }

	openMsg, ok := featureLinkCommand(feature, linkActionOpen)().(linkActionMsg)
	if !ok || openMsg.err != nil || opened != "https://github.com/example/orc/pull/42" {
		t.Fatalf("open message = %#v, target = %q", openMsg, opened)
	}
	copyMsg, ok := featureLinkCommand(feature, linkActionCopy)().(linkActionMsg)
	if !ok || copyMsg.err != nil || copied != opened {
		t.Fatalf("copy message = %#v, target = %q", copyMsg, copied)
	}
	checksMsg, ok := featureLinkCommand(feature, linkActionChecks)().(ciChecksMsg)
	if !ok || checksMsg.err != nil || checked != opened || checksMsg.output != "checks pass" {
		t.Fatalf("checks message = %#v, target = %q", checksMsg, checked)
	}
}

func TestFeatureLinkCommandReportsMissingPR(t *testing.T) {
	featureDir := t.TempDir()
	content := "## Links\n\n- Ticket: https://tickets.example/ORC-9\n- PR: <!-- URL when available -->\n"
	if err := os.WriteFile(filepath.Join(featureDir, ticketFilename), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	feature := &featureRow{s: &state.State{Ticket: "ORC-9"}, featureDir: featureDir}
	msg, ok := featureLinkCommand(feature, linkActionChecks)().(linkActionMsg)
	if !ok || msg.err == nil || !strings.Contains(msg.err.Error(), "no PR URL") {
		t.Fatalf("message = %#v", msg)
	}
}

func TestCIMessageOpensExistingViewer(t *testing.T) {
	m := NewWithMux("", nil)
	m.width = 100
	m.height = 30
	m.view = viewDetail
	m.detail.feature = &featureRow{s: &state.State{Ticket: "ORC-9"}}

	updated, _ := m.Update(ciChecksMsg{ticket: "ORC-9", output: "build  pass", err: errors.New("exit status 1")})
	got := asModel(t, updated)
	if got.view != viewFile || got.viewer.title != "CI checks" || got.viewer.context != "ORC-9" {
		t.Fatalf("viewer = %#v", got.viewer)
	}
	content := got.viewer.viewport.View()
	if !strings.Contains(content, "build  pass") || !strings.Contains(content, "exit status 1") {
		t.Fatalf("viewer content = %q", content)
	}
}
