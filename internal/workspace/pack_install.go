package workspace

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

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
	targetOwners := map[string]string{}
	existingAliases := make([]string, 0, len(existing))
	for alias := range existing {
		existingAliases = append(existingAliases, alias)
	}
	sort.Strings(existingAliases)
	for _, alias := range existingAliases {
		target := existing[alias]
		targetOwners[target] = alias
	}
	nextAliases := make([]string, 0, len(next))
	for alias := range next {
		nextAliases = append(nextAliases, alias)
	}
	sort.Strings(nextAliases)
	for _, alias := range nextAliases {
		target := next[alias]
		if prev, ok := existing[alias]; ok && prev != target {
			return fmt.Errorf("%s alias %q points to both %s and %s", kind, alias, prev, target)
		}
		if prevAlias, ok := targetOwners[target]; ok && prevAlias != alias {
			return fmt.Errorf("%s aliases %q and %q both point to %s", kind, prevAlias, alias, target)
		}
		targetOwners[target] = alias
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
	data, err := os.ReadFile(filepath.Join(root, config.Filename))
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", config.Filename, err)
	}
	workflowConfig, err := readPackWorkflowConfig(source)
	if err != nil {
		return "", err
	}
	existingWorkflowCount := len(cfg.Workflows)

	out := string(data)
	if strings.TrimSpace(workflowConfig.Entries) != "" {
		out = insertTopLevelSectionEntries(out, "workflows", workflowConfig.Entries)
	}
	out = mergeAliasesIntoOrcYAML(out, source.Manifest.Aliases)
	if existingWorkflowCount == 0 && len(source.Manifest.Provides.Workflows) == 1 {
		out = setDefaultWorkflow(out, source.Manifest.Provides.Workflows[0].ID)
	}
	return out, nil
}

type workflowFileConfig struct {
	Workflows map[string]config.WorkflowDef `yaml:"workflows"`
	Entries   string                        `yaml:"-"`
}

func readPackWorkflowConfig(source installPackSource) (workflowFileConfig, error) {
	merged := workflowFileConfig{Workflows: map[string]config.WorkflowDef{}}
	declared := map[string]struct{}{}
	for _, workflow := range source.Manifest.Provides.Workflows {
		declared[workflow.ID] = struct{}{}
	}
	seenPath := map[string]bool{}
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
		if _, ok := parsed.Workflows[workflow.ID]; !ok {
			return workflowFileConfig{}, fmt.Errorf("workflow %s does not define declared workflow %q", workflow.Path, workflow.ID)
		}
		for name, workflowDef := range parsed.Workflows {
			if _, ok := declared[name]; !ok {
				return workflowFileConfig{}, fmt.Errorf("workflow %s defines undeclared workflow %q", workflow.Path, name)
			}
			merged.Workflows[name] = workflowDef
		}
		if !seenPath[workflow.Path] {
			seenPath[workflow.Path] = true
			merged.Entries += stripWorkflowsHeader(string(data))
		}
	}
	return merged, nil
}

func insertTopLevelSectionEntries(content, section, entries string) string {
	content = strings.TrimRight(content, "\n")
	entries = strings.TrimRight(entries, "\n")
	if entries == "" {
		return content + "\n"
	}

	lines := strings.Split(content, "\n")
	start := topLevelSectionLine(lines, section)
	if start == -1 {
		return content + "\n\n" + section + ":\n" + entries + "\n"
	}
	inlineEmpty := inlineEmptySection(lines[start], section)
	if inlineEmpty {
		lines[start] = section + ":"
	}

	insert := start + 1
	if !inlineEmpty {
		insert = len(lines)
		for i := start + 1; i < len(lines); i++ {
			line := lines[i]
			if strings.TrimSpace(line) == "" {
				continue
			}
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				insert = i
				break
			}
		}
	}

	next := append([]string{}, lines[:insert]...)
	next = append(next, strings.Split(entries, "\n")...)
	next = append(next, lines[insert:]...)
	return strings.Join(next, "\n") + "\n"
}

func mergeAliasesIntoOrcYAML(content string, aliases PackAliases) string {
	groups := []struct {
		name   string
		values map[string]string
	}{
		{name: "workflows", values: aliases.Workflows},
		{name: "stages", values: aliases.Stages},
		{name: "workers", values: aliases.Workers},
	}
	for _, group := range groups {
		if len(group.values) == 0 {
			continue
		}
		content = insertAliasGroupEntries(content, group.name, renderAliasPairs(group.values))
	}
	return content
}

