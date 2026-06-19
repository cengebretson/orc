package workspace_test

import (
	"testing"

	"github.com/cengebretson/orc/internal/health"
	"github.com/cengebretson/orc/internal/workspace"
)

// TestEmbeddedPacks_ClosureComplete installs every embedded pack and asserts its
// workflow is closure-complete: every worker the workflow routes to has a worker
// file, and every stage (including loop stages) has a stage file. A pack is the
// closed set of "a workflow + its workers + its stage files", so this catches an
// authoring mistake — e.g. editing a pack's workflow.yaml to reference a worker
// that isn't in the pack — at CI time, instead of when a user installs the pack
// and runs `orc doctor`. It reuses the exact check doctor runs (the health
// "workflow refs" result), so the guarantee can't drift from doctor's behavior.
func TestEmbeddedPacks_ClosureComplete(t *testing.T) {
	packs, err := workspace.ListPacks()
	if err != nil {
		t.Fatalf("ListPacks: %v", err)
	}
	if len(packs) == 0 {
		t.Fatal("no embedded packs found")
	}

	for _, p := range packs {
		t.Run(p.Name, func(t *testing.T) {
			dir := t.TempDir()
			if err := workspace.Init(workspace.InitOptions{Root: dir, SkipDefaultPack: true}); err != nil {
				t.Fatalf("Init base workspace: %v", err)
			}
			if err := workspace.InstallPack(workspace.PackInstallOptions{Root: dir, Pack: p.Name}); err != nil {
				t.Fatalf("InstallPack %s: %v", p.Name, err)
			}

			report := health.Run(dir)
			var refs *health.Result
			for i := range report.Results {
				if report.Results[i].Name == "workflow refs" {
					refs = &report.Results[i]
					break
				}
			}
			if refs == nil {
				t.Fatal("no 'workflow refs' result in health report")
			}
			if refs.Status != health.OK {
				t.Errorf("pack %q is not closure-complete: %s", p.Name, refs.Detail)
			}
		})
	}
}
