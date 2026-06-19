package workspace

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"gopkg.in/yaml.v3"
)

//go:embed all:templates
var templateFS embed.FS

const (
	baseDir  = "templates/_base"
	packsDir = "templates/packs"
)

type InitOptions struct {
	Root   string
	Packs  []string // packs to install; empty = ["default"]; ["none"] = base only
	DryRun bool
	Force  bool
}

// PackInfo is the metadata declared in a pack's pack.yaml.
type PackInfo struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Schema      int      `yaml:"schema"`
	Engines     []string `yaml:"engines"`
	Provides    struct {
		Workflow string   `yaml:"workflow"`
		Workers  []string `yaml:"workers"`
		Stages   []string `yaml:"stages"`
	} `yaml:"provides"`
}

type fileEntry struct {
	dest    string
	content string
}

type packInstallInfo struct {
	SourceType  string `yaml:"source_type"`
	SourceRef   string `yaml:"source_ref"`
	ResolvedRef string `yaml:"resolved_ref,omitempty"`
}

func Init(opts InitOptions) error {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return fmt.Errorf("resolving workspace path: %w", err)
	}

	entries, err := collectEntries(opts)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return printDryRun(root, entries)
	}

	return writeEntries(root, entries, opts.Force)
}

// resolvePacks applies the default and the "none" sentinel. An empty selection
// means the default pack; ["none"] means base only (no pack), and cannot be
// combined with real packs.
func resolvePacks(requested []string) ([]string, error) {
	if len(requested) == 0 {
		return []string{"default"}, nil
	}
	for _, p := range requested {
		if p == "none" {
			if len(requested) > 1 {
				return nil, fmt.Errorf("--pack none cannot be combined with other packs")
			}
			return nil, nil
		}
	}
	return requested, nil
}

// loadPack reads and parses a pack's pack.yaml.
func loadPack(name string) (PackInfo, error) {
	manifest, err := loadEmbeddedPackManifest(name)
	if err != nil {
		return PackInfo{}, err
	}
	return packInfoFromManifest(manifest), nil
}

func loadEmbeddedPackManifest(name string) (PackManifestV1, error) {
	data, err := templateFS.ReadFile(path.Join(packsDir, name, "pack.yaml"))
	if err != nil {
		return PackManifestV1{}, fmt.Errorf("unknown pack %q (run `orc init --list-packs` to see available packs)", name)
	}
	var manifest PackManifestV1
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return PackManifestV1{}, fmt.Errorf("parsing pack.yaml for %q: %w", name, err)
	}
	return manifest, nil
}

func packInfoFromManifest(manifest PackManifestV1) PackInfo {
	info := PackInfo{
		Name:        manifest.Name,
		Description: manifest.Description,
		Schema:      manifest.Schema,
		Engines:     manifest.Engines,
	}
	if len(manifest.Provides.Workflows) > 0 {
		info.Provides.Workflow = manifest.Provides.Workflows[0].ID
	}
	for _, worker := range manifest.Provides.Workers {
		info.Provides.Workers = append(info.Provides.Workers, worker.ID)
	}
	for _, stage := range manifest.Provides.Stages {
		info.Provides.Stages = append(info.Provides.Stages, stage.ID)
	}
	return info
}

