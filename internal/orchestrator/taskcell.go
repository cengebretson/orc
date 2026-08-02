package orchestrator

import (
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/state"
)

func resolveTaskCell(root, cwd string, s *state.State, meta mux.Metadata) (mux.TaskCellSpec, bool) {
	cfg, err := config.Load(root)
	if err != nil || cfg.Settings.Herdr == nil || cfg.Settings.Herdr.TaskCell == nil {
		return mux.TaskCellSpec{}, false
	}
	settings := cfg.Settings.Herdr.TaskCell
	testCommand := strings.TrimSpace(settings.TestCommand)
	watchCommand := ""
	if settings.Watch {
		watchCommand = "orc --workspace " + shellQuote(root) + " --mux herdr watch " + shellQuote(s.Ticket)
	}
	if testCommand == "" && watchCommand == "" {
		return mux.TaskCellSpec{}, false
	}
	return mux.TaskCellSpec{
		CWD: cwd, TestCommand: testCommand, WatchCommand: watchCommand, Metadata: meta,
	}, true
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
