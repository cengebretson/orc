package config

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/worktreesetup"
)

type ValidationError struct {
	Path    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

type ValidationErrors []ValidationError

func (errs ValidationErrors) Error() string {
	if len(errs) == 0 {
		return ""
	}
	var parts []string
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}

// Validate checks the workspace configuration contract. workerIDs is the set of
// worker IDs loaded from workers/<namespace>/*.md.
func Validate(cfg *Config, workerIDs []string) ValidationErrors {
	if cfg == nil {
		return ValidationErrors{{Path: "orc.yaml", Message: "config is required"}}
	}

	workerSet := make(map[string]bool, len(workerIDs))
	for _, id := range workerIDs {
		workerSet[id] = true
	}

	var errs ValidationErrors
	errs = append(errs, validateAliasTargets("aliases.workflows", cfg.Aliases.Workflows)...)
	errs = append(errs, validateAliasTargets("aliases.stages", cfg.Aliases.Stages)...)
	errs = append(errs, validateAliasTargets("aliases.workers", cfg.Aliases.Workers)...)
	errs = append(errs, validateRepos(cfg.Repos)...)
	errs = append(errs, validateRepoRoutes(cfg.Routing, cfg.Repos)...)
	if cfg.Settings.WorkspaceRefresh < 0 {
		errs = append(errs, ValidationError{
			Path:    "settings.workspace_refresh",
			Message: "workspace_refresh must be zero or greater",
		})
	}
	if cfg.Settings.ArtifactPolicy != "" && cfg.Settings.ArtifactPolicy != "warn" && cfg.Settings.ArtifactPolicy != "block" {
		errs = append(errs, ValidationError{
			Path:    "settings.artifact_policy",
			Message: `artifact_policy must be "warn" or "block"`,
		})
	}
	if settings := cfg.Settings.ContextPressure; settings != nil &&
		!(contextpressure.Thresholds{Green: settings.Green, Yellow: settings.Yellow, Red: settings.Red}).Valid() {
		errs = append(errs, ValidationError{
			Path:    "settings.context_pressure",
			Message: "thresholds must satisfy 0 <= green < yellow < red <= 100",
		})
	}
	for i, event := range cfg.Settings.Notify.On {
		switch strings.ToLower(strings.TrimSpace(event)) {
		case "blocked", "complete", "error", "all":
		default:
			errs = append(errs, ValidationError{
				Path:    fmt.Sprintf("settings.notify.on[%d]", i),
				Message: `event must be "blocked", "complete", "error", or "all"`,
			})
		}
	}
	if settings := cfg.Settings.Parking; settings != nil {
		if len(settings.AutoPark) > 0 && len(settings.WakeOn) == 0 {
			errs = append(errs, ValidationError{Path: "settings.parking.wake_on", Message: "at least one wake condition is required when auto_park is enabled"})
		}
		for i, status := range settings.AutoPark {
			switch strings.ToLower(strings.TrimSpace(status)) {
			case "pending", "active", "paused", "done", "archived":
			default:
				errs = append(errs, ValidationError{Path: fmt.Sprintf("settings.parking.auto_park[%d]", i), Message: "status must be pending, active, paused, done, or archived"})
			}
		}
		for i, condition := range settings.WakeOn {
			switch strings.ToLower(strings.TrimSpace(condition)) {
			case "status_change", "attention", "stage_change":
			default:
				errs = append(errs, ValidationError{Path: fmt.Sprintf("settings.parking.wake_on[%d]", i), Message: "condition must be status_change, attention, or stage_change"})
			}
		}
	}

	if len(cfg.Workflows) > 0 {
		defaultWorkflow := cfg.ResolveWorkflow(cfg.DefaultWorkflow())
		if defaultWorkflow == "" {
			errs = append(errs, ValidationError{
				Path:    "settings.default_workflow",
				Message: "default workflow is required when workflows are configured",
			})
		} else if _, ok := cfg.Workflows[defaultWorkflow]; !ok {
			errs = append(errs, ValidationError{
				Path:    "settings.default_workflow",
				Message: fmt.Sprintf("workflow %q not found", defaultWorkflow),
			})
		}
	}

	for workflowName, workflow := range cfg.Workflows {
		workflowPath := fmt.Sprintf("workflows.%s", workflowName)
		if len(workflow.Stages) == 0 {
			errs = append(errs, ValidationError{
				Path:    workflowPath + ".stages",
				Message: "workflow must define at least one stage",
			})
			continue
		}

		names := map[string]string{}
		for i, stage := range workflow.Stages {
			stagePath := fmt.Sprintf("%s.stages[%d]", workflowPath, i)
			if stage.Name == "" {
				errs = append(errs, ValidationError{
					Path:    stagePath + ".name",
					Message: "stage name is required",
				})
			} else if previous, ok := names[stage.Name]; ok {
				errs = append(errs, ValidationError{
					Path:    stagePath + ".name",
					Message: fmt.Sprintf("duplicate stage name %q also used at %s", stage.Name, previous),
				})
			} else {
				names[stage.Name] = stagePath + ".name"
			}

			errs = append(errs, validateWorker(stagePath+".worker", stage.Worker, workerSet)...)
			if stage.Advance != "auto" && stage.Advance != "manual" {
				errs = append(errs, ValidationError{
					Path:    stagePath + ".advance",
					Message: `advance must be "auto" or "manual"`,
				})
			}
			errs = append(errs, validateArtifactPaths(stagePath+".required_artifacts", stage.RequiredArtifacts)...)

			if stage.Loop != nil {
				loopPath := stagePath + ".loop"
				if stage.Loop.Via == "" {
					errs = append(errs, ValidationError{
						Path:    loopPath + ".via",
						Message: "loop stage name is required",
					})
				} else if previous, ok := names[stage.Loop.Via]; ok {
					errs = append(errs, ValidationError{
						Path:    loopPath + ".via",
						Message: fmt.Sprintf("duplicate stage name %q also used at %s", stage.Loop.Via, previous),
					})
				} else {
					names[stage.Loop.Via] = loopPath + ".via"
				}

				errs = append(errs, validateWorker(loopPath+".worker", stage.Loop.Worker, workerSet)...)
				if stage.Loop.Max < 0 {
					errs = append(errs, ValidationError{
						Path:    loopPath + ".max",
						Message: "loop max must be zero or greater",
					})
				}
				if stage.Loop.OnMax != "" && stage.Loop.OnMax != "pause" && stage.Loop.OnMax != "fail" {
					errs = append(errs, ValidationError{
						Path:    loopPath + ".on_max",
						Message: `loop on_max must be "pause" or "fail"`,
					})
				}
				errs = append(errs, validateArtifactPaths(loopPath+".required_artifacts", stage.Loop.RequiredArtifacts)...)
			}
		}
	}

	return errs
}

func validateArtifactPaths(path string, artifacts []string) ValidationErrors {
	var errs ValidationErrors
	for i, artifact := range artifacts {
		artifactPath := fmt.Sprintf("%s[%d]", path, i)
		trimmed := strings.TrimSpace(artifact)
		if trimmed == "" {
			errs = append(errs, ValidationError{Path: artifactPath, Message: "artifact path is required"})
			continue
		}
		if strings.HasPrefix(trimmed, "/") {
			errs = append(errs, ValidationError{Path: artifactPath, Message: "artifact path must be relative to the feature folder"})
			continue
		}
		parts := strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == '/' || r == '\\'
		})
		for _, part := range parts {
			if part == ".." {
				errs = append(errs, ValidationError{Path: artifactPath, Message: "artifact path cannot contain .."})
				break
			}
		}
	}
	return errs
}

