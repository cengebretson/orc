package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/health"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workspacectx"
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
	appendConfigChecks(report, root)
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

func appendConfigChecks(report *Report, root string) {
	_, errs, err := workspacectx.LoadValidated(root)
	if err != nil {
		report.Checks = append(report.Checks, Check{
			Group:  "config",
			Name:   "workspace",
			Status: Fail,
			Detail: err.Error(),
		})
		return
	}
	if len(errs) == 0 {
		report.Checks = append(report.Checks, Check{
			Group:  "config",
			Name:   config.Filename,
			Status: OK,
			Detail: "valid",
		})
		return
	}
	for _, err := range errs {
		report.Checks = append(report.Checks, Check{
			Group:  "config",
			Name:   err.Path,
			Status: Fail,
			Detail: err.Message,
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
				Group:  "state locks",
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
				Group:  "state locks",
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
			Group:  "state locks",
			Name:   rel,
			Status: status,
			Detail: detail,
		})
		return nil
	})
	if err != nil {
		report.Checks = append(report.Checks, Check{
			Group:  "state locks",
			Name:   "scan",
			Status: Fail,
			Detail: err.Error(),
		})
		return
	}
	if !found {
		report.Checks = append(report.Checks, Check{
			Group:  "state locks",
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
