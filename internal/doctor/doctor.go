package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/agenthooks"
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/health"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspacectx"
	"github.com/cengebretson/orc/internal/worktreecheck"
	"github.com/cengebretson/orc/internal/worktreesetup"
)

type Status int

const (
	OK Status = iota
	Warning
	Fail
)

func (s Status) String() string {
	switch s {
	case OK:
		return "✓"
	case Warning:
		return "⚠"
	default:
		return "✗"
	}
}

type Check struct {
	Group  string
	Name   string
	Status Status
	Detail string
}

type Report struct {
	Root   string
	Label  string
	Checks []Check
}

func (r *Report) OK() bool {
	for _, c := range r.Checks {
		if c.Status == Fail {
			return false
		}
	}
	return true
}

type Options struct {
	LookPath func(string) (string, error)
	// Fix removes provably-stale state locks (dead PID, or old without a
	// valid PID) instead of only reporting them. Live or ambiguous locks
	// are never touched.
	Fix bool
	// Version is the Orc build version shown by system checks.
	Version string
}

func Run(root string) *Report {
	return RunWithOptions(root, Options{})
}

func RunWithOptions(root string, opts Options) *Report {
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}

	report := &Report{Root: root, Label: "Workspace"}
	appendHealth(report, health.Run(root))
	appendConfigChecks(report, root, opts.LookPath)
	appendFeatureStateChecks(report, root)
	appendStateLockChecks(report, root, opts.Fix)
	appendToolChecks(report, root, opts.LookPath)
	return report
}

func RunSystemWithOptions(opts Options) *Report {
	if opts.LookPath == nil {
		opts.LookPath = exec.LookPath
	}

	report := &Report{Label: "System"}
	appendSystemChecks(report, opts)
	return report
}

func Print(r *Report) {
	switch r.Label {
	case "System":
		fmt.Println("System")
		fmt.Println()
	default:
		fmt.Printf("Workspace: %s\n\n", r.Root)
	}
	var currentGroup string
	for _, c := range r.Checks {
		if c.Group != currentGroup {
			currentGroup = c.Group
			if currentGroup != "" {
				fmt.Printf("  %s\n", currentGroup)
			}
		}
		indent := "  "
		if c.Group != "" {
			indent = "    "
		}
		if c.Detail != "" {
			fmt.Printf("%s%s  %-20s %s\n", indent, c.Status, c.Name, c.Detail)
		} else {
			fmt.Printf("%s%s  %s\n", indent, c.Status, c.Name)
		}
	}
}

// AppendAgentHookChecks adds provider hook readiness without making missing
// optional agents or uninstalled hooks fail the broader doctor report.
func AppendAgentHookChecks(report *Report, plan *agenthooks.Plan, lookPath func(string) (string, error)) {
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	for _, integration := range plan.Integrations {
		status := Warning
		detail := integration.Summary()
		executableReady := false
		if executable, err := lookPath(integration.Executable); err == nil {
			executableReady = true
			detail += "; executable " + executable
			if detected, versionErr := agenthooks.DetectVersion(executable); versionErr == nil {
				detail += "; version " + detected
			} else {
				detail += "; version unavailable"
			}
		} else {
			detail += "; executable unavailable"
		}
		if integration.Err == nil && integration.Ready() {
			if executableReady {
				status = OK
			}
		} else if integration.Err == nil {
			detail += "; run orc doctor --install-agent-hooks"
		}
		report.Checks = append(report.Checks, Check{
			Group: "agent hooks", Name: integration.Engine, Status: status, Detail: detail,
		})
	}
}

func appendHealth(report *Report, h *health.Report) {
	for _, r := range h.Results {
		status := OK
		switch r.Status {
		case health.Missing:
			status = Fail
		case health.Empty:
			status = Warning
		}
		group := r.Group
		if group == "" {
			group = "workspace"
		}
		report.Checks = append(report.Checks, Check{
			Group:  group,
			Name:   r.Name,
			Status: status,
			Detail: r.Detail,
		})
	}
}

func appendConfigChecks(report *Report, root string, lookPath func(string) (string, error)) {
	ctx, errs, err := workspacectx.LoadValidated(root)
	if err != nil {
		report.Checks = append(report.Checks, Check{
			Group:  "orc.yaml",
			Name:   "workspace",
			Status: Fail,
			Detail: err.Error(),
		})
		return
	}
	if len(errs) == 0 {
		if ctx != nil && ctx.Config != nil {
			appendRepoReadinessChecks(report, root, ctx.Config, lookPath)
		}
		return
	}
	for _, err := range errs {
		report.Checks = append(report.Checks, Check{
			Group:  "orc.yaml",
			Name:   err.Path,
			Status: Fail,
			Detail: err.Message,
		})
	}
	if ctx != nil && ctx.Config != nil {
		appendRepoReadinessChecks(report, root, ctx.Config, lookPath)
	}
}

