package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

const (
	AttentionInput   = "input"
	AttentionBlocked = "blocked"
	AttentionReview  = "review"
	AttentionDone    = "done"
	EnvResumedFrom   = "ORC_RESUMED_FROM"
)

// WindowMetadata identifies the Orc work running in a tmux window. STATE.yaml
// remains authoritative; these options provide a live reverse lookup from tmux.
type WindowMetadata struct {
	Ticket            string
	Stage             string
	Worker            string
	Engine            string
	ProviderSessionID string
	FeatureDir        string
}

// Available returns true if tmux is installed and in PATH.
func Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// SessionExists returns true if a tmux session with the given name exists.
func SessionExists(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// WindowExists returns true if the named window exists in the session.
func WindowExists(session, window string) bool {
	target := session + ":" + window
	return exec.Command("tmux", "select-window", "-t", target).Run() == nil
}

// CreateSession creates a detached tmux session with windows named after each workflow.
func CreateSession(slug, featureDir string, workflows []string) error {
	if len(workflows) == 0 {
		workflows = []string{"shell"}
	}

	// Create detached session with first window
	if err := exec.Command("tmux", "new-session", "-d", "-s", slug, "-n", workflows[0], "-c", featureDir).Run(); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	// Add remaining workflow windows
	for _, w := range workflows[1:] {
		if err := exec.Command("tmux", "new-window", "-t", slug, "-n", w, "-c", featureDir).Run(); err != nil {
			return fmt.Errorf("add window %s: %w", w, err)
		}
	}

	// Select the first window — ignore error if focus fails (non-fatal)
	_ = exec.Command("tmux", "select-window", "-t", slug+":"+workflows[0]).Run()
	return nil
}

// SendCommand sends a shell command to a window in the session, creating the window if needed.
// runDir is the working directory the command should execute from.
func SendCommand(session, window, featureDir, runDir string, argv []string) error {
	_, err := SendCommandTarget(session, window, "", featureDir, runDir, argv)
	return err
}

// SendCommandTarget sends a command to an exact pane and returns the pane id.
// When pane is empty, a single pane (or a single pane marked @orc_agent) must
// identify the target; ambiguous windows are rejected rather than guessed.
func SendCommandTarget(session, window, pane, featureDir, runDir string, argv []string) (string, error) {

	// Create window if it doesn't exist
	if !WindowExists(session, window) {
		if err := exec.Command("tmux", "new-window", "-t", session, "-n", window, "-c", featureDir).Run(); err != nil {
			return "", fmt.Errorf("create window %s: %w", window, err)
		}
	}
	if pane == "" {
		var err error
		pane, err = ResolvePaneTarget(session, window)
		if err != nil {
			return "", err
		}
	} else {
		var err error
		pane, err = ValidatePaneTarget(session, window, pane)
		if err != nil {
			return "", err
		}
	}

	// Write command to a temp script to avoid quoting issues with send-keys.
	// The shell and script both exec so tmux's pane PID becomes the provider PID.
	script, err := writeScript(runDir, argv)
	if err != nil {
		return "", err
	}

	if err := exec.Command("tmux", "send-keys", "-t", pane, "exec bash "+shellQuote(script), "Enter").Run(); err != nil {
		return "", fmt.Errorf("send command to pane %s: %w", pane, err)
	}
	return pane, nil
}

// ValidatePaneTarget verifies that a durable pane id still belongs to the
// expected session/window. Stale ids are safely re-resolved within that window.
func ValidatePaneTarget(session, window, pane string) (string, error) {
	if pane == "" {
		return "", nil
	}
	out, err := exec.Command("tmux", "display-message", "-p", "-t", pane, "#{session_name}\t#{window_name}").Output()
	if err == nil {
		fields := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
		if len(fields) == 2 && fields[0] == session && fields[1] == window {
			return pane, nil
		}
	}
	resolved, resolveErr := ResolvePaneTarget(session, window)
	if resolveErr != nil {
		return "", fmt.Errorf("stored tmux pane %s is stale: %w", pane, resolveErr)
	}
	return resolved, nil
}

// ResolvePaneTarget returns the only safe agent pane for a window.
func ResolvePaneTarget(session, window string) (string, error) {
	target := session + ":" + window
	out, err := exec.Command("tmux", "list-panes", "-t", target, "-F", "#{pane_id}\t#{@orc_agent}").Output()
	if err != nil {
		return "", fmt.Errorf("list panes for %s: %w", target, err)
	}
	var panes, marked []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) == 0 || fields[0] == "" {
			continue
		}
		panes = append(panes, fields[0])
		if len(fields) > 1 && fields[1] == "1" {
			marked = append(marked, fields[0])
		}
	}
	if len(marked) == 1 {
		return marked[0], nil
	}
	if len(panes) == 1 {
		return panes[0], nil
	}
	return "", fmt.Errorf("tmux window %s has %d panes and no unique @orc_agent target", target, len(panes))
}

