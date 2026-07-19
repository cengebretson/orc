package workspacesnapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReturnsInvalidConfigErrorInsteadOfPanicking(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "orc.yaml"), []byte("unknown_key: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "unknown_key") {
		t.Fatalf("Load error = %v, want unknown-key configuration error", err)
	}
}