func validateRepos(repos []Repo) ValidationErrors {
	var errs ValidationErrors
	names := make(map[string]int, len(repos))
	paths := make(map[string]int, len(repos))
	for i, repo := range repos {
		repoPath := fmt.Sprintf("repos[%d]", i)
		name := strings.TrimSpace(repo.Name)
		path := strings.TrimSpace(repo.Path)
		if name == "" {
			errs = append(errs, ValidationError{Path: repoPath + ".name", Message: "repo name is required"})
		} else if previous, ok := names[name]; ok {
			errs = append(errs, ValidationError{
				Path:    repoPath + ".name",
				Message: fmt.Sprintf("duplicate repo name %q also used at repos[%d].name", name, previous),
			})
		} else {
			names[name] = i
		}
		if path == "" {
			errs = append(errs, ValidationError{Path: repoPath + ".path", Message: "repo path is required"})
		} else {
			clean := filepath.Clean(path)
			if previous, ok := paths[clean]; ok {
				errs = append(errs, ValidationError{
					Path:    repoPath + ".path",
					Message: fmt.Sprintf("duplicate repo path %q also used at repos[%d].path", path, previous),
				})
			} else {
				paths[clean] = i
			}
		}
		if strings.TrimSpace(repo.WorktreeSetup) == "" {
			continue
		}
		if unknown := worktreesetup.UnknownPlaceholders(repo.WorktreeSetup); len(unknown) > 0 {
			errs = append(errs, ValidationError{
				Path:    repoPath + ".worktree_setup",
				Message: fmt.Sprintf("unknown placeholder(s): %s", strings.Join(unknown, ", ")),
			})
		}
	}
	return errs
}

