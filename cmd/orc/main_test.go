package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspace"
)

func TestRunStatusTicketPrintsDetail(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = fixtureWorkspace()

	out, err := captureStdout(func() error {
		return runStatus(nil, []string{"STORY-123"})
	})
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}

	for _, want := range []string{
		"Ticket:   STORY-123",
		"Stage:     default · develop",
		"Worker:  Bob (Developer) (codex)",
		"Run `orc next` to launch.",
		"⚠  state has problems — run `orc doctor STORY-123` for details",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestRunStatusJSONPrintsActiveAndArchived(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = fixtureWorkspace()
	statusJSON = true

	out, err := captureStdout(func() error {
		return runStatus(nil, nil)
	})
	if err != nil {
		t.Fatalf("runStatus --json: %v", err)
	}

	var payload struct {
		Active   []map[string]any `json:"active"`
		Archived []map[string]any `json:"archived"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal status json: %v\n%s", err, out)
	}
	if len(payload.Active) == 0 {
		t.Fatal("active list is empty")
	}
	if len(payload.Archived) != 1 {
		t.Fatalf("archived count = %d, want 1", len(payload.Archived))
	}
	if payload.Archived[0]["Ticket"] != "STORY-101" {
		t.Fatalf("archived ticket = %v, want STORY-101", payload.Archived[0]["Ticket"])
	}
}

// writeTimedTicket creates features/<slug>/STATE.yaml with a known history:
// 2h in intake, then develop with a 12h pause inside a 20h span (→ 8h active),
// closed as done. Durations are independent of wall-clock time.
func writeTimedTicket(t *testing.T, root, ticket string) {
	t.Helper()
	base := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	at := func(h float64) string {
		return base.Add(time.Duration(h * float64(time.Hour))).Format(time.RFC3339)
	}
	featureDir := filepath.Join(root, "features", ticket)
	if err := os.MkdirAll(featureDir, 0o755); err != nil {
		t.Fatalf("mkdir feature: %v", err)
	}
	s := &state.State{
		Ticket: ticket,
		Slug:   ticket,
		Status: "done",
		Stage:  state.Stage{Name: "code-review"},
		History: []state.HistoryEntry{
			{At: at(0), Stage: "intake", Worker: "agent", Result: "feature context created by orc work"},
			{At: at(2), Stage: "intake", Worker: "agent", Result: "intake done"},
			{At: at(8), Stage: "develop", Worker: "bob", Result: "paused — waiting on review"},
			{At: at(20), Stage: "develop", Worker: "bob", Result: "resumed"},
			{At: at(22), Stage: "develop", Worker: "bob", Result: "ready for review"},
		},
	}
	if err := state.Create(featureDir, s); err != nil {
		t.Fatalf("create ticket: %v", err)
	}
}

func TestRunReportTicketPrintsStageTimings(t *testing.T) {
	resetCommandGlobals(t)
	root := filepath.Join(t.TempDir(), "workspace")
	writeTimedTicket(t, root, "TIME-1")
	globalWorkspace = root

	out, err := captureStdout(func() error {
		return runReport(nil, []string{"TIME-1"})
	})
	if err != nil {
		t.Fatalf("runReport: %v", err)
	}

	for _, want := range []string{
		"TIME-1 · complete",
		"intake",
		"develop",
		"Total",
		"8h", // develop active = 20h wall − 12h paused
		"20h",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("report output missing %q:\n%s", want, out)
		}
	}
}

func TestRunReportTicketJSONShape(t *testing.T) {
	resetCommandGlobals(t)
	root := filepath.Join(t.TempDir(), "workspace")
	writeTimedTicket(t, root, "TIME-1")
	globalWorkspace = root
	reportJSON = true

	out, err := captureStdout(func() error {
		return runReport(nil, []string{"TIME-1"})
	})
	if err != nil {
		t.Fatalf("runReport --json: %v", err)
	}

	var payload struct {
		Ticket string `json:"ticket"`
		Open   bool   `json:"open"`
		Stages []struct {
			Stage         string `json:"stage"`
			ActiveSeconds int64  `json:"active_seconds"`
			WallSeconds   int64  `json:"wall_seconds"`
			Visits        int    `json:"visits"`
		} `json:"stages"`
		TotalActiveSeconds int64 `json:"total_active_seconds"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal report json: %v\n%s", err, out)
	}
	if payload.Ticket != "TIME-1" || payload.Open {
		t.Fatalf("ticket=%q open=%v, want TIME-1 and not open", payload.Ticket, payload.Open)
	}
	var dev int64
	for _, st := range payload.Stages {
		if st.Stage == "develop" {
			dev = st.ActiveSeconds
		}
	}
	if want := int64((8 * time.Hour).Seconds()); dev != want {
		t.Fatalf("develop active_seconds = %d, want %d", dev, want)
	}
	if want := int64((10 * time.Hour).Seconds()); payload.TotalActiveSeconds != want {
		t.Fatalf("total_active_seconds = %d, want %d", payload.TotalActiveSeconds, want)
	}
}

func TestRunReportAggregateAcrossTickets(t *testing.T) {
	resetCommandGlobals(t)
	root := filepath.Join(t.TempDir(), "workspace")
	writeTimedTicket(t, root, "TIME-1")
	writeTimedTicket(t, root, "TIME-2")
	globalWorkspace = root

	out, err := captureStdout(func() error {
		return runReport(nil, nil)
	})
	if err != nil {
		t.Fatalf("runReport aggregate: %v", err)
	}
	if !strings.Contains(out, "across 2 ticket(s)") {
		t.Fatalf("aggregate header missing ticket count:\n%s", out)
	}
	for _, want := range []string{"intake", "develop", "Avg active"} {
		if !strings.Contains(out, want) {
			t.Fatalf("aggregate output missing %q:\n%s", want, out)
		}
	}
}

func TestRunDoctorTicketPrintsValidationReport(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = fixtureWorkspace()

	out, err := captureStdout(func() error {
		return runDoctor(nil, []string{"STORY-123"})
	})
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("runDoctor err = %v, want validation failed", err)
	}
	for _, want := range []string{
		"Ticket: STORY-123",
		"✓  STATE.yaml",
		"✗  STATE.yaml.repos.worktree",
		"Some checks failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestRunDoctorFixRemovesStaleLock(t *testing.T) {
	resetCommandGlobals(t)
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TICKET-1")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte("not-a-pid\n"), 0644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatal(err)
	}
	globalWorkspace = root
	doctorFix = true

	// The bare temp workspace fails other doctor checks; only the lock
	// repair is under test here.
	out, _ := captureStdout(func() error {
		return runDoctor(nil, nil)
	})

	if !strings.Contains(out, "stale lock removed") {
		t.Fatalf("doctor --fix output missing removal notice:\n%s", out)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock should be gone, stat err = %v", err)
	}
}

