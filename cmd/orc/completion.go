package main

import (
	"github.com/cengebretson/orc/internal/state"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

// completeTickets returns ticket IDs from features/ (and optionally _archive/)
// whose status matches one of the allowed values. Pass nil to allow all statuses.
func completeTickets(root string, allowedStatuses []string, includeArchive bool) []string {
	allowed := make(map[string]bool, len(allowedStatuses))
	for _, s := range allowedStatuses {
		allowed[s] = true
	}

	featuresDir := filepath.Join(root, "features")
	searchDirs := []string{featuresDir}
	if includeArchive {
		searchDirs = append(searchDirs, filepath.Join(featuresDir, "_archive"))
	}

	var tickets []string
	for _, dir := range searchDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "_template" || e.Name() == "_archive" {
				continue
			}
			if len(allowedStatuses) > 0 {
				s, err := state.Load(filepath.Join(dir, e.Name()))
				if err != nil || !allowed[s.Status] {
					continue
				}
			}
			// Return the ticket ID portion (prefix up to second hyphen-segment).
			// Fall back to the full dir name if STATE.yaml can't be read cleanly.
			slug := e.Name()
			if s, err := state.Load(filepath.Join(dir, e.Name())); err == nil && s.Ticket != "" {
				tickets = append(tickets, s.Ticket)
			} else {
				tickets = append(tickets, slug)
			}
		}
	}
	return tickets
}

// ticketCompleter returns a ValidArgsFunction for commands that take a ticket ID as their first arg.
func ticketCompleter(statuses []string, includeArchive bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		root, err := resolveRoot(globalWorkspace)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeTickets(root, statuses, includeArchive), cobra.ShellCompDirectiveNoFileComp
	}
}
