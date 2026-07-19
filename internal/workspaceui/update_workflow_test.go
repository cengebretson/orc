package workspaceui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cengebretson/orc/internal/state"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/x/ansi"
)

func TestHandleKeyWorkflowDrillIn(t *testing.T) {
	m := testModel(t)
	m, _ = press(t, m, "tab", "tab") // focus workflows
	m, _ = press(t, m, "enter")
	if m.view != viewWorkflowDetail {
		t.Fatalf("view = %v, want viewWorkflowDetail", m.view)
	}
	if m.navigation.workflowName != "default" {
		t.Errorf("wfDetailName = %q, want default", m.navigation.workflowName)
	}

	// chain has 2 steps + 1 repair step → cursor clamps at 2
	m, _ = press(t, m, "right", "right", "right", "right")
	if m.navigation.workflowCursor != 2 {
		t.Errorf("wfDetailCursor = %d, want clamped at 2", m.navigation.workflowCursor)
	}
	m, _ = press(t, m, "left", "left", "left")
	if m.navigation.workflowCursor != 0 {
		t.Errorf("wfDetailCursor = %d, want clamped at 0", m.navigation.workflowCursor)
	}

	m, _ = press(t, m, "esc")
	if m.view != viewDashboard {
		t.Errorf("esc should return to dashboard, got %v", m.view)
	}
}

func TestHandleKeyWorkflowDetailLeftRightAliases(t *testing.T) {
	m := testModel(t)
	m, _ = press(t, m, "tab", "tab", "enter") // drill into workflows/default

	// chain has 2 steps + 1 repair step → cursor clamps at 2
	m, _ = press(t, m, "right", "l", "right")
	if m.navigation.workflowCursor != 2 {
		t.Errorf("wfDetailCursor = %d, want clamped at 2", m.navigation.workflowCursor)
	}
	m, _ = press(t, m, "left", "h", "left")
	if m.navigation.workflowCursor != 0 {
		t.Errorf("wfDetailCursor = %d, want clamped at 0", m.navigation.workflowCursor)
	}
}

func TestWorkflowDetailScrollsSelectedRowAfterWrappedRoute(t *testing.T) {
	m := testModel(t)
	var steps []routeStep
	for _, n := range []string{
		"default:intake", "default:develop", "default:code-review",
		"default:qa-automation", "default:pr-open", "default:evidence",
		"default:release", "default:monitor",
	} {
		steps = append(steps, routeStep{name: n, label: strings.TrimPrefix(n, "default:"), advance: "auto"})
	}
	m.data.workflows = []workflowChain{{name: "default:standard", label: "default", steps: steps}}
	m.view = viewWorkflowDetail
	m.navigation.workflowName = "default:standard"
	m.navigation.workflowCursor = len(steps) - 1
	m.width = 60
	m.height = 12
	m.viewer.viewport = viewport.New(m.width-4, m.height-6)

	m.reRenderWorkflowDetailAndScroll()

	cursorLine := workflowCursorLine(m.viewer.viewport.View())
	if cursorLine < 0 || cursorLine >= m.viewer.viewport.Height {
		t.Fatalf("selected workflow row should be visible, cursorLine=%d height=%d\n%s", cursorLine, m.viewer.viewport.Height, ansi.Strip(m.viewer.viewport.View()))
	}
}

func TestWorkflowDetailPageDownScrollsViewport(t *testing.T) {
	m := testModel(t)
	var steps []routeStep
	for i := 0; i < 24; i++ {
		steps = append(steps, routeStep{name: fmt.Sprintf("default:stage-%02d", i), label: fmt.Sprintf("stage-%02d", i), advance: "auto"})
	}
	m.data.workflows = []workflowChain{{name: "default:standard", label: "default", steps: steps}}
	m.view = viewWorkflowDetail
	m.navigation.workflowName = "default:standard"
	m.width = 90
	m.height = 12
	m.viewer.viewport = viewport.New(m.width-4, m.height-6)
	m.viewer.viewport.SetContent(renderWorkflowDetail(m.navigation.workflowName, m.data.workflows, nil, filepath.Join(m.root, "stages"), m.data.features, 0, m.width-4))

	m, _ = press(t, m, "pgdown")

	if m.viewer.viewport.YOffset == 0 {
		t.Fatalf("pgdown should scroll long workflow detail vertically")
	}
}

func TestWorkflowDetailSupportsLineAndTopBottomScrolling(t *testing.T) {
	m := testModel(t)
	var steps []routeStep
	for i := 0; i < 24; i++ {
		steps = append(steps, routeStep{name: fmt.Sprintf("default:stage-%02d", i), label: fmt.Sprintf("stage-%02d", i), advance: "auto"})
	}
	m.data.workflows = []workflowChain{{name: "default:standard", label: "default", steps: steps}}
	m.view = viewWorkflowDetail
	m.navigation.workflowName = "default:standard"
	m.width = 90
	m.height = 12
	m.viewer.viewport = viewport.New(m.width-4, m.height-6)
	m.viewer.viewport.SetContent(renderWorkflowDetail(m.navigation.workflowName, m.data.workflows, nil, filepath.Join(m.root, "stages"), m.data.features, 0, m.width-4))

	m, _ = press(t, m, "j")
	if m.viewer.viewport.YOffset != 1 {
		t.Fatalf("j workflow scroll offset = %d, want 1", m.viewer.viewport.YOffset)
	}
	m, _ = press(t, m, "G")
	if !m.viewer.viewport.AtBottom() {
		t.Fatalf("G should reach workflow bottom, offset=%d", m.viewer.viewport.YOffset)
	}
	m, _ = press(t, m, "g")
	if m.viewer.viewport.YOffset != 0 {
		t.Fatalf("g should return workflow to top, offset=%d", m.viewer.viewport.YOffset)
	}
}

