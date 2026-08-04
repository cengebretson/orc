package state

import (
	"fmt"
	"strings"
)

// Question kinds. A paused agent declares which shape of answer it needs so
// Orc can offer the matching control and reject anything else.
const (
	QuestionKindConfirm = "confirm"
	QuestionKindChoice  = "choice"
	QuestionKindText    = "text"
)

// Confirm answers are normalized to these two values, so a reader never has to
// know whether the human typed "y", "Yes", or "YES".
const (
	AnswerYes = "yes"
	AnswerNo  = "no"
)

// QuestionChoice is one selectable option. Key is what gets recorded and what
// the agent reads; Label is what a human sees.
type QuestionChoice struct {
	Key   string `yaml:"key"`
	Label string `yaml:"label,omitempty"`
}

// QuestionRuntime is a structured question a paused agent is waiting on, and
// the answer once a human gives one.
//
// It lives in runtime because it describes work in flight rather than workflow
// state — the same reason runtime.jit does. The answer is recorded here rather
// than only delivered to a terminal: an answer typed into a pane is lost when
// that process dies, and the next agent to pick the ticket up would ask again.
type QuestionRuntime struct {
	Kind       string           `yaml:"kind"`
	Prompt     string           `yaml:"prompt"`
	Choices    []QuestionChoice `yaml:"choices,omitempty"`
	AskedAt    string           `yaml:"asked_at"`
	Answer     string           `yaml:"answer,omitempty"`
	AnsweredAt string           `yaml:"answered_at,omitempty"`
}

// Answered reports whether a human has responded yet.
func (q *QuestionRuntime) Answered() bool {
	return q != nil && q.AnsweredAt != ""
}

// Validate checks that a question is well formed before it is recorded. A
// malformed question would leave a ticket paused with a control nobody can
// satisfy, so this runs at ask time rather than at answer time.
func (q *QuestionRuntime) Validate() error {
	if q == nil {
		return fmt.Errorf("question is required")
	}
	if strings.TrimSpace(q.Prompt) == "" {
		return fmt.Errorf("question prompt is required")
	}
	switch q.Kind {
	case QuestionKindConfirm, QuestionKindText:
		if len(q.Choices) > 0 {
			return fmt.Errorf("%s questions do not take choices", q.Kind)
		}
	case QuestionKindChoice:
		if len(q.Choices) < 2 {
			return fmt.Errorf("choice questions need at least two choices")
		}
		seen := make(map[string]bool, len(q.Choices))
		for _, choice := range q.Choices {
			key := strings.TrimSpace(choice.Key)
			if key == "" {
				return fmt.Errorf("every choice needs a key")
			}
			if seen[strings.ToLower(key)] {
				return fmt.Errorf("duplicate choice key %q", key)
			}
			seen[strings.ToLower(key)] = true
		}
	default:
		return fmt.Errorf("unknown question kind %q — use %s, %s, or %s",
			q.Kind, QuestionKindConfirm, QuestionKindChoice, QuestionKindText)
	}
	return nil
}

// NormalizeAnswer validates a raw human answer against the question's declared
// shape and returns the value to record.
//
// Orc checks only that the answer is one the question offered. It never
// interprets what an option means — that stays with the agent that asked.
func (q *QuestionRuntime) NormalizeAnswer(raw string) (string, error) {
	if q == nil {
		return "", fmt.Errorf("no question is pending")
	}
	value := strings.TrimSpace(raw)
	switch q.Kind {
	case QuestionKindConfirm:
		switch strings.ToLower(value) {
		case "y", "yes", "true":
			return AnswerYes, nil
		case "n", "no", "false":
			return AnswerNo, nil
		default:
			return "", fmt.Errorf("answer %q is not valid for a confirm question — use yes or no", raw)
		}
	case QuestionKindChoice:
		for _, choice := range q.Choices {
			if strings.EqualFold(strings.TrimSpace(choice.Key), value) {
				return choice.Key, nil
			}
		}
		return "", fmt.Errorf("answer %q is not one of the offered choices (%s)", raw, strings.Join(q.ChoiceKeys(), ", "))
	case QuestionKindText:
		if value == "" {
			return "", fmt.Errorf("answer text is required")
		}
		return value, nil
	default:
		return "", fmt.Errorf("unknown question kind %q", q.Kind)
	}
}

// ChoiceKeys lists the offered keys in order, for error messages and controls.
func (q *QuestionRuntime) ChoiceKeys() []string {
	if q == nil {
		return nil
	}
	keys := make([]string, 0, len(q.Choices))
	for _, choice := range q.Choices {
		keys = append(keys, choice.Key)
	}
	return keys
}

// Label returns the human-facing label for an answer value, falling back to the
// value itself for confirm and text answers.
func (q *QuestionRuntime) Label(value string) string {
	if q == nil {
		return value
	}
	for _, choice := range q.Choices {
		if choice.Key == value && strings.TrimSpace(choice.Label) != "" {
			return choice.Label
		}
	}
	return value
}

// AnswerQuestion validates a human answer against the pending question and
// records it durably. The recorded answer is what survives; delivering it to a
// live agent is a separate, best-effort step.
func AnswerQuestion(featureDir, raw string) (*QuestionRuntime, error) {
	var answered *QuestionRuntime
	err := Update(featureDir, func(s *State) error {
		if s.Runtime.Question == nil {
			return fmt.Errorf("no question is pending")
		}
		value, err := s.Runtime.Question.NormalizeAnswer(raw)
		if err != nil {
			return err
		}
		s.Runtime.Question.Answer = value
		s.Runtime.Question.AnsweredAt = timeNow()
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: "human",
			Result: "answered — " + s.Runtime.Question.Label(value),
		})
		copied := *s.Runtime.Question
		answered = &copied
		return nil
	})
	if err != nil {
		return nil, err
	}
	return answered, nil
}

// ClearQuestion removes runtime.question once an agent has consumed the answer.
func ClearQuestion(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.Question = nil
		return nil
	})
}
