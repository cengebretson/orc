package main

import (
	"path/filepath"
	"time"

	"github.com/cengebretson/orc/internal/dashboard"
	"github.com/cengebretson/orc/internal/watch"
	"github.com/spf13/cobra"
)

func runDemo(cmd *cobra.Command, args []string) error {
	root, err := filepath.Abs(globalWorkspace)
	if err != nil {
		return err
	}

	return dashboard.Run(root, demoDashboardOptions())
}

func demoDashboardOptions() dashboard.Options {
	return dashboard.Options{
		Start:    dashboard.SectionWatch,
		Adaptive: true,
		Watch: watch.Options{
			Interval: 5 * time.Second,
			Demo:     true,
		},
	}
}
