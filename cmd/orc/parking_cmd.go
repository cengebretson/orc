package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/parking"
	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/spf13/cobra"
)

func runSessionsPark(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	if !tmux.Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}
	sessions, err := sessionlist.Collect(root, sessionlist.Options{})
	if err != nil {
		return err
	}
	entries, skipped := parkableEntries(sessions)
	printParkingPlan("Park", entries, skipped)
	if len(entries) == 0 {
		return nil
	}
	if sessionsParkDry {
		return nil
	}
	if !sessionsParkYes {
		return fmt.Errorf("refusing to stop tmux sessions without --yes; use --dry to preview")
	}
	path, err := parking.Path(root, "")
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("parking snapshot already exists at %s; unpark it before parking again", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check existing parking snapshot: %w", err)
	}
	snapshot := parking.Snapshot{Workspace: root, ParkedAt: time.Now().UTC(), Sessions: entries}
	if err := parking.Save(path, snapshot); err != nil {
		return fmt.Errorf("save parking snapshot: %w", err)
	}
	for _, entry := range entries {
		if err := tmux.KillSession(entry.TmuxSession); err != nil {
			return fmt.Errorf("parking snapshot saved at %s, but %w", path, err)
		}
	}
	fmt.Printf("Parked %d session(s). Snapshot: %s\n", len(entries), path)
	return nil
}

func parkableEntries(sessions []sessionlist.Session) ([]parking.Entry, int) {
	seen := make(map[string]bool)
	var entries []parking.Entry
	skipped := 0
	for _, session := range sessions {
		if session.Kind != sessionlist.KindManaged || !session.Running || session.Target == nil {
			continue
		}
		if seen[session.Target.Session] {
			continue
		}
		seen[session.Target.Session] = true
		if session.Live == nil || session.Live.ProviderSessionID == "" {
			skipped++
			continue
		}
		cwd := session.Live.CWD
		if cwd == "" {
			cwd = session.FeatureDir
		}
		if _, _, err := telemetry.ResumeArgs(session.Engine, session.Live.ProviderSessionID, cwd); err != nil {
			skipped++
			continue
		}
		entries = append(entries, parking.Entry{
			Ticket: session.Ticket, Stage: session.Stage, Worker: session.Worker,
			Engine: session.Engine, ProviderSessionID: session.Live.ProviderSessionID,
			CWD: cwd, FeatureDir: session.FeatureDir,
			TmuxSession: session.Target.Session, TmuxWindow: session.Target.Window,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Ticket < entries[j].Ticket })
	return entries, skipped
}

func runSessionsUnpark(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	path, err := parking.Path(root, "")
	if err != nil {
		return err
	}
	snapshot, err := parking.Load(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("no parking snapshot for this workspace")
	}
	if err != nil {
		return fmt.Errorf("load parking snapshot: %w", err)
	}
	if snapshot.Workspace != root {
		return fmt.Errorf("parking snapshot belongs to %s, not %s", snapshot.Workspace, root)
	}
	printParkingPlan("Unpark", snapshot.Sessions, 0)
	if len(snapshot.Sessions) == 0 || sessionsUnparkDry {
		return nil
	}
	if !sessionsUnparkYes {
		return fmt.Errorf("refusing to create tmux sessions without --yes; use --dry to preview")
	}
	if !tmux.Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}

	total := len(snapshot.Sessions)
	remaining := append([]parking.Entry(nil), snapshot.Sessions...)
	var failures []error
	for _, entry := range snapshot.Sessions {
		if err := unparkEntry(entry); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.Ticket, err))
			continue
		}
		remaining = removeParkedEntry(remaining, entry.TmuxSession)
		snapshot.Sessions = remaining
		if len(remaining) > 0 {
			if err := parking.Save(path, snapshot); err != nil {
				return fmt.Errorf("update parking snapshot: %w", err)
			}
		}
	}
	if len(remaining) == 0 {
		if err := parking.Remove(path); err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	fmt.Printf("Unparked %d session(s).\n", total)
	return nil
}