// ListPacks returns the metadata for every embedded pack, sorted by name.
func ListPacks() ([]PackInfo, error) {
	dirs, err := templateFS.ReadDir(packsDir)
	if err != nil {
		return nil, fmt.Errorf("reading packs: %w", err)
	}
	var out []PackInfo
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		info, err := loadPack(d.Name())
		if err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

func collectEntries(opts InitOptions) ([]fileEntry, error) {
	packs, err := resolvePacks(opts.Packs)
	if err != nil {
		return nil, err
	}

	var entries []fileEntry
	var baseOrcYAML string

	// 1. Base scaffold — always installed. orc.yaml is held back and assembled
	//    below so the selected packs' workflows can be spliced in.
	err = fs.WalkDir(templateFS, baseDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// all:templates embeds dotfiles (.DS_Store etc.); never scaffold OS junk.
		if d.Name() == ".DS_Store" {
			return nil
		}
		content, err := templateFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", p, err)
		}
		rel := strings.TrimPrefix(p, baseDir+"/")
		if rel == "orc.yaml" {
			baseOrcYAML = string(content)
			return nil
		}
		entries = append(entries, fileEntry{dest: rel, content: string(content)})
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(packs) == 1 && isLocalPackRef(packs[0]) {
		return collectLocalPackEntries(baseOrcYAML, entries, packs[0])
	}
	for _, p := range packs {
		if isLocalPackRef(p) {
			return nil, fmt.Errorf("local filesystem packs cannot be combined with other packs yet")
		}
	}
	// 2. Selected embedded packs are snapshotted under packs/<name>/ and their
	//    runtime resources are materialized under workers/<name>/ and stages/<name>/.
	var manifests []PackManifestV1
	for _, name := range packs {
		manifest, err := loadEmbeddedPackManifest(name)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, manifest)
		packEntries, err := collectEmbeddedPackEntries(name, manifest)
		if err != nil {
			return nil, err
		}
		packEntries = append(packEntries, packProvenanceEntry(name, packInstallInfo{
			SourceType:  "builtin",
			SourceRef:   name,
			ResolvedRef: name,
		}))
		entries = append(entries, packEntries...)
	}
	if err := validatePackComposition(packs, manifests); err != nil {
		return nil, err
	}

	// 3. Assemble orc.yaml from the base config plus each pack's workflow block.
	orcYAML, err := assembleEmbeddedPackOrcYAML(baseOrcYAML, packs, manifests)
	if err != nil {
		return nil, err
	}
	entries = append(entries, fileEntry{dest: "orc.yaml", content: orcYAML})

	return entries, nil
}

func validatePackComposition(packNames []string, manifests []PackManifestV1) error {
	type owner struct {
		pack  string
		kind  string
		value string
	}
	seen := map[string]owner{}
	aliasTargets := map[string]string{}
	var conflicts []string

	check := func(kind, pack string, values []PackResource) {
		for _, v := range values {
			if prev, ok := seen[kind+":"+v.ID]; ok {
				conflicts = append(conflicts, fmt.Sprintf("%s %q is provided by both %s and %s", kind, v.ID, prev.pack, pack))
				continue
			}
			seen[kind+":"+v.ID] = owner{pack: pack, kind: kind, value: v.ID}
		}
	}
	checkAliases := func(kind, pack string, values map[string]string) {
		for alias, target := range values {
			key := "alias:" + kind + ":" + alias
			if prev, ok := aliasTargets[key]; ok {
				if prev == target {
					continue
				}
				if prevOwner, ok := seen[key]; ok {
					conflicts = append(conflicts, fmt.Sprintf("%s alias %q points to both %s and %s", kind, alias, prevOwner.value, target))
				} else {
					conflicts = append(conflicts, fmt.Sprintf("%s alias %q points to both %s and %s", kind, alias, prev, target))
				}
				continue
			}
			aliasTargets[key] = target
			seen[key] = owner{pack: pack, kind: kind, value: target}
		}
	}

	for i, pack := range packNames {
		manifest := manifests[i]
		check("workflow", pack, manifest.Provides.Workflows)
		check("stage", pack, manifest.Provides.Stages)
		check("worker", pack, manifest.Provides.Workers)
		checkAliases("workflow", pack, manifest.Aliases.Workflows)
		checkAliases("stage", pack, manifest.Aliases.Stages)
		checkAliases("worker", pack, manifest.Aliases.Workers)
	}

	if len(conflicts) == 0 {
		return nil
	}
	sort.Strings(conflicts)
	return fmt.Errorf("pack composition conflicts:\n  %s", strings.Join(conflicts, "\n  "))
}

