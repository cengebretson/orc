package main

import "testing"

func TestDashboardIsPrimaryAndTUIIsRemoved(t *testing.T) {
	dashboard, _, err := rootCmd.Find([]string{"dashboard"})
	if err != nil {
		t.Fatal(err)
	}
	if dashboard != dashboardCmd || dashboard.Deprecated != "" {
		t.Fatalf("dashboard command = %#v, want primary non-deprecated command", dashboard)
	}

	legacy, _, err := rootCmd.Find([]string{"tui"})
	if err == nil || legacy == dashboardCmd {
		t.Fatalf("tui command still resolves: command=%v err=%v", legacy, err)
	}
}
