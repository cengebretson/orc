package tmux

import (
	"os/exec"

	"github.com/cengebretson/orc/internal/mux"
)

// Backend adapts this package to mux.Backend. It holds no state — every method
// delegates to the package-level function that already implements it — so it
// is safe to construct wherever one is needed.
type Backend struct{}

// New returns the tmux implementation of mux.Backend.
func New() Backend { return Backend{} }

var _ mux.Backend = Backend{}
var _ mux.TargetBackend = Backend{}
var _ mux.AgentLaunchBackend = Backend{}
var _ mux.AgentControlBackend = Backend{}
var _ mux.TerminalCaptureBackend = Backend{}

// Name identifies this backend.
func (Backend) Name() string { return "tmux" }

// Available reports whether tmux is installed and in PATH.
func (Backend) Available() bool { return Available() }

// CreateSession creates a detached session with one window per entry.
func (Backend) CreateSession(name, dir string, windows []string) error {
	return CreateSession(name, dir, windows)
}

// SessionExists reports whether the named session is live.
func (Backend) SessionExists(name string) bool { return SessionExists(name) }

// KillSession terminates the named session.
func (Backend) KillSession(name string) error { return KillSession(name) }

// ListSessions returns every live session name.
func (Backend) ListSessions() []string { return ListSessions() }

// ListPanes returns every live pane with its Orc metadata.
func (Backend) ListPanes() ([]mux.Pane, error) { return ListPanesDetailed() }

// ResolvePane returns the one safe pane target in a window.
func (Backend) ResolvePane(session, window string) (string, error) {
	return ResolvePaneTarget(session, window)
}

// SetWindowMetadata stamps Orc identity onto a window.
func (Backend) SetWindowMetadata(session, window string, meta mux.Metadata) error {
	return SetWindowMetadata(session, window, meta)
}

// SetPaneMetadata stamps Orc identity onto an exact pane.
func (Backend) SetPaneMetadata(pane string, meta mux.Metadata) error {
	return SetPaneMetadata(pane, meta)
}

// SetSessionEnvironment records a variable in the session environment.
func (Backend) SetSessionEnvironment(session, name, value string) error {
	return SetSessionEnvironment(session, name, value)
}

// Attention returns the live attention state for a window.
func (Backend) Attention(session, window string) string { return WindowAttention(session, window) }

// SendCommand runs argv in the target pane and returns the pane it landed in.
func (Backend) SendCommand(session, window, pane, dir, runDir string, argv []string) (string, error) {
	return SendCommandTarget(session, window, pane, dir, runDir, argv)
}

// AttachSession attaches to a raw "session" or "session:window" target.
func (Backend) AttachSession(target string) error { return Attach(target) }

// AttachPane attaches with an exact pane focused.
func (Backend) AttachPane(session, window, pane string) error {
	return AttachTarget(session, window, pane)
}

// AttachCommand builds the command AttachPane would run.
func (Backend) AttachCommand(session, window, pane string) (*exec.Cmd, error) {
	return AttachCommandTarget(session, window, pane)
}

// AttachHint returns the shell command a human would type to attach.
func (Backend) AttachHint(session, window string) string { return AttachHint(session, window) }

// CreateTarget creates a tmux session and returns its exact initial target.
func (Backend) CreateTarget(name, dir string, tabs []string) (mux.Target, error) {
	if err := CreateSession(name, dir, tabs); err != nil {
		return mux.Target{}, err
	}
	tab := "shell"
	if len(tabs) > 0 {
		tab = tabs[0]
	}
	pane, err := ResolvePaneTarget(name, tab)
	if err != nil {
		return mux.Target{}, err
	}
	return mux.Target{Backend: "tmux", Workspace: name, Tab: tab, Pane: pane}, nil
}

// SendTarget runs argv at an exact tmux target and returns the resolved pane.
func (Backend) SendTarget(target mux.Target, tab, dir, runDir string, argv []string) (mux.Target, error) {
	if tab == "" {
		tab = target.Tab
	}
	pane, err := SendCommandTarget(target.Workspace, tab, target.Pane, dir, runDir, argv)
	if err != nil {
		return mux.Target{}, err
	}
	return mux.Target{Backend: "tmux", Workspace: target.Workspace, Tab: tab, Pane: pane}, nil
}

func (Backend) SetTargetMetadata(target mux.Target, meta mux.Metadata) error {
	if err := SetWindowMetadata(target.Workspace, target.Tab, meta); err != nil {
		return err
	}
	return SetPaneMetadata(target.Pane, meta)
}

func (Backend) AttachTarget(target mux.Target) error {
	return AttachTarget(target.Workspace, target.Tab, target.Pane)
}

func (Backend) AttachTargetHint(target mux.Target) string {
	return AttachHint(target.Workspace, target.Tab)
}

// CaptureTarget reads recent joined terminal text from an exact recorded pane.
func (Backend) CaptureTarget(target mux.Target, lines int) (string, error) {
	return CaptureTarget(target, lines)
}
