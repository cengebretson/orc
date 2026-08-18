package main

import (
	"testing"
	"time"

	"github.com/cengebretson/orc/internal/dashboard"
)

func TestDemoIsPrimaryCommand(t *testing.T) {
	demo, _, err := rootCmd.Find([]string{"demo"})
	if err != nil {
		t.Fatal(err)
	}
	if demo != demoCmd || demo.Hidden || demo.Deprecated != "" {
		t.Fatalf("demo command = %#v, want primary visible command", demo)
	}
	if err := demo.Args(demo, []string{"ticket"}); err == nil {
		t.Fatal("demo accepted an unexpected positional argument")
	}
}

func TestDemoUsesSyntheticLiveDashboard(t *testing.T) {
	opts := demoDashboardOptions()
	if opts.Start != dashboard.SectionWatch || !opts.Adaptive {
		t.Fatalf("dashboard options = %#v, want adaptive synthetic watch section", opts)
	}
	if !opts.Watch.Demo {
		t.Fatalf("watch options = %#v, want synthetic work", opts.Watch)
	}
	if opts.Watch.Interval != 5*time.Second {
		t.Fatalf("watch interval = %s, want 5s", opts.Watch.Interval)
	}
}
