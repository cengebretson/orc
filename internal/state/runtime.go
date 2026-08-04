package state

import "fmt"

// SetMuxRuntime records the exact backend-neutral multiplexer target.
func SetMuxRuntime(featureDir string, target MuxRuntime) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.Mux = &target
		// New writes use runtime.mux. Preserve runtime.tmux only when loading old
		// state; do not write two competing sources of truth.
		s.Runtime.Tmux = nil
		return nil
	})
}

// SetMuxAgentRuntime atomically records an exact multiplexer target and the
// agent instance occupying it. New launch paths should use this once they have
// both identities so readers cannot observe one without the other.
func SetMuxAgentRuntime(featureDir string, target MuxRuntime, agent AgentRuntime) error {
	if target.Backend == "" || target.Workspace == "" || target.Tab == "" || target.Pane == "" {
		return fmt.Errorf("agent runtime requires exact backend, workspace, tab, and pane ids")
	}
	if agent.ID == "" || agent.Instance == "" {
		return fmt.Errorf("agent runtime requires agent and instance ids")
	}
	return Update(featureDir, func(s *State) error {
		s.Runtime.Mux = &target
		s.Runtime.Tmux = nil
		s.Runtime.Agent = &agent
		return nil
	})
}

// SetRuntime is the legacy tmux-shaped entry point. New orchestration code
// should call SetMuxRuntime; this wrapper keeps older callers source-compatible
// while ensuring new STATE.yaml writes use runtime.mux.
func SetRuntime(featureDir, tmuxSession string) error {
	return SetMuxRuntime(featureDir, MuxRuntime{Backend: "tmux", Workspace: tmuxSession})
}

// SetRuntimeTarget records the exact pane used by the active agent. The pane is
// optional for compatibility with existing STATE.yaml files.
func SetRuntimeTarget(featureDir, tmuxSession, pane string) error {
	return SetMuxRuntime(featureDir, MuxRuntime{Backend: "tmux", Workspace: tmuxSession, Pane: pane})
}

// ClearRuntime removes the runtime block from STATE.yaml.
func ClearRuntime(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime = Runtime{}
		return nil
	})
}

// SetJIT writes runtime.jit to STATE.yaml before a jit task launches.
func SetJIT(featureDir, workerID, task string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.JIT = &JITRuntime{
			Worker:    workerID,
			Task:      task,
			StartedAt: timeNow(),
		}
		return nil
	})
}

// ClearJIT removes runtime.jit from STATE.yaml after a jit task completes.
func ClearJIT(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.JIT = nil
		return nil
	})
}

// RepoError is a single structured problem found by ValidateRepos.