func collectEmbeddedPackEntries(name string, manifest PackManifestV1) ([]fileEntry, error) {
	var entries []fileEntry
	packRoot := path.Join(packsDir, name)
	err := fs.WalkDir(templateFS, packRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == ".DS_Store" {
			return nil
		}
		content, err := templateFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", p, err)
		}
		rel := strings.TrimPrefix(p, packRoot+"/")
		entries = append(entries, fileEntry{
			dest:    filepath.ToSlash(filepath.Join("packs", name, rel)),
			content: string(content),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	runtimeEntries, err := collectEmbeddedPackRuntimeEntries(packRoot, manifest)
	if err != nil {
		return nil, err
	}
	entries = append(entries, runtimeEntries...)
	return entries, nil
}

func collectEmbeddedPackRuntimeEntries(packRoot string, manifest PackManifestV1) ([]fileEntry, error) {
	var entries []fileEntry
	for _, stage := range manifest.Provides.Stages {
		content, err := templateFS.ReadFile(path.Join(packRoot, filepath.ToSlash(filepath.Clean(stage.Path))))
		if err != nil {
			return nil, fmt.Errorf("reading stage %s: %w", stage.Path, err)
		}
		entries = append(entries, fileEntry{
			dest:    filepath.ToSlash(filepath.Join("stages", config.ResourcePath(stage.ID))),
			content: string(content),
		})
	}
	for _, worker := range manifest.Provides.Workers {
		content, err := templateFS.ReadFile(path.Join(packRoot, filepath.ToSlash(filepath.Clean(worker.Path))))
		if err != nil {
			return nil, fmt.Errorf("reading worker %s: %w", worker.Path, err)
		}
		entries = append(entries, fileEntry{
			dest:    filepath.ToSlash(filepath.Join("workers", config.ResourcePath(worker.ID))),
			content: string(content),
		})
	}
	return entries, nil
}

func assembleEmbeddedPackOrcYAML(base string, packNames []string, manifests []PackManifestV1) (string, error) {
	var b strings.Builder
	b.WriteString(strings.TrimRight(base, "\n"))
	b.WriteString("\n")

	if len(packNames) == 0 {
		return b.String(), nil
	}

	b.WriteString("\nworkflows:\n")
	for i, name := range packNames {
		packRoot := path.Join(packsDir, name)
		seen := map[string]bool{}
		for _, workflow := range manifests[i].Provides.Workflows {
			if seen[workflow.Path] {
				continue
			}
			seen[workflow.Path] = true
			wf, err := templateFS.ReadFile(path.Join(packRoot, workflow.Path))
			if err != nil {
				return "", fmt.Errorf("reading workflow.yaml for pack %q: %w", name, err)
			}
			b.WriteString(stripWorkflowsHeader(string(wf)))
		}
	}

	packAliases := make([]PackAliases, 0, len(manifests))
	for _, manifest := range manifests {
		packAliases = append(packAliases, manifest.Aliases)
	}
	appendPackAliases(&b, packAliases)

	if len(packNames) == 1 && len(manifests[0].Provides.Workflows) > 0 {
		return setDefaultWorkflow(b.String(), manifests[0].Provides.Workflows[0].ID), nil
	}
	return b.String(), nil
}

func appendPackAliases(b *strings.Builder, aliases []PackAliases) {
	merged := PackAliases{
		Workflows: map[string]string{},
		Workers:   map[string]string{},
		Stages:    map[string]string{},
	}
	for _, a := range aliases {
		for k, v := range a.Workflows {
			merged.Workflows[k] = v
		}
		for k, v := range a.Workers {
			merged.Workers[k] = v
		}
		for k, v := range a.Stages {
			merged.Stages[k] = v
		}
	}
	if len(merged.Workflows) == 0 && len(merged.Workers) == 0 && len(merged.Stages) == 0 {
		return
	}

	b.WriteString("\naliases:\n")
	appendAliasGroup(b, "workflows", merged.Workflows)
	appendAliasGroup(b, "stages", merged.Stages)
	appendAliasGroup(b, "workers", merged.Workers)
}

func appendAliasGroup(b *strings.Builder, name string, aliases map[string]string) {
	if len(aliases) == 0 {
		return
	}
	b.WriteString("  " + name + ":\n")
	keys := make([]string, 0, len(aliases))
	for k := range aliases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "    %s: %s\n", k, aliases[k])
	}
}

