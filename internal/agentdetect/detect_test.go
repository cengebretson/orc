package agentdetect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedRulesPreferTitleAndStripActivityGlyph(t *testing.T) {
	manifest, err := Load(t.TempDir(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	blocked := manifest.Detect("title", "⠋ Action Required")
	if blocked.Lifecycle != "blocked" || blocked.Attention != "blocked" || blocked.RuleID != "title-action-required" || blocked.Title != "Action Required" {
		t.Fatalf("blocked title = %+v", blocked)
	}
	working := manifest.Detect("title", "⠙ Implementing fallback")
	if working.Lifecycle != "working" || working.Title != "Implementing fallback" {
		t.Fatalf("working title = %+v", working)
	}
	if unknown := manifest.Detect("screen", "ordinary shell output"); unknown.Lifecycle != "unknown" {
		t.Fatalf("unknown screen = %+v", unknown)
	}
}

func TestWorkspaceOverrideIsVersionedStrictAndBounded(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, OverrideDir, "v1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "codex.yaml")
	valid := `version: 1
engine: codex
rules:
  - id: local-block
    source: screen
    lifecycle: blocked
    attention: blocked
    priority: 999
    region_lines: 8
    pattern: 'LOCAL APPROVAL'
`
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := Load(root, "codex")
	if err != nil || manifest.Detect("screen", "LOCAL APPROVAL").RuleID != "local-block" {
		t.Fatalf("override = %+v, %v", manifest, err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(valid, "region_lines: 8", "region_lines: 81", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, "codex"); err == nil || !strings.Contains(err.Error(), "between 1 and 80") {
		t.Fatalf("invalid override error = %v", err)
	}
}
