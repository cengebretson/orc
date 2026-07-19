package state

func SetRuntime(featureDir, tmuxSession string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.Tmux = &TmuxRuntime{Session: tmuxSession}
		return nil
	})
}

// SetRuntimeTarget records the exact pane used by the active agent. The pane is
// optional for compatibility with existing STATE.yaml files.
func SetRuntimeTarget(featureDir, tmuxSession, pane string) error {
	return Update(featureDir, func(s *State) error {
		s.Runtime.Tmux = &TmuxRuntime{Session: tmuxSession, Pane: pane}
		return nil
	})
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
