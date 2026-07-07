package artifactcheck

import (
	"bytes"
	"os"
	"path/filepath"
)

var CoreDocs = []string{"TICKET.md", "SPEC.md", "PLAN.md", "DECISIONS.md"}

type Status int

const (
	Missing Status = iota
	Directory
	Empty
	Unchanged
)

func (s Status) String() string {
	switch s {
	case Directory:
		return "directory"
	case Empty:
		return "empty"
	case Unchanged:
		return "unchanged"
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
	case Unchanged:
		return "unchanged from template"
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

// TemplateDir returns the feature scaffold folder for a workspace. Pass it as
// templateDir to also flag artifacts still byte-identical to their scaffolded
// counterpart; pass "" for presence/emptiness checks only.
func TemplateDir(root string) string {
	return filepath.Join(root, "features", "_template")
}

func Check(featureDir, templateDir string, artifacts []string) []Issue {
	var issues []Issue
	for _, artifact := range uniqueArtifacts(artifacts) {
		if status, ready := stat(featureDir, templateDir, artifact); !ready {
			issues = append(issues, Issue{Path: artifact, Status: status})
		}
	}
	return issues
}

func Inspect(featureDir, templateDir string, artifacts []string) []Artifact {
	unique := uniqueArtifacts(artifacts)
	out := make([]Artifact, 0, len(unique))
	for _, artifact := range unique {
		status, ready := stat(featureDir, templateDir, artifact)
		if ready {
			out = append(out, Artifact{Path: artifact, Status: "ok", Ready: true})
			continue
		}
		issue := Issue{Path: artifact, Status: status}
		out = append(out, Artifact{Path: artifact, Status: status.String(), Detail: issue.Detail(), Ready: false})
	}
	return out
}

func stat(featureDir, templateDir, artifact string) (Status, bool) {
	path := filepath.Join(featureDir, artifact)
	info, err := os.Stat(path)
	if err != nil {
		return Missing, false
	}
	if info.IsDir() {
		return Directory, false
	}
	if info.Size() == 0 {
		return Empty, false
	}
	if templateDir != "" && unchangedFromTemplate(path, filepath.Join(templateDir, artifact), info.Size()) {
		return Unchanged, false
	}
	return 0, true
}

// unchangedFromTemplate reports whether the artifact is byte-identical to its
// scaffolded counterpart. orc work copies feature docs verbatim from
// features/_template, so identical bytes mean nobody has written the doc yet.
func unchangedFromTemplate(path, templatePath string, size int64) bool {
	info, err := os.Stat(templatePath)
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return false
	}
	got, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	want, err := os.ReadFile(templatePath)
	if err != nil {
		return false
	}
	return bytes.Equal(got, want)
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
