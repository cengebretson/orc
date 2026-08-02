// Package herdr implements Orc's live multiplexer seam using the herdr CLI.
package herdr

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/mux"
)

type commandRunner func(args ...string) ([]byte, error)

// Backend drives the Herdr session selected by HERDR's caller context.
type Backend struct {
	run commandRunner
}

func New() Backend { return Backend{run: runCLI} }

var _ mux.Backend = Backend{}
var _ mux.TargetBackend = Backend{}
var _ mux.WorktreeTargetBackend = Backend{}
var _ mux.TaskCellBackend = Backend{}
var _ mux.NotificationBackend = Backend{}
var _ mux.AgentControlBackend = Backend{}

func (Backend) Name() string { return "herdr" }

// ShowNotification publishes an Orc transition through Herdr's session-native
// notification surface.
func (b Backend) ShowNotification(notification mux.Notification) error {
	if strings.TrimSpace(notification.Title) == "" {
		return fmt.Errorf("herdr notification requires a title")
	}
	args := []string{"notification", "show", notification.Title}
	if notification.Body != "" {
		args = append(args, "--body", notification.Body)
	}
	if notification.Sound != "" {
		args = append(args, "--sound", notification.Sound)
	}
	if _, err := b.command(args...); err != nil {
		return fmt.Errorf("show herdr notification: %w", err)
	}
	return nil
}

// PromptAgent atomically submits text to the exact recorded Herdr agent pane
// and optionally delegates lifecycle waiting and stall detection to Herdr.
func (b Backend) PromptAgent(target mux.Target, text string, wait bool, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
	if strings.TrimSpace(text) == "" {
		return mux.AgentControlResult{}, fmt.Errorf("herdr agent prompt requires text")
	}
	if !wait && (len(options.Until) > 0 || options.Timeout > 0) {
		return mux.AgentControlResult{}, fmt.Errorf("herdr agent prompt wait options require wait=true")
	}
	target, err := b.resolveExactAgentTarget(target)
	if err != nil {
		return mux.AgentControlResult{}, err
	}
	args := []string{"agent", "prompt", target.Pane, text}
	if wait {
		args = append(args, "--wait")
		args, err = appendAgentWaitOptions(args, options)
		if err != nil {
			return mux.AgentControlResult{}, err
		}
	}
	return b.agentControl(target, args...)
}

// WaitAgent blocks on Herdr's recognized lifecycle state for the exact
// recorded pane. An empty Until list uses Herdr's settled-state defaults.
func (b Backend) WaitAgent(target mux.Target, options mux.AgentControlOptions) (mux.AgentControlResult, error) {
	target, err := b.resolveExactAgentTarget(target)
	if err != nil {
		return mux.AgentControlResult{}, err
	}
	args, err := appendAgentWaitOptions([]string{"agent", "wait", target.Pane}, options)
	if err != nil {
		return mux.AgentControlResult{}, err
	}
	return b.agentControl(target, args...)
}

func appendAgentWaitOptions(args []string, options mux.AgentControlOptions) ([]string, error) {
	for _, status := range options.Until {
		status = strings.TrimSpace(status)
		if status == "" {
			return nil, fmt.Errorf("herdr agent wait status is empty")
		}
		args = append(args, "--until", status)
	}
	if options.Timeout > 0 {
		milliseconds := options.Timeout.Milliseconds()
		if milliseconds == 0 {
			return nil, fmt.Errorf("herdr agent timeout must be at least 1ms")
		}
		args = append(args, "--timeout", strconv.FormatInt(milliseconds, 10))
	}
	return args, nil
}

