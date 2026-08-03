package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/gitmeta"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/spf13/cobra"
)

var collectCtlSessions = sessionlist.Collect

var (
	ctlStateTicket   string
	ctlWatchTicket   string
	ctlWatchInterval time.Duration
	ctlPromptTicket  string
	ctlPromptWait    bool
	ctlPromptUntil   []string
	ctlPromptTimeout time.Duration
	ctlWaitTicket    string
	ctlWaitUntil     []string
	ctlWaitTimeout   time.Duration
	ctlCaptureTicket string
	ctlCaptureLines  int
)

var ctlCmd = &cobra.Command{
	Use:   "ctl",
	Short: "Structured agent control for automation",
}

var ctlStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Read aggregate durable and live session state",
	Args:  cobra.NoArgs,
	RunE:  runCtlStatus,
}

var ctlAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Control a ticket's exact recorded agent",
}

var ctlAgentStateCmd = &cobra.Command{
	Use:   "state",
	Short: "Read recognized lifecycle state for the exact recorded agent",
	Args:  cobra.NoArgs,
	RunE:  runCtlAgentState,
}

var ctlAgentWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Stream recognized agent lifecycle transitions as JSONL",
	Args:  cobra.NoArgs,
	RunE:  runCtlAgentWatch,
}

var ctlSessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Read exact recorded terminal sessions",
}

var ctlSessionCaptureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture text from an exact recorded agent pane",
	Args:  cobra.NoArgs,
	RunE:  runCtlSessionCapture,
}

var ctlAgentPromptCmd = &cobra.Command{
	Use:   "prompt <text>",
	Short: "Atomically prompt an agent and optionally wait for lifecycle state",
	Args:  cobra.ExactArgs(1),
	RunE:  runCtlAgentPrompt,
}

var ctlAgentWaitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Wait for recognized agent lifecycle state",
	Args:  cobra.NoArgs,
	RunE:  runCtlAgentWait,
}

type ctlCommandError struct {
	code    string
	message string
}

func (e *ctlCommandError) Error() string { return e.message }

func ctlError(code, format string, args ...any) error {
	return &ctlCommandError{code: code, message: fmt.Sprintf(format, args...)}
}

func runCtlStatus(_ *cobra.Command, _ []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	sessions, err := collectCtlSessions(root, sessionlist.Options{
		IncludeUnmanaged: true,
		ResolveGit:       gitmeta.Resolve,
		Mux:              muxBackend,
	})
	if err != nil {
		return err
	}
	summary := ctlStatusSummary{Total: len(sessions)}
	for _, session := range sessions {
		switch session.Kind {
		case sessionlist.KindManaged:
			summary.Managed++
		case sessionlist.KindOrphaned:
			summary.Orphaned++
		case sessionlist.KindUnmanaged:
			summary.Unmanaged++
		}
		if ctlSessionNeedsAttention(session) {
			summary.NeedsAttention++
		}
	}
	return printJSON(map[string]any{"type": "status", "summary": summary, "sessions": sessions})
}

type ctlStatusSummary struct {
	Total          int `json:"total"`
	Managed        int `json:"managed"`
	Orphaned       int `json:"orphaned"`
	Unmanaged      int `json:"unmanaged"`
	NeedsAttention int `json:"needs_attention"`
}

func runCtlAgentState(_ *cobra.Command, _ []string) error {
	controller, target, ticketID, err := resolveCtlAgent(ctlStateTicket)
	if err != nil {
		return err
	}
	result, err := controller.StateAgent(target)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"type": "agent_state", "ticket": ticketID, "agent": result})
}

func runCtlAgentWatch(cmd *cobra.Command, _ []string) error {
	if ctlWatchInterval <= 0 {
		return ctlError("invalid_argument", "--interval must be greater than zero")
	}
	writer := io.Writer(os.Stdout)
	if cmd != nil {
		writer = cmd.OutOrStdout()
	}
	previous := make(map[string]ctlAgentSnapshot)
	started := false
	emit := func() error {
		states, err := collectCtlAgentStates(ctlWatchTicket)
		if err != nil {
			return err
		}
		if ctlWatchTicket == "" && len(states) == 0 && !started {
			return ctlError("no_controllable_agents", "no tickets have a live backend with recognized agent lifecycle control")
		}
		started = true
		seenTickets := make(map[string]bool, len(states))
		for _, current := range states {
			seenTickets[current.Ticket] = true
			prior, seen := previous[current.Ticket]
			if seen && ctlAgentSnapshotsEqual(prior, current) {
				continue
			}
			payload := map[string]any{"type": "agent_state", "ticket": current.Ticket, "agent": current.Agent}
			if current.Error != nil {
				payload = map[string]any{"type": "agent_error", "ticket": current.Ticket, "error": current.Error}
			}
			if err := json.NewEncoder(writer).Encode(payload); err != nil {
				return err
			}
			previous[current.Ticket] = current
		}
		stopped := make([]string, 0)
		for ticketID := range previous {
			if !seenTickets[ticketID] {
				stopped = append(stopped, ticketID)
			}
		}
		sort.Strings(stopped)
		for _, ticketID := range stopped {
			if err := json.NewEncoder(writer).Encode(map[string]any{
				"type": "agent_stopped", "ticket": ticketID,
			}); err != nil {
				return err
			}
			delete(previous, ticketID)
		}
		return nil
	}
	if err := emit(); err != nil {
		return err
	}
	ticker := time.NewTicker(ctlWatchInterval)
	defer ticker.Stop()
	ctx := context.Background()
	if cmd != nil {
		ctx = cmd.Context()
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := emit(); err != nil {
				return err
			}
		}
	}
}

