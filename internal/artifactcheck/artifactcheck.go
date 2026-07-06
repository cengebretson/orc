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

func (s Status) String() string {
	switch s {
	case Directory:
		return "directory"
	case Empty:
		return "empty"
	default:
		return "missing"
	}
}

// Message is the human-readable problem description, without the path.
func (s Status) Message() string {
	switch s {
	case Directory:
		return "is a directory"
	case Empty:
		return "empty"
	default:
		return "missing"
	}
}

type Issue struct {
	Path   string
	Status Status
}

type Artifact struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Ready  bool   `json:"ready"`
}

func (i Issue) Detail() string {
	return i.Path + " " + i.Status.Message()
}

func Check(featureDir string, artifacts []string) []Issue {
	var issues []Issue
	for _, artifact := range uniqueArtifacts(artifacts) {
		if status, ready := stat(featureDir, artifact); !ready {
			issues = append(issues, Issue{Path: artifact, Status: status})
		}
	}
	return issues
}

func Inspect(featureDir string, artifacts []string) []Artifact {
	unique := uniqueArtifacts(artifacts)
	out := make([]Artifact, 0, len(unique))
	for _, artifact := range unique {
		status, ready := stat(featureDir, artifact)
		if ready {
			out = append(out, Artifact{Path: artifact, Status: "ok", Ready: true})
			continue
		}
		issue := Issue{Path: artifact, Status: status}
		out = append(out, Artifact{Path: artifact, Status: status.String(), Detail: issue.Detail(), Ready: false})
	}
	return out
}

func stat(featureDir, artifact string) (Status, bool) {
	info, err := os.Stat(filepath.Join(featureDir, artifact))
	if err != nil {
		return Missing, false
	}
	if info.IsDir() {
		return Directory, false
	}
	if info.Size() == 0 {
		return Empty, false
	}
	return 0, true
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