func isLocalPackRef(ref string) bool {
	return ref == "." || filepath.IsAbs(ref) || strings.HasPrefix(ref, "."+string(filepath.Separator)) || strings.Contains(ref, string(filepath.Separator))
}

func collectLocalPackEntries(baseOrcYAML string, entries []fileEntry, packPath string) ([]fileEntry, error) {
	report, err := InspectPack(packPath)
	if err != nil {
		return nil, err
	}
	if !report.OK() {
		return nil, fmt.Errorf("pack validation failed:\n  %s", strings.Join(report.Errors, "\n  "))
	}
	manifest := report.Manifest
	if len(manifest.Provides.Workflows) == 0 {
		return nil, fmt.Errorf("local pack %q must provide at least one workflow", manifest.Name)
	}
	if len(manifest.Provides.Workflows) > 1 {
		return nil, fmt.Errorf("local pack %q provides multiple workflows; --default-workflow is not implemented yet", manifest.Name)
	}

	snapshotEntries, err := collectPackSnapshotEntries(report.Source, manifest.Name)
	if err != nil {
		return nil, err
	}
	entries = append(entries, snapshotEntries...)
	entries = append(entries, packProvenanceEntry(manifest.Name, packInstallInfo{
		SourceType:  "local-path",
		SourceRef:   packPath,
		ResolvedRef: report.Source,
	}))

	runtimeEntries, err := collectPackRuntimeEntries(report.Source, manifest)
	if err != nil {
		return nil, err
	}
	entries = append(entries, runtimeEntries...)

	workflow, err := assembleLocalPackWorkflow(baseOrcYAML, report.Source, manifest)
	if err != nil {
		return nil, err
	}
	entries = append(entries, fileEntry{dest: "orc.yaml", content: workflow})
	return entries, nil
}

func packProvenanceEntry(name string, info packInstallInfo) fileEntry {
	data, err := yaml.Marshal(info)
	if err != nil {
		return fileEntry{dest: filepath.ToSlash(filepath.Join("packs", name, ".orc-pack.yaml")), content: fmt.Sprintf("source_type: %s\nsource_ref: %s\n", info.SourceType, info.SourceRef)}
	}
	return fileEntry{
		dest:    filepath.ToSlash(filepath.Join("packs", name, ".orc-pack.yaml")),
		content: string(data),
	}
}

func collectPackSnapshotEntries(source, name string) ([]fileEntry, error) {
	var entries []fileEntry
	err := filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == ".DS_Store" {
			return nil
		}
		content, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("reading pack file %s: %w", p, err)
		}
		rel, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		dest := filepath.ToSlash(filepath.Join("packs", name, rel))
		entries = append(entries, fileEntry{dest: dest, content: string(content)})
		return nil
	})
	return entries, err
}

func collectPackRuntimeEntries(source string, manifest PackManifestV1) ([]fileEntry, error) {
	var entries []fileEntry
	for _, stage := range manifest.Provides.Stages {
		content, err := os.ReadFile(filepath.Join(source, filepath.Clean(stage.Path)))
		if err != nil {
			return nil, fmt.Errorf("reading stage %s: %w", stage.Path, err)
		}
		entries = append(entries, fileEntry{
			dest:    filepath.ToSlash(filepath.Join("stages", config.ResourcePath(stage.ID))),
			content: string(content),
		})
	}
	for _, worker := range manifest.Provides.Workers {
		content, err := os.ReadFile(filepath.Join(source, filepath.Clean(worker.Path)))
		if err != nil {
			return nil, fmt.Errorf("reading worker %s: %w", worker.Path, err)
		}
		entries = append(entries, fileEntry{
			dest:    filepath.ToSlash(filepath.Join("workers", config.ResourcePath(worker.ID))),
			content: string(content),
		})
	}
	return entries, nil
}

