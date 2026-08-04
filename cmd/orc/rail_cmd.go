package main

import (
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/spf13/cobra"
)

var (
	railLayout string
	railSize   string
)

var railCmd = &cobra.Command{
	Use:   "rail",
	Short: "Manage Orc's tmux watch rail",
	Args:  cobra.NoArgs,
}

var railOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open or reuse the Orc-owned rail in this tmux window",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		root, err := resolveRoot(globalWorkspace)
		if err != nil {
			return err
		}
		return tmux.OpenRail(railOptions(root))
	},
}

var railCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close the Orc-owned rail in this tmux window",
	Args:  cobra.NoArgs,
	RunE:  func(_ *cobra.Command, _ []string) error { return tmux.CloseRail() },
}

var railToggleCmd = &cobra.Command{
	Use:   "toggle",
	Short: "Open or close the Orc-owned rail in this tmux window",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		root, err := resolveRoot(globalWorkspace)
		if err != nil {
			return err
		}
		return tmux.ToggleRail(railOptions(root))
	},
}

var railCollapseCmd = &cobra.Command{
	Use:   "collapse",
	Short: "Resize the rail to its compact width",
	Args:  cobra.NoArgs,
	RunE:  func(_ *cobra.Command, _ []string) error { return tmux.CollapseRail() },
}

var railExpandCmd = &cobra.Command{
	Use:   "expand",
	Short: "Restore the rail's previous expanded size",
	Args:  cobra.NoArgs,
	RunE:  func(_ *cobra.Command, _ []string) error { return tmux.ExpandRail(railSize) },
}

var railToggleCollapsedCmd = &cobra.Command{
	Use:   "toggle-collapsed",
	Short: "Collapse or expand the Orc-owned rail",
	Args:  cobra.NoArgs,
	RunE:  func(_ *cobra.Command, _ []string) error { return tmux.ToggleCollapsedRail(railSize) },
}

func railOptions(root string) tmux.WatchToggleOptions {
	return tmux.WatchToggleOptions{
		Root: root, Interval: "5s",
		Layout: railLayout, Size: railSize,
	}
}
