package watch

import (
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func answerFeature(t *testing.T, question *state.QuestionRuntime) string {
	t.Helper()
	dir := t.TempDir()
	if err := state.Create(dir, &state.State{
		Ticket: "ORC-1", Slug: "orc-1", Status: "active",
		Stage: state.Stage{Name: "develop", Worker: "default:bob"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := state.Pause(dir, question.Prompt, question); err != nil {
		t.Fatal(err)
	}
	return dir
}

func answeringModel(t *testing.T, question *state.QuestionRuntime) Model {
	t.Helper()
	dir := answerFeature(t, question)
	s, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	r := row{ticket: "ORC-1", featureDir: dir, status: "paused", question: s.Runtime.Question}
	// Build the box the way New does; a zero-value textinput panics on Focus.
	box := textinput.New()
	m := Model{width: 80, rows: []row{r}, allRows: []row{r}, promptBox: box}
	if msg := m.beginAnswer(r); msg != "" {
		t.Fatalf("beginAnswer: %s", msg)
	}
	return m
}

func keyPress(s string) tea.KeyMsg {
	if s == "enter" {
		return tea.KeyMsg{Type: tea.KeyEnter}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func recordedAnswer(t *testing.T, featureDir string) *state.QuestionRuntime {
	t.Helper()
	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatal(err)
	}
	return s.Runtime.Question
}

func TestAnsweringConfirmRecordsOnSingleKey(t *testing.T) {
	m := answeringModel(t, &state.QuestionRuntime{Kind: state.QuestionKindConfirm, Prompt: "Delete it?"})
	dir := m.answerRow.featureDir

	updated, _ := m.updateAnswering(keyPress("y"))
	m = watchModel(t, updated)
	if m.answering {
		t.Fatal("confirm answer should close the control")
	}
	if got := recordedAnswer(t, dir); got.Answer != state.AnswerYes {
		t.Fatalf("recorded answer = %q, want yes", got.Answer)
	}
}

func TestAnsweringChoiceMovesAndSelects(t *testing.T) {
	m := answeringModel(t, &state.QuestionRuntime{
		Kind: state.QuestionKindChoice, Prompt: "Which?",
		Choices: []state.QuestionChoice{{Key: "a", Label: "First"}, {Key: "b", Label: "Second"}},
	})
	dir := m.answerRow.featureDir

	updated, _ := m.updateAnswering(keyPress("j"))
	m = watchModel(t, updated)
	if m.answerCursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.answerCursor)
	}
	updated, _ = m.updateAnswering(keyPress("enter"))
	m = watchModel(t, updated)
	if got := recordedAnswer(t, dir); got.Answer != "b" {
		t.Fatalf("recorded answer = %q, want b", got.Answer)
	}
}

// Typing a choice key selects it directly, so a two-option question does not
// require arrowing to an obvious answer.
func TestAnsweringChoiceAcceptsDirectKey(t *testing.T) {
	m := answeringModel(t, &state.QuestionRuntime{
		Kind: state.QuestionKindChoice, Prompt: "Which?",
		Choices: []state.QuestionChoice{{Key: "a"}, {Key: "b"}},
	})
	dir := m.answerRow.featureDir

	updated, _ := m.updateAnswering(keyPress("b"))
	m = watchModel(t, updated)
	if got := recordedAnswer(t, dir); got.Answer != "b" {
		t.Fatalf("recorded answer = %q, want b", got.Answer)
	}
}

// The cursor must not run off either end of the list.
func TestAnsweringChoiceCursorStaysInRange(t *testing.T) {
	m := answeringModel(t, &state.QuestionRuntime{
		Kind: state.QuestionKindChoice, Prompt: "Which?",
		Choices: []state.QuestionChoice{{Key: "a"}, {Key: "b"}},
	})
	for range 5 {
		updated, _ := m.updateAnswering(keyPress("j"))
		m = watchModel(t, updated)
	}
	if m.answerCursor != 1 {
		t.Fatalf("cursor ran past the end: %d", m.answerCursor)
	}
	for range 5 {
		updated, _ := m.updateAnswering(keyPress("k"))
		m = watchModel(t, updated)
	}
	if m.answerCursor != 0 {
		t.Fatalf("cursor ran before the start: %d", m.answerCursor)
	}
}

func TestAnsweringEscapeLeavesQuestionPending(t *testing.T) {
	m := answeringModel(t, &state.QuestionRuntime{Kind: state.QuestionKindConfirm, Prompt: "Delete it?"})
	dir := m.answerRow.featureDir

	updated, _ := m.updateAnswering(tea.KeyMsg{Type: tea.KeyEsc})
	m = watchModel(t, updated)
	if m.answering {
		t.Fatal("escape should close the control")
	}
	if recordedAnswer(t, dir).Answered() {
		t.Fatal("escape must not record an answer")
	}
}

// The dashboard shell must not steal keys while the control is open, or typing
// an answer would switch tabs instead.
func TestAnsweringBlocksSectionSwitching(t *testing.T) {
	m := answeringModel(t, &state.QuestionRuntime{Kind: state.QuestionKindText, Prompt: "Which ticket?"})
	if m.CanSwitchSection() {
		t.Fatal("section switching must be blocked while answering")
	}
}

func TestRenderAnswerActionShowsQuestionAndChoices(t *testing.T) {
	m := answeringModel(t, &state.QuestionRuntime{
		Kind: state.QuestionKindChoice, Prompt: "Which parser approach?",
		Choices: []state.QuestionChoice{{Key: "a", Label: "Rewrite"}, {Key: "b", Label: "Patch"}},
	})
	view := m.renderAnswerAction()
	for _, want := range []string{"ANSWER", "ORC-1", "Which parser approach?", "Rewrite", "Patch", "enter select"} {
		if !strings.Contains(view, want) {
			t.Fatalf("answer panel missing %q:\n%s", want, view)
		}
	}
}
