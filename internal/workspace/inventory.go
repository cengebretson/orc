package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cengebretson/orc/internal/config"
	"gopkg.in/yaml.v3"
)

type PackInstallInfo struct {
	SourceType  string `yaml:"source_type"`
	SourceRef   string `yaml:"source_ref"`
	ResolvedRef string `yaml:"resolved_ref,omitempty"`
}

// InstalledPack describes one pack snapshot installed in a workspace.
type InstalledPack struct {
	Name          string
	Path          string
	Inspection    *PackInspection
	Install       *PackInstallInfo
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
		install, err := loadPackInstallInfo(dir)
		if err != nil {
			return nil, err
		}
		pack := InstalledPack{
			Name:       entry.Name(),
			Path:       filepath.Join("packs", entry.Name()),
			Inspection: inspection,
			Install:    install,
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

func loadPackInstallInfo(dir string) (*PackInstallInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".orc-pack.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading install metadata: %w", err)
	}
	var info PackInstallInfo
	if err := yaml.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("parsing install metadata: %w", err)
	}
	return &info, nil
}
