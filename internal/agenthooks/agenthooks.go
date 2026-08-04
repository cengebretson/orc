// Package agenthooks installs and inspects Orc-owned lifecycle hooks for
// supported coding agents without taking ownership of unrelated user config.
package agenthooks

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const hookFilename = "orc-agent-state.sh"

//go:embed assets/orc-agent-state.sh
var hookScript []byte

// Options controls path resolution. Empty values use the current process
// environment and user home directory.
type Options struct {
	HomeDir         string
	CodexHome       string
	ClaudeConfigDir string
	OrcBinary       string
}

// Change describes one idempotent filesystem operation.
type Change struct {
	Path    string
	Kind    string
	Mode    fs.FileMode
	Content []byte
}

// Integration is the complete plan and health result for one agent engine.
type Integration struct {
	Engine          string
	Executable      string
	ConfigDir       string
	HookPath        string
	ConfigPath      string
	SupportedStates []string
	Changes         []Change
	Err             error
}

// Plan contains both provider integrations. Apply refuses partial writes when
// either integration could not be planned safely.
type Plan struct {
	Integrations []Integration
}

// BuildPlan reads and validates existing configuration, then computes exact
// idempotent changes without writing.
func BuildPlan(opts Options) *Plan {
	resolved, err := resolveOptions(opts)
	if err != nil {
		return &Plan{Integrations: []Integration{
			{Engine: "codex", Err: err},
			{Engine: "claude", Err: err},
		}}
	}
	return &Plan{Integrations: []Integration{
		planIntegration(codexDefinition(resolved)),
		planIntegration(claudeDefinition(resolved)),
	}}
}

// Apply writes every changed file atomically. Planning errors are reported
// before the first write so malformed user configuration remains untouched.
func Apply(plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("agent hook plan is nil")
	}
	if err := plan.Err(); err != nil {
		return err
	}
	for _, integration := range plan.Integrations {
		for _, change := range integration.Changes {
			if change.Kind == "unchanged" {
				continue
			}
			if err := writeAtomic(change.Path, change.Content, change.Mode); err != nil {
				return fmt.Errorf("install %s hook: %w", integration.Engine, err)
			}
		}
	}
	return nil
}

// Err joins all planning failures. Callers use it to validate dry runs without
// applying the plan.
func (plan *Plan) Err() error {
	if plan == nil {
		return fmt.Errorf("agent hook plan is nil")
	}
	var planningErrors []error
	for _, integration := range plan.Integrations {
		if integration.Err != nil {
			planningErrors = append(planningErrors, fmt.Errorf("%s: %w", integration.Engine, integration.Err))
		}
	}
	if len(planningErrors) > 0 {
		return errors.Join(planningErrors...)
	}
	return nil
}

// Ready reports whether both the managed script and provider config already
// match the current installer output.
func (integration Integration) Ready() bool {
	if integration.Err != nil || len(integration.Changes) == 0 {
		return false
	}
	for _, change := range integration.Changes {
		if change.Kind != "unchanged" {
			return false
		}
	}
	return true
}

type resolvedOptions struct {
	homeDir         string
	codexHome       string
	claudeConfigDir string
	orcBinary       string
}

type definition struct {
	engine     string
	executable string
	configDir  string
	hookPath   string
	configPath string
	orcBinary  string
	events     []eventDefinition
}

type eventDefinition struct {
	name      string
	lifecycle string
	matcher   string
}

func resolveOptions(opts Options) (resolvedOptions, error) {
	if runtime.GOOS == "windows" {
		return resolvedOptions{}, fmt.Errorf("agent hook installation is not yet supported on Windows")
	}
	home := strings.TrimSpace(opts.HomeDir)
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return resolvedOptions{}, fmt.Errorf("resolve home directory: %w", err)
		}
	}
	codexHome := firstNonEmpty(opts.CodexHome, os.Getenv("CODEX_HOME"), filepath.Join(home, ".codex"))
	claudeDir := firstNonEmpty(opts.ClaudeConfigDir, os.Getenv("CLAUDE_CONFIG_DIR"), filepath.Join(home, ".claude"))
	orcBinary := strings.TrimSpace(opts.OrcBinary)
	if orcBinary == "" {
		var err error
		orcBinary, err = os.Executable()
		if err != nil {
			return resolvedOptions{}, fmt.Errorf("resolve orc executable: %w", err)
		}
	}
	for name, value := range map[string]string{
		"home": home, "CODEX_HOME": codexHome, "CLAUDE_CONFIG_DIR": claudeDir, "orc executable": orcBinary,
	} {
		if !filepath.IsAbs(value) {
			return resolvedOptions{}, fmt.Errorf("%s path must be absolute: %s", name, value)
		}
	}
	return resolvedOptions{
		homeDir: filepath.Clean(home), codexHome: filepath.Clean(codexHome),
		claudeConfigDir: filepath.Clean(claudeDir), orcBinary: filepath.Clean(orcBinary),
	}, nil
}

