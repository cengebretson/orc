package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanMutationsClassifiesExistingFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.md"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries := []fileEntry{{dest: "existing.md", content: "new"}, {dest: "new.md", content: "new"}}

	plan, err := planMutations(root, entries, mutationPlanOptions{existing: skipExisting})
	if err != nil {
		t.Fatal(err)
	}
	if plan.files[0].action != skipFile || plan.files[1].action != createFile {
		t.Fatalf("actions = %v, %v; want skip, create", plan.files[0].action, plan.files[1].action)
	}

	plan, err = planMutations(root, entries, mutationPlanOptions{existing: replaceExisting})
	if err != nil {
		t.Fatal(err)
	}
	if plan.files[0].action != updateFile {
		t.Fatalf("replace action = %v, want update", plan.files[0].action)
	}
}

func TestMutationPlanRollsBackCreatesAndUpdates(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "orc.yaml")
	if err := os.WriteFile(existing, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := planMutations(root, []fileEntry{
		{dest: "created.md", content: "created"},
		{dest: "orc.yaml", content: "updated"},
		{dest: "failure.md", content: "failure"},
	}, mutationPlanOptions{existing: rejectExisting, allowUpdates: map[string]bool{"orc.yaml": true}})
	if err != nil {
		t.Fatal(err)
	}
	plan.writeFile = func(dest string, data []byte, mode os.FileMode) error {
		if filepath.Base(dest) == "failure.md" {
			return errors.New("injected write failure")
		}
		return writeFileAtomic(dest, data, mode)
	}
	if _, err := plan.Apply(); err == nil || !strings.Contains(err.Error(), "injected write failure") {
		t.Fatalf("Apply error = %v", err)
	}
	if got, _ := os.ReadFile(existing); string(got) != "original" {
		t.Fatalf("updated file was not restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(root, "created.md")); !os.IsNotExist(err) {
		t.Fatalf("created file was not removed: %v", err)
	}
}

func TestPlanMutationsRejectsDuplicateAndEscapingPaths(t *testing.T) {
	root := t.TempDir()
	_, err := planMutations(root, []fileEntry{{dest: "same"}, {dest: "same"}}, mutationPlanOptions{})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}
	_, err = planMutations(root, []fileEntry{{dest: "../outside"}}, mutationPlanOptions{})
	if err == nil || !strings.Contains(err.Error(), "escapes root") {
		t.Fatalf("escape error = %v", err)
	}
}

func TestMutationPlanAppliesCreatesAndAtomicUpdates(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "orc.yaml")
	if err := os.WriteFile(existing, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := planMutations(root, []fileEntry{
		{dest: "packs/new.md", content: "created"},
		{dest: "orc.yaml", content: "updated"},
	}, mutationPlanOptions{existing: rejectExisting, allowUpdates: map[string]bool{"orc.yaml": true}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := plan.Apply()
	if err != nil {
		t.Fatal(err)
	}
	if result.created != 1 || result.updated != 1 || result.skipped != 0 {
		t.Fatalf("result = %+v", result)
	}
	if got, _ := os.ReadFile(existing); string(got) != "updated" {
		t.Fatalf("orc.yaml = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "packs", "new.md")); string(got) != "created" {
		t.Fatalf("new file = %q", got)
	}
}
