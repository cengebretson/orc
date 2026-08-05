package workspace_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspace"
)

func TestPromptSlug(t *testing.T) {
	tests := map[string]string{
		"Investigate the intermittent API timeout": "investigate-the-intermittent-api-timeout",
		"do this,,,,":                        "do-this",
		"  Keep   spaces_and punctuation!  ": "keep-spaces-and-punctuation",
		"🔥":                                  "",
		strings.Repeat("a", 60):              strings.Repeat("a", 48),
	}
	for input, want := range tests {
		if got := workspace.PromptSlug(input); got != want {
			t.Errorf("PromptSlug(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLocalRunCreatesSequentialFeature(t *testing.T) {
	root := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: root}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	first, err := workspace.LocalRun(workspace.LocalRunOptions{
		Root:        root,
		Instruction: "Investigate the intermittent API timeout",
		Worker:      "default:bob",
	})
	if err != nil {
		t.Fatalf("LocalRun first: %v", err)
	}
	if first.Ticket != "LOCAL-1" || first.Slug != "LOCAL-1-investigate-the-intermittent-api-timeout" {
		t.Fatalf("first result = %+v", first)
	}

	repoPath := filepath.Join(root, "projects", "api")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	second, err := workspace.LocalRun(workspace.LocalRunOptions{
		Root:        root,
		Instruction: "Check retry behavior",
		Slug:        "api retries",
		Worker:      "default:ada",
		RepoName:    "api",
		RepoPath:    repoPath,
	})
	if err != nil {
		t.Fatalf("LocalRun second: %v", err)
	}
	if second.Ticket != "LOCAL-2" || second.Slug != "LOCAL-2-api-retries" {
		t.Fatalf("second result = %+v", second)
	}

	s, err := state.Load(second.FeatureDir)
	if err != nil {
		t.Fatalf("Load state: %v", err)
	}
	if s.Workflow != workspace.LocalWorkflow || s.Stage.Name != "default:adhoc" || s.Stage.Worker != "default:ada" {
		t.Fatalf("state workflow/stage/worker = %q/%q/%q", s.Workflow, s.Stage.Name, s.Stage.Worker)
	}
	if s.Repos["api"].Main != repoPath || s.NextAction.CWD != repoPath {
		t.Fatalf("repository state = %+v, cwd = %q", s.Repos, s.NextAction.CWD)
	}
	if s.NextAction.Worker != "default:ada" || !strings.Contains(s.NextAction.Prompt, "Check retry behavior") {
		t.Fatalf("next action = %+v", s.NextAction)
	}
	if !strings.Contains(s.NextAction.Prompt, `orc mark LOCAL-2 done --result "<summary of what was done>"`) {
		t.Fatalf("next action prompt is missing the exact completion signal: %q", s.NextAction.Prompt)
	}
	if got := s.History[0].Result; got != "local feature created by orc run" {
		t.Fatalf("history result = %q", got)
	}
	ticketDoc, err := os.ReadFile(filepath.Join(second.FeatureDir, "TICKET.md"))
	if err != nil {
		t.Fatalf("Read TICKET.md: %v", err)
	}
	if !strings.Contains(string(ticketDoc), "Check retry behavior") || !strings.Contains(string(ticketDoc), "no external tracker ticket") {
		t.Fatalf("TICKET.md = %s", ticketDoc)
	}
}

func TestLocalRunSequenceIsSerialized(t *testing.T) {
	root := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: root}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	const count = 8
	results := make(chan string, count)
	errs := make(chan error, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := workspace.LocalRun(workspace.LocalRunOptions{
				Root:        root,
				Instruction: "Concurrent local task",
				Worker:      "default:bob",
			})
			if err != nil {
				errs <- err
				return
			}
			results <- result.Ticket
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("LocalRun: %v", err)
	}
	seen := map[string]bool{}
	for ticket := range results {
		if seen[ticket] {
			t.Fatalf("duplicate ticket %s", ticket)
		}
		seen[ticket] = true
	}
	if len(seen) != count {
		t.Fatalf("ticket count = %d, want %d", len(seen), count)
	}
}

func TestEnsureLocalWorkflowMigratesOlderWorkspace(t *testing.T) {
	root := t.TempDir()
	original := `settings:
  default_workflow: custom:standard
workflows:
  "custom:standard":
    stages:
      - name: custom:work
        worker: custom:bob
        advance: auto
`
	if err := os.WriteFile(filepath.Join(root, config.Filename), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := workspace.EnsureLocalWorkflow(root, "custom:bob")
	if err != nil {
		t.Fatalf("EnsureLocalWorkflow: %v", err)
	}
	if !changed {
		t.Fatal("EnsureLocalWorkflow changed = false, want true")
	}

	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.DefaultWorkflow() != "custom:standard" {
		t.Fatalf("default workflow = %q, want custom:standard", cfg.DefaultWorkflow())
	}
	if _, ok := cfg.Workflows["custom:standard"]; !ok {
		t.Fatal("existing workflow was removed")
	}
	local, ok := cfg.Workflows[workspace.LocalWorkflow]
	if !ok || len(local.Stages) != 1 || local.Stages[0].Worker != "custom:bob" {
		t.Fatalf("local workflow = %+v, present = %v", local, ok)
	}
	stagePath := filepath.Join(root, "stages", "default", "adhoc.md")
	if data, err := os.ReadFile(stagePath); err != nil || !strings.Contains(string(data), "orc mark") {
		t.Fatalf("local stage guide: data=%q err=%v", data, err)
	}

	changed, err = workspace.EnsureLocalWorkflow(root, "custom:other")
	if err != nil {
		t.Fatalf("EnsureLocalWorkflow second call: %v", err)
	}
	if changed {
		t.Fatal("EnsureLocalWorkflow second changed = true, want false")
	}
}
