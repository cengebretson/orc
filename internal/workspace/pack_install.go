package workspace

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/cengebretson/orc/internal/config"
	"gopkg.in/yaml.v3"
)

type PackInstallOptions struct {
	Root string
	Pack string
}

func InstallPack(opts PackInstallOptions) error {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return fmt.Errorf("resolving workspace path: %w", err)
	}

	source, err := loadInstallPack(opts.Pack)
	if err != nil {
		return err
	}
	if err := validatePackInstall(root, source.Manifest); err != nil {
		return err
	}

	entries, err := collectInstallPackEntries(source)
	if err != nil {
		return err
	}

	orcYAML, err := mergePackIntoOrcYAML(root, source)
	if err != nil {
		return err
	}
	entries = append(entries, fileEntry{dest: config.Filename, content: orcYAML})

	return writePackInstallEntries(root, entries)
}

type installPackSource struct {
	Ref         string
	Source      string
	SourceType  string
	SourceRef   string
	ResolvedRef string
	Manifest    PackManifestV1
	Embedded    bool
}

func loadInstallPack(ref string) (installPackSource, error) {
	if isLocalPackRef(ref) {
		report, err := InspectPack(ref)
		if err != nil {
			return installPackSource{}, err
		}
		if !report.OK() {
			return installPackSource{}, fmt.Errorf("pack validation failed:\n  %s", joinLines(report.Errors))
		}
		return installPackSource{
			Ref:         ref,
			Source:      report.Source,
			SourceType:  "local-path",
			SourceRef:   ref,
			ResolvedRef: report.Source,
			Manifest:    report.Manifest,
		}, nil
	}

	manifest, err := loadEmbeddedPackManifest(ref)
	if err != nil {
		return installPackSource{}, err
	}
	return installPackSource{
		Ref:         ref,
		Source:      path.Join(packsDir, ref),
		SourceType:  "builtin",
		SourceRef:   ref,
		ResolvedRef: ref,
		Manifest:    manifest,
		Embedded:    true,
	}, nil
}

func validatePackInstall(root string, manifest PackManifestV1) error {
	if _, err := os.Stat(filepath.Join(root, config.Filename)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("orc workspace not initialized: %s not found", config.Filename)
		}
		return fmt.Errorf("reading %s: %w", config.Filename, err)
	}
	if _, err := os.Stat(filepath.Join(root, "packs", manifest.Name)); err == nil {
		return fmt.Errorf("pack %q is already installed", manifest.Name)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking installed pack %q: %w", manifest.Name, err)
	}

	installed, err := ListInstalledPacks(root)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(installed)+1)
	manifests := make([]PackManifestV1, 0, len(installed)+1)
	for _, pack := range installed {
		if pack.Inspection == nil {
			continue
		}
		names = append(names, pack.Name)
		manifests = append(manifests, pack.Inspection.Manifest)
	}
	names = append(names, manifest.Name)
	manifests = append(manifests, manifest)
	if err := validatePackComposition(names, manifests); err != nil {
		return err
	}

	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	if err := validatePackAgainstConfig(cfg, manifest); err != nil {
		return err
	}
	for _, entry := range runtimeEntriesForManifest(manifest) {
		if _, err := os.Stat(filepath.Join(root, entry)); err == nil {
			return fmt.Errorf("install would overwrite existing runtime file %s", entry)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("checking runtime file %s: %w", entry, err)
		}
	}

	return nil
}

func validatePackAgainstConfig(cfg *config.Config, manifest PackManifestV1) error {
	for _, workflow := range manifest.Provides.Workflows {
		if _, ok := cfg.Workflows[workflow.ID]; ok {
			return fmt.Errorf("workflow %q already exists in %s", workflow.ID, config.Filename)
		}
	}
	if err := checkAliasMerge("workflow", cfg.Aliases.Workflows, manifest.Aliases.Workflows); err != nil {
		return err
	}
	if err := checkAliasMerge("stage", cfg.Aliases.Stages, manifest.Aliases.Stages); err != nil {
		return err
	}
	if err := checkAliasMerge("worker", cfg.Aliases.Workers, manifest.Aliases.Workers); err != nil {
		return err
	}
	return nil
}

func checkAliasMerge(kind string, existing, next map[string]string) error {
	for alias, target := range next {
		if prev, ok := existing[alias]; ok && prev != target {
			return fmt.Errorf("%s alias %q points to both %s and %s", kind, alias, prev, target)
		}
	}
	return nil
}

