// Package muxtest provides a configurable mux.Backend for tests.
//
// Every method is backed by an optional function field. A nil field returns the
// benign default — an absent multiplexer, an empty inventory, a successful
// no-op write — so a test sets only the handful of methods it actually cares
// about instead of stubbing the whole interface.
package muxtest

import (
	"os/exec"

	"github.com/cengebretson/orc/internal/mux"
)

// Fake is a mux.Backend whose behavior is supplied per test. The zero value is
// usable: it reports the multiplexer as unavailable and every read as empty.
type Fake struct {
	NameFunc                  func() string
	AvailableFunc             func() bool
	CreateSessionFunc         func(name, dir string, windows []string) error
	SessionExistsFunc         func(name string) bool
	KillSessionFunc           func(name string) error
	ListSessionsFunc          func() []string
	ListPanesFunc             func() ([]mux.Pane, error)
	ResolvePaneFunc           func(session, window string) (string, error)
	SetWindowMetadataFunc     func(session, window string, meta mux.Metadata) error
	SetPaneMetadataFunc       func(pane string, meta mux.Metadata) error
	SetSessionEnvironmentFunc func(session, name, value string) error
	AttentionFunc             func(session, window string) string
	SendCommandFunc           func(session, window, pane, dir, runDir string, argv []string) (string, error)
	AttachSessionFunc         func(target string) error
	AttachPaneFunc            func(session, window, pane string) error
	AttachCommandFunc         func(session, window, pane string) (*exec.Cmd, error)
	AttachHintFunc            func(session, window string) string
}

var _ mux.Backend = (*Fake)(nil)

// Available reports the multiplexer as present. Unlike the other defaults this
// one is opt-in: a test that wants the multiplexer used must say so, which
// keeps "no multiplexer" the behavior a test gets by accident rather than the
// one it gets by surprise.
func (f *Fake) Available() bool {
	if f.AvailableFunc == nil {
		return false
	}
	return f.AvailableFunc()
}

func (f *Fake) Name() string {
	if f.NameFunc == nil {
		return "fake"
	}
	return f.NameFunc()
}

func (f *Fake) CreateSession(name, dir string, windows []string) error {
	if f.CreateSessionFunc == nil {
		return nil
	}
	return f.CreateSessionFunc(name, dir, windows)
}

func (f *Fake) SessionExists(name string) bool {
	if f.SessionExistsFunc == nil {
		return false
	}
	return f.SessionExistsFunc(name)
}

func (f *Fake) KillSession(name string) error {
	if f.KillSessionFunc == nil {
		return nil
	}
	return f.KillSessionFunc(name)
}

func (f *Fake) ListSessions() []string {
	if f.ListSessionsFunc == nil {
		return nil
	}
	return f.ListSessionsFunc()
}

func (f *Fake) ListPanes() ([]mux.Pane, error) {
	if f.ListPanesFunc == nil {
		return nil, nil
	}
	return f.ListPanesFunc()
}

func (f *Fake) ResolvePane(session, window string) (string, error) {
	if f.ResolvePaneFunc == nil {
		return "", nil
	}
	return f.ResolvePaneFunc(session, window)
}

func (f *Fake) SetWindowMetadata(session, window string, meta mux.Metadata) error {
	if f.SetWindowMetadataFunc == nil {
		return nil
	}
	return f.SetWindowMetadataFunc(session, window, meta)
}

func (f *Fake) SetPaneMetadata(pane string, meta mux.Metadata) error {
	if f.SetPaneMetadataFunc == nil {
		return nil
	}
	return f.SetPaneMetadataFunc(pane, meta)
}

func (f *Fake) SetSessionEnvironment(session, name, value string) error {
	if f.SetSessionEnvironmentFunc == nil {
		return nil
	}
	return f.SetSessionEnvironmentFunc(session, name, value)
}

func (f *Fake) Attention(session, window string) string {
	if f.AttentionFunc == nil {
		return ""
	}
	return f.AttentionFunc(session, window)
}

func (f *Fake) SendCommand(session, window, pane, dir, runDir string, argv []string) (string, error) {
	if f.SendCommandFunc == nil {
		return "", nil
	}
	return f.SendCommandFunc(session, window, pane, dir, runDir, argv)
}

func (f *Fake) AttachSession(target string) error {
	if f.AttachSessionFunc == nil {
		return nil
	}
	return f.AttachSessionFunc(target)
}

func (f *Fake) AttachPane(session, window, pane string) error {
	if f.AttachPaneFunc == nil {
		return nil
	}
	return f.AttachPaneFunc(session, window, pane)
}

func (f *Fake) AttachCommand(session, window, pane string) (*exec.Cmd, error) {
	if f.AttachCommandFunc == nil {
		return nil, nil
	}
	return f.AttachCommandFunc(session, window, pane)
}

func (f *Fake) AttachHint(session, window string) string {
	if f.AttachHintFunc == nil {
		return session + ":" + window
	}
	return f.AttachHintFunc(session, window)
}