func unparkEntry(entry parking.Entry) error {
	if tmux.SessionExists(entry.TmuxSession) {
		panes, err := tmux.ListPanesDetailed()
		if err != nil {
			return fmt.Errorf("inspect existing tmux session %s: %w", entry.TmuxSession, err)
		}
		pane, ok := restoredPane(entry, panes)
		if !ok {
			return fmt.Errorf("tmux session %s already exists without matching parked provider identity", entry.TmuxSession)
		}
		if err := state.SetRuntimeTarget(entry.FeatureDir, entry.TmuxSession, pane); err != nil {
			return err
		}
		return nil
	}
	binary, argv, err := telemetry.ResumeArgs(entry.Engine, entry.ProviderSessionID, entry.CWD)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", binary)
	}
	if _, err := os.Stat(entry.CWD); err != nil {
		return fmt.Errorf("cwd %s is unavailable", entry.CWD)
	}
	if err := tmux.CreateSession(entry.TmuxSession, entry.FeatureDir, []string{entry.TmuxWindow}); err != nil {
		return err
	}
	created := true
	defer func() {
		if created {
			_ = tmux.KillSession(entry.TmuxSession)
		}
	}()
	if err := tmux.SetSessionEnvironment(entry.TmuxSession, tmux.EnvResumedFrom, entry.ProviderSessionID); err != nil {
		return err
	}
	launchArgv := resumedLaunchArgv(binary, argv, entry.ProviderSessionID)
	pane, err := tmux.SendCommandTarget(entry.TmuxSession, entry.TmuxWindow, "", entry.FeatureDir, entry.CWD, launchArgv)
	if err != nil {
		return err
	}
	metadata := tmux.WindowMetadata{
		Ticket: entry.Ticket, Stage: entry.Stage, Worker: entry.Worker,
		Engine: entry.Engine, ProviderSessionID: entry.ProviderSessionID,
		FeatureDir: entry.FeatureDir,
	}
	if err := tmux.SetWindowMetadata(entry.TmuxSession, entry.TmuxWindow, metadata); err != nil {
		return err
	}
	if err := tmux.SetPaneMetadata(pane, metadata); err != nil {
		return err
	}
	if err := state.SetRuntimeTarget(entry.FeatureDir, entry.TmuxSession, pane); err != nil {
		return err
	}
	created = false
	return nil
}

func restoredPane(entry parking.Entry, panes []tmux.Pane) (string, bool) {
	for _, pane := range panes {
		engine := pane.ProviderEngine
		if engine == "" {
			engine = pane.Engine
		}
		if !pane.Agent || pane.Session != entry.TmuxSession || pane.Window != entry.TmuxWindow ||
			pane.Ticket != entry.Ticket || pane.Stage != entry.Stage ||
			!strings.EqualFold(engine, entry.Engine) || pane.ProviderSessionID != entry.ProviderSessionID {
			continue
		}
		return pane.ID, true
	}
	return "", false
}

func resumedLaunchArgv(binary string, argv []string, providerSessionID string) []string {
	launch := []string{"env", tmux.EnvResumedFrom + "=" + providerSessionID, binary}
	return append(launch, argv...)
}

func printParkingPlan(action string, entries []parking.Entry, skipped int) {
	if len(entries) == 0 {
		fmt.Printf("%s: no resumable managed sessions found.\n", action)
	} else {
		fmt.Printf("%s %d managed session(s):\n", action, len(entries))
		for _, entry := range entries {
			fmt.Printf("  %-14s  %-7s  %s  %s\n", entry.Ticket, entry.Engine, entry.TmuxSession, entry.ProviderSessionID)
		}
	}
	if skipped > 0 {
		fmt.Printf("Skipped %d running session(s) without safe provider resume metadata.\n", skipped)
	}
}

func removeParkedEntry(entries []parking.Entry, tmuxSession string) []parking.Entry {
	out := entries[:0]
	for _, entry := range entries {
		if entry.TmuxSession != tmuxSession {
			out = append(out, entry)
		}
	}
	return out
}
