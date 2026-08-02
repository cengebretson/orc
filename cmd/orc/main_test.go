package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/mux/muxtest"
	orcnotify "github.com/cengebretson/orc/internal/notify"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/workspace"
)

type ctlTestBackend struct {
	*muxtest.Fake
	stateFunc  func(mux.Target) (mux.AgentControlResult, error)
	promptFunc func(mux.Target, string, bool, mux.AgentControlOptions) (mux.AgentControlResult, error)
	waitFunc   func(mux.Target, mux.AgentControlOptions) (mux.AgentControlResult, error)
}

func (b *ctlTestBackend) StateAgent(target mux.Target) (mux.AgentControlResult, error) {
	return b.stateFunc(target)
}

func (b *ctlTestBackend) PromptAgent(target mux.Target, text string, wait bool, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
	return b.promptFunc(target, text, wait, options)
}

func (b *ctlTestBackend) WaitAgent(target mux.Target, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
	return b.waitFunc(target, options)
}

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
		"Stage:     default:standard · default:develop",
		"Worker:  Bob (Developer) (codex)",
		"Run `orc next` to launch.",
		"⚠  state has problems — run `orc doctor STORY-123` for details",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestRunCtlAgentStateTargetsRecordedHerdrPane(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Runtime.Mux = &state.MuxRuntime{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"}
		s.Runtime.Tmux = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var gotTarget mux.Target
	muxBackend = &ctlTestBackend{
		Fake: &muxtest.Fake{NameFunc: func() string { return "herdr" }},
		stateFunc: func(target mux.Target) (mux.AgentControlResult, error) {
			gotTarget = target
			return mux.AgentControlResult{
				Backend: "herdr", Target: target, Agent: "codex", Name: "builder",
				Lifecycle: "working", StateChangeSeq: 14,
			}, nil
		},
		promptFunc: func(mux.Target, string, bool, mux.AgentControlOptions) (mux.AgentControlResult, error) {
			t.Fatal("unexpected prompt")
			return mux.AgentControlResult{}, nil
		},
		waitFunc: func(mux.Target, mux.AgentControlOptions) (mux.AgentControlResult, error) {
			t.Fatal("unexpected wait")
			return mux.AgentControlResult{}, nil
		},
	}
	ctlStateTicket = "HOT-42"

	out, err := captureStdout(func() error { return runCtlAgentState(nil, nil) })
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget.Pane != "w9:p1" {
		t.Fatalf("target = %#v", gotTarget)
	}
	var payload struct {
		Type   string                 `json:"type"`
		Ticket string                 `json:"ticket"`
		Agent  mux.AgentControlResult `json:"agent"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\n%s", err, out)
	}
	if payload.Type != "agent_state" || payload.Ticket != "HOT-42" || payload.Agent.Lifecycle != "working" || payload.Agent.StateChangeSeq != 14 {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestRunCtlAgentPromptTargetsRecordedHerdrPane(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Runtime.Mux = &state.MuxRuntime{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"}
		s.Runtime.Tmux = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var gotTarget mux.Target
	var gotText string
	var gotOptions mux.AgentControlOptions
	muxBackend = &ctlTestBackend{
		Fake: &muxtest.Fake{NameFunc: func() string { return "herdr" }},
		promptFunc: func(target mux.Target, text string, wait bool, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
			if !wait {
				t.Fatal("prompt wait = false")
			}
			gotTarget, gotText, gotOptions = target, text, options
			return mux.AgentControlResult{Backend: "herdr", Target: target, Agent: "codex", Lifecycle: "blocked"}, nil
		},
		waitFunc: func(mux.Target, mux.AgentControlOptions) (mux.AgentControlResult, error) {
			t.Fatal("unexpected wait")
			return mux.AgentControlResult{}, nil
		},
	}
	ctlPromptTicket = "HOT-42"
	ctlPromptWait = true
	ctlPromptUntil = []string{"blocked"}
	ctlPromptTimeout = 2 * time.Minute

	out, err := captureStdout(func() error { return runCtlAgentPrompt(nil, []string{"review this"}) })
	if err != nil {
		t.Fatal(err)
	}
	if gotTarget.Pane != "w9:p1" || gotText != "review this" || gotOptions.Timeout != 2*time.Minute || len(gotOptions.Until) != 1 || gotOptions.Until[0] != "blocked" {
		t.Fatalf("target=%#v text=%q options=%#v", gotTarget, gotText, gotOptions)
	}
	if !strings.Contains(out, `"agent_prompted"`) || !strings.Contains(out, `"blocked"`) {
		t.Fatalf("output = %s", out)
	}
}

func TestRunCtlAgentWaitUsesBackendLifecycleWait(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Runtime.Mux = &state.MuxRuntime{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"}
		s.Runtime.Tmux = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	muxBackend = &ctlTestBackend{
		Fake: &muxtest.Fake{NameFunc: func() string { return "herdr" }},
		promptFunc: func(mux.Target, string, bool, mux.AgentControlOptions) (mux.AgentControlResult, error) {
			t.Fatal("unexpected prompt")
			return mux.AgentControlResult{}, nil
		},
		waitFunc: func(target mux.Target, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
			if target.Pane != "w9:p1" || options.Timeout != 30*time.Second || len(options.Until) != 1 || options.Until[0] != "done" {
				t.Fatalf("target=%#v options=%#v", target, options)
			}
			return mux.AgentControlResult{Backend: "herdr", Target: target, Agent: "codex", Lifecycle: "done"}, nil
		},
	}
	ctlWaitTicket = "HOT-42"
	ctlWaitUntil = []string{"done"}
	ctlWaitTimeout = 30 * time.Second

	out, err := captureStdout(func() error { return runCtlAgentWait(nil, nil) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"agent_waited"`) || !strings.Contains(out, `"done"`) {
		t.Fatalf("output = %s", out)
	}
}

func TestRunCtlAgentPromptRejectsBackendWithoutLifecycleControl(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Runtime.Mux = &state.MuxRuntime{Backend: "tmux", Workspace: "hot-42", Tab: "develop", Pane: "%9"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ctlPromptTicket = "HOT-42"

	err := runCtlAgentPrompt(nil, []string{"review this"})
	var commandErr *ctlCommandError
	if !errors.As(err, &commandErr) || commandErr.code != "unsupported_backend" {
		t.Fatalf("error = %#v", err)
	}
}

func TestWriteCtlErrorPreservesHerdrStallCode(t *testing.T) {
	var out bytes.Buffer
	writeCtlError(&out, &mux.AgentControlError{Backend: "herdr", Code: "agent_prompt_stalled", Message: "no observed state change"})
	if !strings.Contains(out.String(), `"code":"agent_prompt_stalled"`) || !strings.Contains(out.String(), `"message":"no observed state change"`) {
		t.Fatalf("error output = %s", out.String())
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

func TestRunArtifactsCurrentStageReportsMissingArtifacts(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = writeArtifactsWorkspace(t)

	out, err := captureStdout(func() error {
		return runArtifacts(nil, []string{"ART-1"})
	})
	if err == nil || !strings.Contains(err.Error(), "artifacts not ready") {
		t.Fatalf("runArtifacts err = %v, want artifacts not ready\n%s", err, out)
	}
	for _, want := range []string{
		"Artifacts: ART-1",
		"Scope:     current",
		"current stage",
		"DECISIONS.md missing",
		"develop/HANDOFF.md missing",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("artifacts output missing %q:\n%s", want, out)
		}
	}
}

// A core doc that is also a stage required_artifact must appear once, not once
// per check list — the current-stage group dedups core-vs-required like the
// block gate does.
func TestRunArtifactsCurrentStageDedupsCoreAndRequired(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = writeArtifactsWorkspace(t)
	artifactsJSON = true

	out, err := captureStdout(func() error {
		return runArtifacts(nil, []string{"ART-1"})
	})
	if err == nil || !strings.Contains(err.Error(), "artifacts not ready") {
		t.Fatalf("runArtifacts --json err = %v, want artifacts not ready\n%s", err, out)
	}
	var payload artifactReport
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal artifacts json: %v\n%s", err, out)
	}
	planCount := 0
	for _, group := range payload.Groups {
		for _, artifact := range group.Artifacts {
			if artifact.Path == "PLAN.md" {
				planCount++
			}
		}
	}
	if planCount != 1 {
		t.Fatalf("PLAN.md appears %d times in current-stage report, want 1:\n%s", planCount, out)
	}
}

func TestRunArtifactsJSONAllScope(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = writeArtifactsWorkspace(t)
	artifactsAll = true
	artifactsJSON = true

	out, err := captureStdout(func() error {
		return runArtifacts(nil, []string{"ART-1"})
	})
	if err == nil || !strings.Contains(err.Error(), "artifacts not ready") {
		t.Fatalf("runArtifacts --json err = %v, want artifacts not ready\n%s", err, out)
	}
	var payload artifactReport
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("unmarshal artifacts json: %v\n%s", err, out)
	}
	if payload.Ticket != "ART-1" || payload.Scope != "all" || payload.OK {
		t.Fatalf("payload ticket=%q scope=%q ok=%v, want ART-1/all/false", payload.Ticket, payload.Scope, payload.OK)
	}
	var foundQA bool
	for _, group := range payload.Groups {
		if group.Name == "qa-automation" {
			foundQA = true
		}
	}
	if !foundQA {
		t.Fatalf("all-scope payload missing qa-automation group: %+v", payload.Groups)
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

func TestRunDoctorSystemPrintsInstallReport(t *testing.T) {
	resetCommandGlobals(t)
	doctorSystem = true
	version = "1.2.3"
	doctorLookPath = func(name string) (string, error) {
		switch name {
		case "orc":
			return "/usr/local/bin/orc", nil
		case "tmux", "chafa", "claude", "codex", "cursor":
			return "", os.ErrNotExist
		default:
			return "", os.ErrNotExist
		}
	}

	out, err := captureStdout(func() error {
		return runDoctor(nil, nil)
	})
	if err != nil {
		t.Fatalf("runDoctor --system: %v", err)
	}
	for _, want := range []string{
		"System",
		"1.2.3",
		"orc",
		"tmux",
		"chafa",
		"claude",
		"codex",
		"cursor",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("system doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestRunDoctorSystemRejectsTicketArg(t *testing.T) {
	resetCommandGlobals(t)
	doctorSystem = true

	_, err := captureStdout(func() error {
		return runDoctor(nil, []string{"TEST-1"})
	})
	if err == nil || !strings.Contains(err.Error(), "doctor --system does not accept a ticket") {
		t.Fatalf("runDoctor --system ticket err = %v", err)
	}
}

func TestResolveRootFindsNearestWorkspace(t *testing.T) {
	resetCommandGlobals(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "orc.yaml"), []byte("repos: []\nworkflows: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "worktrees", "repo", "feature")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)

	got, err := resolveRoot(".")
	if err != nil {
		t.Fatalf("resolveRoot: %v", err)
	}
	if got != root {
		t.Fatalf("resolveRoot = %q, want %q", got, root)
	}
}

func TestResolveRootExplicitPathDoesNotRequireWorkspaceMarker(t *testing.T) {
	root := t.TempDir()
	got, err := resolveRoot(root)
	if err != nil {
		t.Fatalf("resolveRoot explicit path: %v", err)
	}
	if got != root {
		t.Fatalf("resolveRoot explicit path = %q, want %q", got, root)
	}
}

func TestResolveRootMissingWorkspaceIsClear(t *testing.T) {
	resetCommandGlobals(t)
	dir := t.TempDir()
	t.Chdir(dir)

	_, err := resolveRoot(".")
	if err == nil {
		t.Fatal("resolveRoot should fail outside an orc workspace")
	}
	for _, want := range []string{"orc workspace not found", "--workspace"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("resolveRoot error missing %q: %v", want, err)
		}
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

func TestRunMarkSendsNotificationsAfterSuccessfulTransitions(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantEvent string
		wantStage string
	}{
		{name: "pause", args: []string{"HOT-42", "pause", "waiting"}, wantEvent: "blocked", wantStage: "hotfix:patch"},
		{name: "done", args: []string{"HOT-42", "done"}, wantEvent: "complete", wantStage: "hotfix:patch"},
		{name: "next", args: []string{"HOT-42", "next"}, wantEvent: "complete", wantStage: "hotfix:patch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCommandGlobals(t)
			globalWorkspace = mutableFixtureWorkspace(t)
			if tt.name == "next" {
				featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
				if err := state.Update(featureDir, func(s *state.State) error {
					s.Stage.Name = "hotfix:intake"
					s.Stage.Worker = "default:fred"
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			var events []orcnotify.Event
			sendTransitionNotification = func(_ config.NotifySettings, event orcnotify.Event) error {
				events = append(events, event)
				return nil
			}
			if _, err := captureStdout(func() error { return runMark(nil, tt.args) }); err != nil {
				t.Fatalf("runMark: %v", err)
			}
			if len(events) != 1 {
				t.Fatalf("notifications = %#v, want one", events)
			}
			got := events[0]
			if got.Name != tt.wantEvent || got.Ticket != "HOT-42" || got.Slug != "HOT-42-login-500-error" || got.Stage != tt.wantStage || got.Workflow != "hotfix:standard" || got.WorkDir != globalWorkspace {
				t.Fatalf("notification = %#v", got)
			}
		})
	}
}

func TestRunMarkNotificationFailureDoesNotRollBackTransition(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	sendTransitionNotification = func(config.NotifySettings, orcnotify.Event) error {
		return errors.New("desktop notifier unavailable")
	}

	if _, err := captureStdout(func() error { return runMark(nil, []string{"HOT-42", "done"}) }); err != nil {
		t.Fatalf("runMark done: %v", err)
	}
	if got := loadTicketState(t, globalWorkspace, "HOT-42").Status; got != "done" {
		t.Fatalf("status = %q, want done", got)
	}
}

func TestRunMarkSendsNativeNotificationForRecordedHerdrRuntime(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	featureDir := filepath.Join(globalWorkspace, "features", "HOT-42-login-500-error")
	if err := state.Update(featureDir, func(s *state.State) error {
		s.Runtime.Mux = &state.MuxRuntime{Backend: "herdr", Workspace: "w9", Tab: "w9:t1", Pane: "w9:p1"}
		s.Runtime.Tmux = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	var backendName string
	var got orcnotify.Event
	sendNativeTransitionNotification = func(backend mux.Backend, event orcnotify.Event) error {
		backendName = backend.Name()
		got = event
		return nil
	}
	sendTransitionNotification = func(config.NotifySettings, orcnotify.Event) error { return nil }

	if _, err := captureStdout(func() error {
		return runMark(nil, []string{"HOT-42", "pause", "waiting"})
	}); err != nil {
		t.Fatal(err)
	}
	if backendName != "herdr" || got.Name != "blocked" || got.Ticket != "HOT-42" {
		t.Fatalf("native notification backend=%q event=%#v", backendName, got)
	}
}

func TestRunMarkNativeNotificationFailureDoesNotSkipConfiguredNotification(t *testing.T) {
	resetCommandGlobals(t)
	globalWorkspace = mutableFixtureWorkspace(t)
	sendNativeTransitionNotification = func(mux.Backend, orcnotify.Event) error {
		return errors.New("herdr notification unavailable")
	}
	customCalls := 0
	sendTransitionNotification = func(config.NotifySettings, orcnotify.Event) error {
		customCalls++
		return nil
	}

	if _, err := captureStdout(func() error {
		return runMark(nil, []string{"HOT-42", "done"})
	}); err != nil {
		t.Fatal(err)
	}
	if customCalls != 1 {
		t.Fatalf("configured notification calls = %d, want 1", customCalls)
	}
	if got := loadTicketState(t, globalWorkspace, "HOT-42").Status; got != "done" {
		t.Fatalf("status = %q, want done", got)
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

func writeArtifactsWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mkdir := func(path string) {
		t.Helper()
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	mkdir(filepath.Join(root, "workers", "default"))
	mkdir(filepath.Join(root, "features", "ART-1"))
	writeCommandFile(t, filepath.Join(root, "orc.yaml"), `
settings:
  default_workflow: default
  artifact_policy: warn
workflows:
  default:
    stages:
      - name: develop
        worker: default:bob
        advance: manual
        required_artifacts:
          - PLAN.md
          - develop/HANDOFF.md
      - name: qa-automation
        worker: default:bob
        advance: auto
        required_artifacts:
          - qa-automation/RUNS.md
`)
	writeCommandFile(t, filepath.Join(root, "workers", "default", "bob.md"), `---
id: default:bob
name: Bob
engine: codex
---
`)
	featureDir := filepath.Join(root, "features", "ART-1")
	writeCommandFile(t, filepath.Join(featureDir, "TICKET.md"), "# Ticket\n")
	writeCommandFile(t, filepath.Join(featureDir, "SPEC.md"), "# Spec\n")
	writeCommandFile(t, filepath.Join(featureDir, "PLAN.md"), "# Plan\n")
	if err := state.Create(featureDir, &state.State{
		Ticket:   "ART-1",
		Slug:     "ART-1",
		Status:   "active",
		Workflow: "default",
		Stage:    state.Stage{Name: "develop", Worker: "default:bob"},
	}); err != nil {
		t.Fatalf("create artifact state: %v", err)
	}
	return root
}

func resetCommandGlobals(t *testing.T) {
	t.Helper()

	oldWorkspace := globalWorkspace
	oldMux := globalMux
	oldMuxBackend := muxBackend
	oldVersion := version
	oldDoctorFix := doctorFix
	oldDoctorSystem := doctorSystem
	oldDoctorLookPath := doctorLookPath
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
	oldArtifactsAll := artifactsAll
	oldArtifactsJSON := artifactsJSON
	oldPackInspectJSON := packInspectJSON
	oldSendTransitionNotification := sendTransitionNotification
	oldSendNativeTransitionNotification := sendNativeTransitionNotification
	oldCtlStateTicket := ctlStateTicket
	oldCtlPromptTicket := ctlPromptTicket
	oldCtlPromptWait := ctlPromptWait
	oldCtlPromptUntil := ctlPromptUntil
	oldCtlPromptTimeout := ctlPromptTimeout
	oldCtlWaitTicket := ctlWaitTicket
	oldCtlWaitUntil := ctlWaitUntil
	oldCtlWaitTimeout := ctlWaitTimeout
	t.Cleanup(func() {
		globalWorkspace = oldWorkspace
		globalMux = oldMux
		muxBackend = oldMuxBackend
		version = oldVersion
		doctorFix = oldDoctorFix
		doctorSystem = oldDoctorSystem
		doctorLookPath = oldDoctorLookPath
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
		artifactsAll = oldArtifactsAll
		artifactsJSON = oldArtifactsJSON
		packInspectJSON = oldPackInspectJSON
		sendTransitionNotification = oldSendTransitionNotification
		sendNativeTransitionNotification = oldSendNativeTransitionNotification
		ctlStateTicket = oldCtlStateTicket
		ctlPromptTicket = oldCtlPromptTicket
		ctlPromptWait = oldCtlPromptWait
		ctlPromptUntil = oldCtlPromptUntil
		ctlPromptTimeout = oldCtlPromptTimeout
		ctlWaitTicket = oldCtlWaitTicket
		ctlWaitUntil = oldCtlWaitUntil
		ctlWaitTimeout = oldCtlWaitTimeout
	})

	globalWorkspace = "."
	globalMux = ""
	muxBackend = tmux.New()
	version = "dev"
	doctorFix = false
	doctorSystem = false
	doctorLookPath = exec.LookPath
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
	artifactsAll = false
	artifactsJSON = false
	packInspectJSON = false
	ctlStateTicket = ""
	ctlPromptTicket = ""
	ctlPromptWait = false
	ctlPromptUntil = nil
	ctlPromptTimeout = 2 * time.Minute
	ctlWaitTicket = ""
	ctlWaitUntil = nil
	ctlWaitTimeout = 2 * time.Minute
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