func TestRunDoctorTicketFixRemovesStaleLock(t *testing.T) {
	resetCommandGlobals(t)
	root := t.TempDir()
	featureDir := filepath.Join(root, "features", "TEST-1")
	if err := os.MkdirAll(featureDir, 0755); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(featureDir, "STATE.yaml.lock")
	if err := os.WriteFile(lockPath, []byte("999999999\n"), 0644); err != nil {
		t.Fatal(err)
	}
	globalWorkspace = root
	doctorFix = true

	// The bare feature dir fails validation; only the lock repair is
	// under test here.
	out, _ := captureStdout(func() error {
		return runDoctor(nil, []string{"TEST-1"})
	})

	if !strings.Contains(out, "✓ removed stale STATE.yaml.lock") {
		t.Fatalf("doctor <ticket> --fix output missing removal notice:\n%s", out)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lock should be gone, stat err = %v", err)
	}
}

func TestRunJITDryPrintsResolvedWorkerAndPrompt(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = fixtureWorkspace()
	jitDry = true
	jitWorker = "default:bob"

	out, err := captureStdout(func() error {
		return runJIT(nil, []string{"STORY-123", "check state"})
	})
	if err != nil {
		t.Fatalf("runJIT --dry: %v", err)
	}

	for _, want := range []string{
		"Worker:  Bob (Developer) (codex)",
		"Model:   gpt-5.5",
		"Would run:",
		"## JIT task: STORY-123",
		"check state",
		"orc mark STORY-123 jit",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("jit output missing %q:\n%s", want, out)
		}
	}
}

