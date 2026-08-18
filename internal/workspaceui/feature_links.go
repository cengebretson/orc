package workspaceui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
)

const ticketFilename = "TICKET.md"

type featureLinks struct {
	Ticket string
	PR     string
}

type linkAction uint8

const (
	linkActionOpen linkAction = iota
	linkActionCopy
	linkActionChecks
)

type linkActionMsg struct {
	message string
	err     error
}

type ciChecksMsg struct {
	ticket string
	output string
	err    error
}

var (
	labeledLinkPattern = regexp.MustCompile(`(?i)^\s*[-*]\s*(ticket|issue|pr|pull request)\s*:\s*(.*)$`)
	httpURLPattern     = regexp.MustCompile(`https?://[^\s<>\])]+`)
	openExternalURL    = platformOpenURL
	writeClipboard     = clipboard.WriteAll
	fetchPRChecks      = runPRChecks
)

func loadFeatureLinks(featureDir string) (featureLinks, error) {
	content, err := os.ReadFile(filepath.Join(featureDir, ticketFilename))
	if err != nil {
		return featureLinks{}, fmt.Errorf("read %s: %w", ticketFilename, err)
	}
	return parseFeatureLinks(string(content)), nil
}

func parseFeatureLinks(content string) featureLinks {
	var links featureLinks
	for _, line := range strings.Split(content, "\n") {
		match := labeledLinkPattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		rawURL := httpURLPattern.FindString(match[2])
		if !validHTTPURL(rawURL) {
			continue
		}
		switch strings.ToLower(match[1]) {
		case "ticket", "issue":
			if links.Ticket == "" {
				links.Ticket = rawURL
			}
		case "pr", "pull request":
			if links.PR == "" {
				links.PR = rawURL
			}
		}
	}
	return links
}

func validHTTPURL(rawURL string) bool {
	parsed, err := url.ParseRequestURI(rawURL)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (links featureLinks) preferred() (string, string, error) {
	if links.PR != "" {
		return links.PR, "PR", nil
	}
	if links.Ticket != "" {
		return links.Ticket, "ticket", nil
	}
	return "", "", errors.New("no ticket or PR URL in TICKET.md")
}

func featureLinkCommand(feature *featureRow, action linkAction) tea.Cmd {
	if feature == nil || feature.s == nil {
		return linkResultCmd("", errors.New("no feature selected"))
	}
	links, err := loadFeatureLinks(feature.featureDir)
	if err != nil {
		return linkResultCmd("", err)
	}
	if action == linkActionChecks {
		if links.PR == "" {
			return linkResultCmd("", errors.New("no PR URL in TICKET.md"))
		}
		return prChecksCmd(feature.s.Ticket, links.PR)
	}
	target, label, err := links.preferred()
	if err != nil {
		return linkResultCmd("", err)
	}
	if action == linkActionCopy {
		return copyLinkCmd(target, label)
	}
	return openLinkCmd(target, label)
}

func openLinkCmd(target, label string) tea.Cmd {
	return func() tea.Msg {
		if err := openExternalURL(target); err != nil {
			return linkActionMsg{err: fmt.Errorf("open %s: %w", label, err)}
		}
		return linkActionMsg{message: "Opened " + label}
	}
}

func copyLinkCmd(target, label string) tea.Cmd {
	return func() tea.Msg {
		if err := writeClipboard(target); err != nil {
			return linkActionMsg{err: fmt.Errorf("copy %s URL: %w", label, err)}
		}
		return linkActionMsg{message: "Copied " + label + " URL"}
	}
}

func prChecksCmd(ticket, target string) tea.Cmd {
	return func() tea.Msg {
		output, err := fetchPRChecks(target)
		return ciChecksMsg{ticket: ticket, output: output, err: err}
	}
}

func linkResultCmd(message string, err error) tea.Cmd {
	return func() tea.Msg { return linkActionMsg{message: message, err: err} }
}

func clearNoticeAfter(epoch uint64) tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return noticeClearMsg{epoch: epoch} })
}

func platformOpenURL(target string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", target)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.Command("xdg-open", target)
	}
	return command.Run()
}

func runPRChecks(target string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "gh", "pr", "checks", target).CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return string(output), errors.New("timed out loading PR checks")
	}
	if err != nil {
		return string(output), fmt.Errorf("gh pr checks: %w", err)
	}
	return string(output), nil
}
