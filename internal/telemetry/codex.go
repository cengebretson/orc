package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func (d *Discoverer) discoverCodex(home string, budget *refreshBudget) ([]Live, error) {
	files, err := recentFiles(filepath.Join(home, ".codex", "sessions"), "*.jsonl", 200)
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	var out []Live
	seen := make(map[string]bool)
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || info.ModTime().Before(cutoff) {
			continue
		}
		seen[path] = true
		live := Live{
			Engine:            "codex",
			ProviderSessionID: sessionIDFromFilename(path),
			LastActive:        info.ModTime(),
			State:             "idle",
		}
		if metadata, working, readErr := d.transcript(path, "codex", budget); readErr == nil {
			live = mergeTranscript(live, metadata, working)
		}
		if live.ProviderSessionID != "" {
			out = append(out, live)
		}
	}
	d.prune(filepath.Join(home, ".codex", "sessions"), seen)
	return out, nil
}

func parseCodexLine(line []byte, live *Live, working *bool) bool {
	var envelope struct {
		Timestamp string          `json:"timestamp"`
		Type      string          `json:"type"`
		Payload   json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(line, &envelope) != nil {
		return false
	}
	if t := parseTime(envelope.Timestamp); t.After(live.LastActive) {
		live.LastActive = t
	}
	switch envelope.Type {
	case "session_meta":
		var payload struct {
			ID        string `json:"id"`
			SessionID string `json:"session_id"`
			CWD       string `json:"cwd"`
		}
		if json.Unmarshal(envelope.Payload, &payload) != nil {
			return false
		}
		live.ProviderSessionID = payload.SessionID
		if live.ProviderSessionID == "" {
			live.ProviderSessionID = payload.ID
		}
		live.CWD = payload.CWD
	case "turn_context":
		var payload struct {
			CWD    string `json:"cwd"`
			Model  string `json:"model"`
			Effort string `json:"effort"`
		}
		if json.Unmarshal(envelope.Payload, &payload) != nil {
			return false
		}
		if payload.CWD != "" {
			live.CWD = payload.CWD
		}
		live.Model = payload.Model
		live.Effort = payload.Effort
	case "event_msg":
		var payload struct {
			Type string `json:"type"`
			Info struct {
				Total struct {
					Tokens uint64 `json:"total_tokens"`
				} `json:"total_token_usage"`
				Last struct {
					Tokens uint64 `json:"total_tokens"`
				} `json:"last_token_usage"`
				ContextWindow uint64 `json:"model_context_window"`
			} `json:"info"`
		}
		if json.Unmarshal(envelope.Payload, &payload) != nil {
			return false
		}
		switch payload.Type {
		case "task_started":
			*working = true
		case "task_complete":
			*working = false
		case "token_count":
			live.ContextUsed = payload.Info.Last.Tokens
			if live.ContextUsed == 0 {
				live.ContextUsed = payload.Info.Total.Tokens
			}
			live.ContextLimit = payload.Info.ContextWindow
		}
	}
	return true
}
