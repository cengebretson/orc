package state

import (
	"time"
)

func timeNow() string {
	return time.Now().Format(time.RFC3339)
}

const Filename = "STATE.yaml"

const SchemaVersion = 1

const lockTimeout = 5 * time.Second

const staleLockAge = 30 * time.Second

type LockStatus int

const (
	LockMissing LockStatus = iota
	LockActive
	LockStale
)

type LockInfo struct {
	Path   string
	Status LockStatus
	PID    int
	Age    time.Duration
	Detail string
}

type State struct {
	SchemaVersion int    `yaml:"schema_version,omitempty"`
	Ticket        string `yaml:"ticket"`
	Slug          string `yaml:"slug"`
	Status        string `yaml:"status"`
	Workflow      string `yaml:"workflow,omitempty"`

	Stage Stage `yaml:"stage"`

	StageCounts map[string]int `yaml:"stage_counts,omitempty"`

	Runtime Runtime `yaml:"runtime,omitempty"`

	Repos map[string]Repo `yaml:"repos"`

	Inputs  IOSet `yaml:"inputs"`
	Outputs IOSet `yaml:"outputs"`

	NextAction NextAction `yaml:"next_action"`

	History []HistoryEntry `yaml:"history"`
}

type Runtime struct {
	Tmux *TmuxRuntime `yaml:"tmux,omitempty"`
	JIT  *JITRuntime  `yaml:"jit,omitempty"`
}

type TmuxRuntime struct {
	Session string `yaml:"session"`
	Pane    string `yaml:"pane,omitempty"`
}

type JITRuntime struct {
	Worker    string `yaml:"worker"`
	Task      string `yaml:"task"`
	StartedAt string `yaml:"started_at"`
}

type Stage struct {
	Worker string `yaml:"worker"`
	Name   string `yaml:"name"`
}

type Repo struct {
	Main     string `yaml:"main"`
	Worktree string `yaml:"worktree"`
	Branch   string `yaml:"branch"`
}

type IOSet struct {
	Ready     []string `yaml:"ready"`
	Required  []string `yaml:"required"`
	Completed []string `yaml:"completed"`
}

type NextAction struct {
	Worker string `yaml:"worker"`
	Prompt string `yaml:"prompt"`
	CWD    string `yaml:"cwd"`
}

type HistoryEntry struct {
	At     string `yaml:"at"`
	Stage  string `yaml:"stage"`
	Worker string `yaml:"worker"`
	Result string `yaml:"result"`
}

// Load reads STATE.yaml from the given feature directory.
