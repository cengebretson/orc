package parking

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const Version = 1

type Entry struct {
	Ticket            string `json:"ticket"`
	Stage             string `json:"stage"`
	Worker            string `json:"worker,omitempty"`
	Engine            string `json:"engine"`
	ProviderSessionID string `json:"provider_session_id"`
	CWD               string `json:"cwd"`
	FeatureDir        string `json:"feature_dir"`
	TmuxSession       string `json:"tmux_session"`
	TmuxWindow        string `json:"tmux_window"`
}

type Snapshot struct {
	Version   int       `json:"version"`
	Workspace string    `json:"workspace"`
	ParkedAt  time.Time `json:"parked_at"`
	Sessions  []Entry   `json:"sessions"`
}

func Path(root, home string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	sum := sha256.Sum256([]byte(abs))
	name := hex.EncodeToString(sum[:6]) + ".json"
	return filepath.Join(home, ".local", "state", "orc", "parked", name), nil
}

func Save(path string, snapshot Snapshot) error {
	if snapshot.Version == 0 {
		snapshot.Version = Version
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".parked-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close() //nolint:errcheck
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func Load(path string) (Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Snapshot{}, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return Snapshot{}, err
	}
	if snapshot.Version != Version {
		return Snapshot{}, fmt.Errorf("unsupported parked-session snapshot version %d", snapshot.Version)
	}
	return snapshot, nil
}

func Remove(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
