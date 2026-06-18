package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cengebretson/orc/internal/config"
)

// InstalledPack describes one pack snapshot installed in a workspace.
type InstalledPack struct {
	Name          string
	Path          string
	Inspection    *PackInspection
	UsedWorkflows []string
	Active        bool
}

// ListInstalledPacks scans packs/<name>/ snapshots in a workspace and returns
// their validated manifests plus basic usage information from orc.yaml.
func ListInstalledPacks(root string) ([]InstalledPack, error) {
	packRoot := filepath.Join(root, "packs")
	entries, err := os.ReadDir(packRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading packs/: %w", err)
	}

	cfg, _ := config.Load(root)
	used := map[string]struct{}{}
	if cfg != nil {
		for workflowName := range cfg.Workflows {
			used[cfg.ResolveWorkflow(workflowName)] = struct{}{}
		}
		if defaultWorkflow := cfg.ResolveWorkflow(cfg.DefaultWorkflow()); defaultWorkflow != "" {
			used[defaultWorkflow] = struct{}{}
		}
	}

	var out []InstalledPack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(packRoot, entry.Name())
		inspection, err := InspectPack(dir)
		if err != nil {
			return nil, err
		}
		pack := InstalledPack{
			Name:       entry.Name(),
			Path:       filepath.Join("packs", entry.Name()),
			Inspection: inspection,
		}
		for _, wf := range inspection.Manifest.Provides.Workflows {
			if _, ok := used[wf.ID]; ok {
				pack.UsedWorkflows = append(pack.UsedWorkflows, wf.ID)
			}
		}
		sort.Strings(pack.UsedWorkflows)
		pack.Active = len(pack.UsedWorkflows) > 0
		out = append(out, pack)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}
