package workspace

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

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
	data, err := templateFS.ReadFile(path.Join(packsDir, name, "pack.yaml"))
	if err != nil {
		return PackInfo{}, fmt.Errorf("unknown pack %q (run `orc init --list-packs` to see available packs)", name)
	}
	var info PackInfo
	if err := yaml.Unmarshal(data, &info); err != nil {
		return PackInfo{}, fmt.Errorf("parsing pack.yaml for %q: %w", name, err)
	}
	return info, nil
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

	// 2. Selected packs — only workers/ and stages/ are scaffolded; pack.yaml and
	//    workflow.yaml are metadata consumed by the assembler.
	var infos []PackInfo
	seenWorker := map[string]string{} // workers/<file> -> pack that provided it
	for _, name := range packs {
		info, err := loadPack(name)
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)

		packRoot := path.Join(packsDir, name)
		err = fs.WalkDir(templateFS, packRoot, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if d.Name() == ".DS_Store" {
				return nil
			}
			rel := strings.TrimPrefix(p, packRoot+"/")
			if rel == "pack.yaml" || rel == "workflow.yaml" {
				return nil
			}
			content, err := templateFS.ReadFile(p)
			if err != nil {
				return fmt.Errorf("reading template %s: %w", p, err)
			}
			if strings.HasPrefix(rel, "workers/") {
				if prev, ok := seenWorker[rel]; ok {
					return fmt.Errorf("packs %q and %q both provide %s", prev, name, rel)
				}
				seenWorker[rel] = name
			}
			entries = append(entries, fileEntry{dest: rel, content: string(content)})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// 3. Assemble orc.yaml from the base config plus each pack's workflow block.
	orcYAML, err := assembleOrcYAML(baseOrcYAML, packs, infos)
	if err != nil {
		return nil, err
	}
	entries = append(entries, fileEntry{dest: "orc.yaml", content: orcYAML})

	return entries, nil
}

// assembleOrcYAML splices each selected pack's workflow.yaml under a single
// workflows: key in the base orc.yaml. It is text-based on purpose: a yaml
// round-trip would drop the base file's comments and reformat it, so a
// single-pack install would no longer reproduce the hand-written orc.yaml
// byte-for-byte. The first pack's block carries the workflows: header verbatim;
// subsequent packs contribute their indented body only.
func assembleOrcYAML(base string, packNames []string, infos []PackInfo) (string, error) {
	var b strings.Builder
	b.WriteString(strings.TrimRight(base, "\n"))
	b.WriteString("\n")

	if len(packNames) == 0 {
		return b.String(), nil // --pack none: base only, no workflows
	}

	b.WriteString("\nworkflows:\n")
	providesDefault := false
	for i, name := range packNames {
		wf, err := templateFS.ReadFile(path.Join(packsDir, name, "workflow.yaml"))
		if err != nil {
			return "", fmt.Errorf("reading workflow.yaml for pack %q: %w", name, err)
		}
		b.WriteString(stripWorkflowsHeader(string(wf)))
		if infos[i].Provides.Workflow == "default" {
			providesDefault = true
		}
	}

	out := b.String()
	if !providesDefault {
		out = setDefaultWorkflow(out, infos[0].Provides.Workflow)
	}
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