func TestRunNextDryDoesNotMutatePendingState(t *testing.T) {
	resetCommandGlobals(t)
	// Copy the fixture so a mutation bug can't corrupt the real testdata.
	globalWorkspace = mutableFixtureWorkspace(t)
	nextDry = true

	before := loadTicketState(t, globalWorkspace, "FEAT-001")
	if before.Status != "pending" {
		t.Fatalf("fixture precondition: FEAT-001 status = %q, want pending", before.Status)
	}

	out, err := captureStdout(func() error {
		return runNext(nil, []string{"FEAT-001"})
	})
	if err != nil {
		t.Fatalf("runNext --dry: %v", err)
	}
	if !strings.Contains(out, "Would run:") {
		t.Fatalf("dry output missing preview:\n%s", out)
	}

	after := loadTicketState(t, globalWorkspace, "FEAT-001")
	if after.Status != "pending" {
		t.Errorf("--dry mutated status: %q, want pending", after.Status)
	}
	if len(after.History) != len(before.History) {
		t.Errorf("--dry appended history: %d entries, want %d", len(after.History), len(before.History))
	}
}

func TestRunMarkPauseUpdatesCopiedFixture(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	out, err := captureStdout(func() error {
		return runMark(nil, []string{"HOT-42", "pause", "waiting", "for", "ops"})
	})
	if err != nil {
		t.Fatalf("runMark pause: %v", err)
	}
	if !strings.Contains(out, "Status:  paused") || !strings.Contains(out, "Reason:  waiting for ops") {
		t.Fatalf("pause output unexpected:\n%s", out)
	}

	s := loadTicketState(t, globalWorkspace, "HOT-42")
	if s.Status != "paused" {
		t.Fatalf("status = %q, want paused", s.Status)
	}
	if got := s.History[len(s.History)-1].Result; got != "paused — waiting for ops" {
		t.Fatalf("last history result = %q", got)
	}
}

func TestRunMarkStartUpdatesPendingTicket(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "pending"
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	out, err := captureStdout(func() error {
		return runMark(nil, []string{"HOT-42", "start"})
	})
	if err != nil {
		t.Fatalf("runMark start: %v", err)
	}
	if !strings.Contains(out, "Status:  active") {
		t.Fatalf("start output unexpected:\n%s", out)
	}

	s := loadTicketState(t, globalWorkspace, "HOT-42")
	if s.Status != "active" {
		t.Fatalf("status = %q, want active", s.Status)
	}
}

func TestRunMarkResumeUpdatesPausedTicket(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	out, err := captureStdout(func() error {
		return runMark(nil, []string{"STORY-789", "resume"})
	})
	if err != nil {
		t.Fatalf("runMark resume: %v", err)
	}
	if !strings.Contains(out, "Status:  active") {
		t.Fatalf("resume output unexpected:\n%s", out)
	}

	s := loadTicketState(t, globalWorkspace, "STORY-789")
	if s.Status != "active" {
		t.Fatalf("status = %q, want active", s.Status)
	}
	if got := s.History[len(s.History)-1].Result; got != "resumed" {
		t.Fatalf("last history result = %q, want resumed", got)
	}
	if s.NextAction.Worker != "" {
		t.Fatalf("NextAction.Worker = %q, want cleared", s.NextAction.Worker)
	}
}

func TestRunMarkStartRejectsPaused(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	err := runMark(nil, []string{"STORY-789", "start"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "resume") {
		t.Fatalf("error should mention resume, got: %v", err)
	}
}

func TestRunMarkResumeRejectsNonPaused(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	err := runMark(nil, []string{"HOT-42", "resume"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "paused") {
		t.Fatalf("error should mention paused, got: %v", err)
	}
}

func TestRunMarkDoneUpdatesCopiedFixture(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	markResult = "implemented and verified"

	out, err := captureStdout(func() error {
		return runMark(nil, []string{"HOT-42", "done"})
	})
	if err != nil {
		t.Fatalf("runMark done: %v", err)
	}
	if !strings.Contains(out, "Status:  done") {
		t.Fatalf("done output unexpected:\n%s", out)
	}

	s := loadTicketState(t, globalWorkspace, "HOT-42")
	if s.Status != "done" {
		t.Fatalf("status = %q, want done", s.Status)
	}
	if got := s.History[len(s.History)-1].Result; got != "implemented and verified" {
		t.Fatalf("last history result = %q", got)
	}
}

func TestRunMarkDoneRejectsPendingTicket(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Status = "pending"
		return nil
	}); err != nil {
		t.Fatalf("Update setup: %v", err)
	}

	_, err := captureStdout(func() error {
		return runMark(nil, []string{"HOT-42", "done"})
	})
	if err == nil || !strings.Contains(err.Error(), "cannot mark HOT-42 done from status \"pending\"") {
		t.Fatalf("runMark done err = %v", err)
	}
}