func runCtlSessionCapture(_ *cobra.Command, _ []string) error {
	if ctlCaptureLines <= 0 {
		return ctlError("invalid_argument", "--lines must be greater than zero")
	}
	if ctlCaptureLines > mux.MaxCaptureLines {
		return ctlError("invalid_argument", "--lines must not exceed %d", mux.MaxCaptureLines)
	}
	backend, target, ticketID, err := resolveCtlTarget(ctlCaptureTicket)
	if err != nil {
		return err
	}
	capturer, ok := backend.(mux.TerminalCaptureBackend)
	if !ok {
		return ctlError("unsupported_backend", "%s does not provide exact terminal capture", backend.Name())
	}
	text, err := capturer.CaptureTarget(target, ctlCaptureLines)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{
		"type": "session_capture", "ticket": ticketID, "target": target,
		"lines": ctlCaptureLines, "text": text,
	})
}

func runCtlAgentPrompt(cmd *cobra.Command, args []string) error {
	if cmd != nil && !ctlPromptWait && (cmd.Flags().Changed("timeout") || len(ctlPromptUntil) > 0) {
		return ctlError("invalid_argument", "--timeout and --until require --wait")
	}
	controller, target, ticketID, err := resolveCtlAgent(ctlPromptTicket)
	if err != nil {
		return err
	}
	until, err := validateCtlUntil(ctlPromptUntil)
	if err != nil {
		return err
	}
	options := mux.AgentControlOptions{Until: until}
	if ctlPromptWait {
		if ctlPromptTimeout <= 0 {
			return ctlError("invalid_argument", "--timeout must be greater than zero")
		}
		options.Timeout = ctlPromptTimeout
	}
	result, err := controller.PromptAgent(target, args[0], ctlPromptWait, options)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"type": "agent_prompted", "ticket": ticketID, "agent": result})
}

func runCtlAgentWait(_ *cobra.Command, _ []string) error {
	controller, target, ticketID, err := resolveCtlAgent(ctlWaitTicket)
	if err != nil {
		return err
	}
	until, err := validateCtlUntil(ctlWaitUntil)
	if err != nil {
		return err
	}
	if ctlWaitTimeout <= 0 {
		return ctlError("invalid_argument", "--timeout must be greater than zero")
	}
	result, err := controller.WaitAgent(target, mux.AgentControlOptions{
		Until: until, Timeout: ctlWaitTimeout,
	})
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"type": "agent_waited", "ticket": ticketID, "agent": result})
}

func resolveCtlAgent(ticketArg string) (mux.AgentControlBackend, mux.Target, string, error) {
	backend, target, ticketID, err := resolveCtlTarget(ticketArg)
	if err != nil {
		return nil, mux.Target{}, "", err
	}
	controller, ok := backend.(mux.AgentControlBackend)
	if !ok {
		return nil, mux.Target{}, "", ctlError(
			"unsupported_backend", "%s does not provide recognized agent lifecycle control", backend.Name(),
		)
	}
	return controller, target, ticketID, nil
}

func resolveCtlTarget(ticketArg string) (mux.Backend, mux.Target, string, error) {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return nil, mux.Target{}, "", err
	}
	t, err := ticket.Load(root, ticketArg)
	if err != nil {
		return nil, mux.Target{}, "", err
	}
	target, ok := runtimeTarget(t.State)
	if !ok || target.Workspace == "" || target.Tab == "" || target.Pane == "" {
		return nil, mux.Target{}, "", ctlError("target_not_recorded", "ticket %s has no exact recorded agent target", t.State.Ticket)
	}
	if err := selectMuxForState(t.State); err != nil {
		return nil, mux.Target{}, "", err
	}
	if muxBackend == nil || muxBackend.Name() != target.Backend {
		selected := ""
		if muxBackend != nil {
			selected = muxBackend.Name()
		}
		return nil, mux.Target{}, "", ctlError(
			"backend_mismatch", "ticket %s uses %s but selected backend is %s", t.State.Ticket, target.Backend, selected,
		)
	}
	return muxBackend, target, t.State.Ticket, nil
}

