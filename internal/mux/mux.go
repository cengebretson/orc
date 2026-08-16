// Package mux defines the terminal multiplexer seam.
//
// Orc is agent-agnostic by contract — worker behavior is gated on the worker's
// engine field rather than hardcoded — but it has been multiplexer-coupled in
// practice: tmux calls were spread across the orchestrator, watch, dashboard,
// and session packages, so every tmux assumption (user options as the metadata
// store, session:window targeting, send-keys as the input path) was reachable
// from anywhere.
//
// This package holds the vocabulary those callers actually need — a pane, the
// identity metadata Orc stamps onto one, the attention states it reads back —
// and the Backend interface that supplies them. internal/tmux is the first and
// currently only implementation.
//
// The interface is deliberately derived from the calls Orc makes today rather
// than from what a multiplexer could theoretically do. It is a seam, not a
// portability layer, and it should grow only when a caller needs it to.
package mux

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// Attention states an agent (or an external tool) can report for a window.
// STATE.yaml remains authoritative for workflow status; these are live urgency
// hints that refine the display, never a replacement for durable state.
const (
	AttentionInput   = "input"
	AttentionBlocked = "blocked"
	AttentionReview  = "review"
	AttentionDone    = "done"
)

const (
	LifecycleIdle    = "idle"
	LifecycleWorking = "working"
	LifecycleBlocked = "blocked"
	LifecycleDone    = "done"
	LifecycleUnknown = "unknown"
)

// Where a lifecycle or attention value came from. Per ADR-0003 only
// registration is authoritative: the agent reported its own state, either
// through Orc's provider hooks or a backend's native agent API.
const (
	SourceHook   = "hook"   // Orc's provider lifecycle hooks
	SourceNative = "native" // a backend's own agent API (herdr)
	SourceLaunch = "launch" // Orc's reset when it starts an agent, not a report
	SourceTitle  = "title"  // inferred from the terminal title
	SourceScreen = "screen" // inferred from pane content
)

// IsRegisteredSource reports whether a value may be acted on rather than only
// displayed: satisfying an `orc ctl` wait, waking parked work, and standing as
// a pane's live lifecycle.
//
// Not stage advancement. A stage advances when an agent or a human runs
// `orc mark <ticket> next`; a stage's `advance: auto|manual` declares which of
// those the stage docs should ask for, and no signal gated here can move a
// pipeline on its own.
//
// Deliberately a positive list. The gates this replaces named the sources known
// to be untrustworthy, which made "authoritative" the default for anything
// unrecognized: a marker written by tmux-attention, a future backend, or a user
// script silently qualified. Sources do arrive from outside Orc — the
// tmux-attention CLI takes a free-text --source, so the value is self-asserted
// and cannot serve as a trust boundary. Naming the two Orc can actually verify
// keeps the default safe as new ones appear.
func IsRegisteredSource(source string) bool {
	return source == SourceHook || source == SourceNative
}

const (
	EnvAgentID       = "ORC_AGENT_ID"
	EnvAgentInstance = "ORC_AGENT_INSTANCE"
	EnvFeatureDir    = "ORC_FEATURE_DIR"
)

// EnvResumedFrom names the environment variable Orc sets when a session is
// recreated from a parked snapshot, so the relaunched provider can resume its
// own prior session rather than starting cold.
const EnvResumedFrom = "ORC_RESUMED_FROM"

// Metadata identifies the Orc work running in a window or pane. STATE.yaml
// remains authoritative; a backend stores this to provide a live reverse
// lookup from a terminal back to the ticket that owns it.
//
// Backends are not required to store it as tmux-style user options — only to
// return what they were given through ListPanes.
type Metadata struct {
	AgentID           string
	AgentInstance     string
	Ticket            string
	Stage             string
	Workflow          string
	Repository        string
	Branch            string
	NextAction        string
	Worker            string
	Engine            string
	Model             string
	ProviderSessionID string
	FeatureDir        string
}

// Target is an exact backend-owned terminal location. Identifiers are opaque:
// callers persist and return them but never derive them from labels.
type Target struct {
	Backend       string `json:"backend"`
	Workspace     string `json:"workspace"`
	Tab           string `json:"tab,omitempty"`
	Pane          string `json:"pane,omitempty"`
	AgentID       string `json:"agent_id,omitempty"`
	AgentInstance string `json:"agent_instance,omitempty"`
}