func TestRunMarkJITClearsRuntimeAndRecordsHistory(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	out, err := captureStdout(func() error {
		return runMark(nil, []string{"PROJ-099", "jit", "review", "completed"})
	})
	if err != nil {
		t.Fatalf("runMark jit: %v", err)
	}
	if !strings.Contains(out, "Done: jit task recorded for PROJ-099") {
		t.Fatalf("jit output unexpected:\n%s", out)
	}

	s := loadTicketState(t, globalWorkspace, "PROJ-099")
	if s.Runtime.JIT != nil {
		t.Fatalf("runtime.jit still present: %#v", s.Runtime.JIT)
	}
	last := s.History[len(s.History)-1]
	if last.Stage != "jit" || last.Worker != "zach-the-reviewer" || last.Result != "review completed" {
		t.Fatalf("last history = %#v", last)
	}
}

func TestRunArchiveMovesDoneTicketToArchive(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	if _, err := captureStdout(func() error {
		return runMark(nil, []string{"HOT-42", "done"})
	}); err != nil {
		t.Fatalf("mark done before archive: %v", err)
	}

	out, err := captureStdout(func() error {
		return runArchive(nil, []string{"HOT-42"})
	})
	if err != nil {
		t.Fatalf("runArchive: %v", err)
	}
	if !strings.Contains(out, "Archived: features/_archive/HOT-42-login-500-error/") {
		t.Fatalf("archive output unexpected:\n%s", out)
	}

	activeDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if _, err := os.Stat(activeDir); !os.IsNotExist(err) {
		t.Fatalf("active dir still exists or stat failed unexpectedly: %v", err)
	}
	archivedDir := filepath.Join(globalWorkspace, "features", "_archive", "HOT-42-login-500-error")
	s, err := state.Load(archivedDir)
	if err != nil {
		t.Fatalf("load archived state: %v", err)
	}
	if s.Status != "archived" {
		t.Fatalf("archived status = %q, want archived", s.Status)
	}
}

