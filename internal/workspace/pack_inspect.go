package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var packNameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type PackManifestV1 struct {
	Schema      int      `json:"schema" yaml:"schema"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Engines     []string `json:"engines,omitempty" yaml:"engines"`
	Provides    struct {
		Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
		Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
		Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
	} `json:"provides" yaml:"provides"`
	Aliases PackAliases `json:"aliases,omitempty" yaml:"aliases"`
}

type PackResource struct {
	ID          string `json:"id" yaml:"id"`
	Path        string `json:"path" yaml:"path"`
	Description string `json:"description,omitempty" yaml:"description"`
}

type PackAliases struct {
	Workflows map[string]string `json:"workflows,omitempty" yaml:"workflows"`
	Workers   map[string]string `json:"workers,omitempty" yaml:"workers"`
	Stages    map[string]string `json:"stages,omitempty" yaml:"stages"`
}

type PackInspection struct {
	Source   string         `json:"source"`
	Manifest PackManifestV1 `json:"manifest"`
	Errors   []string       `json:"errors,omitempty"`
}

func (r PackInspection) OK() bool {
	return len(r.Errors) == 0
}

func InspectPack(root string) (*PackInspection, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving pack path: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(abs, "pack.yaml"))
	if err != nil {
		return nil, fmt.Errorf("reading pack.yaml: %w", err)
	}

	var manifest PackManifestV1
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing pack.yaml: %w", err)
	}

	report := &PackInspection{
		Source:   abs,
		Manifest: manifest,
	}
	report.Errors = validatePackManifest(abs, manifest)
	return report, nil
}

func validatePackManifest(root string, manifest PackManifestV1) []string {
	var errs []string

	if manifest.Schema != 1 {
		errs = append(errs, "schema must be 1")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		errs = append(errs, "name is required")
	} else if !validPackName(manifest.Name) {
		errs = append(errs, fmt.Sprintf("name %q must use lowercase letters, numbers, and hyphens only", manifest.Name))
	}
	if strings.TrimSpace(manifest.Description) == "" {
		errs = append(errs, "description is required")
	}

	provided := map[string]map[string]bool{
		"workflow": map[string]bool{},
		"worker":   map[string]bool{},
		"stage":    map[string]bool{},
	}
	errs = append(errs, validateResources(root, manifest.Name, "workflow", manifest.Provides.Workflows, provided["workflow"])...)
	errs = append(errs, validateResources(root, manifest.Name, "worker", manifest.Provides.Workers, provided["worker"])...)
	errs = append(errs, validateResources(root, manifest.Name, "stage", manifest.Provides.Stages, provided["stage"])...)
	errs = append(errs, validateAliases("workflow", manifest.Aliases.Workflows, provided["workflow"])...)
	errs = append(errs, validateAliases("worker", manifest.Aliases.Workers, provided["worker"])...)
	errs = append(errs, validateAliases("stage", manifest.Aliases.Stages, provided["stage"])...)
	errs = append(errs, validatePackWorkflowClosure(root, manifest, provided)...)

	sort.Strings(errs)
	return errs
}

func validatePackWorkflowClosure(root string, manifest PackManifestV1, provided map[string]map[string]bool) []string {
	var errs []string
	declaredWorkflows := map[string]bool{}
	for _, workflow := range manifest.Provides.Workflows {
		declaredWorkflows[workflow.ID] = true
	}
	seenPath := map[string]bool{}
	for _, workflow := range manifest.Provides.Workflows {
		if strings.TrimSpace(workflow.Path) == "" || seenPath[workflow.Path] {
			continue
		}
		seenPath[workflow.Path] = true
		data, err := os.ReadFile(filepath.Join(root, filepath.Clean(workflow.Path)))
		if err != nil {
			continue
		}
		var parsed workflowFileConfig
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			errs = append(errs, fmt.Sprintf("workflow %s: parsing workflow file: %v", workflow.Path, err))
			continue
		}
		for workflowID, workflowDef := range parsed.Workflows {
			if !declaredWorkflows[workflowID] {
				errs = append(errs, fmt.Sprintf("workflow %s defines undeclared workflow %q", workflow.Path, workflowID))
				continue
			}
			for _, stage := range workflowDef.Stages {
				if stage.Name != "" && !provided["stage"][stage.Name] {
					errs = append(errs, fmt.Sprintf("workflow %q references stage %q not provided by pack %q", workflowID, stage.Name, manifest.Name))
				}
				if stage.Worker != "" && !provided["worker"][stage.Worker] {
					errs = append(errs, fmt.Sprintf("workflow %q references worker %q not provided by pack %q", workflowID, stage.Worker, manifest.Name))
				}
				if stage.Loop != nil {
					if stage.Loop.Via != "" && !provided["stage"][stage.Loop.Via] {
						errs = append(errs, fmt.Sprintf("workflow %q references loop stage %q not provided by pack %q", workflowID, stage.Loop.Via, manifest.Name))
					}
					if stage.Loop.Worker != "" && !provided["worker"][stage.Loop.Worker] {
						errs = append(errs, fmt.Sprintf("workflow %q references loop worker %q not provided by pack %q", workflowID, stage.Loop.Worker, manifest.Name))
					}
				}
			}
		}
	}
	return errs
}

func validateResources(root, packName, kind string, resources []PackResource, seen map[string]bool) []string {
	var errs []string
	for i, r := range resources {
		prefix := fmt.Sprintf("%s[%d]", kind, i)
		if strings.TrimSpace(r.ID) == "" {
			errs = append(errs, prefix+".id is required")
		} else {
			pack, _, ok := splitResourceID(r.ID)
			if !ok {
				errs = append(errs, fmt.Sprintf("%s.id %q must use <pack>:<resource>", prefix, r.ID))
			} else {
				if !validPackName(pack) {
					errs = append(errs, fmt.Sprintf("%s.id pack %q must use lowercase letters, numbers, and hyphens only", prefix, pack))
				}
				if packName != "" && pack != packName {
					errs = append(errs, fmt.Sprintf("%s.id %q must use pack namespace %q", prefix, r.ID, packName))
				}
				if !validResourceID(r.ID) {
					errs = append(errs, fmt.Sprintf("%s.id %q must use lowercase letters, numbers, hyphens, and one colon", prefix, r.ID))
				}
			}
			if seen[r.ID] {
				errs = append(errs, fmt.Sprintf("duplicate %s id %q", kind, r.ID))
			}
			seen[r.ID] = true
		}

		if strings.TrimSpace(r.Path) == "" {
			errs = append(errs, prefix+".path is required")
			continue
		}
		if err := validatePackPath(root, r.Path); err != nil {
			errs = append(errs, fmt.Sprintf("%s.path %q: %v", prefix, r.Path, err))
		}
	}
	return errs
}

func validateAliases(kind string, aliases map[string]string, provided map[string]bool) []string {
	var errs []string
	targetOwners := map[string]string{}
	keys := make([]string, 0, len(aliases))
	for alias := range aliases {
		keys = append(keys, alias)
	}
	sort.Strings(keys)
	for _, alias := range keys {
		target := aliases[alias]
		if !validPackName(alias) {
			errs = append(errs, fmt.Sprintf("%s alias %q must use lowercase letters, numbers, and hyphens only", kind, alias))
		}
		if previous, ok := targetOwners[target]; ok {
			errs = append(errs, fmt.Sprintf("%s aliases %q and %q both point to %s", kind, previous, alias, target))
		} else {
			targetOwners[target] = alias
		}
		if !validResourceID(target) {
			errs = append(errs, fmt.Sprintf("%s alias %q target %q must use <pack>:<resource>", kind, alias, target))
			continue
		}
		if !provided[target] {
			errs = append(errs, fmt.Sprintf("%s alias %q points to unknown %s %q", kind, alias, kind, target))
		}
	}
	return errs
}

func validatePackPath(root, rel string) error {
	if filepath.IsAbs(rel) {
		return fmt.Errorf("must be relative")
	}
	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return fmt.Errorf("must stay inside pack")
	}
	info, err := os.Stat(filepath.Join(root, clean))
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("must be a file")
	}
	return nil
}

func validPackName(s string) bool {
	return packNameRE.MatchString(s)
}

func validResourceID(s string) bool {
	pack, resource, ok := splitResourceID(s)
	return ok && validPackName(pack) && validPackName(resource)
}

func splitResourceID(s string) (string, string, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], parts[0] != "" && parts[1] != ""
}

func (r PackInspection) MarshalJSON() ([]byte, error) {
	type alias PackInspection
	return json.Marshal(struct {
		OK bool `json:"ok"`
		alias
	}{
		OK:    r.OK(),
		alias: alias(r),
	})
}
