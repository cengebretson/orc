package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/workers"
)

var errRunSelectionCancelled = errors.New("run selection cancelled")

type runChoice struct {
	Value       string
	Label       string
	Description string
}

var (
	runInputIsTTY = isTTY
	runChoose     = chooseRunChoice
)

func chooseRunChoice(title string, choices []runChoice) (string, error) {
	if len(choices) == 0 {
		return "", fmt.Errorf("no choices available")
	}
	fmt.Println(title)
	for i, choice := range choices {
		fmt.Printf("  %d. %s", i+1, choice.Label)
		if choice.Description != "" {
			fmt.Printf(" — %s", choice.Description)
		}
		fmt.Println()
	}
	for {
		answer := strings.TrimSpace(promptLine("Select a number (q to cancel): "))
		if answer == "" || strings.EqualFold(answer, "q") || strings.EqualFold(answer, "quit") {
			return "", errRunSelectionCancelled
		}
		selected, err := strconv.Atoi(answer)
		if err == nil && selected >= 1 && selected <= len(choices) {
			return choices[selected-1].Value, nil
		}
		fmt.Printf("Choose a number from 1 to %d, or q to cancel.\n", len(choices))
	}
}

func selectRunWorker(requested string, available []*workers.Worker) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		if workers.FindByID(available, requested) == nil {
			return "", fmt.Errorf("worker %q not found in workers/ (available: %s)", requested, runWorkerIDs(available))
		}
		return requested, nil
	}
	if len(available) == 0 {
		return "", fmt.Errorf("no workers are configured in workers/")
	}
	if !runInputIsTTY() {
		return "", fmt.Errorf("worker is required in non-interactive mode (available: %s); pass --worker <id>", runWorkerIDs(available))
	}

	choices := make([]runChoice, 0, len(available))
	for _, worker := range available {
		var detailParts []string
		for _, value := range []string{worker.Name, worker.Engine, worker.Model} {
			if value = strings.TrimSpace(value); value != "" {
				detailParts = append(detailParts, value)
			}
		}
		details := strings.Join(detailParts, " · ")
		choices = append(choices, runChoice{Value: worker.ID, Label: worker.ID, Description: details})
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Value < choices[j].Value })
	return runChoose("Choose a worker:", choices)
}

func runWorkerIDs(available []*workers.Worker) string {
	ids := make([]string, 0, len(available))
	for _, worker := range available {
		ids = append(ids, worker.ID)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}

func selectRunRepositoryForCommand(root, cwd, requested string, repos []config.Repo) (*runRepository, error) {
	selected, err := selectRunRepository(root, cwd, requested, repos)
	if err != nil || selected != nil || strings.TrimSpace(requested) != "" || len(repos) <= 1 {
		return selected, err
	}

	choices := []runChoice{{Label: "Workspace root", Description: "run without a selected repository"}}
	resolved := make(map[string]*runRepository)
	for _, repo := range repos {
		candidate, resolveErr := resolveRunRepository(root, repo, false)
		if resolveErr != nil {
			continue
		}
		resolved[repo.Name] = candidate
		choices = append(choices, runChoice{Value: repo.Name, Label: repo.Name, Description: candidate.Path})
	}
	if len(choices) == 1 {
		return nil, nil
	}
	repositoryChoices := choices[1:]
	sort.Slice(repositoryChoices, func(i, j int) bool { return repositoryChoices[i].Value < repositoryChoices[j].Value })
	if !runInputIsTTY() {
		names := make([]string, 0, len(resolved))
		for name := range resolved {
			names = append(names, name)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("repository cannot be inferred in non-interactive mode (available: %s); pass --repo <name>", strings.Join(names, ", "))
	}
	value, err := runChoose("Choose a repository:", choices)
	if err != nil || value == "" {
		return nil, err
	}
	return resolved[value], nil
}