func (b Backend) resolveExactAgentTarget(target mux.Target) (mux.Target, error) {
	if target.Backend != "" && target.Backend != "herdr" {
		return mux.Target{}, fmt.Errorf("herdr cannot control %s target", target.Backend)
	}
	if target.Workspace == "" || target.Tab == "" || target.Pane == "" {
		return mux.Target{}, fmt.Errorf("herdr agent control requires exact workspace, tab, and pane ids")
	}
	var workspace struct {
		Workspace workspaceInfo `json:"workspace"`
	}
	if err := b.decode(&workspace, "workspace", "get", target.Workspace); err != nil {
		return mux.Target{}, err
	}
	if workspace.Workspace.WorkspaceID != target.Workspace {
		return mux.Target{}, fmt.Errorf("herdr workspace target %s did not resolve exactly", target.Workspace)
	}
	var tab struct {
		Tab tabInfo `json:"tab"`
	}
	if err := b.decode(&tab, "tab", "get", target.Tab); err != nil {
		return mux.Target{}, err
	}
	if tab.Tab.TabID != target.Tab || tab.Tab.WorkspaceID != target.Workspace {
		return mux.Target{}, fmt.Errorf("herdr tab target %s is not in workspace %s", target.Tab, target.Workspace)
	}
	pane, err := b.getPane(target.Pane)
	if err != nil {
		return mux.Target{}, err
	}
	if pane.PaneID != target.Pane || pane.WorkspaceID != target.Workspace || pane.TabID != target.Tab {
		return mux.Target{}, fmt.Errorf("herdr pane target %s is not in recorded tab %s", target.Pane, target.Tab)
	}
	return mux.Target{Backend: "herdr", Workspace: target.Workspace, Tab: target.Tab, Pane: target.Pane}, nil
}

