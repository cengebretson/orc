package orchestrator

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
)

type LaunchMode string

const (
	LaunchModeTmux       LaunchMode = "tmux"
	LaunchModeForeground LaunchMode = "foreground"
)

type LaunchResult struct {
	Mode            LaunchMode
	Session         string
	Window          string
	Pane            string
	AttachHint      string
	Fallbacks       []string
	HistoryWarnings []string
}

type LaunchOptions struct {
	Root       string
	FeatureDir string
	State      *state.State
	Plan       *runner.Plan
	Window     string
	In         io.Reader
	Out        io.Writer
	Err        io.Writer

	DisableTmux         bool
	RequireExistingTmux bool

	OnFallback       func(message string)
	OnHistoryWarning func(message string)
	OnTmuxSend       func(session, window string)
	OnForeground     func()
}

type Launcher struct {
	// Mux is the multiplexer the launcher drives. It replaces the seven
	// separate tmux function fields this struct used to carry — session
	// existence, creation, send, both metadata stamps, availability, and the
	// attach hint were always the same backend, and splitting them let a caller
	// assemble a launcher from mismatched halves.
	Mux mux.Backend

	SetRuntime       func(featureDir, tmuxSession string) error
	SetRuntimeTarget func(featureDir, tmuxSession, pane string) error
	AppendHistory    func(featureDir, stage, workerID, result string) error
	RunForeground    func(opts LaunchOptions) error
}

func NewLauncher() Launcher {
	return Launcher{
		Mux:              tmux.New(),
		SetRuntime:       state.SetRuntime,
		SetRuntimeTarget: state.SetRuntimeTarget,
		AppendHistory:    state.AppendHistory,
		RunForeground:    runForeground,
	}
}

func Launch(opts LaunchOptions) (*LaunchResult, error) {
	launcher := NewLauncher()
	return launcher.Launch(opts)
}

func (l Launcher) Launch(opts LaunchOptions) (*LaunchResult, error) {
	if opts.State == nil {
		return nil, fmt.Errorf("state is required")
	}
	if opts.Plan == nil {
		return nil, fmt.Errorf("plan is required")
	}

	if l.Mux == nil {
		l.Mux = tmux.New()
	}
	if l.SetRuntime == nil {
		l.SetRuntime = state.SetRuntime
	}
	if l.AppendHistory == nil {
		l.AppendHistory = state.AppendHistory
	}
	if l.RunForeground == nil {
		l.RunForeground = runForeground
	}

	result := &LaunchResult{
		Mode:   LaunchModeForeground,
		Window: opts.State.Stage.Name,
	}
	if opts.Window != "" {
		result.Window = opts.Window
	}

	if !opts.DisableTmux && l.Mux.Available() {
		session := opts.State.Slug
		if opts.State.Runtime.Tmux != nil {
			session = opts.State.Runtime.Tmux.Session
		}
		result.Session = session
		tmuxReady := false

		if opts.RequireExistingTmux {
			if opts.State.Runtime.Tmux != nil && l.Mux.SessionExists(session) {
				tmuxReady = true
			}
		} else if opts.State.Runtime.Tmux == nil {
			// No runtime recorded. A session named for the slug may still exist —
			// e.g. a prior launch created it but could not persist runtime. Adopt
			// it rather than re-creating (which would fail as a duplicate and
			// needlessly drop to foreground).
			if l.Mux.SessionExists(session) {
				if err := l.SetRuntime(opts.FeatureDir, session); err != nil {
					result.addFallback(opts, fmt.Sprintf("warning: could not write runtime to STATE.yaml: %v", err))
				}
				tmuxReady = true
			} else if err := l.Mux.CreateSession(session, opts.FeatureDir, stageNamesForTicket(opts.Root, opts.State)); err != nil {
				result.addFallback(opts, fmt.Sprintf("tmux session create failed (%v)", err))
			} else if err := l.SetRuntime(opts.FeatureDir, session); err != nil {
				result.addFallback(opts, fmt.Sprintf("warning: could not write runtime to STATE.yaml: %v", err))
				tmuxReady = true
			} else {
				tmuxReady = true
			}
		} else if !l.Mux.SessionExists(session) {
			if err := l.Mux.CreateSession(session, opts.FeatureDir, stageNamesForTicket(opts.Root, opts.State)); err != nil {
				result.addFallback(opts, fmt.Sprintf("tmux session recreate failed (%v)", err))
			} else {
				tmuxReady = true
			}
		} else {
			tmuxReady = true
		}

		if tmuxReady {
			if opts.OnTmuxSend != nil {
				opts.OnTmuxSend(session, result.Window)
			}
			pane := ""
			if opts.State.Runtime.Tmux != nil {
				pane = opts.State.Runtime.Tmux.Pane
			}
			sentPane, err := l.Mux.SendCommand(session, result.Window, pane, opts.FeatureDir, opts.Plan.CWD, opts.Plan.LaunchArgv)
			if err != nil {
				result.addFallback(opts, fmt.Sprintf("tmux send failed (%v)", err))
			} else {
				result.Pane = sentPane
				// SendCommand creates missing windows (notably the JIT window), so
				// stamp metadata only after the exact target is guaranteed to exist.
				metadata := windowMetadata(opts, result.Window)
				if err := l.Mux.SetWindowMetadata(session, result.Window, metadata); err != nil {
					result.addFallback(opts, fmt.Sprintf("warning: could not set tmux metadata: %v", err))
				}
				if err := l.Mux.SetPaneMetadata(sentPane, metadata); err != nil {
					result.addFallback(opts, fmt.Sprintf("warning: could not set tmux pane metadata: %v", err))
				}
				if l.SetRuntimeTarget != nil {
					if err := l.SetRuntimeTarget(opts.FeatureDir, session, sentPane); err != nil {
						result.addFallback(opts, fmt.Sprintf("warning: could not write tmux pane to STATE.yaml: %v", err))
					}
				}
				result.Mode = LaunchModeTmux
				result.AttachHint = l.Mux.AttachHint(session, result.Window)
				l.recordLaunch(opts, result, fmt.Sprintf("launched in tmux session %s:%s", session, result.Window))
				return result, nil
			}
		}
	}

	if opts.OnForeground != nil {
		opts.OnForeground()
	}
	if err := l.RunForeground(opts); err != nil {
		l.recordLaunch(opts, result, fmt.Sprintf("launch failed in foreground: %v", err))
		return result, err
	}
	l.recordLaunch(opts, result, "launched in foreground")
	return result, nil
}

