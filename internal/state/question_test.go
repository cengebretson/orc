package state_test

import (
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
)

func questionFeature(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := state.Create(dir, &state.State{
		Ticket: "ORC-1", Slug: "orc-1", Status: "active",
		Stage: state.Stage{Name: "develop", Worker: "default:bob"},
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPauseRecordsChoiceQuestion(t *testing.T) {
	dir := questionFeature(t)
	question := &state.QuestionRuntime{
		Kind:   state.QuestionKindChoice,
		Prompt: "Which approach?",
		Choices: []state.QuestionChoice{
			{Key: "a", Label: "Rewrite the parser"},
			{Key: "b", Label: "Patch in place"},
		},
	}
	if err := state.Pause(dir, "Which approach?", question); err != nil {
		t.Fatal(err)
	}

	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != "paused" {
		t.Fatalf("status = %q, want paused", s.Status)
	}
	got := s.Runtime.Question
	if got == nil || got.Kind != state.QuestionKindChoice || len(got.Choices) != 2 {
		t.Fatalf("question = %#v", got)
	}
	if got.AskedAt == "" {
		t.Fatal("AskedAt was not stamped")
	}
	if got.Answered() {
		t.Fatal("a freshly asked question must not read as answered")
	}
}

// A question Orc cannot offer a control for would strand the ticket, so a
// malformed one is refused at ask time rather than at answer time.
func TestPauseRejectsMalformedQuestions(t *testing.T) {
	for _, test := range []struct {
		name     string
		question state.QuestionRuntime
		wantErr  string
	}{
		{"unknown kind", state.QuestionRuntime{Kind: "poll", Prompt: "?"}, "unknown question kind"},
		{"no prompt", state.QuestionRuntime{Kind: state.QuestionKindText}, "prompt is required"},
		{"one choice", state.QuestionRuntime{Kind: state.QuestionKindChoice, Prompt: "?",
			Choices: []state.QuestionChoice{{Key: "a"}}}, "at least two"},
		{"duplicate keys", state.QuestionRuntime{Kind: state.QuestionKindChoice, Prompt: "?",
			Choices: []state.QuestionChoice{{Key: "a"}, {Key: "A"}}}, "duplicate choice key"},
		{"confirm with choices", state.QuestionRuntime{Kind: state.QuestionKindConfirm, Prompt: "?",
			Choices: []state.QuestionChoice{{Key: "a"}, {Key: "b"}}}, "do not take choices"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := questionFeature(t)
			err := state.Pause(dir, "reason", &test.question)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("Pause error = %v, want %q", err, test.wantErr)
			}
			s, loadErr := state.Load(dir)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if s.Status == "paused" || s.Runtime.Question != nil {
				t.Fatal("a rejected question must not pause the ticket or be recorded")
			}
		})
	}
}

func TestAnswerQuestionNormalizesAndRecords(t *testing.T) {
	for _, test := range []struct {
		name     string
		question state.QuestionRuntime
		raw      string
		want     string
	}{
		{"confirm y", state.QuestionRuntime{Kind: state.QuestionKindConfirm, Prompt: "Delete it?"}, "y", state.AnswerYes},
		{"confirm YES", state.QuestionRuntime{Kind: state.QuestionKindConfirm, Prompt: "Delete it?"}, "YES", state.AnswerYes},
		{"confirm no", state.QuestionRuntime{Kind: state.QuestionKindConfirm, Prompt: "Delete it?"}, "n", state.AnswerNo},
		{"choice by key", state.QuestionRuntime{Kind: state.QuestionKindChoice, Prompt: "Which?",
			Choices: []state.QuestionChoice{{Key: "a"}, {Key: "b"}}}, "B", "b"},
		{"text", state.QuestionRuntime{Kind: state.QuestionKindText, Prompt: "Which ticket?"}, "  PROJ-9  ", "PROJ-9"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := questionFeature(t)
			if err := state.Pause(dir, test.question.Prompt, &test.question); err != nil {
				t.Fatal(err)
			}
			answered, err := state.AnswerQuestion(dir, test.raw)
			if err != nil {
				t.Fatal(err)
			}
			if answered.Answer != test.want {
				t.Fatalf("answer = %q, want %q", answered.Answer, test.want)
			}
			s, err := state.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if s.Runtime.Question.Answer != test.want || !s.Runtime.Question.Answered() {
				t.Fatalf("persisted question = %#v", s.Runtime.Question)
			}
			last := s.History[len(s.History)-1]
			if last.Worker != "human" || !strings.HasPrefix(last.Result, "answered") {
				t.Fatalf("history entry = %#v", last)
			}
		})
	}
}

// Orc's job is to guarantee the answer is one the question offered. Anything
// else is refused, and refusing must leave the question still pending.
func TestAnswerQuestionRejectsUnofferedAnswers(t *testing.T) {
	dir := questionFeature(t)
	question := &state.QuestionRuntime{
		Kind: state.QuestionKindChoice, Prompt: "Which?",
		Choices: []state.QuestionChoice{{Key: "a"}, {Key: "b"}},
	}
	if err := state.Pause(dir, "Which?", question); err != nil {
		t.Fatal(err)
	}
	if _, err := state.AnswerQuestion(dir, "c"); err == nil || !strings.Contains(err.Error(), "not one of the offered choices") {
		t.Fatalf("AnswerQuestion error = %v", err)
	}
	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Runtime.Question.Answered() {
		t.Fatal("a rejected answer must leave the question pending")
	}
}

func TestAnswerQuestionRequiresAPendingQuestion(t *testing.T) {
	dir := questionFeature(t)
	if _, err := state.AnswerQuestion(dir, "yes"); err == nil || !strings.Contains(err.Error(), "no question is pending") {
		t.Fatalf("AnswerQuestion error = %v", err)
	}
}

// Resume clears the question the same way it clears NextAction: by then the
// launch prompt has already carried the answer to the agent.
func TestResumeClearsTheQuestion(t *testing.T) {
	dir := questionFeature(t)
	question := &state.QuestionRuntime{Kind: state.QuestionKindConfirm, Prompt: "Go ahead?"}
	if err := state.Pause(dir, "Go ahead?", question); err != nil {
		t.Fatal(err)
	}
	if _, err := state.AnswerQuestion(dir, "yes"); err != nil {
		t.Fatal(err)
	}
	if err := state.Resume(dir); err != nil {
		t.Fatal(err)
	}
	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Runtime.Question != nil {
		t.Fatalf("question survived resume: %#v", s.Runtime.Question)
	}
}

// The answer has to outlive the process that was asked, which is the whole
// reason it is recorded rather than only typed into a pane.
func TestAnsweredQuestionSurvivesReload(t *testing.T) {
	dir := questionFeature(t)
	question := &state.QuestionRuntime{
		Kind: state.QuestionKindChoice, Prompt: "Which?",
		Choices: []state.QuestionChoice{{Key: "a", Label: "First"}, {Key: "b", Label: "Second"}},
	}
	if err := state.Pause(dir, "Which?", question); err != nil {
		t.Fatal(err)
	}
	if _, err := state.AnswerQuestion(dir, "b"); err != nil {
		t.Fatal(err)
	}
	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Runtime.Question.Label(s.Runtime.Question.Answer); got != "Second" {
		t.Fatalf("label = %q, want Second", got)
	}
}