func assembleLocalPackWorkflow(base, source string, manifest PackManifestV1) (string, error) {
	var b strings.Builder
	b.WriteString(strings.TrimRight(base, "\n"))
	b.WriteString("\n\nworkflows:\n")

	seen := map[string]bool{}
	for _, workflow := range manifest.Provides.Workflows {
		if seen[workflow.Path] {
			continue
		}
		seen[workflow.Path] = true
		content, err := os.ReadFile(filepath.Join(source, filepath.Clean(workflow.Path)))
		if err != nil {
			return "", fmt.Errorf("reading workflow %s: %w", workflow.Path, err)
		}
		b.WriteString(stripWorkflowsHeader(string(content)))
	}
	appendPackAliases(&b, []PackAliases{manifest.Aliases})
	out := setDefaultWorkflow(b.String(), manifest.Provides.Workflows[0].ID)
	return out, nil
}

// stripWorkflowsHeader drops the leading "workflows:" line from a pack's
// workflow.yaml, leaving the indented workflow entries.
func stripWorkflowsHeader(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// setDefaultWorkflow rewrites the settings.default_workflow value. Used only
// when no selected pack provides a workflow named "default", so the default
// install path never touches the line and stays byte-identical.
func setDefaultWorkflow(s, wf string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if idx := strings.Index(ln, "default_workflow:"); idx >= 0 && strings.TrimSpace(ln[:idx]) == "" {
			lines[i] = ln[:idx] + "default_workflow: " + wf
			break
		}
	}
	return strings.Join(lines, "\n")
}

func printDryRun(root string, entries []fileEntry) error {
	fmt.Printf("Dry run — workspace root: %s\n\n", root)

	dirs := map[string]bool{}
	for _, e := range entries {
		dir := filepath.Dir(e.dest)
		if dir != "." {
			dirs[dir] = true
		}
	}

	// print directories first
	for dir := range dirs {
		fmt.Printf("  mkdir  %s\n", dir)
	}

	fmt.Println()

	for _, e := range entries {
		dest := filepath.Join(root, e.dest)
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("  skip   %s (already exists)\n", e.dest)
		} else {
			fmt.Printf("  create %s\n", e.dest)
		}
	}

	printSetupNextSteps(root, true)

	return nil
}

func writeEntries(root string, entries []fileEntry, force bool) error {
	created := 0
	skipped := 0

	for _, e := range entries {
		dest := filepath.Join(root, e.dest)

		if _, err := os.Stat(dest); err == nil && !force {
			fmt.Printf("  skip   %s\n", e.dest)
			skipped++
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("creating directory for %s: %w", e.dest, err)
		}

		if err := os.WriteFile(dest, []byte(e.content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", e.dest, err)
		}

		fmt.Printf("  create %s\n", e.dest)
		created++
	}

	// always create empty dirs that hold runtime artifacts
	runtimeDirs := []string{"worktrees", "projects", "features"}
	for _, dir := range runtimeDirs {
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	writeGitignore(root, force)

	fmt.Printf("\nDone. %d created, %d skipped.\n", created, skipped)
	if skipped > 0 {
		fmt.Println("Use --force to overwrite existing files.")
	}
	printSetupNextSteps(root, false)

	return nil
}

func printSetupNextSteps(root string, dryRun bool) {
	if dryRun {
		fmt.Printf("\nWould create workspace at: %s\n\n", root)
		fmt.Println("Would run next:")
	} else {
		fmt.Printf("\nWorkspace ready at: %s\n\n", root)
		fmt.Println("Next:")
	}
	fmt.Printf("  cd %s\n", root)
	fmt.Println(`  claude "Read SETUP.md and follow the setup instructions"`)
	fmt.Println("  # or:")
	fmt.Println(`  codex "Read SETUP.md and follow the setup instructions"`)
	fmt.Println("  orc doctor")
}

func writeGitignore(root string, force bool) {
	dest := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(dest); err == nil && !force {
		return
	}
	content := "worktrees/\n"
	_ = os.WriteFile(dest, []byte(content), 0644)
}