func windowMetadata(opts LaunchOptions, window string) mux.Metadata {
	metadata := mux.Metadata{
		Ticket:     opts.State.Ticket,
		Stage:      window,
		FeatureDir: opts.FeatureDir,
	}
	if opts.Plan.Worker != nil {
		metadata.Worker = opts.Plan.Worker.ID
		if metadata.Worker == "" {
			metadata.Worker = opts.Plan.Worker.Name
		}
		metadata.Engine = opts.Plan.Worker.Engine
	}
	return metadata
}

func (r *LaunchResult) addFallback(opts LaunchOptions, message string) {
	r.Fallbacks = append(r.Fallbacks, message)
	if opts.OnFallback != nil {
		opts.OnFallback(message)
	}
}

func (l Launcher) recordLaunch(opts LaunchOptions, result *LaunchResult, message string) {
	if opts.FeatureDir == "" {
		return
	}
	stage := opts.Plan.Stage
	if stage == "" {
		stage = result.Window
	}
	workerID := ""
	if opts.Plan.Worker != nil {
		workerID = opts.Plan.Worker.ID
	}
	if err := l.AppendHistory(opts.FeatureDir, stage, workerID, message); err != nil {
		warning := fmt.Sprintf("warning: could not record launch history: %v", err)
		result.HistoryWarnings = append(result.HistoryWarnings, warning)
		if opts.OnHistoryWarning != nil {
			opts.OnHistoryWarning(warning)
		}
	}
}

func stageNamesForTicket(root string, s *state.State) []string {
	workflowCfg, err := config.Load(root)
	if err != nil {
		return nil
	}
	workflow := s.Workflow
	if workflow == "" {
		workflow = workflowCfg.DefaultWorkflow()
	}
	return workflowCfg.StageNames(workflow)
}

func runForeground(opts LaunchOptions) error {
	c := exec.Command(opts.Plan.LaunchArgv[0], opts.Plan.LaunchArgv[1:]...)
	c.Stdin = opts.In
	if c.Stdin == nil {
		c.Stdin = os.Stdin
	}
	c.Stdout = opts.Out
	if c.Stdout == nil {
		c.Stdout = os.Stdout
	}
	c.Stderr = opts.Err
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
	c.Dir = opts.Plan.CWD
	return c.Run()
}
