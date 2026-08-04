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
	Mux      *MuxRuntime      `yaml:"mux,omitempty"`
	Tmux     *TmuxRuntime     `yaml:"tmux,omitempty"`
	Agent    *AgentRuntime    `yaml:"agent,omitempty"`
	JIT      *JITRuntime      `yaml:"jit,omitempty"`
	Question *QuestionRuntime `yaml:"question,omitempty"`
}

// MuxRuntime records the exact target created by a terminal multiplexer.
// Workspace, tab, and pane are opaque backend identifiers; labels such as a
// ticket slug or stage name are presentation and must not be used as identity.
type MuxRuntime struct {
	Backend   string `yaml:"backend"`
	Workspace string `yaml:"workspace"`
	Tab       string `yaml:"tab,omitempty"`
	Pane      string `yaml:"pane,omitempty"`
}

// AgentRuntime identifies the durable Orc agent and its current live launch.
// ID survives provider resumes; Instance changes whenever the process is
// replaced, even when the multiplexer pane itself survives.
type AgentRuntime struct {
	ID                string `yaml:"id"`
	Instance          string `yaml:"instance"`
	Stage             string `yaml:"stage,omitempty"`
	Engine            string `yaml:"engine,omitempty"`
	ProviderSessionID string `yaml:"provider_session,omitempty"`
}

type TmuxRuntime struct {
	Session string `yaml:"session"`
	Pane    string `yaml:"pane,omitempty"`
}

// MuxTarget returns the backend-neutral live target. Legacy runtime.tmux state
// is projected as a tmux target without mutating the loaded state.
func (r Runtime) MuxTarget(stage string) (MuxRuntime, bool) {
	if r.Mux != nil && r.Mux.Backend != "" && r.Mux.Workspace != "" {
		target := *r.Mux
		if target.Tab == "" {
			target.Tab = stage
		}
		return target, true
	}
	if r.Tmux != nil && r.Tmux.Session != "" {
		return MuxRuntime{
			Backend:   "tmux",
			Workspace: r.Tmux.Session,
			Tab:       stage,
			Pane:      r.Tmux.Pane,
		}, true
	}
	return MuxRuntime{}, false
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