// SetWindowMetadata stamps Orc identity onto a tmux window using user options.
// Empty values are omitted so partial metadata can still be recorded safely.
func SetWindowMetadata(session, window string, metadata WindowMetadata) error {
	target := session + ":" + window
	options := []struct {
		name  string
		value string
	}{
		{"@orc_ticket", metadata.Ticket},
		{"@orc_stage", metadata.Stage},
		{"@orc_worker", metadata.Worker},
		{"@orc_engine", metadata.Engine},
		{"@orc_provider_engine", metadata.Engine},
		{"@orc_provider_session", metadata.ProviderSessionID},
		{"@orc_feature_dir", metadata.FeatureDir},
	}
	for _, option := range options {
		if option.value == "" {
			if option.name == "@orc_provider_engine" || option.name == "@orc_provider_session" {
				if err := exec.Command("tmux", "set-option", "-w", "-u", "-t", target, option.name).Run(); err != nil {
					return fmt.Errorf("clear %s on %s: %w", option.name, target, err)
				}
			}
			continue
		}
		if err := exec.Command("tmux", "set-option", "-w", "-t", target, option.name, option.value).Run(); err != nil {
			return fmt.Errorf("set %s on %s: %w", option.name, target, err)
		}
	}
	return nil
}

// SetPaneMetadata stamps the exact agent pane used by Orc.
func SetPaneMetadata(pane string, metadata WindowMetadata) error {
	options := []struct {
		name  string
		value string
	}{
		{"@orc_agent", "1"},
		{"@orc_ticket", metadata.Ticket},
		{"@orc_stage", metadata.Stage},
		{"@orc_worker", metadata.Worker},
		{"@orc_engine", metadata.Engine},
		{"@orc_provider_engine", metadata.Engine},
		{"@orc_provider_session", metadata.ProviderSessionID},
		{"@orc_feature_dir", metadata.FeatureDir},
	}
	for _, option := range options {
		if option.value == "" {
			if option.name == "@orc_provider_engine" || option.name == "@orc_provider_session" {
				if err := exec.Command("tmux", "set-option", "-p", "-u", "-t", pane, option.name).Run(); err != nil {
					return fmt.Errorf("clear %s on pane %s: %w", option.name, pane, err)
				}
			}
			continue
		}
		if err := exec.Command("tmux", "set-option", "-p", "-t", pane, option.name, option.value).Run(); err != nil {
			return fmt.Errorf("set %s on pane %s: %w", option.name, pane, err)
		}
	}
	return nil
}

// SetSessionEnvironment records live correlation metadata in tmux's session
// environment. Callers must also pass the value to an already-running pane
// process when they need the provider itself to inherit it.
func SetSessionEnvironment(session, name, value string) error {
	if !validEnvironmentName(name) {
		return fmt.Errorf("invalid tmux environment name %q", name)
	}
	if err := exec.Command("tmux", "set-environment", "-t", session, name, value).Run(); err != nil {
		return fmt.Errorf("set tmux environment %s on %s: %w", name, session, err)
	}
	return nil
}

func validEnvironmentName(name string) bool {
	if name == "" {
		return false
	}
	for index, char := range name {
		if index == 0 && char >= '0' && char <= '9' {
			return false
		}
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' {
			continue
		}
		return false
	}
	return true
}

// SessionEnvironment reads a value from a tmux session environment table.
func SessionEnvironment(session, name string) (string, error) {
	if !validEnvironmentName(name) {
		return "", fmt.Errorf("invalid tmux environment name %q", name)
	}
	out, err := exec.Command("tmux", "show-environment", "-t", session, name).Output()
	if err != nil {
		return "", fmt.Errorf("read tmux environment %s on %s: %w", name, session, err)
	}
	line := strings.TrimSpace(string(out))
	_, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", fmt.Errorf("tmux environment %s on %s has invalid output", name, session)
	}
	return value, nil
}

