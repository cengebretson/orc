package workspace_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/workspace"
)

func TestInspectPackValidLocalPack(t *testing.T) {
	dir := writeLocalPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
engines:
  - codex
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
      description: Fast hotfix workflow
  workers:
    - id: hotfix:bob
      path: workers/bob.md
      description: Implementation worker
  stages:
    - id: hotfix:develop
      path: stages/develop.md
      description: Implement the fix
aliases:
  workflows:
    hotfix: hotfix:standard
  workers:
    bob: hotfix:bob
  stages:
    develop: hotfix:develop
`)

	report, err := workspace.InspectPack(dir)
	if err != nil {
		t.Fatalf("InspectPack: %v", err)
	}
	if !report.OK() {
		t.Fatalf("InspectPack errors: %v", report.Errors)
	}
	if report.Manifest.Name != "hotfix" {
		t.Fatalf("pack name = %q, want hotfix", report.Manifest.Name)
	}
}

func TestInspectPackReportsValidationErrors(t *testing.T) {
	dir := writeLocalPack(t, "bad", `schema: 1
name: Bad Pack
description: Broken pack
provides:
  workflows:
    - id: other:standard
      path: missing.yaml
  workers:
    - id: bad:bob
      path: workers/bob.md
aliases:
  workflows:
    default: bad:missing
`)

	report, err := workspace.InspectPack(dir)
	if err != nil {
		t.Fatalf("InspectPack: %v", err)
	}
	if report.OK() {
		t.Fatal("InspectPack unexpectedly passed")
	}
	for _, want := range []string{
		`name "Bad Pack" must use lowercase letters, numbers, and hyphens only`,
		`workflow[0].id "other:standard" must use pack namespace "Bad Pack"`,
		`workflow[0].path "missing.yaml"`,
		`workflow alias "default" points to unknown workflow "bad:missing"`,
	} {
		if !containsError(report.Errors, want) {
			t.Fatalf("errors missing %q:\n%s", want, strings.Join(report.Errors, "\n"))
		}
	}
}

func writeLocalPack(t *testing.T, name, manifest string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	for _, sub := range []string{"workers", "stages"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	writePackFile(t, filepath.Join(dir, "pack.yaml"), manifest)
	writePackFile(t, filepath.Join(dir, "workflow.yaml"), "workflows: {}\n")
	writePackFile(t, filepath.Join(dir, "workers", "bob.md"), `---
id: hotfix:bob
name: Bob
engine: codex
---

# Bob
`)
	writePackFile(t, filepath.Join(dir, "stages", "develop.md"), "# Develop\n")
	return dir
}

func writePackFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func containsError(errors []string, want string) bool {
	for _, err := range errors {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}