func (b Backend) agentControl(target mux.Target, args ...string) (mux.AgentControlResult, error) {
	out, commandErr := b.command(args...)
	envelope, decodeErr := decodeResponseEnvelope(out)
	if decodeErr == nil && envelope.Error != nil {
		return mux.AgentControlResult{}, &mux.AgentControlError{
			Backend: "herdr", Code: envelope.Error.Code, Message: envelope.Error.Message,
		}
	}
	if commandErr != nil {
		return mux.AgentControlResult{}, commandErr
	}
	if decodeErr != nil {
		return mux.AgentControlResult{}, decodeErr
	}
	var result struct {
		Agent struct {
			Name           string `json:"name"`
			Agent          string `json:"agent"`
			AgentStatus    string `json:"agent_status"`
			StateChangeSeq uint64 `json:"state_change_seq"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(envelope.Result, &result); err != nil {
		return mux.AgentControlResult{}, fmt.Errorf("decode herdr agent result: %w", err)
	}
	if result.Agent.AgentStatus == "" {
		return mux.AgentControlResult{}, fmt.Errorf("herdr agent response has no lifecycle state")
	}
	return mux.AgentControlResult{
		Backend: "herdr", Target: target, Agent: result.Agent.Agent, Name: result.Agent.Name,
		Lifecycle: result.Agent.AgentStatus, StateChangeSeq: result.Agent.StateChangeSeq,
	}, nil
}

func (b Backend) Available() bool {
	if _, err := exec.LookPath("herdr"); err != nil {
		return false
	}
	_, err := b.command("status", "server")
	return err == nil
}

func (b Backend) CreateSession(name, dir string, tabs []string) error {
	_, err := b.CreateTarget(name, dir, tabs)
	return err
}

func (b Backend) CreateTarget(name, dir string, tabs []string) (mux.Target, error) {
	args := []string{"workspace", "create", "--cwd", dir, "--label", name, "--env", "ORC=1", "--no-focus"}
	var created createResult
	if err := b.decode(&created, args...); err != nil {
		return mux.Target{}, err
	}
	return b.finishCreatedTarget(created, tabs, dir)
}

// CreateWorktreeTarget creates a new linked worktree or reopens the exact
// checkout already recorded by Orc, then returns Herdr's opaque target IDs.
func (b Backend) CreateWorktreeTarget(spec mux.WorktreeTargetSpec) (mux.Target, error) {
	args := []string{"worktree"}
	if _, err := os.Stat(spec.WorktreeDir); err == nil {
		args = append(args, "open", "--cwd", spec.SourceDir, "--path", spec.WorktreeDir)
	} else if !os.IsNotExist(err) {
		return mux.Target{}, fmt.Errorf("inspect worktree %s: %w", spec.WorktreeDir, err)
	} else {
		args = append(args, "create", "--cwd", spec.SourceDir, "--branch", spec.Branch, "--path", spec.WorktreeDir)
	}
	args = append(args, "--label", spec.Name, "--no-focus", "--json")

	var created createResult
	if err := b.decode(&created, args...); err != nil {
		return mux.Target{}, err
	}
	return b.finishCreatedTarget(created, spec.Tabs, spec.WorktreeDir)
}

func (b Backend) finishCreatedTarget(created createResult, tabs []string, dir string) (mux.Target, error) {
	if created.Workspace.WorkspaceID == "" || created.Tab.TabID == "" || created.RootPane.PaneID == "" {
		return mux.Target{}, fmt.Errorf("herdr workspace creation returned incomplete target")
	}
	if len(tabs) > 0 && tabs[0] != "" && created.Tab.Label != tabs[0] {
		if _, err := b.command("tab", "rename", created.Tab.TabID, tabs[0]); err != nil {
			return mux.Target{}, fmt.Errorf("rename initial herdr tab: %w", err)
		}
		created.Tab.Label = tabs[0]
	}
	for _, label := range tabs[1:] {
		if label == "" {
			continue
		}
		if _, err := b.command("tab", "create", "--workspace", created.Workspace.WorkspaceID, "--cwd", dir, "--label", label, "--env", "ORC=1", "--no-focus"); err != nil {
			return mux.Target{}, fmt.Errorf("create herdr tab %q: %w", label, err)
		}
	}
	return mux.Target{Backend: "herdr", Workspace: created.Workspace.WorkspaceID, Tab: created.Tab.TabID, Pane: created.RootPane.PaneID}, nil
}

func (b Backend) SessionExists(name string) bool {
	_, err := b.resolveWorkspace(name)
	return err == nil
}

func (b Backend) KillSession(name string) error {
	workspace, err := b.resolveWorkspace(name)
	if err != nil {
		return err
	}
	if workspace.WorkspaceID != name {
		return fmt.Errorf("refusing to close herdr workspace by label %q; an exact workspace id is required", name)
	}
	_, err = b.command("workspace", "close", workspace.WorkspaceID)
	return err
}

func (b Backend) ListSessions() []string {
	workspaces, err := b.listWorkspaces()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(workspaces))
	for _, workspace := range workspaces {
		out = append(out, workspace.WorkspaceID)
	}
	return out
}

func (b Backend) ListPanes() ([]mux.Pane, error) {
	workspaces, err := b.listWorkspaces()
	if err != nil {
		return nil, err
	}
	var out []mux.Pane
	for _, workspace := range workspaces {
		panes, err := b.listPaneInfo(workspace.WorkspaceID)
		if err != nil {
			return nil, err
		}
		for _, pane := range panes {
			attention := lifecycleAttention(pane.AgentStatus)
			entry := mux.Pane{
				Backend: "herdr", ID: pane.PaneID, Session: workspace.WorkspaceID, Window: pane.TabID,
				CWD: firstNonEmpty(pane.ForegroundCWD, pane.CWD), Agent: pane.Agent != "",
				Engine: pane.Agent, ProviderEngine: pane.Agent, Attention: attention,
				Lifecycle: pane.AgentStatus,
			}
			if pane.AgentSession.Value != "" {
				entry.ProviderSessionID = pane.AgentSession.Value
			}
			entry.Ticket = pane.token("ticket")
			entry.Stage = pane.token("stage")
			entry.Worker = pane.token("worker")
			entry.FeatureDir = pane.token("feature_dir")
			out = append(out, entry)
		}
	}
	return out, nil
}

func (b Backend) ResolvePane(session, tab string) (string, error) {
	target, err := b.resolveTarget(mux.Target{Backend: "herdr", Workspace: session, Tab: tab})
	return target.Pane, err
}

func (b Backend) SetWindowMetadata(session, tab string, meta mux.Metadata) error {
	target, err := b.resolveTarget(mux.Target{Backend: "herdr", Workspace: session, Tab: tab})
	if err != nil {
		return err
	}
	return b.SetTargetMetadata(target, meta)
}

func (b Backend) SetPaneMetadata(pane string, meta mux.Metadata) error {
	return b.reportPaneMetadata(pane, meta)
}

func (Backend) SetSessionEnvironment(session, name, value string) error {
	return fmt.Errorf("herdr session environment is immutable; pass %s when creating the workspace or tab", name)
}

func (b Backend) Attention(session, tab string) string {
	resolved, err := b.resolveTarget(mux.Target{Backend: "herdr", Workspace: session, Tab: tab})
	if err != nil {
		return ""
	}
	panes, err := b.listPaneInfo(resolved.Workspace)
	if err != nil {
		return ""
	}
	var candidates []mux.Pane
	for _, pane := range panes {
		if pane.TabID == resolved.Tab {
			candidates = append(candidates, mux.Pane{Attention: lifecycleAttention(pane.AgentStatus)})
		}
	}
	state, _ := mux.RollUpAttention(candidates)
	return state
}

func (b Backend) SendCommand(session, tab, pane, dir, runDir string, argv []string) (string, error) {
	target, err := b.SendTarget(mux.Target{Backend: "herdr", Workspace: session, Tab: tab, Pane: pane}, tab, dir, runDir, argv)
	return target.Pane, err
}

func (b Backend) SendTarget(target mux.Target, tab, dir, runDir string, argv []string) (mux.Target, error) {
	if len(argv) == 0 {
		return mux.Target{}, fmt.Errorf("launch argv is empty")
	}
	target, err := b.resolveOrCreateTab(target, tab, dir)
	if err != nil {
		return mux.Target{}, err
	}

	engine := strings.ToLower(filepath.Base(argv[0]))
	if engine == "claude" || engine == "codex" {
		prompt := argv[len(argv)-1]
		if _, err := b.command("agent", "get", target.Pane); err != nil {
			name := agentName(target.Workspace, tab)
			args := []string{"agent", "start", name, "--kind", engine, "--pane", target.Pane}
			if len(argv) > 2 {
				args = append(args, "--")
				args = append(args, argv[1:len(argv)-1]...)
			}
			if err := b.startAgent(args); err != nil {
				return mux.Target{}, fmt.Errorf("start herdr agent: %w", err)
			}
		}
		if _, err := b.command("agent", "prompt", target.Pane, prompt); err != nil {
			return mux.Target{}, fmt.Errorf("prompt herdr agent: %w", err)
		}
		return target, nil
	}

	if _, err := b.command("pane", "run", target.Pane, shellCommand(argv)); err != nil {
		return mux.Target{}, fmt.Errorf("run command in herdr pane: %w", err)
	}
	return target, nil
}

// ConfigureTaskCell creates Orc-owned utility panes beside the exact agent
// pane. Existing task panes are identified by metadata, never by labels alone,
// so repeated launches do not duplicate panes or adopt user-created ones.
func (b Backend) ConfigureTaskCell(target mux.Target, spec mux.TaskCellSpec) error {
	if spec.TestCommand == "" && spec.WatchCommand == "" {
		return nil
	}
	if spec.Metadata.FeatureDir == "" {
		return fmt.Errorf("herdr task cell requires an exact Orc feature directory")
	}
	target, err := b.resolveTarget(target)
	if err != nil {
		return err
	}
	panes, err := b.listPaneInfo(target.Workspace)
	if err != nil {
		return err
	}

	testPane, err := taskPane(panes, target.Tab, "tests", spec.Metadata.FeatureDir)
	if err != nil {
		return err
	}
	watchPane, err := taskPane(panes, target.Tab, "watch", spec.Metadata.FeatureDir)
	if err != nil {
		return err
	}

	if spec.TestCommand != "" && testPane.PaneID == "" {
		testPane, err = b.splitPane(target.Pane, "right", "0.35", spec.CWD)
		if err != nil {
			return fmt.Errorf("create herdr tests pane: %w", err)
		}
		if err := b.prepareTaskPane(testPane.PaneID, "tests", spec.TestCommand, spec.Metadata); err != nil {
			_, _ = b.command("pane", "close", testPane.PaneID)
			return err
		}
	}

	if spec.WatchCommand != "" && watchPane.PaneID == "" {
		parent, direction, ratio := target.Pane, "right", "0.35"
		if spec.TestCommand != "" && testPane.PaneID != "" {
			parent, direction, ratio = testPane.PaneID, "down", "0.5"
		}
		watchPane, err = b.splitPane(parent, direction, ratio, spec.CWD)
		if err != nil {
			return fmt.Errorf("create herdr watch pane: %w", err)
		}
		if err := b.prepareTaskPane(watchPane.PaneID, "watch", spec.WatchCommand, spec.Metadata); err != nil {
			_, _ = b.command("pane", "close", watchPane.PaneID)
			return err
		}
	}
	return nil
}

func (b Backend) splitPane(parent, direction, ratio, cwd string) (paneInfo, error) {
	var result struct {
		Pane paneInfo `json:"pane"`
	}
	if err := b.decode(&result, "pane", "split", parent, "--direction", direction, "--ratio", ratio, "--cwd", cwd, "--env", "ORC=1", "--no-focus"); err != nil {
		return paneInfo{}, err
	}
	if result.Pane.PaneID == "" {
		return paneInfo{}, fmt.Errorf("herdr pane split returned no pane id")
	}
	return result.Pane, nil
}

func (b Backend) prepareTaskPane(pane, kind, command string, meta mux.Metadata) error {
	meta.Worker = kind
	meta.Engine = ""
	meta.Model = ""
	meta.ProviderSessionID = ""
	if _, err := b.command("pane", "rename", pane, kind); err != nil {
		return fmt.Errorf("rename herdr %s pane: %w", kind, err)
	}
	if _, err := b.command("pane", "run", pane, command); err != nil {
		return fmt.Errorf("run herdr %s pane: %w", kind, err)
	}
	args := []string{
		"pane", "report-metadata", pane, "--source", "orc", "--display-agent", kind,
		"--token", "task_cell=" + kind, "--token", "orc_task_cell_owner=" + meta.FeatureDir,
	}
	args = appendTokens(args, meta)
	if _, err := b.command(args...); err != nil {
		return fmt.Errorf("mark herdr %s pane: %w", kind, err)
	}
	return nil
}

func taskPane(panes []paneInfo, tab, kind, owner string) (paneInfo, error) {
	var matches []paneInfo
	for _, pane := range panes {
		if pane.TabID == tab && pane.token("task_cell") == kind && pane.token("orc_task_cell_owner") == owner {
			matches = append(matches, pane)
		}
	}
	if len(matches) > 1 {
		return paneInfo{}, fmt.Errorf("herdr tab %s has %d Orc-owned %s panes", tab, len(matches), kind)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return paneInfo{}, nil
}

// startAgent tolerates the short shell-startup window after Herdr creates a
// workspace or tab. Creation returns exact IDs before the interactive shell is
// necessarily ready to be replaced, during which agent start reports
// agent_pane_busy. Retry only that structured condition; every other failure is
// returned immediately.
func (b Backend) startAgent(args []string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := b.command(args...)
		if err == nil {
			return nil
		}
		if !strings.Contains(err.Error(), "agent_pane_busy") || time.Now().After(deadline) {
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (b Backend) SetTargetMetadata(target mux.Target, meta mux.Metadata) error {
	workspace, err := b.resolveWorkspace(target.Workspace)
	if err != nil {
		return err
	}
	workspaceArgs := []string{"workspace", "report-metadata", workspace.WorkspaceID, "--source", "orc", "--token", "owned=1"}
	workspaceArgs = appendTokens(workspaceArgs, meta)
	if _, err := b.command(workspaceArgs...); err != nil {
		return fmt.Errorf("report herdr workspace metadata: %w", err)
	}
	if target.Pane == "" {
		target, err = b.resolveTarget(target)
		if err != nil {
			return err
		}
	}
	return b.reportPaneMetadata(target.Pane, meta)
}

func (b Backend) AttachSession(target string) error {
	workspace, err := b.resolveWorkspace(strings.SplitN(target, ":", 2)[0])
	if err != nil {
		return err
	}
	_, err = b.command("workspace", "focus", workspace.WorkspaceID)
	return err
}

func (b Backend) AttachPane(session, tab, pane string) error {
	return b.AttachTarget(mux.Target{Backend: "herdr", Workspace: session, Tab: tab, Pane: pane})
}

func (b Backend) AttachTarget(target mux.Target) error {
	target, err := b.resolveTarget(target)
	if err != nil {
		return err
	}
	if os.Getenv("HERDR_ENV") == "1" {
		if _, err := b.command("agent", "focus", target.Pane); err == nil {
			return nil
		}
		_, err = b.command("tab", "focus", target.Tab)
		return err
	}
	pane, err := b.getPane(target.Pane)
	if err != nil {
		return err
	}
	_, err = b.command("terminal", "attach", pane.TerminalID)
	return err
}

func (b Backend) AttachCommand(session, tab, pane string) (*exec.Cmd, error) {
	target, err := b.resolveTarget(mux.Target{Backend: "herdr", Workspace: session, Tab: tab, Pane: pane})
	if err != nil {
		return nil, err
	}
	if os.Getenv("HERDR_ENV") == "1" {
		return exec.Command("herdr", "agent", "focus", target.Pane), nil
	}
	info, err := b.getPane(target.Pane)
	if err != nil {
		return nil, err
	}
	return exec.Command("herdr", "terminal", "attach", info.TerminalID), nil
}

func (b Backend) AttachHint(session, tab string) string {
	return b.AttachTargetHint(mux.Target{Backend: "herdr", Workspace: session, Tab: tab})
}

func (b Backend) AttachTargetHint(target mux.Target) string {
	resolved, err := b.resolveTarget(target)
	if err != nil || resolved.Pane == "" {
		return "herdr workspace focus " + target.Workspace
	}
	return "herdr agent attach " + resolved.Pane
}

func (b Backend) resolveOrCreateTab(target mux.Target, label, dir string) (mux.Target, error) {
	workspace, err := b.resolveWorkspace(target.Workspace)
	if err != nil {
		return mux.Target{}, err
	}
	target.Workspace = workspace.WorkspaceID
	if label == "" {
		label = target.Tab
	}
	if target.Tab != "" {
		if tab, err := b.resolveTab(target.Workspace, target.Tab); err == nil && (label == "" || tab.Label == label || tab.TabID == label) {
			target.Tab = tab.TabID
			return b.resolveTarget(target)
		}
	}
	if tab, err := b.resolveTab(target.Workspace, label); err == nil {
		target.Tab = tab.TabID
		return b.resolveTarget(target)
	}
	var created createResult
	if err := b.decode(&created, "tab", "create", "--workspace", target.Workspace, "--cwd", dir, "--label", label, "--env", "ORC=1", "--no-focus"); err != nil {
		return mux.Target{}, err
	}
	return mux.Target{Backend: "herdr", Workspace: target.Workspace, Tab: created.Tab.TabID, Pane: created.RootPane.PaneID}, nil
}

func (b Backend) resolveTarget(target mux.Target) (mux.Target, error) {
	workspace, err := b.resolveWorkspace(target.Workspace)
	if err != nil {
		return mux.Target{}, err
	}
	tab, err := b.resolveTab(workspace.WorkspaceID, target.Tab)
	if err != nil {
		return mux.Target{}, err
	}
	if target.Pane != "" {
		pane, err := b.getPane(target.Pane)
		if err == nil && pane.WorkspaceID == workspace.WorkspaceID && pane.TabID == tab.TabID {
			return mux.Target{Backend: "herdr", Workspace: workspace.WorkspaceID, Tab: tab.TabID, Pane: pane.PaneID}, nil
		}
	}
	panes, err := b.listPaneInfo(workspace.WorkspaceID)
	if err != nil {
		return mux.Target{}, err
	}
	var matches []paneInfo
	for _, pane := range panes {
		if pane.TabID == tab.TabID {
			matches = append(matches, pane)
		}
	}
	if len(matches) != 1 {
		return mux.Target{}, fmt.Errorf("herdr tab %s has %d panes and no exact pane target", tab.TabID, len(matches))
	}
	return mux.Target{Backend: "herdr", Workspace: workspace.WorkspaceID, Tab: tab.TabID, Pane: matches[0].PaneID}, nil
}

func (b Backend) resolveWorkspace(value string) (workspaceInfo, error) {
	if value == "" {
		return workspaceInfo{}, fmt.Errorf("herdr workspace id is empty")
	}
	var got struct {
		Workspace workspaceInfo `json:"workspace"`
	}
	if err := b.decode(&got, "workspace", "get", value); err == nil && got.Workspace.WorkspaceID != "" {
		return got.Workspace, nil
	}
	workspaces, err := b.listWorkspaces()
	if err != nil {
		return workspaceInfo{}, err
	}
	var matches []workspaceInfo
	for _, workspace := range workspaces {
		if workspace.Label == value {
			matches = append(matches, workspace)
		}
	}
	if len(matches) != 1 {
		return workspaceInfo{}, fmt.Errorf("herdr workspace label %q matched %d workspaces", value, len(matches))
	}
	return matches[0], nil
}

func (b Backend) resolveTab(workspace, value string) (tabInfo, error) {
	if value != "" {
		var got struct {
			Tab tabInfo `json:"tab"`
		}
		if err := b.decode(&got, "tab", "get", value); err == nil && got.Tab.WorkspaceID == workspace {
			return got.Tab, nil
		}
	}
	tabs, err := b.listTabs(workspace)
	if err != nil {
		return tabInfo{}, err
	}
	var matches []tabInfo
	for _, tab := range tabs {
		if tab.Label == value || (value == "" && tab.Focused) {
			matches = append(matches, tab)
		}
	}
	if len(matches) != 1 {
		return tabInfo{}, fmt.Errorf("herdr tab %q in %s matched %d tabs", value, workspace, len(matches))
	}
	return matches[0], nil
}

func (b Backend) listWorkspaces() ([]workspaceInfo, error) {
	var result struct {
		Workspaces []workspaceInfo `json:"workspaces"`
	}
	if err := b.decode(&result, "workspace", "list"); err != nil {
		return nil, err
	}
	return result.Workspaces, nil
}

func (b Backend) listTabs(workspace string) ([]tabInfo, error) {
	var result struct {
		Tabs []tabInfo `json:"tabs"`
	}
	if err := b.decode(&result, "tab", "list", "--workspace", workspace); err != nil {
		return nil, err
	}
	return result.Tabs, nil
}

func (b Backend) listPaneInfo(workspace string) ([]paneInfo, error) {
	var result struct {
		Panes []paneInfo `json:"panes"`
	}
	if err := b.decode(&result, "pane", "list", "--workspace", workspace); err != nil {
		return nil, err
	}
	return result.Panes, nil
}

func (b Backend) getPane(id string) (paneInfo, error) {
	var result struct {
		Pane paneInfo `json:"pane"`
	}
	if err := b.decode(&result, "pane", "get", id); err != nil {
		return paneInfo{}, err
	}
	return result.Pane, nil
}

func (b Backend) reportPaneMetadata(pane string, meta mux.Metadata) error {
	args := []string{"pane", "report-metadata", pane, "--source", "orc"}
	if display := firstNonEmpty(meta.Worker, meta.Engine); display != "" {
		args = append(args, "--display-agent", display)
	}
	args = appendTokens(args, meta)
	_, err := b.command(args...)
	return err
}

func appendTokens(args []string, meta mux.Metadata) []string {
	values := map[string]string{
		"ticket": meta.Ticket, "stage": meta.Stage, "worker": meta.Worker,
		"engine": meta.Engine, "provider_session": meta.ProviderSessionID,
		"feature_dir": meta.FeatureDir, "workflow": meta.Workflow,
		"repository": meta.Repository, "branch": meta.Branch,
		"next_action": meta.NextAction, "model": meta.Model,
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if values[key] != "" {
			args = append(args, "--token", key+"="+values[key])
		}
	}
	return args
}

type responseEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeResponseEnvelope(out []byte) (responseEnvelope, error) {
	var envelope responseEnvelope
	if err := json.Unmarshal(out, &envelope); err != nil {
		return responseEnvelope{}, fmt.Errorf("decode herdr response: %w", err)
	}
	return envelope, nil
}

func (b Backend) decode(dst any, args ...string) error {
	out, err := b.command(args...)
	if err != nil {
		return err
	}
	envelope, err := decodeResponseEnvelope(out)
	if err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("herdr %s: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if len(envelope.Result) == 0 {
		return fmt.Errorf("herdr response has no result")
	}
	if err := json.Unmarshal(envelope.Result, dst); err != nil {
		return fmt.Errorf("decode herdr result: %w", err)
	}
	return nil
}

func (b Backend) command(args ...string) ([]byte, error) {
	runner := b.run
	if runner == nil {
		runner = runCLI
	}
	return runner(args...)
}

func runCLI(args ...string) ([]byte, error) {
	cmd := exec.Command("herdr", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("herdr %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return out, nil
}

type createResult struct {
	Workspace workspaceInfo `json:"workspace"`
	Tab       tabInfo       `json:"tab"`
	RootPane  paneInfo      `json:"root_pane"`
}

type workspaceInfo struct {
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
}

type tabInfo struct {
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
}

type agentSession struct {
	Agent string `json:"agent"`
	Value string `json:"value"`
}

type paneInfo struct {
	PaneID        string            `json:"pane_id"`
	WorkspaceID   string            `json:"workspace_id"`
	TabID         string            `json:"tab_id"`
	TerminalID    string            `json:"terminal_id"`
	Agent         string            `json:"agent"`
	AgentStatus   string            `json:"agent_status"`
	AgentSession  agentSession      `json:"agent_session"`
	CWD           string            `json:"cwd"`
	ForegroundCWD string            `json:"foreground_cwd"`
	Tokens        map[string]string `json:"tokens"`
	Metadata      map[string]any    `json:"metadata"`
}

func (p paneInfo) token(name string) string {
	if p.Tokens != nil {
		return p.Tokens[name]
	}
	if p.Metadata != nil {
		if tokens, ok := p.Metadata["tokens"].(map[string]any); ok {
			if value, ok := tokens[name].(string); ok {
				return value
			}
		}
	}
	return ""
}

func lifecycleAttention(state string) string {
	switch state {
	case "blocked":
		return mux.AttentionBlocked
	case "done":
		return mux.AttentionDone
	default:
		return ""
	}
}

func agentName(workspace, tab string) string {
	raw := strings.ToLower("orc-" + workspace + "-" + tab)
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-_")
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		name = "orc-" + name
	}
	if len(name) > 32 {
		name = name[:32]
	}
	return name
}

func shellCommand(argv []string) string {
	parts := make([]string, len(argv))
	for i, value := range argv {
		parts[i] = "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
