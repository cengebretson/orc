package resume_test

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/resume"
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

func TestBuild_PopulatesContext(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	ctx, err := resume.Build(ws, featureDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if ctx.Ticket == "" {
		t.Error("Ticket is empty")
	}
	if ctx.Stage == "" {
		t.Error("Stage is empty")
	}
	if ctx.Status == "" {
		t.Error("Status is empty")
	}
}

func TestBuild_PromptUsesOrcMark(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	ctx, err := resume.Build(ws, featureDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if !strings.Contains(ctx.Prompt, "orc mark") {
		t.Error("prompt does not contain 'orc mark'")
	}
	if strings.Contains(ctx.Prompt, "orc start") {
		t.Error("prompt contains stale 'orc start' command")
	}
	if strings.Contains(ctx.Prompt, "orc advance") {
		t.Error("prompt contains stale 'orc advance' command")
	}
	if strings.Contains(ctx.Prompt, "orc wait ") {
		t.Error("prompt contains stale 'orc wait' command")
	}
	if strings.Contains(ctx.Prompt, "orc block ") {
		t.Error("prompt contains stale 'orc block' command")
	}
}

func TestBuild_PromptContainsAllEndInstructions(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	ctx, err := resume.Build(ws, featureDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	for _, want := range []string{"next", "pause"} {
		if !strings.Contains(ctx.Prompt, want) {
			t.Errorf("prompt missing end instruction %q", want)
		}
	}
}

func TestBuild_PromptContainsTicket(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	ctx, err := resume.Build(ws, featureDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if !strings.Contains(ctx.Prompt, ctx.Ticket) {
		t.Errorf("prompt does not contain ticket ID %q", ctx.Ticket)
	}
}

func TestBuild_PromptUsesNamespacedStagePath(t *testing.T) {
	ws := fixtureWorkspace()
	featureDir := fixtureFeatureDir(ws, "STORY-123")
	if featureDir == "" {
		t.Fatal("fixture STORY-123 not found")
	}

	ctx, err := resume.Build(ws, featureDir)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if !strings.Contains(ctx.Prompt, "`stages/default/develop.md`") {
		t.Errorf("prompt missing namespaced stage path:\n%s", ctx.Prompt)
	}
	if strings.Contains(ctx.Prompt, "`stages/default:develop.md`") {
		t.Errorf("prompt contains unresolved stage ID path:\n%s", ctx.Prompt)
	}
}

func TestBuild_ErrorOnMissingFeatureDir(t *testing.T) {
	ws := fixtureWorkspace()
	_, err := resume.Build(ws, "/nonexistent/feature/dir")
	if err == nil {
		t.Error("expected error for missing feature dir, got nil")
	}
}
