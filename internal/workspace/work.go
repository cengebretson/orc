package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/state"
)

type WorkOptions struct {
	Root     string
	Ticket   string // e.g. FLYWL-123
	Slug     string // optional override suffix, e.g. "add-user-export"
	Workflow string // pipeline name, defaults to "default"
}

type WorkResult struct {
	FeatureDir string
	Slug       string
}

func Work(opts WorkOptions) (*WorkResult, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("resolving workspace path: %w", err)
	}

	ticket := strings.ToUpper(strings.TrimSpace(opts.Ticket))
	if ticket == "" {
		return nil, fmt.Errorf("ticket ID is required")
	}

	slug := buildSlug(ticket, opts.Slug)
	featureDir := filepath.Join(root, "features", slug)

	// check for any existing feature with this ticket prefix (e.g. FLYWL-123-some-slug)
	if existing, err := findExistingFeature(root, ticket); err == nil {
		return nil, fmt.Errorf("feature for %q already exists: features/%s\nUse `orc next %s` to continue working on it", ticket, existing, ticket)
	}

	// also guard the exact path in case the slug matches directly
	if _, err := os.Stat(featureDir); err == nil {
		return nil, fmt.Errorf("feature %q already exists\nUse `orc next %s` to continue working on it", slug, ticket)
	}

	templateDir := filepath.Join(root, "features", "_template")
	if _, err := os.Stat(templateDir); err != nil {
		return nil, fmt.Errorf("features/_template not found — run `orc init` first")
	}

	cfg, err := config.Load(root)
	if err != nil {
		return nil, fmt.Errorf("loading workspace config: %w", err)
	}
	workflowName := opts.Workflow
	explicitWorkflow := workflowName != ""
	if workflowName == "" {
		workflowName = cfg.DefaultWorkflow()
	}
	if _, ok := cfg.Workflows[workflowName]; !ok {
		known := cfg.Names()
		suffix := ""
		if len(known) > 0 {
			suffix = fmt.Sprintf(" (available: %s)", strings.Join(known, ", "))
		}
		if !explicitWorkflow && cfg.Settings.DefaultWorkflow == "" {
			return nil, fmt.Errorf("no workflow specified and no default_workflow set in orc.yaml%s", suffix)
		}
		return nil, fmt.Errorf("workflow %q not found in orc.yaml%s", workflowName, suffix)
	}
	stages := cfg.StageNames(workflowName)
	if len(stages) == 0 {
		return nil, fmt.Errorf("workflow %q has no stages defined in orc.yaml", workflowName)
	}
	firstStage := stages[0]
	firstStageConfig, ok := cfg.StageConfig(workflowName, firstStage)
	if !ok {
		return nil, fmt.Errorf("stage %q not found in workflow %q", firstStage, workflowName)
	}

	if err := copyDir(templateDir, featureDir); err != nil {
		return nil, fmt.Errorf("creating feature folder: %w", err)
	}

	if err := writeStateYAML(featureDir, ticket, slug, workflowName, firstStage, firstStageConfig.Worker); err != nil {
		return nil, fmt.Errorf("writing STATE.yaml: %w", err)
	}

	return &WorkResult{FeatureDir: featureDir, Slug: slug}, nil
}

// findExistingFeature returns the name of any feature folder that starts with ticket.
func findExistingFeature(root, ticket string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "features"))
	if err != nil {
		return "", err
	}
	upper := strings.ToUpper(ticket)
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_template" {
			continue
		}
		name := strings.ToUpper(e.Name())
		if name == upper || strings.HasPrefix(name, upper+"-") {
			return e.Name(), nil
		}
	}
	return "", fmt.Errorf("not found")
}

func buildSlug(ticket, suffix string) string {
	if suffix == "" {
		return ticket
	}
	// normalise suffix: lowercase, spaces to hyphens
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	suffix = strings.ReplaceAll(suffix, " ", "-")
	return ticket + "-" + suffix
}

// writeStateYAML stamps STATE.yaml with real ticket/slug values, replacing
// the template placeholder copied into the feature dir. Uses the canonical
// state.State schema so the scaffold can never drift from what state.Load
// reads back.
func writeStateYAML(featureDir, ticket, slug, workflowName, firstStage, firstWorker string) error {
	s := &state.State{
		SchemaVersion: state.SchemaVersion,
		Ticket:        ticket,
		Slug:          slug,
		Status:        "pending",
		Workflow:      workflowName,
		Stage: state.Stage{
			Name: firstStage,
		},
		NextAction: state.NextAction{
			Worker: firstWorker,
			Prompt: fmt.Sprintf("Load ticket %s and populate TICKET.md, SPEC.md, and PLAN.md. Update STATE.yaml when complete.", ticket),
			CWD:    ".",
		},
		History: []state.HistoryEntry{
			{
				At:     time.Now().Format(time.RFC3339),
				Stage:  firstStage,
				Worker: "agent",
				Result: "feature context created by orc work",
			},
		},
	}
	return state.Create(featureDir, s)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck

	_, err = io.Copy(out, in)
	return err
}
