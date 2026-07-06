package orchestrator

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
)

func fixtureWorkspace() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "workspace")
}

func copyFixtureWorkspace(t *testing.T) string {
	t.Helper()
	src := fixtureWorkspace()
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copy fixture workspace: %v", err)
	}
	return dst
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

func TestAdvanceMovesToNextStage(t *testing.T) {
	root := copyFixtureWorkspace(t)
	featureDir := filepath.Join(root, "features", "STORY-123-add-user-auth")
	clearRepoValidationFields(t, featureDir)
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Stage.Name = "default:intake"
		s.Stage.Worker = "default:fred"
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	result, err := Advance(AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
		Result:     "ready",
	})
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if result.Outcome != AdvanceOutcomeAdvanced {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, AdvanceOutcomeAdvanced)
	}
	if result.Previous != "default:intake" || result.Next != "default:develop" {
		t.Fatalf("transition = %s -> %s, want default:intake -> default:develop", result.Previous, result.Next)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Status != "pending" || s.Stage.Name != "default:develop" {
		t.Fatalf("state status/stage = %s/%s, want pending/default:develop", s.Status, s.Stage.Name)
	}
}

func TestAdvanceRejectsManualGate(t *testing.T) {
	root := copyFixtureWorkspace(t)
	featureDir := filepath.Join(root, "features", "STORY-123-add-user-auth")
	clearRepoValidationFields(t, featureDir)

	_, err := Advance(AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
	})
	if err == nil {
		t.Fatal("expected manual gate error")
	}
}

func TestAdvanceRejectsPendingTicket(t *testing.T) {
	root := copyFixtureWorkspace(t)
	featureDir := filepath.Join(root, "features", "STORY-123-add-user-auth")
	clearRepoValidationFields(t, featureDir)
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "pending"
		s.Stage.Name = "intake"
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	_, err := Advance(AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
	})
	if err == nil {
		t.Fatal("expected pending status error")
	}
}

func TestAdvanceBlocksWhenArtifactPolicyBlock(t *testing.T) {
	root := copyFixtureWorkspace(t)
	featureDir := filepath.Join(root, "features", "STORY-123-add-user-auth")
	clearRepoValidationFields(t, featureDir)
	enableArtifactPolicyBlock(t, root)
	if err := os.Remove(filepath.Join(featureDir, "PLAN.md")); err != nil {
		t.Fatalf("remove PLAN.md: %v", err)
	}
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Stage.Name = "default:intake"
		s.Stage.Worker = "default:fred"
		s.Status = "active"
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	_, err := Advance(AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
		Result:     "ready",
	})
	if err == nil {
		t.Fatal("expected artifact policy error")
	}
	if !strings.Contains(err.Error(), "artifact_policy=block") || !strings.Contains(err.Error(), "PLAN.md missing") {
		t.Fatalf("error = %v", err)
	}
}

// A manual stage must steer the agent to `orc mark pause` even when the block
// policy would also reject the advance — pause is the escape valve.
func TestAdvanceManualGateTakesPrecedenceOverArtifactPolicy(t *testing.T) {
	root := copyFixtureWorkspace(t)
	featureDir := filepath.Join(root, "features", "STORY-123-add-user-auth")
	clearRepoValidationFields(t, featureDir)
	enableArtifactPolicyBlock(t, root)
	if err := os.Remove(filepath.Join(featureDir, "PLAN.md")); err != nil {
		t.Fatalf("remove PLAN.md: %v", err)
	}
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Stage.Name = "default:develop"
		s.Stage.Worker = "default:bob"
		s.Status = "active"
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	_, err := Advance(AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
	})
	if err == nil {
		t.Fatal("expected manual gate error")
	}
	if !strings.Contains(err.Error(), "advance: manual") {
		t.Fatalf("error = %v, want manual gate guidance", err)
	}
	if strings.Contains(err.Error(), "artifact_policy") {
		t.Fatalf("error = %v, want manual gate to take precedence over artifact policy", err)
	}
}

func TestAdvanceRejectsUnknownWorkerOverride(t *testing.T) {
	root := copyFixtureWorkspace(t)
	featureDir := filepath.Join(root, "features", "STORY-123-add-user-auth")
	clearRepoValidationFields(t, featureDir)
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Stage.Name = "intake"
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	_, err := Advance(AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
		Worker:     "missing-worker",
	})
	if err == nil {
		t.Fatal("expected unknown worker error")
	}
}

func TestAdvancePausesWhenLoopLimitReached(t *testing.T) {
	root := copyFixtureWorkspace(t)
	featureDir := filepath.Join(root, "features", "STORY-123-add-user-auth")
	clearRepoValidationFields(t, featureDir)
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "active"
		s.Stage.Name = "default:develop"
		s.StageCounts = map[string]int{"default:code-review": 3}
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	result, err := Advance(AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
		Stage:      "default:code-review",
	})
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if result.Outcome != AdvanceOutcomePaused {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, AdvanceOutcomePaused)
	}
	if result.Reason != "loop limit reached (3/3 for default:code-review)" {
		t.Fatalf("Reason = %q", result.Reason)
	}

	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Status != "paused" {
		t.Fatalf("status = %q, want paused", s.Status)
	}
}

func enableArtifactPolicyBlock(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "orc.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read orc.yaml: %v", err)
	}
	updated := strings.Replace(string(data), "\n  artifact_policy: warn", "\n  artifact_policy: block", 1)
	if updated == string(data) {
		t.Fatal("artifact_policy setting not found")
	}
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatalf("write orc.yaml: %v", err)
	}
}

func clearRepoValidationFields(t *testing.T, featureDir string) {
	t.Helper()
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Repos = nil
		s.NextAction.CWD = ""
		return nil
	}); err != nil {
		t.Fatalf("clear repo validation fields: %v", err)
	}
}
