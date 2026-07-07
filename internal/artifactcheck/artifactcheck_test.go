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

	issues := Check(dir, "", []string{"PLAN.md", "SPEC.md", "develop", "TICKET.md", "", "TICKET.md"})

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
		{Issue{Path: "PLAN.md", Status: Unchanged}, "PLAN.md unchanged from template"},
	} {
		if got := tc.issue.Detail(); got != tc.want {
			t.Errorf("Detail() = %q, want %q", got, tc.want)
		}
	}
}

func TestCheckFlagsUnchangedFromTemplate(t *testing.T) {
	featureDir := t.TempDir()
	templateDir := t.TempDir()

	// PLAN.md is byte-identical to the template — nobody has written it yet.
	if err := os.WriteFile(filepath.Join(templateDir, "PLAN.md"), []byte("scaffold"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "PLAN.md"), []byte("scaffold"), 0644); err != nil {
		t.Fatal(err)
	}
	// SPEC.md diverges from the template — real work landed.
	if err := os.WriteFile(filepath.Join(templateDir, "SPEC.md"), []byte("scaffold"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(featureDir, "SPEC.md"), []byte("real content"), 0644); err != nil {
		t.Fatal(err)
	}

	issues := Check(featureDir, templateDir, []string{"PLAN.md", "SPEC.md"})

	want := []Issue{{Path: "PLAN.md", Status: Unchanged}}
	if len(issues) != len(want) {
		t.Fatalf("issues = %+v, want %+v", issues, want)
	}
	if issues[0] != want[0] {
		t.Errorf("issues[0] = %+v, want %+v", issues[0], want[0])
	}

	// With no template dir the byte comparison is skipped: an unchanged file
	// passes the presence-only check.
	if issues := Check(featureDir, "", []string{"PLAN.md"}); len(issues) != 0 {
		t.Errorf("presence-only Check = %+v, want no issues", issues)
	}
}

func TestInspectReportsEveryArtifact(t *testing.T) {
	dir := writeArtifactFixture(t)

	artifacts := Inspect(dir, "", []string{"PLAN.md", "SPEC.md", "develop", "TICKET.md"})

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