func validateRepoRoutes(routes []RepoRoute, repos []Repo) ValidationErrors {
	var errs ValidationErrors
	repoNames := make(map[string]bool, len(repos))
	for _, repo := range repos {
		repoNames[repo.Name] = true
	}
	seenLabels := make(map[string]string)
	seenComponents := make(map[string]string)
	for i, route := range routes {
		routePath := fmt.Sprintf("routing[%d]", i)
		if len(route.Labels) == 0 && len(route.Components) == 0 {
			errs = append(errs, ValidationError{Path: routePath, Message: "route must define at least one label or component"})
		}
		errs = append(errs, validateRouteSignals(routePath+".labels", route.Labels, seenLabels)...)
		errs = append(errs, validateRouteSignals(routePath+".components", route.Components, seenComponents)...)
		if len(route.Repos) == 0 {
			errs = append(errs, ValidationError{Path: routePath + ".repos", Message: "route must select at least one repo"})
		}
		seenRepos := map[string]int{}
		for j, name := range route.Repos {
			path := fmt.Sprintf("%s.repos[%d]", routePath, j)
			name = strings.TrimSpace(name)
			if name == "" {
				errs = append(errs, ValidationError{Path: path, Message: "repo name is required"})
			} else if previous, ok := seenRepos[name]; ok {
				errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("duplicate repo %q also used at %s.repos[%d]", name, routePath, previous)})
			} else if !repoNames[name] {
				errs = append(errs, ValidationError{Path: path, Message: fmt.Sprintf("repo %q not found in repos", name)})
			}
			seenRepos[name] = j
		}
	}
	return errs
}

func validateRouteSignals(path string, values []string, seen map[string]string) ValidationErrors {
	var errs ValidationErrors
	for i, value := range values {
		valuePath := fmt.Sprintf("%s[%d]", path, i)
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			errs = append(errs, ValidationError{Path: valuePath, Message: "routing value is required"})
			continue
		}
		key := strings.ToLower(trimmed)
		if previous, ok := seen[key]; ok {
			errs = append(errs, ValidationError{Path: valuePath, Message: fmt.Sprintf("routing value %q is already used at %s", trimmed, previous)})
			continue
		}
		seen[key] = valuePath
	}
	return errs
}

func validateAliasTargets(path string, aliases map[string]string) ValidationErrors {
	seen := map[string]string{}
	var errs ValidationErrors
	keys := make([]string, 0, len(aliases))
	for alias := range aliases {
		keys = append(keys, alias)
	}
	sort.Strings(keys)
	for _, alias := range keys {
		target := aliases[alias]
		if previous, ok := seen[target]; ok {
			errs = append(errs, ValidationError{
				Path:    path + "." + alias,
				Message: fmt.Sprintf("alias target %q is already used by alias %q", target, previous),
			})
			continue
		}
		seen[target] = alias
	}
	return errs
}

func validateWorker(path, workerID string, workerSet map[string]bool) ValidationErrors {
	if workerID == "" {
		return ValidationErrors{{Path: path, Message: "worker is required"}}
	}
	if !workerSet[workerID] {
		return ValidationErrors{{Path: path, Message: fmt.Sprintf("worker %q not found in workers/", workerID)}}
	}
	return nil
}