// WindowOption reads a tmux user option from a specific window.
func WindowOption(session, window, option string) (string, error) {
	target := session + ":" + window
	out, err := exec.Command("tmux", "show-options", "-w", "-qv", "-t", target, option).Output()
	if err != nil {
		return "", fmt.Errorf("read %s on %s: %w", option, target, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// WindowAttention returns the supported tmux-attention state for a window.
// Unknown, cleared, and unreadable values are treated as no live overlay.
func WindowAttention(session, window string) string {
	value, err := WindowOption(session, window, "@agent_attention")
	if err != nil {
		return ""
	}
	switch strings.ToLower(value) {
	case AttentionInput, AttentionBlocked, AttentionReview, AttentionDone:
		return strings.ToLower(value)
	default:
		return ""
	}
}

// Attach attaches to a session (or switches if already inside tmux).
// target is "session" or "session:window".
func Attach(target string) error {
	if os.Getenv("TMUX") != "" {
		return exec.Command("tmux", "switch-client", "-t", target).Run()
	}
	cmd := exec.Command("tmux", "attach-session", "-t", target)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AttachTarget focuses an exact agent pane. Older STATE.yaml files without a
// pane id are resolved only when the stage window has one safe target.
func AttachTarget(session, window, pane string) error {
	cmd, err := AttachCommandTarget(session, window, pane)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AttachCommandTarget validates a stored pane before constructing the attach
// command. This prevents reused or stale pane ids from focusing unrelated work.
func AttachCommandTarget(session, window, pane string) (*exec.Cmd, error) {
	var err error
	if pane == "" {
		pane, err = ResolvePaneTarget(session, window)
	} else {
		pane, err = ValidatePaneTarget(session, window, pane)
	}
	if err != nil {
		return nil, err
	}
	return AttachCommand(session, window, pane), nil
}

// AttachCommand constructs the tmux command used by CLI and Bubble Tea paths.
func AttachCommand(session, window, pane string) *exec.Cmd {
	target := session + ":" + window
	if pane == "" {
		if os.Getenv("TMUX") != "" {
			return exec.Command("tmux", "switch-client", "-t", target)
		}
		return exec.Command("tmux", "attach-session", "-t", target)
	}
	if os.Getenv("TMUX") != "" {
		return exec.Command("tmux", "switch-client", "-t", session, ";", "select-pane", "-t", pane)
	}
	return exec.Command("tmux", "select-pane", "-t", pane, ";", "attach-session", "-t", target)
}

// KillSession kills a tmux session by name.
func KillSession(name string) error {
	if err := exec.Command("tmux", "kill-session", "-t", name).Run(); err != nil {
		return fmt.Errorf("kill session %s: %w", name, err)
	}
	return nil
}

// ListSessions returns all tmux session names, or nil if none exist.
func ListSessions() []string {
	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").Output()
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}

// Pane describes the process and Orc metadata attached to a tmux pane.
type Pane struct {
	ID                string `json:"id"`
	Session           string `json:"session"`
	Window            string `json:"window"`
	CWD               string `json:"cwd,omitempty"`
	Command           string `json:"command,omitempty"`
	PID               int    `json:"pid,omitempty"`
	Agent             bool   `json:"agent"`
	Ticket            string `json:"ticket,omitempty"`
	Stage             string `json:"stage,omitempty"`
	Worker            string `json:"worker,omitempty"`
	Engine            string `json:"engine,omitempty"`
	ProviderEngine    string `json:"provider_engine,omitempty"`
	ProviderSessionID string `json:"provider_session_id,omitempty"`
	FeatureDir        string `json:"feature_dir,omitempty"`
	Attention         string `json:"attention,omitempty"`
}

// ListPanesDetailed returns all tmux panes with the small metadata surface Orc
// owns. A missing tmux server is the empty inventory, not an error.
func ListPanesDetailed() ([]Pane, error) {
	if !Available() {
		return nil, nil
	}
	format := strings.Join([]string{
		"#{pane_id}", "#{session_name}", "#{window_name}",
		"#{pane_current_path}", "#{pane_current_command}", "#{pane_pid}",
		"#{@orc_agent}", "#{@orc_ticket}", "#{@orc_stage}",
		"#{@orc_worker}", "#{@orc_engine}", "#{@orc_provider_engine}",
		"#{@orc_provider_session}", "#{@orc_feature_dir}",
		"#{@agent_attention}",
	}, "\t")
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", format).CombinedOutput()
	if err != nil {
		message := strings.ToLower(string(out))
		if strings.Contains(message, "no server running") || strings.Contains(message, "failed to connect") || strings.Contains(message, "error connecting") {
			return nil, nil
		}
		return nil, fmt.Errorf("list tmux panes: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return parseDetailedPanes(out), nil
}

func parseDetailedPanes(out []byte) []Pane {
	var panes []Pane
	// Remove record terminators only. TrimSpace would also remove the final tab
	// when @agent_attention is empty and make an otherwise valid pane look short.
	text := strings.TrimRight(string(out), "\r\n")
	if text == "" {
		return nil
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		fields := strings.Split(line, "\t")
		if len(fields) < 15 || fields[0] == "" {
			continue
		}
		pid, _ := strconv.Atoi(fields[5])
		panes = append(panes, Pane{
			ID: fields[0], Session: fields[1], Window: fields[2],
			CWD: fields[3], Command: fields[4], PID: pid, Agent: fields[6] == "1",
			Ticket: fields[7], Stage: fields[8], Worker: fields[9], Engine: fields[10],
			ProviderEngine: fields[11], ProviderSessionID: fields[12],
			FeatureDir: fields[13], Attention: fields[14],
		})
	}
	return panes
}

// AttachHint returns the command string a user should run to attach.
func AttachHint(session, window string) string {
	return fmt.Sprintf("tmux attach -t %s:%s", session, window)
}

type WatchToggleOptions struct {
	Root      string
	Ticket    string
	Interval  string
	Wide      bool
	View      string
	PetSize   string
	PetLayout string
	Layout    string
	Size      string
	ExecPath  string
}

// ToggleWatchPane closes an existing watch pane in the current tmux window, or
// opens a narrow right split running orc watch and marks it for future toggles.
func ToggleWatchPane(opts WatchToggleOptions) error {
	if os.Getenv("TMUX") == "" {
		return fmt.Errorf("--tmux-toggle must be run inside tmux")
	}
	if !Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}

	pane, err := findWatchPane()
	if err != nil {
		return err
	}
	if pane != "" {
		if err := exec.Command("tmux", "kill-pane", "-t", pane).Run(); err != nil {
			return fmt.Errorf("close watch pane %s: %w", pane, err)
		}
		return nil
	}

	size := opts.Size
	if size == "" {
		size = "32"
	}
	layoutFlag, err := watchSplitFlag(opts.Layout)
	if err != nil {
		return err
	}
	cmd := buildWatchCommand(opts)
	out, err := exec.Command("tmux", "split-window", layoutFlag, "-l", size, "-P", "-F", "#{pane_id}", cmd).Output()
	if err != nil {
		return fmt.Errorf("open watch pane: %w", err)
	}
	newPane := strings.TrimSpace(string(out))
	if newPane == "" {
		return fmt.Errorf("open watch pane: tmux did not return a pane id")
	}
	if err := exec.Command("tmux", "set-option", "-p", "-t", newPane, "@orc_watch", "1").Run(); err != nil {
		return fmt.Errorf("mark watch pane %s: %w", newPane, err)
	}
	return nil
}

func watchSplitFlag(layout string) (string, error) {
	switch layout {
	case "", "right":
		return "-h", nil
	case "bottom":
		return "-v", nil
	default:
		return "", fmt.Errorf("unsupported tmux watch layout %q (use right or bottom)", layout)
	}
}

func findWatchPane() (string, error) {
	out, err := exec.Command("tmux", "list-panes", "-F", "#{pane_id}\t#{@orc_watch}").Output()
	if err != nil {
		return "", fmt.Errorf("list panes: %w", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) >= 2 && fields[1] == "1" {
			return fields[0], nil
		}
	}
	return "", nil
}

func buildWatchCommand(opts WatchToggleOptions) string {
	execPath := opts.ExecPath
	if execPath == "" {
		if p, err := os.Executable(); err == nil {
			execPath = p
		}
	}
	if execPath == "" {
		execPath = "orc"
	}

	argv := []string{execPath}
	if opts.Root != "" {
		argv = append(argv, "--workspace", opts.Root)
	}
	argv = append(argv, "watch")
	if opts.Ticket != "" {
		argv = append(argv, opts.Ticket)
	}
	if opts.Interval != "" {
		argv = append(argv, "--interval", opts.Interval)
	}
	if opts.Wide {
		argv = append(argv, "--wide")
	}
	if opts.View != "" && opts.View != "rail" {
		argv = append(argv, "--view", opts.View)
	}
	if opts.PetSize != "" && opts.PetSize != "normal" {
		argv = append(argv, "--pet-size", opts.PetSize)
	}
	if opts.PetLayout != "" && opts.PetLayout != "responsive" {
		argv = append(argv, "--pet-layout", opts.PetLayout)
	}
	return shellJoin(argv)
}

func shellJoin(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func writeScript(runDir string, argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("launch command is empty")
	}
	f, err := os.CreateTemp("", "orc-launch-*.sh")
	if err != nil {
		return "", fmt.Errorf("temp script: %w", err)
	}
	defer f.Close() //nolint:errcheck

	var parts []string
	for _, arg := range argv {
		parts = append(parts, shellQuote(arg))
	}
	// cd to the right directory, remove the script, and replace the shell with
	// the provider so tmux reports the provider PID for exact correlation.
	// The cd must not fall through: if runDir was removed, launching the agent
	// from the wrong directory would silently run repo commands against the
	// wrong tree, so exit instead.
	if _, err := fmt.Fprintf(f, "#!/usr/bin/env bash\ntrap 'rm -f %s' EXIT\ncd %s || exit 1\nrm -f %s\ntrap - EXIT\nexec %s\n",
		shellQuote(f.Name()),
		shellQuote(runDir),
		shellQuote(f.Name()),
		strings.Join(parts, " "),
	); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}
	return f.Name(), nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\"'\\$`!;|&<>(){}") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