// WorktreeTargetSpec describes a backend-native workspace rooted in a Git
// worktree. SourceDir is the existing checkout Herdr should derive from;
// WorktreeDir is the checkout Orc will record and later remove through its
// normal ownership-aware archive path.
type WorktreeTargetSpec struct {
	Name        string
	Repository  string
	SourceDir   string
	WorktreeDir string
	Branch      string
	Tabs        []string
}

// TaskCellSpec describes optional utility panes arranged beside an agent.
// Commands are user-configured shell text; the backend is responsible for
// targeting only panes it can prove Orc owns and for preserving user focus.
type TaskCellSpec struct {
	CWD          string
	TestCommand  string
	WatchCommand string
	Metadata     Metadata
}

// Notification is a backend-native attention message. Sound is semantic:
// "request" asks for human input, "done" announces completion, and "none"
// suppresses an audible cue when the backend supports those distinctions.
type Notification struct {
	Title string
	Body  string
	Sound string
}

// AgentControlOptions configures a backend-native lifecycle wait. Until is a
// list of exact backend lifecycle states. An empty list means the backend's
// documented settled states.
type AgentControlOptions struct {
	Until   []string
	Timeout time.Duration
	Context context.Context
}

// AgentControlResult is the structured live state returned by a state read,
// prompt, or wait. Target remains the exact opaque location recorded by Orc.
type AgentControlResult struct {
	Backend        string `json:"backend"`
	Target         Target `json:"target"`
	Agent          string `json:"agent,omitempty"`
	Name           string `json:"name,omitempty"`
	Lifecycle      string `json:"lifecycle"`
	StateChangeSeq uint64 `json:"state_change_seq,omitempty"`
}

// AgentEvent is one authoritative lifecycle transition emitted by a supported
// provider hook from the exact tmux pane hosting the agent.
type AgentEvent struct {
	AgentID           string
	AgentInstance     string
	Engine            string
	ProviderSessionID string
	Lifecycle         string
	EventID           string
}

// AgentEventResult reports the committed lifecycle sequence for hook
// diagnostics and idempotence tests.
type AgentEventResult struct {
	Target         Target `json:"target"`
	Lifecycle      string `json:"lifecycle"`
	StateChangeSeq uint64 `json:"state_change_seq"`
	Duplicate      bool   `json:"duplicate,omitempty"`
	FeatureDir     string `json:"feature_dir,omitempty"`
	Notify         bool   `json:"notify,omitempty"`
}

// AgentControlError preserves a backend's stable automation error code, such
// as Herdr's agent_prompt_stalled or timeout.
type AgentControlError struct {
	Backend string
	Code    string
	Message string
}

func (e *AgentControlError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return fmt.Sprintf("%s agent control: %s", e.Backend, e.Code)
	}
	return fmt.Sprintf("%s agent control: %s: %s", e.Backend, e.Code, e.Message)
}

// AttentionRank orders attention states by how much they need a human, most
// urgent first. An unrecognized or empty state ranks last.
//
// The order is deliberate: blocked work has stopped and cannot continue,
// input is stopped but answerable, review is finished work waiting on a
// decision, and done needs nothing. Anything a caller does not recognize is
// treated as no signal rather than guessed at.
func AttentionRank(state string) int {
	switch state {
	case AttentionBlocked:
		return 0
	case AttentionInput:
		return 1
	case AttentionReview:
		return 2
	case AttentionDone:
		return 3
	default:
		return 4
	}
}

// RollUpAttention reduces the panes of one window to the single attention
// state that window should display, and the time that state began.
//
// A window can host more than one agent — a stage agent beside a jit task, or
// a split the user made themselves — and they report independently. The most
// urgent state wins, so a window is never shown as done while something in it
// is blocked. Ties go to the *earliest* timestamp, so the elapsed time tracks
// the agent that has been waiting longest rather than whichever most recently
// changed.
//
// Returns the empty string when no pane reports a recognized state.
func RollUpAttention(panes []Pane) (state string, since int64) {
	for _, pane := range panes {
		rank := AttentionRank(pane.Attention)
		if rank == AttentionRank("") {
			continue
		}
		if state == "" || rank < AttentionRank(state) {
			state, since = pane.Attention, pane.AttentionSince
			continue
		}
		// Same urgency: prefer the one that has been in it longest. A zero
		// timestamp means the reporter did not say, which must not win a tie
		// against a real one by looking like the distant past.
		if rank == AttentionRank(state) && pane.AttentionSince != 0 {
			if since == 0 || pane.AttentionSince < since {
				since = pane.AttentionSince
			}
		}
	}
	return state, since
}

