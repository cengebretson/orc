package config

import (
	"fmt"
	"strings"
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
			}
		}
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
