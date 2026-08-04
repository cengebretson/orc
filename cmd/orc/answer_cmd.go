package main

import (
	"fmt"
	"strings"

	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/spf13/cobra"
)

var answerCmd = &cobra.Command{
	Use:   "answer <ticket> [value]",
	Short: "Answer the question a paused ticket is waiting on",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAnswer,
}

func runAnswer(_ *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}
	question := t.State.Runtime.Question
	if question == nil {
		return fmt.Errorf("%s is not waiting on a question", t.State.Ticket)
	}

	// With no value, show what is being asked rather than guessing at an answer.
	if len(args) == 1 {
		printQuestion(t.State.Ticket, question)
		return nil
	}

	answered, err := state.AnswerQuestion(t.FeatureDir, strings.Join(args[1:], " "))
	if err != nil {
		return err
	}
	fmt.Printf("Ticket:  %s\n", t.State.Ticket)
	fmt.Printf("Answer:  %s\n", answered.Label(answered.Answer))

	// The answer is already durable. Nudging the live agent is best effort: it
	// may have exited, been replaced, or never have had hooks installed, none of
	// which should fail the command or lose the recorded answer.
	if note := nudgeAnsweredAgent(t.State, answered); note != "" {
		fmt.Printf("Agent:   %s\n", note)
	}
	fmt.Printf("\nRun `orc next %s` to continue.\n", t.State.Ticket)
	return nil
}

func printQuestion(ticketID string, question *state.QuestionRuntime) {
	fmt.Printf("Ticket:  %s\n", ticketID)
	fmt.Printf("Asks:    %s\n", question.Prompt)
	switch question.Kind {
	case state.QuestionKindConfirm:
		fmt.Printf("Answer:  yes or no\n")
	case state.QuestionKindChoice:
		fmt.Printf("Answer:  one of\n")
		for _, choice := range question.Choices {
			if choice.Label != "" {
				fmt.Printf("           %-10s %s\n", choice.Key, choice.Label)
			} else {
				fmt.Printf("           %s\n", choice.Key)
			}
		}
	case state.QuestionKindText:
		fmt.Printf("Answer:  free text\n")
	}
	if question.Answered() {
		fmt.Printf("\nAlready answered %s: %s\n", question.AnsweredAt, question.Label(question.Answer))
		return
	}
	fmt.Printf("\nAnswer with `orc answer %s <value>`.\n", ticketID)
}

// nudgeAnsweredAgent tells a live agent an answer is waiting, and returns a
// short note about what happened. It never returns an error: the durable record
// is the answer, and this is only a prod to make the agent look at it.
func nudgeAnsweredAgent(s *state.State, question *state.QuestionRuntime) string {
	target, ok := s.Runtime.MuxTarget(s.Stage.Name)
	if !ok || s.Runtime.Agent == nil {
		return ""
	}
	if err := selectMuxForState(s); err != nil {
		return "not notified: " + err.Error()
	}
	controller, ok := muxBackend.(mux.AgentPromptBackend)
	if !ok || controller.Name() != target.Backend {
		return "not notified: " + target.Backend + " does not support prompting"
	}
	text := fmt.Sprintf("The human answered %q: %s. Read runtime.question in STATE.yaml and continue.",
		question.Prompt, question.Label(question.Answer))
	_, err := controller.PromptAgent(mux.Target{
		Backend: target.Backend, Workspace: target.Workspace, Tab: target.Tab, Pane: target.Pane,
		AgentID: s.Runtime.Agent.ID, AgentInstance: s.Runtime.Agent.Instance,
	}, text, false, mux.AgentControlOptions{})
	if err != nil {
		return "not notified: " + err.Error()
	}
	return "notified"
}