func TestRunArchiveRefusesActiveTicket(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	_, err := captureStdout(func() error {
		return runArchive(nil, []string{"STORY-123"})
	})
	if err == nil || !strings.Contains(err.Error(), "must be done") {
		t.Fatalf("runArchive err = %v, want refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(globalWorkspace, "features", "STORY-123-add-user-auth")); statErr != nil {
		t.Fatalf("active ticket folder missing after refused archive: %v", statErr)
	}
}

func TestRunDeleteRefusesActiveTicket(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)

	_, err := captureStdout(func() error {
		return runDelete(nil, []string{"STORY-123"})
	})
	if err == nil || !strings.Contains(err.Error(), "must be done or archived") {
		t.Fatalf("runDelete err = %v, want refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(globalWorkspace, "features", "STORY-123-add-user-auth")); statErr != nil {
		t.Fatalf("active ticket folder missing after refused delete: %v", statErr)
	}
}

func TestRunPackInspectPrintsSummary(t *testing.T) {
	resetCommandGlobals(t)
	dir := writeCommandPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
      description: Fast hotfix workflow
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workflows:
    hotfix: hotfix:standard
`)

	out, err := captureStdout(func() error {
		return runPackInspect(nil, []string{dir})
	})
	if err != nil {
		t.Fatalf("runPackInspect: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Pack: hotfix",
		"Fast production fix workflow",
		"hotfix:standard",
		"hotfix:develop",
		"hotfix:bob",
		"workflow hotfix",
		"OK",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack inspect output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPackAvailablePrintsBuiltInPacks(t *testing.T) {
	resetCommandGlobals(t)

	out, err := captureStdout(func() error {
		return runPackAvailable(nil, nil)
	})
	if err != nil {
		t.Fatalf("runPackAvailable: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Available packs:",
		"default",
		"orc pack install <name>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack available output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPackInstallInstallsLocalPack(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: globalWorkspace, SkipDefaultPack: true}); err != nil {
		t.Fatalf("init workspace: %v", err)
	}
	dir := writeCommandPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workflows:
    hotfix: hotfix:standard
`)
	writeCommandFile(t, filepath.Join(dir, "workflow.yaml"), `workflows:
  "hotfix:standard":
    description: Fast production fix workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)

	out, err := captureStdout(func() error {
		return runPackInstall(nil, []string{dir})
	})
	if err != nil {
		t.Fatalf("runPackInstall: %v\n%s", err, out)
	}
	for _, want := range []string{
		"create packs/hotfix/pack.yaml",
		"create stages/hotfix/develop.md",
		"create workers/hotfix/bob.md",
		"Pack installed.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack install output missing %q:\n%s", want, out)
		}
	}
	if _, err := os.Stat(filepath.Join(globalWorkspace, "packs", "hotfix", ".orc-pack.yaml")); err != nil {
		t.Fatalf("missing install provenance: %v", err)
	}
}

func TestRunPackListShowsInstalledLocalPackSource(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: globalWorkspace, SkipDefaultPack: true}); err != nil {
		t.Fatalf("init workspace: %v", err)
	}
	dir := writeCommandPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workflows:
    hotfix: hotfix:standard
`)
	writeCommandFile(t, filepath.Join(dir, "workflow.yaml"), `workflows:
  "hotfix:standard":
    description: Fast production fix workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)
	if err := runPackInstall(nil, []string{dir}); err != nil {
		t.Fatalf("runPackInstall: %v", err)
	}

	out, err := captureStdout(func() error {
		return runPackList(nil, nil)
	})
	if err != nil {
		t.Fatalf("runPackList: %v\n%s", err, out)
	}
	for _, want := range []string{
		"hotfix",
		"active",
		"source: local-path (" + dir + ")",
		"workflows: hotfix:standard",
		"active workflows: hotfix:standard",
		"alias: hotfix -> hotfix:standard",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack list output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPackShowShowsInstalledLocalPackSource(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: globalWorkspace, SkipDefaultPack: true}); err != nil {
		t.Fatalf("init workspace: %v", err)
	}
	dir := writeCommandPack(t, "hotfix", `schema: 1
name: hotfix
description: Fast production fix workflow
provides:
  workflows:
    - id: hotfix:standard
      path: workflow.yaml
  workers:
    - id: hotfix:bob
      path: workers/bob.md
  stages:
    - id: hotfix:develop
      path: stages/develop.md
aliases:
  workflows:
    hotfix: hotfix:standard
`)
	writeCommandFile(t, filepath.Join(dir, "workflow.yaml"), `workflows:
  "hotfix:standard":
    description: Fast production fix workflow
    stages:
      - name: hotfix:develop
        worker: hotfix:bob
        advance: auto
`)
	if err := runPackInstall(nil, []string{dir}); err != nil {
		t.Fatalf("runPackInstall: %v", err)
	}

	out, err := captureStdout(func() error {
		return runPackShow(nil, []string{"hotfix"})
	})
	if err != nil {
		t.Fatalf("runPackShow: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Pack: hotfix",
		"Status: active",
		"Source: local-path (" + dir + ")",
		"Active workflows: hotfix:standard",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack show output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPackListPrintsInstalledPacks(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: globalWorkspace}); err != nil {
		t.Fatalf("init workspace: %v", err)
	}

	out, err := captureStdout(func() error {
		return runPackList(nil, nil)
	})
	if err != nil {
		t.Fatalf("runPackList: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Installed packs:",
		"default",
		"active",
		"path: packs/default",
		"source: builtin (default)",
		"workflows: default:standard",
		"active workflows: default:standard",
		"alias: default -> default:standard",
		"alias: bob -> default:bob",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack list output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPackShowPrintsOneInstalledPack(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = t.TempDir()
	if err := workspace.Init(workspace.InitOptions{Root: globalWorkspace}); err != nil {
		t.Fatalf("init workspace: %v", err)
	}

	out, err := captureStdout(func() error {
		return runPackShow(nil, []string{"default"})
	})
	if err != nil {
		t.Fatalf("runPackShow: %v\n%s", err, out)
	}
	for _, want := range []string{
		"Pack: default",
		"Installed: packs/default",
		"Status: active",
		"Source: builtin (default)",
		"Workflows:",
		"default:standard",
		"Active workflows: default:standard",
		"Workers:",
		"workflow default",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("pack show output missing %q:\n%s", want, out)
		}
	}
}

func TestRunPackInspectReturnsErrorForInvalidPack(t *testing.T) {
	resetCommandGlobals(t)
	dir := writeCommandPack(t, "bad", `schema: 1
name: bad
description: Broken pack
provides:
  workflows:
    - id: bad:standard
      path: missing.yaml
`)

	out, err := captureStdout(func() error {
		return runPackInspect(nil, []string{dir})
	})
	if err == nil || !strings.Contains(err.Error(), "pack validation failed") {
		t.Fatalf("runPackInspect err = %v, want validation failure\n%s", err, out)
	}
	if !strings.Contains(out, `workflow[0].path "missing.yaml"`) {
		t.Fatalf("pack inspect output missing validation error:\n%s", out)
	}
}

func fixtureWorkspace() string {
	return "../../testdata/workspace"
}

func mutableFixtureWorkspace(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workspace")
	if err := os.CopyFS(root, os.DirFS(fixtureWorkspace())); err != nil {
		t.Fatalf("copy fixture workspace: %v", err)
	}
	return root
}

func loadTicketState(t *testing.T, root, query string) *state.State {
	t.Helper()
	featureDir, err := state.FindFeatureDirWithArchive(root, query)
	if err != nil {
		t.Fatalf("FindFeatureDirWithArchive(%q): %v", query, err)
	}
	s, err := state.Load(featureDir)
	if err != nil {
		t.Fatalf("Load(%q): %v", featureDir, err)
	}
	return s
}

func resetCommandGlobals(t *testing.T) {
	t.Helper()

	oldWorkspace := globalWorkspace
	oldDoctorFix := doctorFix
	oldStatusJSON := statusJSON
	oldNextDry := nextDry
	oldNextWorker := nextWorker
	oldNextJSON := nextJSON
	oldJITDry := jitDry
	oldJITWorker := jitWorker
	oldJITTmux := jitTmux
	oldMarkWorker := markWorker
	oldMarkResult := markResult
	oldMarkStage := markStage
	oldReportJSON := reportJSON
	oldReportArchived := reportArchived
	oldPackInspectJSON := packInspectJSON
	t.Cleanup(func() {
		globalWorkspace = oldWorkspace
		doctorFix = oldDoctorFix
		statusJSON = oldStatusJSON
		nextDry = oldNextDry
		nextWorker = oldNextWorker
		nextJSON = oldNextJSON
		jitDry = oldJITDry
		jitWorker = oldJITWorker
		jitTmux = oldJITTmux
		markWorker = oldMarkWorker
		markResult = oldMarkResult
		markStage = oldMarkStage
		reportJSON = oldReportJSON
		reportArchived = oldReportArchived
		packInspectJSON = oldPackInspectJSON
	})

	globalWorkspace = "."
	doctorFix = false
	statusJSON = false
	nextDry = false
	nextWorker = ""
	nextJSON = false
	jitDry = false
	jitWorker = ""
	jitTmux = false
	markWorker = ""
	markResult = ""
	markStage = ""
	reportJSON = false
	reportArchived = false
	packInspectJSON = false
}

func writeCommandPack(t *testing.T, name, manifest string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	for _, sub := range []string{"workers", "stages"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}
	writeCommandFile(t, filepath.Join(dir, "pack.yaml"), manifest)
	writeCommandFile(t, filepath.Join(dir, "workflow.yaml"), "workflows: {}\n")
	writeCommandFile(t, filepath.Join(dir, "workers", "bob.md"), `---
id: hotfix:bob
name: Bob
engine: codex
---

# Bob
`)
	writeCommandFile(t, filepath.Join(dir, "stages", "develop.md"), "# Develop\n")
	return dir
}

func writeCommandFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func captureStdout(fn func() error) (string, error) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w

	var buf bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&buf, r)
		readDone <- copyErr
	}()

	fnErr := fn()
	closeErr := w.Close()
	os.Stdout = orig
	readErr := <-readDone
	_ = r.Close()

	return buf.String(), errors.Join(fnErr, closeErr, readErr)
}