func TestHandleKeyStageViewerLeftRight(t *testing.T) {
	m := testModel(t)
	m.root = t.TempDir()
	stagesDir := filepath.Join(m.root, "stages")
	if err := os.MkdirAll(stagesDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"develop", "code-review", "pr-repair"} {
		if err := os.WriteFile(filepath.Join(stagesDir, name+".md"), []byte("# "+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	m, _ = press(t, m, "tab", "tab", "enter") // drill into workflows/default
	m, _ = press(t, m, "enter")               // open stage 0 in the viewer
	if m.view != viewFile {
		t.Fatalf("view = %v, want viewFile", m.view)
	}
	if !strings.Contains(m.viewer.title, "develop · step 1 of 3") {
		t.Fatalf("viewerTitle = %q, want develop step 1 of 3", m.viewer.title)
	}

	// right walks pipeline order, updating cursor, title, and path
	m, _ = press(t, m, "right")
	if m.navigation.workflowCursor != 1 {
		t.Errorf("wfDetailCursor = %d, want 1", m.navigation.workflowCursor)
	}
	if !strings.Contains(m.viewer.title, "code-review · step 2 of 3") {
		t.Errorf("viewerTitle = %q, want code-review step 2 of 3", m.viewer.title)
	}

	// continues into repair steps and clamps at the end
	m, _ = press(t, m, "l", "right")
	if m.navigation.workflowCursor != 2 {
		t.Errorf("wfDetailCursor = %d, want clamped at 2", m.navigation.workflowCursor)
	}
	if !strings.Contains(m.viewer.title, "pr-repair · step 3 of 3") {
		t.Errorf("viewerTitle = %q, want pr-repair step 3 of 3", m.viewer.title)
	}

	// left walks back and clamps at the start
	m, _ = press(t, m, "left", "h", "left")
	if m.navigation.workflowCursor != 0 {
		t.Errorf("wfDetailCursor = %d, want clamped at 0", m.navigation.workflowCursor)
	}

	// esc returns to the workflow detail page with the cursor where we left it
	m, _ = press(t, m, "right", "esc")
	if m.view != viewWorkflowDetail {
		t.Fatalf("esc should return to workflow detail, got %v", m.view)
	}
	if m.navigation.workflowCursor != 1 {
		t.Errorf("wfDetailCursor = %d after esc, want 1", m.navigation.workflowCursor)
	}
}

func TestHandleKeyStageViewerUsesNamespacedStagePath(t *testing.T) {
	m := testModel(t)
	m.root = t.TempDir()
	m.data.workflows = []workflowChain{{
		name:  "default:standard",
		steps: []routeStep{{name: "default:develop", advance: "auto"}},
	}}
	m.navigation.items[sectionWorkflows] = []sectionItem{{label: "default:standard", path: ""}}

	stagePath := filepath.Join(m.root, "stages", "default", "develop.md")
	if err := os.MkdirAll(filepath.Dir(stagePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stagePath, []byte("# namespaced stage"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, _ = press(t, m, "tab", "tab", "enter") // drill into workflows/default:standard
	m, _ = press(t, m, "enter")               // open default:develop
	if m.view != viewFile {
		t.Fatalf("view = %v, want viewFile", m.view)
	}
	if body := ansi.Strip(m.viewer.viewport.View()); !strings.Contains(body, "namespaced stage") {
		t.Fatalf("stage viewer did not open namespaced stage file:\n%s", body)
	}
}

func TestHandleKeyAttachTmux(t *testing.T) {
	m := testModel(t)
	// no live session → no command
	_, cmd := press(t, m, "t")
	if cmd != nil {
		t.Error("t without a live tmux session should be a no-op")
	}
	m.data.features[0].s.Runtime.Tmux = &state.TmuxRuntime{Session: "story-1"}
	m.data.features[0].tmuxLive = true
	_, cmd = press(t, m, "t")
	if cmd == nil {
		t.Error("t with a live tmux session should return an attach command")
	}
}

func TestHandleKeyRainbowEasterEgg(t *testing.T) {
	m, cmd := press(t, testModel(t), "o", "r", "c")
	if m.effects.rainbowStep != rainbowSteps {
		t.Errorf("rainbowStep = %d, want %d", m.effects.rainbowStep, rainbowSteps)
	}
	if cmd == nil {
		t.Error("orc easter egg should schedule a rainbow tick")
	}
}