func collectInstallPackEntries(source installPackSource) ([]fileEntry, error) {
	var entries []fileEntry
	var err error
	if source.Embedded {
		entries, err = collectEmbeddedPackEntries(source.Ref, source.Manifest)
	} else {
		entries, err = collectPackSnapshotEntries(source.Source, source.Manifest.Name)
		if err == nil {
			var runtime []fileEntry
			runtime, err = collectPackRuntimeEntries(source.Source, source.Manifest)
			entries = append(entries, runtime...)
		}
	}
	if err != nil {
		return nil, err
	}
	entries = append(entries, packProvenanceEntry(source.Manifest.Name, packInstallInfo{
		SourceType:  source.SourceType,
		SourceRef:   source.SourceRef,
		ResolvedRef: source.ResolvedRef,
	}))
	return entries, nil
}

func mergePackIntoOrcYAML(root string, source installPackSource) (string, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return "", err
	}
	workflowConfig, err := readPackWorkflowConfig(source)
	if err != nil {
		return "", err
	}
	if cfg.Workflows == nil {
		cfg.Workflows = map[string]config.WorkflowDef{}
	}
	existingWorkflowCount := len(cfg.Workflows)
	for name, workflow := range workflowConfig.Workflows {
		cfg.Workflows[name] = workflow
	}
	mergeConfigAliases(&cfg.Aliases, source.Manifest.Aliases)
	if existingWorkflowCount == 0 && len(source.Manifest.Provides.Workflows) == 1 {
		cfg.Settings.DefaultWorkflow = source.Manifest.Provides.Workflows[0].ID
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshaling %s: %w", config.Filename, err)
	}
	return string(data), nil
}

type workflowFileConfig struct {
	Workflows map[string]config.WorkflowDef `yaml:"workflows"`
}

func readPackWorkflowConfig(source installPackSource) (workflowFileConfig, error) {
	merged := workflowFileConfig{Workflows: map[string]config.WorkflowDef{}}
	for _, workflow := range source.Manifest.Provides.Workflows {
		var data []byte
		var err error
		if source.Embedded {
			data, err = templateFS.ReadFile(path.Join(source.Source, workflow.Path))
		} else {
			data, err = os.ReadFile(filepath.Join(source.Source, filepath.Clean(workflow.Path)))
		}
		if err != nil {
			return workflowFileConfig{}, fmt.Errorf("reading workflow %s: %w", workflow.Path, err)
		}
		var parsed workflowFileConfig
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return workflowFileConfig{}, fmt.Errorf("parsing workflow %s: %w", workflow.Path, err)
		}
		for name, workflow := range parsed.Workflows {
			merged.Workflows[name] = workflow
		}
	}
	return merged, nil
}

func mergeConfigAliases(dst *config.Aliases, src PackAliases) {
	if len(src.Workflows) > 0 && dst.Workflows == nil {
		dst.Workflows = map[string]string{}
	}
	for alias, target := range src.Workflows {
		dst.Workflows[alias] = target
	}
	if len(src.Stages) > 0 && dst.Stages == nil {
		dst.Stages = map[string]string{}
	}
	for alias, target := range src.Stages {
		dst.Stages[alias] = target
	}
	if len(src.Workers) > 0 && dst.Workers == nil {
		dst.Workers = map[string]string{}
	}
	for alias, target := range src.Workers {
		dst.Workers[alias] = target
	}
}

func runtimeEntriesForManifest(manifest PackManifestV1) []string {
	var entries []string
	for _, stage := range manifest.Provides.Stages {
		entries = append(entries, filepath.ToSlash(filepath.Join("stages", config.ResourcePath(stage.ID))))
	}
	for _, worker := range manifest.Provides.Workers {
		entries = append(entries, filepath.ToSlash(filepath.Join("workers", config.ResourcePath(worker.ID))))
	}
	return entries
}

func writePackInstallEntries(root string, entries []fileEntry) error {
	for _, e := range entries {
		dest := filepath.Join(root, e.dest)
		if e.dest == config.Filename {
			continue
		}
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("install would overwrite existing file %s", e.dest)
		} else if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("checking %s: %w", e.dest, err)
		}
	}

	for _, e := range entries {
		dest := filepath.Join(root, e.dest)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", e.dest, err)
		}
		if err := os.WriteFile(dest, []byte(e.content), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", e.dest, err)
		}
		fmt.Printf("  create %s\n", e.dest)
	}
	fmt.Printf("\nPack installed. %d files updated.\n", len(entries))
	return nil
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	out := lines[0]
	for _, line := range lines[1:] {
		out += "\n  " + line
	}
	return out
}