func codexDefinition(opts resolvedOptions) definition {
	hookPath := filepath.Join(opts.codexHome, "hooks", hookFilename)
	return definition{
		engine: "codex", executable: "codex", configDir: opts.codexHome,
		hookPath: hookPath, configPath: filepath.Join(opts.codexHome, "hooks.json"), orcBinary: opts.orcBinary,
		events: []eventDefinition{
			{name: "SessionStart", lifecycle: "idle"},
			{name: "UserPromptSubmit", lifecycle: "working"},
			{name: "PermissionRequest", lifecycle: "blocked"},
			{name: "PostToolUse", lifecycle: "working"},
			{name: "Stop", lifecycle: "idle"},
		},
	}
}

func claudeDefinition(opts resolvedOptions) definition {
	hookPath := filepath.Join(opts.claudeConfigDir, "hooks", hookFilename)
	return definition{
		engine: "claude", executable: "claude", configDir: opts.claudeConfigDir,
		hookPath: hookPath, configPath: filepath.Join(opts.claudeConfigDir, "settings.json"), orcBinary: opts.orcBinary,
		events: []eventDefinition{
			{name: "SessionStart", lifecycle: "idle"},
			{name: "UserPromptSubmit", lifecycle: "working"},
			{name: "PermissionRequest", lifecycle: "blocked"},
			{name: "Notification", lifecycle: "blocked", matcher: "permission_prompt"},
			{name: "PostToolUse", lifecycle: "working"},
			{name: "Stop", lifecycle: "idle"},
			{name: "StopFailure", lifecycle: "blocked"},
		},
	}
}

func planIntegration(def definition) Integration {
	integration := Integration{
		Engine: def.engine, Executable: def.executable, ConfigDir: def.configDir,
		HookPath: def.hookPath, ConfigPath: def.configPath,
		SupportedStates: []string{"idle", "working", "blocked"},
	}
	hookChange, err := planFile(def.hookPath, hookScript, 0o700)
	if err != nil {
		integration.Err = err
		return integration
	}
	existingConfig, configMode, err := readOptionalFile(def.configPath, 0o600)
	if err != nil {
		integration.Err = err
		return integration
	}
	updatedConfig, err := mergeConfig(existingConfig, def)
	if err != nil {
		integration.Err = fmt.Errorf("update %s: %w", def.configPath, err)
		return integration
	}
	configChange, err := planFileWithExisting(def.configPath, existingConfig, updatedConfig, configMode)
	if err != nil {
		integration.Err = err
		return integration
	}
	integration.Changes = []Change{hookChange, configChange}
	return integration
}

func planFile(path string, desired []byte, mode fs.FileMode) (Change, error) {
	existing, existingMode, err := readOptionalFile(path, mode)
	if err != nil {
		return Change{}, err
	}
	if len(existing) > 0 {
		mode = existingMode
	}
	return planFileWithExisting(path, existing, desired, mode)
}

func planFileWithExisting(path string, existing, desired []byte, mode fs.FileMode) (Change, error) {
	kind := "unchanged"
	if existing == nil {
		kind = "create"
	} else if string(existing) != string(desired) {
		kind = "update"
	}
	content := append([]byte(nil), desired...)
	return Change{Path: path, Kind: kind, Mode: mode.Perm(), Content: content}, nil
}

func readOptionalFile(path string, defaultMode fs.FileMode) ([]byte, fs.FileMode, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, defaultMode, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, fmt.Errorf("%s is a symlink; refusing to replace it", path)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("%s is not a regular file", path)
	}
	return data, info.Mode().Perm(), nil
}

func writeAtomic(path string, content []byte, mode fs.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".orc-agent-hook-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(mode.Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temporary file for %s: %w", path, err)
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary file for %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// DetectVersion returns the first non-empty line from an agent CLI's bounded
// --version probe.
func DetectVersion(executable string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, executable, "--version").CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("%s --version timed out", executable)
	}
	if err != nil {
		return "", fmt.Errorf("%s --version: %w", executable, err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if version := strings.TrimSpace(line); version != "" {
			return version, nil
		}
	}
	return "", fmt.Errorf("%s --version returned no version", executable)
}

// Summary returns stable human-readable status for doctor output.
func (integration Integration) Summary() string {
	if integration.Err != nil {
		return integration.Err.Error()
	}
	if integration.Ready() {
		return fmt.Sprintf("ready (%s); config %s", strings.Join(integration.SupportedStates, ", "), integration.ConfigPath)
	}
	var pending []string
	for _, change := range integration.Changes {
		if change.Kind != "unchanged" {
			pending = append(pending, change.Kind+" "+change.Path)
		}
	}
	sort.Strings(pending)
	return strings.Join(pending, "; ")
}
