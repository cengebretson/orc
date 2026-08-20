package orchestrator

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"

	"github.com/cengebretson/orc/internal/agentidentity"
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/tmuxattention"
	"github.com/cengebretson/orc/internal/workers"
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

	SetMuxRuntime        func(featureDir string, target state.MuxRuntime) error
	SetMuxAgentRuntime   func(featureDir string, target state.MuxRuntime, agent state.AgentRuntime) error
	SetRuntime           func(featureDir, tmuxSession string) error
	SetRuntimeTarget     func(featureDir, tmuxSession, pane string) error
	RecordWorktree       func(featureDir, root string, launch worktreeLaunch) error
	WriteWorktreeContext func(worktreeDir, project, featureSlug string) error
	AppendHistory        func(featureDir, stage, workerID, result string) error
	RunForeground        func(opts LaunchOptions) error
	NewAgentID           func() (string, error)
	NewInstanceID        func() (string, error)
}

func NewLauncher() Launcher {
	return Launcher{
		Mux:                  tmux.New(),
		SetMuxRuntime:        state.SetMuxRuntime,
		SetMuxAgentRuntime:   state.SetMuxAgentRuntime,
		SetRuntime:           state.SetRuntime,
		SetRuntimeTarget:     state.SetRuntimeTarget,
		RecordWorktree:       recordWorktree,
		WriteWorktreeContext: tmuxattention.WriteWorktreeContext,
		AppendHistory:        state.AppendHistory,
		RunForeground:        runForeground,
		NewAgentID:           agentidentity.NewAgentID,
		NewInstanceID:        agentidentity.NewInstanceID,
	}
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
	if l.SetMuxRuntime == nil {
		l.SetMuxRuntime = state.SetMuxRuntime
	}
	if l.SetMuxAgentRuntime == nil {
		l.SetMuxAgentRuntime = state.SetMuxAgentRuntime
	}
	if l.RecordWorktree == nil {
		l.RecordWorktree = recordWorktree
	}
	if l.AppendHistory == nil {
		l.AppendHistory = state.AppendHistory
	}
	if l.RunForeground == nil {
		l.RunForeground = runForeground
	}
	if l.NewAgentID == nil {
		l.NewAgentID = agentidentity.NewAgentID
	}
	if l.NewInstanceID == nil {
		l.NewInstanceID = agentidentity.NewInstanceID
	}

	result := &LaunchResult{
		Mode:   LaunchModeForeground,
		Window: opts.State.Stage.Name,
	}
	if opts.Window != "" {
		result.Window = opts.Window
	}

	if !opts.DisableTmux && l.Mux.Available() {
		if backend, ok := l.Mux.(mux.TargetBackend); ok {
			if launched, err := l.launchTarget(backend, opts, result); err != nil {
				return nil, err
			} else if launched {
				return result, nil
			}
			// Target-capable backends own their fallback reporting. If they could
			// not launch, continue in the foreground rather than trying the legacy
			// tmux-shaped path a second time.
			goto foreground
		}

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

foreground:
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

func (l Launcher) launchTarget(backend mux.TargetBackend, opts LaunchOptions, result *LaunchResult) (bool, error) {
	logicalWindow := result.Window
	tabs := stageNamesForTicket(opts.Root, opts.State)
	worktree, hasWorktree := resolveWorktreeLaunch(opts, tabs)
	worktreeReady := false
	nativeWorktreeCreated := false
	stored, hasStored := opts.State.Runtime.MuxTarget(result.Window)
	if hasStored && stored.Backend != backend.Name() {
		return false, fmt.Errorf("ticket runtime belongs to %s, not selected backend %s", stored.Backend, backend.Name())
	}

	target := mux.Target{Backend: backend.Name(), Workspace: opts.State.Slug, Tab: result.Window}
	if hasStored {
		target = mux.Target{Backend: stored.Backend, Workspace: stored.Workspace, Tab: stored.Tab, Pane: stored.Pane}
	}

	ready := false
	if opts.RequireExistingTmux {
		ready = hasStored && backend.SessionExists(target.Workspace)
	} else if hasStored && backend.SessionExists(target.Workspace) {
		ready = true
	} else if !hasStored && backend.Name() == "tmux" && backend.SessionExists(target.Workspace) {
		// Compatibility adoption is safe for tmux's conventional slug-named
		// sessions. Other backends should make label lookup unambiguous.
		ready = true
	} else {
		var (
			created mux.Target
			err     error
		)
		if worktreeBackend, ok := backend.(mux.WorktreeTargetBackend); ok && hasWorktree {
			created, err = worktreeBackend.CreateWorktreeTarget(worktree.Spec)
			if err == nil {
				nativeWorktreeCreated = true
				if err = l.RecordWorktree(opts.FeatureDir, opts.Root, worktree); err == nil {
					applyWorktreeState(opts.State, worktree)
					worktreeReady = true
				}
			}
		} else {
			created, err = backend.CreateTarget(opts.State.Slug, opts.FeatureDir, tabs)
		}
		if err != nil {
			if nativeWorktreeCreated {
				return false, fmt.Errorf("record native worktree: %w", err)
			}
			result.addFallback(opts, fmt.Sprintf("%s workspace create failed (%v)", backend.Name(), err))
			return false, nil
		}
		target = created
		ready = true
	}
	if !ready {
		return false, nil
	}
	if hasWorktree && !worktreeReady {
		if _, err := os.Stat(worktree.Spec.WorktreeDir); err == nil {
			worktreeReady = true
		}
	}

	if opts.OnTmuxSend != nil {
		opts.OnTmuxSend(target.Workspace, result.Window)
	}
	launchDir := opts.FeatureDir
	runDir := opts.Plan.CWD
	launchArgv := opts.Plan.LaunchArgv
	if worktreeReady {
		if l.WriteWorktreeContext != nil {
			if err := l.WriteWorktreeContext(worktree.Spec.WorktreeDir, opts.State.Ticket, opts.State.Slug); err != nil {
				result.addFallback(opts, fmt.Sprintf("warning: could not write tmux-attention worktree context: %v", err))
			}
		}
		launchDir = worktree.Spec.WorktreeDir
		if !pathWithin(runDir, worktree.Spec.WorktreeDir) {
			runDir = worktree.Spec.WorktreeDir
		}
		if opts.Plan.Worker != nil {
			launchArgv = workers.LaunchArgs(opts.Plan.Worker, opts.Root, runDir, opts.Plan.Prompt)
		}
	}
	metadata := windowMetadata(opts, logicalWindow)
	var agentRuntime *state.AgentRuntime
	if preparer, ok := backend.(mux.AgentLaunchBackend); ok && opts.Plan.Worker != nil {
		agent, err := l.nextAgentRuntime(opts)
		if err != nil {
			return false, err
		}
		agentRuntime = &agent
		metadata.AgentID = agent.ID
		metadata.AgentInstance = agent.Instance
		metadata.ProviderSessionID = agent.ProviderSessionID
		target, launchArgv, err = preparer.PrepareAgentLaunch(target, result.Window, launchDir, metadata, launchArgv)
		if err != nil {
			result.addFallback(opts, fmt.Sprintf("%s agent prepare failed (%v)", backend.Name(), err))
			return false, nil
		}
	}
	sent, err := backend.SendTarget(target, result.Window, launchDir, runDir, launchArgv)
	if err != nil {
		result.addFallback(opts, fmt.Sprintf("%s send failed (%v)", backend.Name(), err))
		return false, nil
	}
	result.Session = sent.Workspace
	result.Window = sent.Tab
	result.Pane = sent.Pane

	if agentRuntime != nil && sent.Pane != target.Pane {
		return false, fmt.Errorf("%s agent launch moved from prepared pane %s to %s", backend.Name(), target.Pane, sent.Pane)
	}
	if err := backend.SetTargetMetadata(sent, metadata); err != nil {
		result.addFallback(opts, fmt.Sprintf("warning: could not set %s metadata: %v", backend.Name(), err))
	}
	if taskBackend, ok := backend.(mux.TaskCellBackend); ok {
		if spec, enabled := resolveTaskCell(opts.Root, runDir, opts.State, metadata); enabled {
			if err := taskBackend.ConfigureTaskCell(sent, spec); err != nil {
				result.addFallback(opts, fmt.Sprintf("warning: could not configure %s task cell: %v", backend.Name(), err))
			}
		}
	}
	runtimeTarget := state.MuxRuntime{
		Backend: sent.Backend, Workspace: sent.Workspace, Tab: sent.Tab, Pane: sent.Pane,
	}
	var runtimeErr error
	if agentRuntime != nil {
		runtimeErr = l.SetMuxAgentRuntime(opts.FeatureDir, runtimeTarget, *agentRuntime)
	} else {
		runtimeErr = l.SetMuxRuntime(opts.FeatureDir, runtimeTarget)
	}
	if runtimeErr != nil {
		result.addFallback(opts, fmt.Sprintf("warning: could not write %s target to STATE.yaml: %v", backend.Name(), runtimeErr))
	}

	result.Mode = LaunchModeTmux
	result.AttachHint = backend.AttachTargetHint(sent)
	l.recordLaunch(opts, result, fmt.Sprintf("launched in %s workspace %s tab %s", backend.Name(), sent.Workspace, sent.Tab))
	return true, nil
}

func (l Launcher) nextAgentRuntime(opts LaunchOptions) (state.AgentRuntime, error) {
	engine := ""
	stage := opts.Plan.Stage
	if stage == "" {
		stage = opts.Window
	}
	if stage == "" && opts.State != nil {
		stage = opts.State.Stage.Name
	}
	if opts.Plan.Worker != nil {
		engine = opts.Plan.Worker.Engine
	}
	agentID := ""
	providerSessionID := ""
	if existing := opts.State.Runtime.Agent; stage != "jit" && existing != nil &&
		(existing.Stage == "" || existing.Stage == stage) && (existing.Engine == "" || existing.Engine == engine) {
		agentID = existing.ID
		providerSessionID = existing.ProviderSessionID
	}
	if agentID == "" {
		var err error
		agentID, err = l.NewAgentID()
		if err != nil {
			return state.AgentRuntime{}, err
		}
	}
	instance, err := l.NewInstanceID()
	if err != nil {
		return state.AgentRuntime{}, err
	}
	return state.AgentRuntime{
		ID: agentID, Instance: instance, Stage: stage, Engine: engine, ProviderSessionID: providerSessionID,
	}, nil
}

func windowMetadata(opts LaunchOptions, window string) mux.Metadata {
	metadata := mux.Metadata{
		Ticket:      opts.State.Ticket,
		Stage:       window,
		Workflow:    opts.State.Workflow,
		NextAction:  opts.State.NextAction.Prompt,
		FeatureDir:  opts.FeatureDir,
		FeatureSlug: opts.State.Slug,
	}
	repositories := make([]string, 0, len(opts.State.Repos))
	for name := range opts.State.Repos {
		repositories = append(repositories, name)
	}
	sort.Strings(repositories)
	for _, name := range repositories {
		repo := opts.State.Repos[name]
		metadata.Repository = name
		metadata.Branch = repo.Branch
		break
	}
	if opts.Plan.Worker != nil {
		metadata.Worker = opts.Plan.Worker.ID
		if metadata.Worker == "" {
			metadata.Worker = opts.Plan.Worker.Name
		}
		metadata.Engine = opts.Plan.Worker.Engine
		metadata.Model = opts.Plan.Worker.Model
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
