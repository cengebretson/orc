package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

const Filename = "orc.yaml"

type Repo struct {
	Name    string `yaml:"name"`
	Path    string `yaml:"path"`
	Purpose string `yaml:"purpose"`
}

type Settings struct {
	DefaultWorkflow string   `yaml:"default_workflow"`
	AutoArchive     bool     `yaml:"auto_archive"`
	AutoTmux        bool     `yaml:"auto_tmux"`
	AutoNext        bool     `yaml:"auto_next"`
	TuiRefresh      int      `yaml:"tui_refresh"` // seconds; 0 means use default (60)
	Quotes          []string `yaml:"quotes"`
	Theme           string   `yaml:"theme"` // e.g. "catppuccin-mocha"; defaults to catppuccin-mocha
}

// WorkflowDef is a named sequence of stages.
type WorkflowDef struct {
	Description string     `yaml:"description,omitempty"`
	Stages      []StageDef `yaml:"stages"`
}

type Aliases struct {
	Workflows map[string]string `yaml:"workflows,omitempty"`
	Stages    map[string]string `yaml:"stages,omitempty"`
	Workers   map[string]string `yaml:"workers,omitempty"`
}

// LoopDef configures a loop stage attached to a pipeline stage.
// The loop stage (Via) runs when the owning stage needs to cycle back.
// It is not part of the linear pipeline — only reachable via the loop or orc jit.
type LoopDef struct {
	Via    string `yaml:"via"`
	Worker string `yaml:"worker"`
	Max    int    `yaml:"max"`
	OnMax  string `yaml:"on_max"` // "pause" (default) or "fail"
}

// StageDef is one step in a workflow.
type StageDef struct {
	Name    string   `yaml:"name"`
	Worker  string   `yaml:"worker"`
	Advance string   `yaml:"advance"`
	Loop    *LoopDef `yaml:"loop,omitempty"`
}

type Config struct {
	Repos     []Repo                 `yaml:"repos"`
	Settings  Settings               `yaml:"settings"`
	Aliases   Aliases                `yaml:"aliases,omitempty"`
	Workflows map[string]WorkflowDef `yaml:"workflows"`
}

// TuiRefreshInterval returns the configured auto-refresh interval, defaulting to 60s.
func (c *Config) TuiRefreshInterval() time.Duration {
	if c.Settings.TuiRefresh > 0 {
		return time.Duration(c.Settings.TuiRefresh) * time.Second
	}
	return 60 * time.Second
}

// DefaultWorkflow returns the configured default workflow name, or "" if not set.
func (c *Config) DefaultWorkflow() string {
	return c.Settings.DefaultWorkflow
}

// Names returns all workflow names, sorted.
func (c *Config) Names() []string {
	names := make([]string, 0, len(c.Workflows))
	for k := range c.Workflows {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// Stages returns the ordered StageDefs for the named workflow.
func (c *Config) Stages(name string) []StageDef {
	return c.Workflows[name].Stages
}

// WorkflowDescription returns the human-readable description for the named
// workflow, or "" if none is set.
func (c *Config) WorkflowDescription(name string) string {
	return c.Workflows[name].Description
}

// StageNames returns just the stage names for the named workflow.
func (c *Config) StageNames(name string) []string {
	stages := c.Workflows[name].Stages
	names := make([]string, len(stages))
	for i, s := range stages {
		names[i] = s.Name
	}
	return names
}

// NextStage returns the stage that follows current in the named workflow.
// Returns "" if current is the last stage or not found.
func (c *Config) NextStage(workflowName, current string) string {
	stages := c.Workflows[workflowName].Stages
	for i, s := range stages {
		if s.Name == current && i+1 < len(stages) {
			return stages[i+1].Name
		}
	}
	return ""
}

// StageConfig returns the StageDef for a named stage in a named workflow.
// Also resolves loop stages (stages referenced via Loop.Via on any pipeline stage).
func (c *Config) StageConfig(workflowName, stageName string) (StageDef, bool) {
	for _, s := range c.Workflows[workflowName].Stages {
		if s.Name == stageName {
			return s, true
		}
	}
	// Check if it's a loop stage — return a synthetic StageDef with the loop's worker.
	for _, s := range c.Workflows[workflowName].Stages {
		if s.Loop != nil && s.Loop.Via == stageName {
			return StageDef{Name: stageName, Worker: s.Loop.Worker}, true
		}
	}
	return StageDef{}, false
}

// LoopConfig returns the LoopDef for a stage, if it has one.
func (c *Config) LoopConfig(workflowName, stageName string) (*LoopDef, bool) {
	for _, s := range c.Workflows[workflowName].Stages {
		if s.Name == stageName && s.Loop != nil {
			return s.Loop, true
		}
	}
	return nil, false
}

// IsLoopStage returns true if stageName is a loop stage (referenced via Loop.Via) in the workflow.
func (c *Config) IsLoopStage(workflowName, stageName string) bool {
	for _, s := range c.Workflows[workflowName].Stages {
		if s.Loop != nil && s.Loop.Via == stageName {
			return true
		}
	}
	return false
}

// OwnerStage returns the pipeline stage that owns the given loop stage.
func (c *Config) OwnerStage(workflowName, loopStageName string) (string, bool) {
	for _, s := range c.Workflows[workflowName].Stages {
		if s.Loop != nil && s.Loop.Via == loopStageName {
			return s.Name, true
		}
	}
	return "", false
}

// Load reads orc.yaml from the workspace root.
// Returns an empty Config (not an error) if the file does not exist.
func Load(root string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(root, Filename))
	if os.IsNotExist(err) {
		return &Config{
			Workflows: map[string]WorkflowDef{},
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", Filename, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", Filename, err)
	}
	if cfg.Workflows == nil {
		cfg.Workflows = map[string]WorkflowDef{}
	}
	return &cfg, nil
}
