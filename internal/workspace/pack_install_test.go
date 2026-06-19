package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePackInstallEntriesRollsBackCreatedFiles(t *testing.T) {
	root := t.TempDir()
	readOnly := filepath.Join(root, "readonly")
	if err := os.Mkdir(readOnly, 0o555); err != nil {
		t.Fatalf("mkdir readonly: %v", err)
	}
	defer func() {
		_ = os.Chmod(readOnly, 0o755)
	}()

	err := writePackInstallEntries(root, []fileEntry{
		{dest: "packs/hotfix/pack.yaml", content: "name: hotfix\n"},
		{dest: "readonly/file.md", content: "blocked\n"},
	})
	if err == nil {
		t.Fatal("writePackInstallEntries returned nil error")
	}
	if !strings.Contains(err.Error(), "writing readonly/file.md") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "packs", "hotfix", "pack.yaml")); !os.IsNotExist(err) {
		t.Fatalf("created file was not rolled back: %v", err)
	}
}
