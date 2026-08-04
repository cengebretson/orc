package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"gopkg.in/yaml.v3"
)

const Filename = "orc.yaml"

type Repo struct {
	Name          string   `yaml:"name"`
	Path          string   `yaml:"path"`
	Purpose       string   `yaml:"purpose"`
	WorktreeSetup string   `yaml:"worktree_setup,omitempty"`
	AgentHints    []string `yaml:"agent_hints,omitempty"`
}

// RepoRoute maps unambiguous ticket signals to one or more repositories.
// Workflow selection remains independent under settings/workflows.
type RepoRoute struct {
	Labels     []string `yaml:"labels,omitempty"`
	Components []string `yaml:"components,omitempty"`
	Repos      []string `yaml:"repos"`
}

type Settings struct {
	DefaultWorkflow  string                   `yaml:"default_workflow"`
	ArtifactPolicy   string                   `yaml:"artifact_policy,omitempty"`
	AutoArchive      bool                     `yaml:"auto_archive"`
	AutoTmux         bool                     `yaml:"auto_tmux"`
	AutoNext         bool                     `yaml:"auto_next"`
	WorkspaceRefresh int                      `yaml:"workspace_refresh"` // seconds; 0 means use default (60)
	Quotes           []string                 `yaml:"quotes"`
	Theme            string                   `yaml:"theme"` // e.g. "catppuccin-mocha"; defaults to catppuccin-mocha
	ContextPressure  *ContextPressureSettings `yaml:"context_pressure,omitempty"`
	Notify           NotifySettings           `yaml:"notify,omitempty"`
	Herdr            *HerdrSettings           `yaml:"herdr,omitempty"`
	Parking          *ParkingSettings         `yaml:"parking,omitempty"`
	Rail             *RailSettings            `yaml:"rail,omitempty"`
}

// RailSettings controls presentation-only timing in the tmux Live rail.
type RailSettings struct {
	StuckAfter string `yaml:"stuck_after,omitempty"`
}

// ParkingSettings enables reversible display-only parking in Live views.
type ParkingSettings struct {
	AutoPark []string `yaml:"auto_park,omitempty"`
	WakeOn   []string `yaml:"wake_on,omitempty"`
}

// HerdrSettings contains opt-in UI behavior for the native Herdr backend.
type HerdrSettings struct {
	TaskCell *HerdrTaskCellSettings `yaml:"task_cell,omitempty"`
}

// HerdrTaskCellSettings adds Orc-owned utility panes beside the stage agent.
// TestCommand is passed to the user's shell through `herdr pane run`; Watch
// launches Orc's ticket rail when enabled.
type HerdrTaskCellSettings struct {
	TestCommand string `yaml:"test_command,omitempty"`
	Watch       bool   `yaml:"watch,omitempty"`
}

// NotifySettings configures an optional best-effort command after selected
// workflow events.
type NotifySettings struct {
	On      []string `yaml:"on,omitempty"`
	Command string   `yaml:"command,omitempty"`
}

type ContextPressureSettings struct {
	Green  int `yaml:"green"`
	Yellow int `yaml:"yellow"`
	Red    int `yaml:"red"`
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
	Via               string   `yaml:"via"`
	Worker            string   `yaml:"worker"`
	Max               int      `yaml:"max"`
	OnMax             string   `yaml:"on_max"` // "pause" (default) or "fail"
	RequiredArtifacts []string `yaml:"required_artifacts,omitempty"`
}

// StageDef is one step in a workflow.
type StageDef struct {
	Name              string   `yaml:"name"`
	Worker            string   `yaml:"worker"`
	Advance           string   `yaml:"advance"`
	RequiredArtifacts []string `yaml:"required_artifacts,omitempty"`
	Loop              *LoopDef `yaml:"loop,omitempty"`
}

type Config struct {
	Repos     []Repo                 `yaml:"repos"`
	Routing   []RepoRoute            `yaml:"routing,omitempty"`
	Settings  Settings               `yaml:"settings"`
	Aliases   Aliases                `yaml:"aliases,omitempty"`
	Workflows map[string]WorkflowDef `yaml:"workflows"`
}

// WorkspaceRefreshInterval returns the configured Workspace auto-refresh interval, defaulting to 60s.
func (c *Config) WorkspaceRefreshInterval() time.Duration {
	if c.Settings.WorkspaceRefresh > 0 {
		return time.Duration(c.Settings.WorkspaceRefresh) * time.Second
	}
	return 60 * time.Second
}

// DefaultWorkflow returns the configured default workflow name, or "" if not set.
func (c *Config) DefaultWorkflow() string {
	return c.Settings.DefaultWorkflow
}

func (c *Config) ArtifactPolicy() string {
	if c == nil || c.Settings.ArtifactPolicy == "" {
		return "warn"
	}
	return c.Settings.ArtifactPolicy
}

