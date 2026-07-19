package state

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func Load(featureDir string) (*State, error) {
	path := filepath.Join(featureDir, Filename)
	return loadPath(path)
}

func loadPath(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var s State
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if s.SchemaVersion == 0 {
		s.SchemaVersion = SchemaVersion
	}

	return &s, nil
}

// Update loads STATE.yaml, applies mutate, and writes the file back atomically.
func Update(featureDir string, mutate func(*State) error) error {
	path := filepath.Join(featureDir, Filename)
	unlock, err := lockPath(path)
	if err != nil {
		return err
	}
	defer unlock()

	s, err := loadPath(path)
	if err != nil {
		return err
	}
	if err := mutate(s); err != nil {
		return err
	}
	return savePath(path, s)
}

// Create writes a fresh STATE.yaml in featureDir through the same locked,
// atomic temp-file path as Update. Used when scaffolding a feature, where the
// file holds template placeholders (or does not exist) and there is no prior
// state to read.
func Create(featureDir string, s *State) error {
	path := filepath.Join(featureDir, Filename)
	unlock, err := lockPath(path)
	if err != nil {
		return err
	}
	defer unlock()
	return savePath(path, s)
}

func savePath(path string, s *State) error {
	out, err := yaml.Marshal(s)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+Filename+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp state file: %w", err)
	}
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp state file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replacing %s: %w", path, err)
	}
	cleanup = false
	return nil
}
