package workspacectx_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/workspacectx"
)

func TestLoad(t *testing.T) {
	root := writeWorkspace(t, `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: intake
        worker: custom:fred
        advance: auto
`, []string{"custom:fred"})

	ctx, err := workspacectx.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if ctx.Config.DefaultWorkflow() != "default" {
		t.Fatalf("DefaultWorkflow = %q, want default", ctx.Config.DefaultWorkflow())
	}
	if len(ctx.Workers) != 1 || ctx.Workers[0].ID != "custom:fred" {
		t.Fatalf("Workers = %#v, want one custom:fred worker", ctx.Workers)
	}
	if len(ctx.WorkerIDs) != 1 || ctx.WorkerIDs[0] != "custom:fred" {
		t.Fatalf("WorkerIDs = %#v, want [custom:fred]", ctx.WorkerIDs)
	}
}

func TestLoadValidatedReturnsValidationErrors(t *testing.T) {
	root := writeWorkspace(t, `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: intake
        worker: missing
        advance: auto
`, []string{"custom:fred"})

	ctx, errs, err := workspacectx.LoadValidated(root)
	if err != nil {
		t.Fatalf("LoadValidated error = %v", err)
	}
	if ctx == nil {
		t.Fatal("ctx is nil")
	}
	if len(errs) == 0 {
		t.Fatal("expected validation errors, got none")
	}
	if errs[0].Path != "workflows.default.stages[0].worker" {
		t.Fatalf("first error path = %q", errs[0].Path)
	}
}

func TestLoadReturnsWorkerParseError(t *testing.T) {
	root := writeWorkspace(t, `
settings:
  default_workflow: default
workflows:
  default:
    stages:
      - name: intake
        worker: custom:fred
        advance: auto
`, nil)
	writeFile(t, filepath.Join(root, "workers", "custom", "broken.md"), "not frontmatter\n")

	_, err := workspacectx.Load(root)
	if err == nil {
		t.Fatal("expected worker parse error")
	}
	if !strings.Contains(err.Error(), "loading workers") {
		t.Fatalf("error = %v, want loading workers context", err)
	}
}

func TestLoadMissingConfigIsExplicit(t *testing.T) {
	root := t.TempDir()
	writeWorker(t, root, "custom:fred")

	ctx, err := workspacectx.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ctx.Config.Workflows) != 0 {
		t.Fatalf("Workflows = %#v, want empty for missing orc.yaml", ctx.Config.Workflows)
	}

	_, errs, err := workspacectx.LoadValidated(root)
	if err != nil {
		t.Fatalf("LoadValidated error = %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("validation errors = %#v, want none for empty config", errs)
	}
}

func TestWorkerIDs(t *testing.T) {
	root := writeWorkspace(t, ``, []string{"custom:bob", "custom:fred"})
	ctx, err := workspacectx.Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := workspacectx.WorkerIDs(ctx.Workers)
	if len(got) != 2 || got[0] != "custom:bob" || got[1] != "custom:fred" {
		t.Fatalf("WorkerIDs = %#v, want [custom:bob custom:fred]", got)
	}
}

func writeWorkspace(t *testing.T, orcYAML string, workerIDs []string) string {
	t.Helper()
	root := t.TempDir()
	if orcYAML != "" {
		writeFile(t, filepath.Join(root, "orc.yaml"), orcYAML)
	}
	for _, id := range workerIDs {
		writeWorker(t, root, id)
	}
	return root
}

func writeWorker(t *testing.T, root, id string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "workers", strings.ReplaceAll(id, ":", "/")+".md"), `---
id: `+id+`
name: `+id+`
engine: claude
---
`)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