type ctlAgentSnapshot struct {
	Ticket string
	Agent  mux.AgentControlResult
	Error  *ctlAgentWatchError
}

type ctlAgentWatchError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ctlAgentSnapshotsEqual(left, right ctlAgentSnapshot) bool {
	if left.Ticket != right.Ticket || left.Agent != right.Agent {
		return false
	}
	if left.Error == nil || right.Error == nil {
		return left.Error == nil && right.Error == nil
	}
	return *left.Error == *right.Error
}

func collectCtlAgentStates(ticketArg string) ([]ctlAgentSnapshot, error) {
	tickets, err := ctlAgentTickets(ticketArg)
	if err != nil {
		return nil, err
	}
	result := make([]ctlAgentSnapshot, 0, len(tickets))
	for _, ticketID := range tickets {
		controller, target, resolvedTicket, err := resolveCtlAgent(ticketID)
		if err != nil {
			var commandErr *ctlCommandError
			if ticketArg == "" && errors.As(err, &commandErr) && commandErr.code == "unsupported_backend" {
				continue
			}
			if ticketArg == "" {
				result = append(result, ctlAgentSnapshot{Ticket: ticketID, Error: ctlAgentError(err)})
				continue
			}
			return nil, err
		}
		agent, err := controller.StateAgent(target)
		if err != nil {
			if ticketArg == "" {
				result = append(result, ctlAgentSnapshot{Ticket: resolvedTicket, Error: ctlAgentError(err)})
				continue
			}
			return nil, err
		}
		result = append(result, ctlAgentSnapshot{Ticket: resolvedTicket, Agent: agent})
	}
	if len(result) == 0 && ticketArg != "" {
		return nil, ctlError("no_controllable_agents", "no tickets have a live backend with recognized agent lifecycle control")
	}
	return result, nil
}

func ctlAgentError(err error) *ctlAgentWatchError {
	code := "agent_unavailable"
	var commandErr *ctlCommandError
	if errors.As(err, &commandErr) {
		code = commandErr.code
	}
	return &ctlAgentWatchError{Code: code, Message: err.Error()}
}

func ctlAgentTickets(ticketArg string) ([]string, error) {
	if strings.TrimSpace(ticketArg) != "" {
		return []string{ticketArg}, nil
	}
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return nil, err
	}
	features, err := featurelist.Collect(root, featurelist.Options{})
	if err != nil {
		return nil, err
	}
	var tickets []string
	for _, feature := range features {
		if feature == nil || feature.State == nil || feature.Archived || feature.LoadError != nil {
			continue
		}
		target, ok := runtimeTarget(feature.State)
		if !ok || target.Workspace == "" || target.Tab == "" || target.Pane == "" {
			continue
		}
		tickets = append(tickets, feature.State.Ticket)
	}
	sort.Strings(tickets)
	return tickets, nil
}

func ctlSessionNeedsAttention(session sessionlist.Session) bool {
	values := []string{session.Status, session.Attention, session.Lifecycle}
	if session.Live != nil {
		values = append(values, session.Live.State)
	}
	for _, value := range values {
		switch strings.ToLower(value) {
		case "input", "blocked", "review", "paused":
			return true
		}
	}
	return false
}

func validateCtlUntil(values []string) ([]string, error) {
	valid := map[string]bool{"idle": true, "working": true, "blocked": true, "done": true, "unknown": true}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if !valid[value] {
			return nil, ctlError("invalid_argument", "invalid lifecycle state %q (use idle, working, blocked, done, or unknown)", value)
		}
		result = append(result, value)
	}
	return result, nil
}

func isCtlInvocation(args []string) bool {
	for _, arg := range args {
		if arg == "ctl" {
			return true
		}
		if arg == "--" {
			return false
		}
	}
	return false
}

func prepareCtlOutput(args []string) {
	if isCtlInvocation(args) {
		rootCmd.SilenceErrors = true
		rootCmd.SilenceUsage = true
	}
}

func writeCtlError(w io.Writer, err error) {
	code := "ctl_failed"
	message := err.Error()
	var commandErr *ctlCommandError
	if errors.As(err, &commandErr) {
		code = commandErr.code
		message = commandErr.message
	}
	var agentErr *mux.AgentControlError
	if errors.As(err, &agentErr) {
		code = agentErr.Code
		message = agentErr.Message
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
