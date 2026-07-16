package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxJSONLLineBytes = 10 * 1024 * 1024

// Live is provider-specific runtime information. All fields are optional and
// must be treated as an overlay; Orc's durable feature state remains primary.
type Live struct {
	Engine            string    `json:"engine,omitempty"`
	ProviderSessionID string    `json:"provider_session_id,omitempty"`
	ObservedSessionID string    `json:"observed_session_id,omitempty"`
	Correlation       string    `json:"correlation,omitempty"`
	Model             string    `json:"model,omitempty"`
	Effort            string    `json:"effort,omitempty"`
	State             string    `json:"state,omitempty"`
	CWD               string    `json:"cwd,omitempty"`
	ContextUsed       uint64    `json:"context_used,omitempty"`
	ContextLimit      uint64    `json:"context_limit,omitempty"`
	LastActive        time.Time `json:"last_active,omitempty"`
	PID               int       `json:"pid,omitempty"`
	PaneTarget        string    `json:"pane_target,omitempty"`
	Managed           bool      `json:"managed"`
	Ticket            string    `json:"ticket,omitempty"`
}

// ResumeArgs returns the provider CLI argv for an already-discovered session.
func ResumeArgs(engine, id, cwd string) (string, []string, error) {
	switch strings.ToLower(engine) {
	case "claude":
		return "claude", []string{"--resume", id}, nil
	case "codex":
		return "codex", []string{"resume", id, "--cd", cwd}, nil
	default:
		return "", nil, fmt.Errorf("resume is not supported for engine %q", engine)
	}
}

// Discover reads recent provider metadata through the process-wide incremental
// discoverer. Malformed provider files are skipped individually.
func Discover(home string) ([]Live, error) {
	return defaultDiscoverer.Discover(home)
}

func recentFiles(root, pattern string, limit int) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		matched, _ := filepath.Match(pattern, entry.Name())
		if matched {
			files = append(files, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		left, _ := os.Stat(files[i])
		right, _ := os.Stat(files[j])
		if left == nil || right == nil {
			return files[i] < files[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}
