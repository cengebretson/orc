package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/spf13/cobra"
)

var (
	ctlPromptTicket  string
	ctlPromptWait    bool
	ctlPromptUntil   []string
	ctlPromptTimeout time.Duration
	ctlWaitTicket    string
	ctlWaitUntil     []string
	ctlWaitTimeout   time.Duration
)

var ctlCmd = &cobra.Command{
	Use:   "ctl",
	Short: "Structured agent control for automation",
}

var ctlAgentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Control a ticket's exact recorded agent",
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
	controller, ok := muxBackend.(mux.AgentControlBackend)
	if !ok {
		return nil, mux.Target{}, "", ctlError(
			"unsupported_backend", "%s does not provide recognized agent prompt/wait semantics", muxBackend.Name(),
		)
	}
	return controller, target, t.State.Ticket, nil
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
