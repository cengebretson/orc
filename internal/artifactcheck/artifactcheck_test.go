package artifactcheck

import (
	"os"
	"path/filepath"
	"testing"
)

func writeArtifactFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PLAN.md"), []byte("plan"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "develop"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestCheckReportsOnlyProblems(t *testing.T) {
	dir := writeArtifactFixture(t)

	issues := Check(dir, []string{"PLAN.md", "SPEC.md", "develop", "TICKET.md", "", "TICKET.md"})

	want := []Issue{
		{Path: "SPEC.md", Status: Empty},
		{Path: "develop", Status: Directory},
		{Path: "TICKET.md", Status: Missing},
	}
	if len(issues) != len(want) {
		t.Fatalf("issues = %+v, want %+v", issues, want)
	}
	for i, w := range want {
		if issues[i] != w {
			t.Errorf("issues[%d] = %+v, want %+v", i, issues[i], w)
		}
	}
}

func TestIssueDetail(t *testing.T) {
	for _, tc := range []struct {
		issue Issue
		want  string
	}{
		{Issue{Path: "TICKET.md", Status: Missing}, "TICKET.md missing"},
		{Issue{Path: "SPEC.md", Status: Empty}, "SPEC.md empty"},
		{Issue{Path: "develop", Status: Directory}, "develop is a directory"},
	} {
		if got := tc.issue.Detail(); got != tc.want {
			t.Errorf("Detail() = %q, want %q", got, tc.want)
		}
	}
}

func TestInspectReportsEveryArtifact(t *testing.T) {
	dir := writeArtifactFixture(t)

	artifacts := Inspect(dir, []string{"PLAN.md", "SPEC.md", "develop", "TICKET.md"})

	want := []Artifact{
		{Path: "PLAN.md", Status: "ok", Ready: true},
		{Path: "SPEC.md", Status: "empty", Detail: "SPEC.md empty"},
		{Path: "develop", Status: "directory", Detail: "develop is a directory"},
		{Path: "TICKET.md", Status: "missing", Detail: "TICKET.md missing"},
	}
	if len(artifacts) != len(want) {
		t.Fatalf("artifacts = %+v, want %+v", artifacts, want)
	}
	for i, w := range want {
		if artifacts[i] != w {
			t.Errorf("artifacts[%d] = %+v, want %+v", i, artifacts[i], w)
		}
	}
}
