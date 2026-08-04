package main

import (
	"fmt"

	"github.com/cengebretson/orc/internal/herdr"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
)

// muxBackend is the terminal multiplexer the CLI drives. Commands go through
// this rather than calling internal/tmux directly, so the choice of
// multiplexer is made in one place instead of at every call site.
//
// It is a variable rather than a constructor call so tests can substitute a
// fake without threading a backend through every command handler.
var muxBackend mux.Backend = tmux.New()

func selectMuxBackend(name string) error {
	switch name {
	case "", "tmux":
		muxBackend = tmux.New()
	case "herdr":
		muxBackend = herdr.New()
	default:
		return fmt.Errorf("unknown multiplexer %q (use tmux or herdr)", name)
	}
	return nil
}

func selectMuxForState(s *state.State) error {
	if globalMux != "" || s == nil {
		return nil
	}
	target, ok := s.Runtime.MuxTarget(s.Stage.Name)
	if !ok || target.Backend == "" || (muxBackend != nil && muxBackend.Name() == target.Backend) {
		return nil
	}
	return selectMuxBackend(target.Backend)
}

func runtimeTarget(s *state.State) (mux.Target, bool) {
	if s == nil {
		return mux.Target{}, false
	}
	target, ok := s.Runtime.MuxTarget(s.Stage.Name)
	if !ok {
		return mux.Target{}, false
	}
	resolved := mux.Target{Backend: target.Backend, Workspace: target.Workspace, Tab: target.Tab, Pane: target.Pane}
	if s.Runtime.Agent != nil {
		resolved.AgentID = s.Runtime.Agent.ID
		resolved.AgentInstance = s.Runtime.Agent.Instance
	}
	return resolved, true
}