// Pane describes one terminal pane: the process running in it, and whatever
// Orc metadata the backend has stamped on it.
type Pane struct {
	Backend           string `json:"backend,omitempty"`
	ID                string `json:"id"`
	AgentID           string `json:"agent_id,omitempty"`
	AgentInstance     string `json:"agent_instance,omitempty"`
	Session           string `json:"session"`
	Window            string `json:"window"`
	CWD               string `json:"cwd,omitempty"`
	Command           string `json:"command,omitempty"`
	PID               int    `json:"pid,omitempty"`
	Agent             bool   `json:"agent"`
	Ticket            string `json:"ticket,omitempty"`
	Stage             string `json:"stage,omitempty"`
	Worker            string `json:"worker,omitempty"`
	Engine            string `json:"engine,omitempty"`
	ProviderEngine    string `json:"provider_engine,omitempty"`
	ProviderSessionID string `json:"provider_session_id,omitempty"`
	FeatureDir        string `json:"feature_dir,omitempty"`
	Attention         string `json:"attention,omitempty"`
	AttentionSource   string `json:"attention_source,omitempty"`
	Lifecycle         string `json:"lifecycle,omitempty"`
	LifecycleSource   string `json:"lifecycle_source,omitempty"`
	StateChangeSeq    uint64 `json:"state_change_seq,omitempty"`
	SeenSeq           uint64 `json:"seen_seq,omitempty"`
	LifecycleSince    int64  `json:"lifecycle_since,omitempty"`
	Title             string `json:"title,omitempty"`
	DisplayTitle      string `json:"display_title,omitempty"`
	ObservedLifecycle string `json:"observed_lifecycle,omitempty"`
	ObservationSource string `json:"observation_source,omitempty"`
	ObservationRule   string `json:"observation_rule,omitempty"`
	ObservationSince  int64  `json:"observation_since,omitempty"`
	// AttentionSince is when the pane entered its current attention state, in
	// epoch seconds. Zero means the reporter did not say — treat it as unknown
	// rather than as the epoch.
	AttentionSince int64 `json:"attention_since,omitempty"`
}

// Backend is a terminal multiplexer Orc can drive.
//
// Implementations must treat an absent multiplexer as an empty inventory
// rather than an error: Available reports false, ListPanes and ListSessions
// return nothing, and SessionExists reports false. Orc's read paths run on
// every dashboard refresh and must not fail because no server is running.
type Backend interface {
	// Name identifies the backend in diagnostics and in STATE.yaml.
	Name() string

	// Available reports whether this multiplexer is installed and usable.
	Available() bool

	// CreateSession creates a detached session rooted at dir, with one window
	// per entry in windows.
	CreateSession(name, dir string, windows []string) error

	// SessionExists reports whether a session of this name is live.
	SessionExists(name string) bool

	// KillSession terminates a session and the processes inside it.
	KillSession(name string) error

	// ListSessions returns every live session name, or nil when there are none.
	ListSessions() []string

	// ListPanes returns every live pane with the Orc metadata stamped on it.
	// A missing server is the empty inventory, not an error.
	ListPanes() ([]Pane, error)

	// ResolvePane returns the pane a command or attach should target in the
	// given window. Ambiguous windows are an error rather than a guess — the
	// wrong pane means keystrokes land in someone else's agent.
	ResolvePane(session, window string) (string, error)

	// SetWindowMetadata stamps Orc identity onto a window.
	SetWindowMetadata(session, window string, meta Metadata) error

	// SetPaneMetadata stamps Orc identity onto the exact agent pane.
	SetPaneMetadata(pane string, meta Metadata) error

	// SetSessionEnvironment records a variable in the session environment, for
	// processes the session starts later.
	SetSessionEnvironment(session, name, value string) error

	// Attention returns the live attention state for a window, or the empty
	// string when none is set. See the Attention* constants.
	Attention(session, window string) string

	// SendCommand runs argv in the target pane, creating the window if needed,
	// and returns the pane it landed in. An empty pane resolves through
	// ResolvePane. runDir is the working directory for the command itself.
	SendCommand(session, window, pane, dir, runDir string, argv []string) (string, error)

	// AttachSession attaches to a session by raw target ("session" or
	// "session:window"), switching instead when already inside the multiplexer.
	AttachSession(target string) error

	// AttachPane attaches with an exact pane focused, validating a stored pane
	// id first so a stale id cannot focus unrelated work.
	AttachPane(session, window, pane string) error

	// AttachCommand builds the command AttachPane would run, for callers that
	// must hand over the terminal themselves (the Bubble Tea paths).
	AttachCommand(session, window, pane string) (*exec.Cmd, error)

	// AttachHint returns the shell command a human would type to attach, for
	// display in launch output and ticket summaries.
	AttachHint(session, window string) string
}