func appendRepoReadinessChecks(report *Report, root string, cfg *config.Config, lookPath func(string) (string, error)) {
	hasWorktreeSetup := false
	for _, repo := range cfg.Repos {
		if strings.TrimSpace(repo.WorktreeSetup) == "" {
			continue
		}
		hasWorktreeSetup = true
		name := repo.Name
		if name == "" {
			name = repo.Path
		}
		checkName := "repos." + name + ".worktree_setup"
		if !worktreesetup.ReferencesWorktreePath(repo.WorktreeSetup) {
			report.Checks = append(report.Checks, Check{
				Group:  "orc.yaml",
				Name:   checkName,
				Status: Warning,
				Detail: "does not include {{worktree_path}}; command may create a worktree outside orc state",
			})
		}
		report.Checks = append(report.Checks, worktreeSetupCommandCheck(root, checkName+".command", repo.WorktreeSetup, lookPath))
		if len(repo.AgentHints) == 0 {
			report.Checks = append(report.Checks, Check{
				Group:  "orc.yaml",
				Name:   "repos." + name + ".agent_hints",
				Status: Warning,
				Detail: "none configured; agents may miss repo-specific setup and test guidance",
			})
		}
	}
	if hasWorktreeSetup {
		if info, err := os.Stat(filepath.Join(root, "worktrees")); err != nil || !info.IsDir() {
			report.Checks = append(report.Checks, Check{
				Group:  "orc.yaml",
				Name:   "worktrees/",
				Status: Warning,
				Detail: "missing; setup commands expect workspace worktrees/ destinations",
			})
		}
	}
}

func worktreeSetupCommandCheck(root, name, setup string, lookPath func(string) (string, error)) Check {
	command := firstCommandToken(setup)
	if command == "" {
		return Check{Group: "orc.yaml", Name: name, Status: Warning, Detail: "empty setup command"}
	}
	if shellBuiltins[command] {
		return Check{Group: "orc.yaml", Name: name, Status: OK, Detail: "starts with shell builtin " + command + "; not checked"}
	}
	if strings.Contains(command, "/") {
		path := command
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return Check{Group: "orc.yaml", Name: name, Status: Warning, Detail: "command path not found: " + command}
		}
		if info.IsDir() {
			return Check{Group: "orc.yaml", Name: name, Status: Warning, Detail: "command path is a directory: " + command}
		}
		if info.Mode().Perm()&0o111 == 0 {
			return Check{Group: "orc.yaml", Name: name, Status: Warning, Detail: "command is not executable: " + command}
		}
		return Check{Group: "orc.yaml", Name: name, Status: OK, Detail: "command found: " + command}
	}
	if _, err := lookPath(command); err != nil {
		return Check{Group: "orc.yaml", Name: name, Status: Warning, Detail: "command not found in PATH: " + command}
	}
	return Check{Group: "orc.yaml", Name: name, Status: OK, Detail: "command found: " + command}
}

// shellBuiltins are command tokens that never resolve via PATH on every
// platform; a setup command starting with one cannot be verified here.
var shellBuiltins = map[string]bool{
	"cd":     true,
	".":      true,
	"source": true,
	"export": true,
	"set":    true,
	"eval":   true,
	"exec":   true,
	"if":     true,
	"for":    true,
	"while":  true,
}

func firstCommandToken(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	for _, field := range fields {
		if strings.Contains(field, "=") && !strings.Contains(field, "/") {
			continue
		}
		return strings.Trim(field, `"'`)
	}
	return ""
}

func appendFeatureStateChecks(report *Report, root string) {
	featuresDir := filepath.Join(root, "features")
	entries, err := os.ReadDir(featuresDir)
	if err != nil {
		return
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		featureDir := filepath.Join(featuresDir, entry.Name())
		statePath := filepath.Join(featureDir, state.Filename)
		if _, err := os.Stat(statePath); err != nil {
			continue
		}
		found = true
		s, err := state.Load(featureDir)
		if err != nil {
			report.Checks = append(report.Checks, Check{
				Group:  "feature state",
				Name:   entry.Name(),
				Status: Fail,
				Detail: err.Error(),
			})
			continue
		}
		appendFeatureWorktreeChecks(report, root, s)
	}
	if !found {
		return
	}
	hasFeatureStateCheck := false
	for _, check := range report.Checks {
		if check.Group == "feature state" {
			hasFeatureStateCheck = true
			break
		}
	}
	if !hasFeatureStateCheck {
		report.Checks = append(report.Checks, Check{
			Group:  "feature state",
			Name:   "worktrees",
			Status: OK,
			Detail: "recorded worktrees reconciled",
		})
	}
}

