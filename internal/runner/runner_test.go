package runner_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
)

func fixtureWorkspace() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "workspace")
}

func fixtureFeatureDir(ws, ticket string) string {
	entries, _ := filepath.Glob(filepath.Join(ws, "features", ticket+"*"))
	if len(entries) == 0 {
		return ""
	}
	return entries[0]
}

func TestCompute_ResolvesWorkerFromConfig(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	plan, err := runner.Compute(ws, featureDir, "")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if plan.Worker == nil {
		t.Fatal("expected a worker, got nil")
	}
	// STORY-123 has stage.worker set, so it resolves via the state-assigned worker.
	if plan.WorkerReason != "stage owner" && plan.WorkerReason != "workflow default" {
		t.Errorf("WorkerReason = %q, want stage owner or workflow default", plan.WorkerReason)
	}
}

func TestCompute_FlagOverrideTakesPriority(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	plan, err := runner.Compute(ws, featureDir, "default:fred")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if plan.Worker.ID != "default:fred" {
		t.Errorf("Worker.ID = %q, want default:fred", plan.Worker.ID)
	}
	if plan.WorkerReason != "flag override" {
		t.Errorf("WorkerReason = %q, want \"flag override\"", plan.WorkerReason)
	}
}

func TestCompute_UnknownFlagOverrideReturnsError(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	_, err := runner.Compute(ws, featureDir, "no-such-worker")
	if err == nil {
		t.Fatal("expected error for unknown --worker flag, got nil")
	}
	if !strings.Contains(err.Error(), "no-such-worker") {
		t.Errorf("error %q does not mention the unknown worker id", err.Error())
	}
}

func TestCompute_PromptContainsPreamble(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	plan, err := runner.Compute(ws, featureDir, "")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if !strings.Contains(plan.Prompt, "orc mark") {
		t.Errorf("prompt missing preamble orc start instruction")
	}
	if !strings.Contains(plan.Prompt, "AGENTS.md") {
		t.Errorf("prompt missing AGENTS.md reference")
	}
}

func TestCompute_PromptContainsEndInstruction(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	plan, err := runner.Compute(ws, featureDir, "")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	// STORY-123 is at develop stage which has a next stage, so end instruction must be present
	if plan.EndInstruction == "" {
		t.Error("expected end instruction for non-final stage, got empty")
	}
	if !strings.Contains(plan.Prompt, plan.EndInstruction) {
		t.Error("end instruction not appended to prompt")
	}
}

func TestCompute_LaunchCommandNonEmpty(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	plan, err := runner.Compute(ws, featureDir, "")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if plan.LaunchCommand == "" {
		t.Error("LaunchCommand is empty")
	}
	if len(plan.LaunchArgv) == 0 {
		t.Error("LaunchArgv is empty")
	}
}

func TestCompute_WorkflowAndStagePopulated(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	s, _ := state.Load(featureDir)
	plan, err := runner.Compute(ws, featureDir, "")
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}

	if plan.Stage != s.Stage.Name {
		t.Errorf("Stage = %q, want %q", plan.Stage, s.Stage.Name)
	}
	if plan.Workflow == "" {
		t.Error("Workflow is empty")
	}
	if plan.Ticket != s.Ticket {
		t.Errorf("Ticket = %q, want %q", plan.Ticket, s.Ticket)
	}
}

