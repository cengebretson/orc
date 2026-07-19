package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func FindFeatureDir(workspaceRoot, query string) (string, error) {
	featuresDir := filepath.Join(workspaceRoot, "features")
	if _, err := os.ReadDir(featuresDir); err != nil {
		return "", fmt.Errorf("reading features/: %w", err)
	}
	notFound := func(q string) error {
		return fmt.Errorf("no feature found matching %q — create one with `orc work %s`", q, q)
	}
	return matchFeature(query, notFound, featuresDir)
}

// FindFeatureDirWithArchive searches both features/ and features/_archive/ for a ticket match.
func FindFeatureDirWithArchive(workspaceRoot, query string) (string, error) {
	featuresDir := filepath.Join(workspaceRoot, "features")
	notFound := func(q string) error {
		return fmt.Errorf("no feature found matching %q", q)
	}
	return matchFeature(query, notFound, featuresDir, filepath.Join(featuresDir, "_archive"))
}

// matchFeature scans the given directories for a feature folder whose name
// equals or is prefixed by query (case-insensitive), skipping the _template and
// _archive scaffolding dirs. notFound builds the error for the zero-match case
// from the normalized query. Unreadable dirs are skipped.
func matchFeature(query string, notFound func(q string) error, dirs ...string) (string, error) {
	query = strings.ToUpper(strings.TrimSpace(query))

	var matches []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "_template" || e.Name() == "_archive" {
				continue
			}
			upper := strings.ToUpper(e.Name())
			if upper == query || strings.HasPrefix(upper, query) {
				matches = append(matches, filepath.Join(dir, e.Name()))
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", notFound(query)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = filepath.Base(m)
		}
		return "", fmt.Errorf("ambiguous slug %q matches multiple features:\n  %s\nUse the full slug", query, strings.Join(names, "\n  "))
	}
}

// SetRuntime writes the runtime.tmux session name to STATE.yaml.
