package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/config"
)

func TestSelectRunRepositoryExplicit(t *testing.T) {
	root := t.TempDir()
	api := filepath.Join(root, "projects", "api")
	if err := os.MkdirAll(api, 0o755); err != nil {
		t.Fatal(err)
	}
	repos := []config.Repo{
		{Name: "web", Path: "projects/web"},
		{Name: "api", Path: "projects/api"},
	}

	got, err := selectRunRepository(root, root, "api", repos)
	if err != nil {
		t.Fatalf("selectRunRepository: %v", err)
	}
	if got == nil || got.Name != "api" || got.Path != canonicalRunPath(api) || got.Inferred {
		t.Fatalf("selected repository = %+v", got)
	}
}

func TestSelectRunRepositoryInfersDeepestCheckout(t *testing.T) {
	root := t.TempDir()
	monorepo := filepath.Join(root, "projects", "suite")
	api := filepath.Join(monorepo, "services", "api")
	cwd := filepath.Join(api, "internal", "handler")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	repos := []config.Repo{
		{Name: "suite", Path: monorepo},
		{Name: "api", Path: api},
	}

	got, err := selectRunRepository(root, cwd, "", repos)
	if err != nil {
		t.Fatalf("selectRunRepository: %v", err)
	}
	if got == nil || got.Name != "api" || !got.Inferred {
		t.Fatalf("selected repository = %+v", got)
	}
}

func TestSelectRunRepositoryLeavesUnmatchedMultiRepoUnset(t *testing.T) {
	root := t.TempDir()
	repos := []config.Repo{
		{Name: "web", Path: filepath.Join(root, "web")},
		{Name: "api", Path: filepath.Join(root, "api")},
	}

	got, err := selectRunRepository(root, t.TempDir(), "", repos)
	if err != nil {
		t.Fatalf("selectRunRepository: %v", err)
	}
	if got != nil {
		t.Fatalf("selected repository = %+v, want nil", got)
	}
}

func TestSelectRunRepositoryRejectsUnknownName(t *testing.T) {
	root := t.TempDir()
	_, err := selectRunRepository(root, root, "worker", []config.Repo{
		{Name: "api", Path: filepath.Join(root, "api")},
		{Name: "web", Path: filepath.Join(root, "web")},
	})
	if err == nil || !strings.Contains(err.Error(), `repository "worker" not found (available: api, web)`) {
		t.Fatalf("error = %v", err)
	}
}