func TestCompute_NextActionResolutionCases(t *testing.T) {
	root := writeRunnerWorkspace(t)

	tests := []struct {
		name          string
		stateYAML     string
		override      string
		wantWorker    string
		wantReason    string
		wantWorkflow  string
		wantStage     string
		wantEndParts  []string
		rejectEndPart string
	}{
		{
			name: "default workflow and automatic advance",
			stateYAML: `
ticket: TICKET-1
slug: TICKET-1
status: pending
stage:
  name: intake
next_action:
  prompt: Start intake.
  cwd: .
`,
			wantWorker:   "default:fred",
			wantReason:   "workflow default",
			wantWorkflow: "default",
			wantStage:    "intake",
			wantEndParts: []string{"orc mark TICKET-1 next --result", "When this stage is complete"},
		},
		{
			name: "explicit worker override wins",
			stateYAML: `
ticket: TICKET-2
slug: TICKET-2
status: pending
stage:
  name: intake
next_action:
  prompt: Start intake.
  cwd: .
`,
			override:     "default:brian",
			wantWorker:   "default:brian",
			wantReason:   "flag override",
			wantWorkflow: "default",
			wantStage:    "intake",
			wantEndParts: []string{"orc mark TICKET-2 next --result"},
		},
		{
			name: "stage worker beats workflow default",
			stateYAML: `
ticket: TICKET-3
slug: TICKET-3
status: active
stage:
  worker: default:fred
  name: develop
next_action:
  prompt: Continue development.
  cwd: .
`,
			wantWorker:   "default:fred",
			wantReason:   "stage owner",
			wantWorkflow: "default",
			wantStage:    "develop",
			wantEndParts: []string{"orc mark TICKET-3 pause", "--stage code-review"},
		},
		{
			name: "manual stage with loop gives branch instructions",
			stateYAML: `
ticket: TICKET-4
slug: TICKET-4
status: active
stage:
  name: develop
next_action:
  prompt: Continue development.
  cwd: .
`,
			wantWorker:   "default:bob",
			wantReason:   "workflow default",
			wantWorkflow: "default",
			wantStage:    "develop",
			wantEndParts: []string{"orc mark TICKET-4 pause", "--stage code-review", "enter code-review loop"},
		},
		{
			name: "loop stage returns to owner without branching",
			stateYAML: `
ticket: TICKET-5
slug: TICKET-5
status: active
stage:
  name: code-review
next_action:
  prompt: Review the work.
  cwd: .
`,
			wantWorker:    "default:zach",
			wantReason:    "workflow default",
			wantWorkflow:  "default",
			wantStage:     "code-review",
			wantEndParts:  []string{"orc mark TICKET-5 next --result", "When your work is complete"},
			rejectEndPart: "--stage code-review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			featureDir := writeRunnerFeature(t, root, tt.stateYAML)
			plan, err := runner.Compute(root, featureDir, tt.override)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			if plan.Worker.ID != tt.wantWorker {
				t.Errorf("Worker.ID = %q, want %q", plan.Worker.ID, tt.wantWorker)
			}
			if plan.WorkerReason != tt.wantReason {
				t.Errorf("WorkerReason = %q, want %q", plan.WorkerReason, tt.wantReason)
			}
			if plan.Workflow != tt.wantWorkflow {
				t.Errorf("Workflow = %q, want %q", plan.Workflow, tt.wantWorkflow)
			}
			if plan.Stage != tt.wantStage {
				t.Errorf("Stage = %q, want %q", plan.Stage, tt.wantStage)
			}
			for _, part := range tt.wantEndParts {
				if !strings.Contains(plan.EndInstruction, part) {
					t.Errorf("EndInstruction missing %q:\n%s", part, plan.EndInstruction)
				}
			}
			if tt.rejectEndPart != "" && strings.Contains(plan.EndInstruction, tt.rejectEndPart) {
				t.Errorf("EndInstruction contains rejected %q:\n%s", tt.rejectEndPart, plan.EndInstruction)
			}
		})
	}
}

func writeRunnerWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeRunnerFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: intake
        worker: default:fred
        advance: auto
      - name: develop
        worker: default:bob
        advance: manual
        loop:
          via: code-review
          worker: default:zach
          max: 3
          on_max: pause
      - name: qa-automation
        worker: default:brian
        advance: auto
`)
	for _, id := range []string{"default:bob", "default:fred", "default:zach", "default:brian"} {
		writeRunnerFile(t, filepath.Join(root, "workers", strings.ReplaceAll(id, ":", "/")+".md"), `---
id: `+id+`
name: `+id+`
engine: claude
---
`)
	}
	return root
}

func writeRunnerFeature(t *testing.T, root, stateYAML string) string {
	t.Helper()
	featureDir := filepath.Join(root, "features", strings.ReplaceAll(t.Name(), "/", "-"))
	writeRunnerFile(t, filepath.Join(featureDir, "STATE.yaml"), stateYAML)
	return featureDir
}

func writeRunnerFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
