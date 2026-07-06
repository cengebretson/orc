package artifactcheck

import (
	"os"
	"path/filepath"
)

var CoreDocs = []string{"TICKET.md", "SPEC.md", "PLAN.md", "DECISIONS.md"}

type Status int

const (
	Missing Status = iota
	Directory
	Empty
)

type Issue struct {
	Path   string
	Status Status
}

func (i Issue) Detail() string {
	switch i.Status {
	case Missing:
		return i.Path + " missing"
	case Directory:
		return i.Path + " is a directory"
	case Empty:
		return i.Path + " empty"
	default:
		return i.Path
	}
}

func Check(featureDir string, artifacts []string) []Issue {
	var issues []Issue
	for _, artifact := range uniqueArtifacts(artifacts) {
		path := filepath.Join(featureDir, artifact)
		info, err := os.Stat(path)
		if err != nil {
			issues = append(issues, Issue{Path: artifact, Status: Missing})
			continue
		}
		if info.IsDir() {
			issues = append(issues, Issue{Path: artifact, Status: Directory})
			continue
		}
		if info.Size() == 0 {
			issues = append(issues, Issue{Path: artifact, Status: Empty})
		}
	}
	return issues
}

func uniqueArtifacts(artifacts []string) []string {
	seen := make(map[string]bool, len(artifacts))
	var out []string
	for _, artifact := range artifacts {
		if artifact == "" || seen[artifact] {
			continue
		}
		seen[artifact] = true
		out = append(out, artifact)
	}
	return out
}
