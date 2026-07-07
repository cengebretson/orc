package workers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Worker struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Engine string `yaml:"engine"` // claude | codex | cursor
	Model  string `yaml:"model"`

	Args map[string]string `yaml:"args"` // extra flags: --key value (claude) or -c key=value (codex)

	FilePath string `yaml:"-"` // set at load time, not in frontmatter
}

// Load parses all namespaced worker markdown files in the given directory.
func Load(workersDir string) ([]*Worker, error) {
	var workers []*Worker
	err := filepath.WalkDir(workersDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".md" || filepath.Base(path) == "_template.md" {
			return nil
		}

		id, ok, err := workerIDFromPath(workersDir, path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		w, err := parseWorkerFile(path, id)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		if w.ID != id {
			return fmt.Errorf("parsing %s: worker id %q must match file path %q", path, w.ID, id)
		}
		workers = append(workers, w)
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning workers/: %w", err)
	}
	return workers, nil
}

func workerIDFromPath(workersDir, path string) (string, bool, error) {
	rel, err := filepath.Rel(workersDir, path)
	if err != nil {
		return "", false, err
	}
	if filepath.Dir(rel) == "." {
		return "", false, nil
	}
	withoutExt := strings.TrimSuffix(rel, filepath.Ext(rel))
	parts := strings.Split(filepath.ToSlash(withoutExt), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false, fmt.Errorf("worker file %s must live at workers/<namespace>/<worker>.md", path)
	}
	return parts[0] + ":" + parts[1], true, nil
}

// FindByID returns the worker with the given ID, or nil.
func FindByID(workers []*Worker, id string) *Worker {
	for _, w := range workers {
		if w.ID == id {
			return w
		}
	}
	return nil
}

// LaunchCommand renders the launch command string for display.
func LaunchCommand(w *Worker, workspaceRoot, cwd, prompt string) string {
	args := LaunchArgs(w, workspaceRoot, cwd, prompt)
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\n") {
			parts[i] = fmt.Sprintf("%q", a)
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}

// LaunchArgs returns the argv slice for executing a worker's launch command.
// workspaceRoot is always included so agents start with full context.
// cwd is where repo commands should run (the worktree).
// prompt is the instruction string.
func LaunchArgs(w *Worker, workspaceRoot, cwd, prompt string) []string {
	switch strings.ToLower(w.Engine) {
	case "codex":
		model := w.Model
		if model == "" {
			model = "default"
		}
		args := []string{"codex", "--model", model}
		for _, k := range sortedKeys(w.Args) {
			args = append(args, "-c", k+"="+w.Args[k])
		}
		return append(args, "--cd", cwd, prompt)
	case "cursor":
		// Cursor launches its interactive editor at cwd; the `cursor` binary has
		// no headless prompt flag, and passing prompt as a positional arg would
		// make it open the prompt text as a file path. The agent reads the task
		// from STATE.yaml / the stage file once the editor is open, so the prompt
		// is intentionally not forwarded here.
		return []string{"cursor", cwd}
	default: // claude
		args := []string{"claude", "--add-dir", workspaceRoot}
		for _, k := range sortedKeys(w.Args) {
			args = append(args, "--"+k, w.Args[k])
		}
		return append(args, prompt)
	}
}

// parseWorkerFile reads a markdown file and extracts YAML frontmatter.
func parseWorkerFile(path, defaultID string) (*Worker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	fm, err := extractFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("no valid frontmatter: %w", err)
	}

	var w Worker
	if err := yaml.Unmarshal([]byte(fm), &w); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}

	if w.ID == "" {
		w.ID = defaultID
	}

	w.FilePath = path
	return &w, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func extractFrontmatter(content string) (string, error) {
	if !strings.HasPrefix(content, "---") {
		return "", fmt.Errorf("missing opening ---")
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return "", fmt.Errorf("missing closing ---")
	}
	return strings.TrimSpace(rest[:end]), nil
}
