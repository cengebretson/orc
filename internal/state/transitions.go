package state

func Start(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: "started",
		})
		s.Status = "active"
		return nil
	})
}

// Resume marks a paused feature as active again and records the continuation.
// It clears the human-directed NextAction that Pause sets so the agent can write fresh context.
func Resume(featureDir string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: "resumed",
		})
		s.Status = "active"
		s.NextAction = NextAction{}
		return nil
	})
}

// Pause marks the feature as paused (waiting for human input or external blocker).
func Pause(featureDir, reason string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: "paused — " + reason,
		})

		s.Status = "paused"
		s.NextAction.Worker = "human"
		s.NextAction.Prompt = reason
		return nil
	})
}

// Next advances the feature to the next stage, records a history entry, and saves STATE.yaml.
// When stageName is empty (no stages remain), status is set to "done".
func Next(featureDir, stageName, worker, result string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: result,
		})

		if stageName != "" {
			s.Stage.Name = stageName
			if s.StageCounts == nil {
				s.StageCounts = map[string]int{}
			}
			s.StageCounts[stageName]++
			s.Status = "pending"
		} else {
			s.Status = "done"
		}
		if worker != "" {
			s.Stage.Worker = worker
		}
		s.NextAction = NextAction{}
		return nil
	})
}

// Done marks the feature as done (all stages complete or explicitly closed).
func Done(featureDir, result string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  s.Stage.Name,
			Worker: s.Stage.Worker,
			Result: result,
		})
		s.Status = "done"
		s.NextAction = NextAction{}
		return nil
	})
}

// SetStatus updates only the status field in STATE.yaml, preserving all other content.
func SetStatus(featureDir, status string) error {
	return Update(featureDir, func(s *State) error {
		s.Status = status
		return nil
	})
}

// AppendHistory loads STATE.yaml, appends a history entry, and saves — no other fields touched.
func AppendHistory(featureDir, stage, workerID, result string) error {
	return Update(featureDir, func(s *State) error {
		s.History = append(s.History, HistoryEntry{
			At:     timeNow(),
			Stage:  stage,
			Worker: workerID,
			Result: result,
		})
		return nil
	})
}

// FindFeatureDir locates the feature directory for the given slug or ticket ID.
// Supports full slug match or prefix match on ticket ID (e.g. "FLYWL-123").
