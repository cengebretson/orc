package stage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStageFiles(t *testing.T) {
	root := t.TempDir()
	stagesDir := filepath.Join(root, Dir)
	if err := os.Mkdir(stagesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagesDir, "build.md"), []byte("# Build\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagesDir, "review.md"), []byte("# Review\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagesDir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(stagesDir, "nested.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	if !Exists(root, "build") {
		t.Fatal("Exists(build) = false, want true")
	}
	if Exists(root, "missing") {
		t.Fatal("Exists(missing) = true, want false")
	}
	got, err := Read(root, "build")
	if err != nil || string(got) != "# Build\n" {
		t.Fatalf("Read(build) = %q, %v", got, err)
	}
	if _, err := Read(root, "missing"); !os.IsNotExist(err) {
		t.Fatalf("Read(missing) error = %v, want not-exist", err)
	}
	if got := List(root); !reflect.DeepEqual(got, []string{"build", "review"}) {
		t.Fatalf("List() = %#v", got)
	}
}

func TestListMissingDirectory(t *testing.T) {
	if got := List(t.TempDir()); got != nil {
		t.Fatalf("List() = %#v, want nil", got)
	}
}
