package workspace

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"gopkg.in/yaml.v3"
)

const localStage = "default:adhoc"

// EnsureLocalWorkflow adds the local-run workflow to workspaces created before
// it was bundled with the default pack. Existing workflows and stage guides are
// never replaced.
func EnsureLocalWorkflow(root, worker string) (bool, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return false, err
	}
	if workflow, ok := cfg.Workflows[LocalWorkflow]; ok {
		if err := validateLocalWorkflow(workflow); err != nil {
			return false, err
		}
		return ensureLocalStageGuide(root)
	}

	workflow := config.WorkflowDef{
		Description: "Standalone local work created by orc run",
		Stages: []config.StageDef{{
			Name:    localStage,
			Worker:  worker,
			Advance: "auto",
		}},
	}
	data, err := yaml.Marshal(struct {
		Workflows map[string]config.WorkflowDef `yaml:"workflows"`
	}{Workflows: map[string]config.WorkflowDef{LocalWorkflow: workflow}})
	if err != nil {
		return false, fmt.Errorf("encoding local workflow: %w", err)
	}

	orcYAML, err := readWorkspaceConfig(root)
	if err != nil {
		return false, err
	}
	yamlEntries := stripWorkflowsHeader(string(data))
	lines := strings.Split(yamlEntries, "\n")
	for i := range lines {
		lines[i] = strings.TrimPrefix(lines[i], "  ")
	}
	orcYAML = insertTopLevelSectionEntries(orcYAML, "workflows", strings.Join(lines, "\n"))

	stageContent, err := localStageGuideContent()
	if err != nil {
		return false, err
	}
	entries := []fileEntry{
		{dest: config.Filename, content: orcYAML},
		{dest: filepath.ToSlash(filepath.Join("stages", config.ResourcePath(localStage))), content: stageContent},
	}
	plan, err := planMutations(root, entries, mutationPlanOptions{
		existing:     skipExisting,
		allowUpdates: map[string]bool{config.Filename: true},
	})
	if err != nil {
		return false, err
	}
	if _, err := plan.Apply(); err != nil {
		return false, err
	}
	return true, nil
}

func validateLocalWorkflow(workflow config.WorkflowDef) error {
	if len(workflow.Stages) != 1 || workflow.Stages[0].Name != localStage {
		return fmt.Errorf("workflow %q already exists but is not the one-stage local workflow required by `orc run`", LocalWorkflow)
	}
	return nil
}

func ensureLocalStageGuide(root string) (bool, error) {
	dest := filepath.Join(root, "stages", config.ResourcePath(localStage))
	if _, err := os.Stat(dest); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("checking local stage guide: %w", err)
	}
	content, err := localStageGuideContent()
	if err != nil {
		return false, err
	}
	plan, err := planMutations(root, []fileEntry{{
		dest:    filepath.ToSlash(filepath.Join("stages", config.ResourcePath(localStage))),
		content: content,
	}}, mutationPlanOptions{existing: skipExisting})
	if err != nil {
		return false, err
	}
	if _, err := plan.Apply(); err != nil {
		return false, err
	}
	return true, nil
}

func readWorkspaceConfig(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, config.Filename))
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", config.Filename, err)
	}
	return string(data), nil
}

func localStageGuideContent() (string, error) {
	data, err := templateFS.ReadFile(path.Join(packsDir, "default", "stages", "adhoc.md"))
	if err != nil {
		return "", fmt.Errorf("reading bundled local stage guide: %w", err)
	}
	return strings.TrimRight(string(data), "\n") + "\n", nil
}
