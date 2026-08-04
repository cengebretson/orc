package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"os"
	"path/filepath"
	"strings"
)

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// promptLine prints the prompt and reads one line from stdin.
func promptLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func resolveWorkflow(root, ticketWorkflow string) string {
	if ticketWorkflow != "" {
		return ticketWorkflow
	}
	cfg, _ := config.Load(root)
	if cfg != nil && cfg.Settings.DefaultWorkflow != "" {
		return cfg.Settings.DefaultWorkflow
	}
	return ""
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printDryRun(plan *runner.Plan, ticket string) {
	fmt.Printf("Worker:  %s  (%s)\n", plan.Worker.Name, plan.WorkerReason)
	fmt.Printf("Engine: %s\n", plan.Worker.Engine)
	if plan.Worker.Model != "" {
		fmt.Printf("Model:   %s\n", plan.Worker.Model)
	}
	fmt.Printf("cwd:     %s\n", plan.CWD)
	fmt.Println()
	fmt.Println("Would run:")
	fmt.Printf("  %s\n", plan.LaunchCommand)
	fmt.Println()
	fmt.Printf("Override worker: orc next %s --worker <worker-id>\n", ticket)
}

func resolveRoot(path string) (string, error) {
	if path == "." {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root, err := findWorkspaceRoot(cwd)
		if err != nil {
			return "", err
		}
		return root, nil
	}
	return filepath.Abs(path)
}

func findWorkspaceRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, config.Filename)); err == nil {
			return dir, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("checking workspace marker in %s: %w", dir, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("orc workspace not found from %s — run from an orc workspace, or pass --workspace /path/to/workspace", start)
		}
		dir = parent
	}
}

// parseLabelSelectors turns repeated --label flags into selectors. One helper
// so every command that filters by label agrees on what "key" and "key=value"
// mean.
func parseLabelSelectors(raw []string) ([]state.LabelSelector, error) {
	selectors := make([]state.LabelSelector, 0, len(raw))
	for _, item := range raw {
		selector, err := state.ParseSelector(item)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, selector)
	}
	return selectors, nil
}
