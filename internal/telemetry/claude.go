package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type claudeProcessSession struct {
	PID             int    `json:"pid"`
	SessionID       string `json:"sessionId"`
	CWD             string `json:"cwd"`
	Status          string `json:"status"`
	UpdatedAt       string `json:"updatedAt"`
	StatusUpdatedAt string `json:"statusUpdatedAt"`
}

func (d *Discoverer) discoverClaude(home string, budget *refreshBudget) ([]Live, error) {
	processFiles, err := recentFiles(filepath.Join(home, ".claude", "sessions"), "*.json", 500)
	if err != nil {
		return nil, err
	}
	// Claude stores subagent transcripts beside resumable top-level UUID
	// transcripts, so collect enough candidates to filter those safely.
	projectFiles, err := recentFiles(filepath.Join(home, ".claude", "projects"), "*.jsonl", 500)
	if err != nil {
		return nil, err
	}
	transcripts := make(map[string]string, len(projectFiles))
	transcriptIDs := make([]string, 0, len(projectFiles))
	for _, path := range projectFiles {
		id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		if !isUUIDLike(id) {
			continue
		}
		if transcripts[id] == "" {
			transcripts[id] = path
			transcriptIDs = append(transcriptIDs, id)
		}
	}
	var out []Live
	active := make(map[string]bool)
	seen := make(map[string]bool)
	for _, path := range processFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var session claudeProcessSession
		if json.Unmarshal(data, &session) != nil || session.PID <= 0 || !processExists(session.PID) {
			continue
		}
		live := Live{
			Engine:            "claude",
			ProviderSessionID: session.SessionID,
			CWD:               session.CWD,
			State:             session.Status,
			PID:               session.PID,
			LastActive:        parseTime(session.StatusUpdatedAt, session.UpdatedAt),
		}
		active[session.SessionID] = true
		if jsonl := transcripts[session.SessionID]; jsonl != "" {
			seen[jsonl] = true
			if metadata, _, readErr := d.transcript(jsonl, "claude", budget); readErr == nil {
				live = mergeTranscript(live, metadata, false)
			}
		}
		out = append(out, live)
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, id := range transcriptIDs {
		path := transcripts[id]
		if active[id] {
			continue
		}
		info, err := os.Stat(path)
		if err != nil || info.ModTime().Before(cutoff) {
			continue
		}
		seen[path] = true
		live := Live{Engine: "claude", ProviderSessionID: id, State: "idle", LastActive: info.ModTime()}
		if metadata, _, readErr := d.transcript(path, "claude", budget); readErr == nil {
			live = mergeTranscript(live, metadata, false)
		}
		out = append(out, live)
	}
	d.prune(filepath.Join(home, ".claude", "projects"), seen)
	return out, nil
}

func isUUIDLike(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for i, char := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			continue
		}
		if !isHexDigit(char) {
			return false
		}
	}
	return true
}

func isHexDigit(char rune) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
}

func parseClaudeLine(line []byte, live *Live) bool {
	var record struct {
		Type      string `json:"type"`
		Timestamp string `json:"timestamp"`
		CWD       string `json:"cwd"`
		Message   struct {
			Model string `json:"model"`
			Usage struct {
				Input         uint64 `json:"input_tokens"`
				CacheCreation uint64 `json:"cache_creation_input_tokens"`
				CacheRead     uint64 `json:"cache_read_input_tokens"`
				Output        uint64 `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &record) != nil {
		return false
	}
	if record.CWD != "" {
		live.CWD = record.CWD
	}
	if t := parseTime(record.Timestamp); t.After(live.LastActive) {
		live.LastActive = t
	}
	if record.Type == "assistant" {
		live.Model = record.Message.Model
		live.ContextUsed = record.Message.Usage.Input + record.Message.Usage.CacheCreation + record.Message.Usage.CacheRead + record.Message.Usage.Output
	}
	return true
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func parseTime(values ...string) time.Time {
	for _, value := range values {
		if value == "" {
			continue
		}
		if millis, err := strconv.ParseInt(value, 10, 64); err == nil {
			return time.UnixMilli(millis)
		}
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