// TargetBackend is the backend-neutral extension used by new launch paths.
// Backend remains embedded so existing tmux-oriented callers can migrate a
// package at a time without losing the safety of a single implementation.
type TargetBackend interface {
	Backend

	CreateTarget(name, dir string, tabs []string) (Target, error)
	SendTarget(target Target, tab, dir, runDir string, argv []string) (Target, error)
	SetTargetMetadata(target Target, meta Metadata) error
	AttachTarget(target Target) error
	AttachTargetHint(target Target) string
}

// AgentLaunchBackend can stamp an exact target and inject identity into a
// provider process before it starts. The returned argv is backend-specific;
// callers must pass it unchanged to SendTarget.
type AgentLaunchBackend interface {
	TargetBackend

	PrepareAgentLaunch(target Target, tab, dir string, meta Metadata, argv []string) (Target, []string, error)
}

// WorktreeTargetBackend is an optional capability for multiplexers that can
// create or reopen a workspace and Git worktree atomically. Backends without
// it continue through CreateTarget unchanged.
type WorktreeTargetBackend interface {
	TargetBackend

	CreateWorktreeTarget(spec WorktreeTargetSpec) (Target, error)
}

// TaskCellBackend is an optional layout capability. It is separate from
// TargetBackend because tmux and foreground launches do not need to adopt
// Herdr's pane topology.
type TaskCellBackend interface {
	TargetBackend

	ConfigureTaskCell(target Target, spec TaskCellSpec) error
}

// NotificationBackend is an optional capability for multiplexers that own a
// native notification surface. Transition callers use it best-effort; durable
// workflow state must never depend on delivery succeeding.
type NotificationBackend interface {
	Backend

	ShowNotification(notification Notification) error
}

// AgentStateBackend is an optional capability for multiplexers with reliable
// authoritative lifecycle detection. Backends must not emulate it by scraping
// screen text.
type AgentStateBackend interface {
	Backend

	StateAgent(target Target) (AgentControlResult, error)
}

// AgentAcknowledgeBackend records that a human explicitly acted on the exact
// agent instance. Reads and terminal captures must never call this capability.
type AgentAcknowledgeBackend interface {
	Backend

	AcknowledgeAgent(target Target) error
}

// AgentWaitBackend extends state reads with a context-cancelable lifecycle
// wait.
type AgentWaitBackend interface {
	AgentStateBackend

	WaitAgent(target Target, options AgentControlOptions) (AgentControlResult, error)
}

// AgentPromptBackend extends lifecycle control with exact-instance prompting.
type AgentPromptBackend interface {
	AgentWaitBackend

	PromptAgent(target Target, text string, wait bool, options AgentControlOptions) (AgentControlResult, error)
}

// AgentControlBackend is the complete control surface retained for callers
// that require state, wait, and prompt together.
type AgentControlBackend interface {
	AgentPromptBackend
}

// TerminalCaptureBackend is an optional capability for reading terminal text
// from an exact recorded target. Captured text is diagnostic content only; it
// must never be interpreted as agent lifecycle state.
const (
	MaxAgentPromptBytes = 64 * 1024
	MaxCaptureLines     = 5000
)

type TerminalCaptureBackend interface {
	Backend

	CaptureTarget(target Target, lines int) (string, error)
}

// FallbackObservationBackend adds presentation-only observations after the
// authoritative pane inventory has been read.
type FallbackObservationBackend interface {
	Backend

	ObserveFallback(root string, panes []Pane) ([]Pane, error)
}
