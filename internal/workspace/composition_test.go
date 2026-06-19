package workspace

import (
	"strings"
	"testing"
)

func TestValidatePackComposition_AllowsDisjointPacks(t *testing.T) {
	manifests := []PackManifestV1{
		{
			Name: "default",
			Provides: struct {
				Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
				Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
				Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
			}{
				Workflows: []PackResource{{ID: "default:standard", Path: "workflow.yaml"}},
				Workers:   []PackResource{{ID: "default:bob", Path: "workers/bob.md"}},
				Stages:    []PackResource{{ID: "default:develop", Path: "stages/develop.md"}},
			},
			Aliases: PackAliases{
				Workflows: map[string]string{"default": "default:standard"},
			},
		},
		{
			Name: "hotfix",
			Provides: struct {
				Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
				Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
				Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
			}{
				Workflows: []PackResource{{ID: "hotfix:standard", Path: "workflow.yaml"}},
				Workers:   []PackResource{{ID: "hotfix:bob", Path: "workers/bob.md"}},
				Stages:    []PackResource{{ID: "hotfix:develop", Path: "stages/develop.md"}},
			},
			Aliases: PackAliases{
				Workflows: map[string]string{"hotfix": "hotfix:standard"},
			},
		},
	}

	if err := validatePackComposition([]string{"default", "hotfix"}, manifests); err != nil {
		t.Fatalf("validatePackComposition: %v", err)
	}
}

func TestValidatePackComposition_RejectsDuplicateWorkerIDs(t *testing.T) {
	manifests := []PackManifestV1{
		{
			Name: "default",
			Provides: struct {
				Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
				Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
				Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
			}{
				Workers: []PackResource{{ID: "default:bob", Path: "workers/bob.md"}},
			},
		},
		{
			Name: "hotfix",
			Provides: struct {
				Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
				Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
				Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
			}{
				Workers: []PackResource{{ID: "default:bob", Path: "workers/bob.md"}},
			},
		},
	}

	err := validatePackComposition([]string{"default", "hotfix"}, manifests)
	if err == nil {
		t.Fatal("validatePackComposition returned nil error")
	}
	if got := err.Error(); got == "" || got == "pack composition conflicts:\n  " {
		t.Fatal("validatePackComposition returned empty conflict message")
	}
}

func TestValidatePackComposition_AllowsSharedAliasTarget(t *testing.T) {
	manifests := []PackManifestV1{
		{
			Name: "default",
			Provides: struct {
				Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
				Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
				Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
			}{
				Workflows: []PackResource{{ID: "default:standard", Path: "workflow.yaml"}},
			},
			Aliases: PackAliases{
				Workflows: map[string]string{"default": "default:standard"},
			},
		},
		{
			Name: "hotfix",
			Provides: struct {
				Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
				Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
				Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
			}{
				Workflows: []PackResource{{ID: "hotfix:standard", Path: "workflow.yaml"}},
			},
			Aliases: PackAliases{
				Workflows: map[string]string{"hotfix": "hotfix:standard"},
			},
		},
	}

	if err := validatePackComposition([]string{"default", "hotfix"}, manifests); err != nil {
		t.Fatalf("validatePackComposition: %v", err)
	}
}

func TestAssembleEmbeddedPackOrcYAML_PreservesBaseDefaultForMultiplePacks(t *testing.T) {
	manifest := PackManifestV1{
		Name: "default",
		Provides: struct {
			Workflows []PackResource `json:"workflows,omitempty" yaml:"workflows"`
			Workers   []PackResource `json:"workers,omitempty" yaml:"workers"`
			Stages    []PackResource `json:"stages,omitempty" yaml:"stages"`
		}{
			Workflows: []PackResource{{ID: "default:standard", Path: "workflow.yaml"}},
		},
	}

	got, err := assembleEmbeddedPackOrcYAML("settings:\n  default_workflow: default\n", []string{"default", "default"}, []PackManifestV1{manifest, manifest})
	if err != nil {
		t.Fatalf("assembleEmbeddedPackOrcYAML: %v", err)
	}
	if want := "default_workflow: default"; !strings.Contains(got, want) {
		t.Fatalf("assembled orc.yaml changed default workflow for multiple packs:\n%s", got)
	}
}

func TestValidatePackComposition_RejectsConflictingAliasTargets(t *testing.T) {
	manifests := []PackManifestV1{
		{
			Name: "default",
			Aliases: PackAliases{
				Workers: map[string]string{"bob": "default:bob"},
			},
		},
		{
			Name: "hotfix",
			Aliases: PackAliases{
				Workers: map[string]string{"bob": "hotfix:bob"},
			},
		},
	}

	err := validatePackComposition([]string{"default", "hotfix"}, manifests)
	if err == nil {
		t.Fatal("validatePackComposition returned nil error")
	}
	if got := err.Error(); !strings.Contains(got, `worker alias "bob" points to both`) {
		t.Fatalf("unexpected conflict message: %s", got)
	}
}
