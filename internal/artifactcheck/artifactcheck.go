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

type Artifact struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
	Ready  bool   `json:"ready"`
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
	for _, artifact := range Inspect(featureDir, artifacts) {
		if artifact.Ready {
			continue
		}
		issues = append(issues, Issue{Path: artifact.Path, Status: statusFromString(artifact.Status)})
	}
	return issues
}

func Inspect(featureDir string, artifacts []string) []Artifact {
	out := make([]Artifact, 0, len(artifacts))
	for _, artifact := range uniqueArtifacts(artifacts) {
		path := filepath.Join(featureDir, artifact)
		info, err := os.Stat(path)
		if err != nil {
			issue := Issue{Path: artifact, Status: Missing}
			out = append(out, Artifact{Path: artifact, Status: "missing", Detail: issue.Detail(), Ready: false})
			continue
		}
		if info.IsDir() {
			issue := Issue{Path: artifact, Status: Directory}
			out = append(out, Artifact{Path: artifact, Status: "directory", Detail: issue.Detail(), Ready: false})
			continue
		}
		if info.Size() == 0 {
			issue := Issue{Path: artifact, Status: Empty}
			out = append(out, Artifact{Path: artifact, Status: "empty", Detail: issue.Detail(), Ready: false})
			continue
		}
		out = append(out, Artifact{Path: artifact, Status: "ok", Ready: true})
	}
	return out
}

func statusFromString(status string) Status {
	switch status {
	case "directory":
		return Directory
	case "empty":
		return Empty
	default:
		return Missing
	}
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