func (c *Config) ContextPressureThresholds() contextpressure.Thresholds {
	if c == nil || c.Settings.ContextPressure == nil {
		return contextpressure.DefaultThresholds()
	}
	settings := c.Settings.ContextPressure
	thresholds := contextpressure.Thresholds{Green: settings.Green, Yellow: settings.Yellow, Red: settings.Red}
	if !thresholds.Valid() {
		return contextpressure.DefaultThresholds()
	}
	return thresholds
}

// ResolveWorkflow returns the canonical workflow ID for a workflow or alias.
// If the name is not an alias, it returns the input unchanged.
func (c *Config) ResolveWorkflow(name string) string {
	if c == nil {
		return name
	}
	if target, ok := c.Aliases.Workflows[name]; ok {
		return target
	}
	return name
}

// ResolveStage returns the canonical stage ID for a stage or alias.
// If the name is not an alias, it returns the input unchanged.
func (c *Config) ResolveStage(name string) string {
	if c == nil {
		return name
	}
	if target, ok := c.Aliases.Stages[name]; ok {
		return target
	}
	return name
}

// WorkflowDisplayName returns an alias for a canonical workflow ID, or the
// canonical ID if no alias points to it.
func (c *Config) WorkflowDisplayName(id string) string {
	if c == nil {
		return id
	}
	return displayAlias(id, c.Aliases.Workflows)
}

// StageDisplayName returns an alias for a canonical stage ID, or the canonical
// ID if no alias points to it.
func (c *Config) StageDisplayName(id string) string {
	if c == nil {
		return id
	}
	return displayAlias(id, c.Aliases.Stages)
}

// WorkerDisplayName returns an alias for a canonical worker ID, or the canonical
// ID if no alias points to it.
func (c *Config) WorkerDisplayName(id string) string {
	if c == nil {
		return id
	}
	return displayAlias(id, c.Aliases.Workers)
}

func displayAlias(id string, aliases map[string]string) string {
	if len(aliases) == 0 {
		return id
	}
	match := ""
	for alias, target := range aliases {
		if target != id {
			continue
		}
		match = alias
	}
	if match == "" {
		return id
	}
	return match
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

// stagesFor returns the ordered StageDefs for a workflow name or alias. It is
// the single lookup path shared by every stage-oriented accessor below.
func (c *Config) stagesFor(name string) []StageDef {
	return c.Workflows[c.ResolveWorkflow(name)].Stages
}

// Stages returns the ordered StageDefs for the named workflow.
func (c *Config) Stages(name string) []StageDef {
	return c.stagesFor(name)
}

// WorkflowDescription returns the human-readable description for the named
// workflow, or "" if none is set.
func (c *Config) WorkflowDescription(name string) string {
	return c.Workflows[c.ResolveWorkflow(name)].Description
}

// StageNames returns just the stage names for the named workflow.
func (c *Config) StageNames(name string) []string {
	stages := c.stagesFor(name)
	names := make([]string, len(stages))
	for i, s := range stages {
		names[i] = s.Name
	}
	return names
}

// NextStage returns the stage that follows current in the named workflow.
// Returns "" if current is the last stage or not found.
func (c *Config) NextStage(workflowName, current string) string {
	current = c.ResolveStage(current)
	stages := c.stagesFor(workflowName)
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
	stageName = c.ResolveStage(stageName)
	stages := c.stagesFor(workflowName)
	for _, s := range stages {
		if s.Name == stageName {
			return s, true
		}
	}
	// Check if it's a loop stage — return a synthetic StageDef with the loop's worker.
	for _, s := range stages {
		if s.Loop != nil && s.Loop.Via == stageName {
			return StageDef{Name: stageName, Worker: s.Loop.Worker, RequiredArtifacts: s.Loop.RequiredArtifacts}, true
		}
	}
	return StageDef{}, false
}

// LoopConfig returns the LoopDef for a stage, if it has one.
func (c *Config) LoopConfig(workflowName, stageName string) (*LoopDef, bool) {
	stageName = c.ResolveStage(stageName)
	for _, s := range c.stagesFor(workflowName) {
		if s.Name == stageName && s.Loop != nil {
			return s.Loop, true
		}
	}
	return nil, false
}

// IsLoopStage returns true if stageName is a loop stage (referenced via Loop.Via) in the workflow.
func (c *Config) IsLoopStage(workflowName, stageName string) bool {
	stageName = c.ResolveStage(stageName)
	for _, s := range c.stagesFor(workflowName) {
		if s.Loop != nil && s.Loop.Via == stageName {
			return true
		}
	}
	return false
}

// OwnerStage returns the pipeline stage that owns the given loop stage.
func (c *Config) OwnerStage(workflowName, loopStageName string) (string, bool) {
	loopStageName = c.ResolveStage(loopStageName)
	for _, s := range c.stagesFor(workflowName) {
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
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", Filename, err)
	}
	if cfg.Workflows == nil {
		cfg.Workflows = map[string]WorkflowDef{}
	}
	return &cfg, nil
}
