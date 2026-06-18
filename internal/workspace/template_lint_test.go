package workspace_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownTemplatesAvoidStaleWorkflowTerms(t *testing.T) {
	roots := []string{
		filepath.Join("templates"),
		filepath.Join("..", "..", "testdata", "workspace"),
	}
	banned := []string{
		"orc mark <ticket> wait",
		"orc mark <ticket> advance",
		"--owner",
		"bob-fast-fixer",
		"`needs changes`",
		"verdict: needs changes",
	}

	for _, root := range roots {
		root := root
		t.Run(filepath.ToSlash(root), func(t *testing.T) {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || filepath.Ext(path) != ".md" {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				content := string(data)
				for _, term := range banned {
					if strings.Contains(content, term) {
						t.Errorf("%s contains stale workflow term %q", filepath.ToSlash(path), term)
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walking %s: %v", root, err)
			}
		})
	}
}
