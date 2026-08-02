package main

import (
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/tmux"
)

// muxBackend is the terminal multiplexer the CLI drives. Commands go through
// this rather than calling internal/tmux directly, so the choice of
// multiplexer is made in one place instead of at every call site.
//
// It is a variable rather than a constructor call so tests can substitute a
// fake without threading a backend through every command handler.
var muxBackend mux.Backend = tmux.New()
