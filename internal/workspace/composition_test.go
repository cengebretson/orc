package workspace

import "testing"

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
