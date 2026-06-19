package workspace_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/workspace"
)

// These golden tests pin the EXACT output of `orc init` — every file it writes
// and the bytes of each — for the default-pack install and the base-only
// (--skip-default-pack) install. The existing presence checks only assert that
// expected files exist; they would stay green even if a migration silently
// changed orc.yaml's assembled workflow, altered a worker body, or leaked the
// sample workers into the base case. These tests turn "output stays
// byte-identical" into an enforced invariant, so the planned _base/ + packs/
// reshuffle and collectEntries rewrite must reproduce the current scaffold
// exactly or fail here.
//
// When a template change is intentional, regenerate the manifests:
//
//	ORC_UPDATE_GOLDEN=1 go test ./internal/workspace/...
//
// then review the testdata/ diff like any other change.

// manifest returns a deterministic "<sha256>  <relpath>" line per regular file
// under root, sorted by path. It pins both the set of files (a dropped or
// leaked file changes the line set) and their exact bytes (a content change
// changes the hash).
func manifest(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		lines = append(lines, fmt.Sprintf("%s  %s", hex.EncodeToString(sum[:]), filepath.ToSlash(rel)))
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n") + "\n"
}

// checkGolden compares got against testdata/<name>, or rewrites it when
// ORC_UPDATE_GOLDEN is set.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	goldenPath := filepath.Join("testdata", name)

	if os.Getenv("ORC_UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", goldenPath, err)
		}
		t.Logf("updated golden: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden %s: %v (regenerate with ORC_UPDATE_GOLDEN=1 go test ./internal/workspace/...)", goldenPath, err)
	}
	if got != string(want) {
		t.Errorf("init output drifted from golden %s.\n"+
			"If the template change is intentional, regenerate with:\n"+
			"  ORC_UPDATE_GOLDEN=1 go test ./internal/workspace/...\n\n"+
			"--- got ---\n%s\n--- want ---\n%s", goldenPath, got, string(want))
	}
}

// TestInit_GoldenDefaultPack pins the default install. Its manifest pins the
// generated file set and exact bytes, including the assembled orc.yaml.
func TestInit_GoldenDefaultPack(t *testing.T) {
	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	checkGolden(t, "init_default.manifest", manifest(t, dir))
}

// TestInit_GoldenSkipDefaultPack pins the base-only install
// (--skip-default-pack): the
// structural scaffold with an empty workflows: block and no pack workers or
// stage files.
func TestInit_GoldenSkipDefaultPack(t *testing.T) {
	dir := t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: dir, SkipDefaultPack: true}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	checkGolden(t, "init_skip_default_pack.manifest", manifest(t, dir))
}
