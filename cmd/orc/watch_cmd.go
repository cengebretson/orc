package main

import (
	"path/filepath"
	"time"

	"github.com/cengebretson/orc/internal/dashboard"
	"github.com/cengebretson/orc/internal/watch"
	"github.com/spf13/cobra"
)

func runWatch(cmd *cobra.Command, args []string) error {
	root := ""
	var err error
	if watchDemo {
		root, err = filepath.Abs(globalWorkspace)
	} else {
		root, err = resolveRoot(globalWorkspace)
	}
	if err != nil {
		return err
	}

	var ticket string
	if len(args) > 0 {
		ticket = args[0]
	}
	interval, err := time.ParseDuration(watchInterval)
	if err != nil {
		return err
	}

	watchOpts := watch.Options{
		Ticket:   ticket,
		Interval: interval,
		Wide:     watchWide,
		Demo:     watchDemo,
		Mux:      muxBackend,
	}
	return dashboard.Run(root, dashboard.Options{
		Start:     dashboard.SectionWatch,
		Adaptive:  true,
		Version:   version,
		BuildDate: buildDate,
		Watch:     watchOpts,
		Mux:       muxBackend,
	})
}