func insertAliasGroupEntries(content, group, entries string) string {
	content = strings.TrimRight(content, "\n")
	lines := strings.Split(content, "\n")
	aliasesLine := topLevelSectionLine(lines, "aliases")
	if aliasesLine == -1 {
		return content + "\n\naliases:\n  " + group + ":\n" + indentLines(entries, 4) + "\n"
	}
	aliasesInlineEmpty := inlineEmptySection(lines[aliasesLine], "aliases")
	if aliasesInlineEmpty {
		lines[aliasesLine] = "aliases:"
	}

	groupLine := nestedSectionLine(lines, aliasesLine, group)
	if groupLine == -1 {
		insert := sectionEndLine(lines, aliasesLine)
		if aliasesInlineEmpty {
			insert = aliasesLine + 1
		}
		next := append([]string{}, lines[:insert]...)
		next = append(next, "  "+group+":")
		next = append(next, strings.Split(indentLines(entries, 4), "\n")...)
		next = append(next, lines[insert:]...)
		return strings.Join(next, "\n") + "\n"
	}
	groupInlineEmpty := inlineEmptySection(strings.TrimSpace(lines[groupLine]), group)
	if groupInlineEmpty {
		lines[groupLine] = "  " + group + ":"
	}

	insert := nestedSectionEndLine(lines, groupLine)
	if groupInlineEmpty {
		insert = groupLine + 1
	}
	next := append([]string{}, lines[:insert]...)
	next = append(next, strings.Split(indentLines(entries, 4), "\n")...)
	next = append(next, lines[insert:]...)
	return strings.Join(next, "\n") + "\n"
}

func renderAliasPairs(aliases map[string]string) string {
	keys := make([]string, 0, len(aliases))
	for key := range aliases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&b, "%s: %s\n", key, aliases[key])
	}
	return strings.TrimRight(b.String(), "\n")
}

func indentLines(s string, spaces int) string {
	if s == "" {
		return s
	}
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func topLevelSectionLine(lines []string, section string) int {
	needle := section + ":"
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			continue
		}
		if trimmed == needle || strings.HasPrefix(trimmed, needle+" ") || strings.HasPrefix(trimmed, needle+" #") {
			return i
		}
	}
	return -1
}

func inlineEmptySection(line, section string) bool {
	trimmed := strings.TrimSpace(line)
	needle := section + ":"
	return strings.HasPrefix(trimmed, needle) && strings.TrimSpace(strings.TrimPrefix(trimmed, needle)) == "{}"
}

func sectionEndLine(lines []string, start int) int {
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			return i
		}
	}
	return len(lines)
}

func nestedSectionLine(lines []string, parent int, section string) int {
	needle := section + ":"
	end := sectionEndLine(lines, parent)
	for i := parent + 1; i < end; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && (trimmed == needle || strings.HasPrefix(trimmed, needle+" ") || strings.HasPrefix(trimmed, needle+" #")) {
			return i
		}
	}
	return -1
}

func nestedSectionEndLine(lines []string, start int) int {
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, "    ") {
			return i
		}
	}
	return len(lines)
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

	var created []string
	for _, e := range entries {
		dest := filepath.Join(root, e.dest)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			rollbackPackInstall(root, created)
			return fmt.Errorf("creating directory for %s: %w", e.dest, err)
		}
		if e.dest == config.Filename {
			if err := writeFileAtomic(dest, []byte(e.content), 0o644); err != nil {
				rollbackPackInstall(root, created)
				return fmt.Errorf("writing %s: %w", e.dest, err)
			}
			fmt.Printf("  update %s\n", e.dest)
			continue
		}
		if err := os.WriteFile(dest, []byte(e.content), 0o644); err != nil {
			rollbackPackInstall(root, created)
			return fmt.Errorf("writing %s: %w", e.dest, err)
		}
		created = append(created, e.dest)
		fmt.Printf("  create %s\n", e.dest)
	}
	fmt.Printf("\nPack installed. %d files updated.\n", len(entries))
	return nil
}

func writeFileAtomic(dest string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "."+filepath.Base(dest)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, dest)
}

func rollbackPackInstall(root string, created []string) {
	for i := len(created) - 1; i >= 0; i-- {
		_ = os.Remove(filepath.Join(root, created[i]))
	}
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
