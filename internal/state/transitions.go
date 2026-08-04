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
// It clears the human-directed NextAction that Pause sets, and any question
// asked with it, so the agent can write fresh context. The launch prompt has
// already carried the answer to the agent by this point, which is the same
// contract runtime.jit follows.
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
		s.Runtime.Question = nil
		return nil
	})
}

// Pause marks the feature as paused (waiting for human input or external
// blocker). A non-nil question records the shape of answer the agent needs, so
// Orc can offer the matching control and reject anything else; pass nil to
// pause on a free-text reason alone.
func Pause(featureDir, reason string, question *QuestionRuntime) error {
	if question != nil {
		if err := question.Validate(); err != nil {
			return err
		}
	}
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
		if question != nil {
			asked := *question
			asked.Answer = ""
			asked.AnsweredAt = ""
			if asked.AskedAt == "" {
				asked.AskedAt = timeNow()
			}
			s.Runtime.Question = &asked
		}
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