func appendFeatureWorktreeChecks(report *Report, root string, s *state.State) {
	for name, repo := range s.Repos {
		if repo.Worktree == "" {
			continue
		}
		p := repo.Worktree
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		if _, err := os.Stat(p); err != nil {
			report.Checks = append(report.Checks, Check{
				Group:  "feature state",
				Name:   s.Slug + "." + name,
				Status: Warning,
				Detail: "recorded worktree missing: " + repo.Worktree,
			})
		}
	}
	for _, finding := range worktreecheck.Reconcile(root, s) {
		status := Warning
		if finding.Severity == worktreecheck.Fail {
			status = Fail
		}
		report.Checks = append(report.Checks, Check{
			Group:  "feature state",
			Name:   s.Slug + "." + finding.RepoName,
			Status: status,
			Detail: finding.Message,
		})
	}
}

func appendToolChecks(report *Report, root string, lookPath func(string) (string, error)) {
	report.Checks = append(report.Checks, executableCheck("tools", "tmux", "tmux", lookPath, true))

	engineNames, err := workerEngines(root)
	if err != nil {
		report.Checks = append(report.Checks, Check{
			Group:  "tools",
			Name:   "workers",
			Status: Fail,
			Detail: fmt.Sprintf("cannot load workers/: %v", err),
		})
		return
	}
	for _, engine := range engineNames {
		required := engine != "cursor"
		report.Checks = append(report.Checks, executableCheck("tools", engine, engine, lookPath, !required))
	}
}

func appendSystemChecks(report *Report, opts Options) {
	report.Checks = append(report.Checks, Check{
		Group:  "install",
		Name:   "version",
		Status: OK,
		Detail: versionOrDefault(opts.Version),
	})
	report.Checks = append(report.Checks, executableCheck("install", "orc", "orc", opts.LookPath, false))

	report.Checks = append(report.Checks,
		executableCheck("tools", "tmux", "tmux", opts.LookPath, true),
		executableCheck("tools", "chafa", "chafa", opts.LookPath, true),
	)
	for _, engine := range []string{"claude", "codex", "cursor"} {
		report.Checks = append(report.Checks, executableCheck("tools", engine, engine, opts.LookPath, true))
	}
}

func versionOrDefault(v string) string {
	if strings.TrimSpace(v) == "" {
		return "dev"
	}
	return v
}

func appendStateLockChecks(report *Report, root string, fix bool) {
	featuresDir := filepath.Join(root, "features")
	if _, err := os.Stat(featuresDir); err != nil {
		return
	}

	found := false
	err := filepath.WalkDir(featuresDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			report.Checks = append(report.Checks, Check{
				Group:  "workspace",
				Name:   filepath.Base(path),
				Status: Fail,
				Detail: err.Error(),
			})
			return nil
		}
		if d.IsDir() {
			if d.Name() == "_template" {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != state.Filename+".lock" {
			return nil
		}

		found = true
		featureDir := filepath.Dir(path)
		rel, relErr := filepath.Rel(featuresDir, featureDir)
		if relErr != nil {
			rel = filepath.Base(featureDir)
		}
		lock, err := state.InspectLock(featureDir)
		if err != nil {
			report.Checks = append(report.Checks, Check{
				Group:  "workspace",
				Name:   rel,
				Status: Fail,
				Detail: err.Error(),
			})
			return nil
		}
		status := Warning
		detail := lock.Detail
		switch lock.Status {
		case state.LockActive:
			detail += " — if no orc process is running, remove the lock file to unlock"
		case state.LockStale:
			if !fix {
				detail += " — will be recovered on next state write"
				break
			}
			removed, fixErr := state.ClearStaleLock(featureDir)
			switch {
			case fixErr != nil:
				status = Fail
				detail += fmt.Sprintf(" — fix failed: %v", fixErr)
			case removed:
				status = OK
				detail += " — stale lock removed"
			default:
				detail += " — not removed (lock changed since inspection)"
			}
		default:
			status = OK
		}
		report.Checks = append(report.Checks, Check{
			Group:  "workspace",
			Name:   rel,
			Status: status,
			Detail: detail,
		})
		return nil
	})
	if err != nil {
		report.Checks = append(report.Checks, Check{
			Group:  "workspace",
			Name:   "scan",
			Status: Fail,
			Detail: err.Error(),
		})
		return
	}
	if !found {
		report.Checks = append(report.Checks, Check{
			Group:  "workspace",
			Name:   state.Filename + ".lock",
			Status: OK,
			Detail: "none found",
		})
	}
}

func executableCheck(group, name, command string, lookPath func(string) (string, error), optional bool) Check {
	path, err := lookPath(command)
	if err == nil {
		return Check{Group: group, Name: name, Status: OK, Detail: path}
	}
	status := Fail
	detail := "not found in PATH"
	if optional {
		status = Warning
		detail += " (optional)"
	}
	return Check{Group: group, Name: name, Status: status, Detail: detail}
}

func workerEngines(root string) ([]string, error) {
	ctx, err := workspacectx.Load(root)
	if err != nil {
		return nil, err
	}
	engines := map[string]bool{}
	for _, w := range ctx.Workers {
		engine := strings.ToLower(strings.TrimSpace(w.Engine))
		if engine == "" {
			engine = "claude"
		}
		engines[engine] = true
	}
	names := make([]string, 0, len(engines))
	for name := range engines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
