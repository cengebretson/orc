package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/sessionlist"
	"github.com/cengebretson/orc/internal/telemetry"
	"github.com/spf13/cobra"
)

func runSessions(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	sessions, err := sessionlist.Collect(root, sessionlist.Options{IncludeUnmanaged: sessionsAll})
	if err != nil {
		return err
	}
	if sessionsJSON {
		return printJSON(sessions)
	}
	if len(sessions) == 0 {
		message := "No managed or orphaned sessions found."
		if !sessionsAll {
			message += " Use --all to include recent unmanaged Claude and Codex sessions."
		}
		fmt.Println(message)
		return nil
	}

	fmt.Printf("%-10s  %-14s  %-9s  %-18s  %-11s  %-17s  %-10s  %s\n", "Kind", "Ticket", "Engine", "Model", "State", "Target", "Context", "Last active")
	fmt.Printf("%-10s  %-14s  %-9s  %-18s  %-11s  %-17s  %-10s  %s\n", "----", "------", "------", "-----", "-----", "------", "-------", "-----------")
	for _, session := range sessions {
		model, liveState, context, active := liveColumns(session.Live)
		if liveState == "" && session.Kind == sessionlist.KindManaged && !session.Running {
			liveState = "stopped"
		}
		target := "-"
		if session.Target != nil {
			target = session.Target.Session + ":" + session.Target.Window
			if session.Target.Pane != "" {
				target += "." + strings.TrimPrefix(session.Target.Pane, "%")
			}
		}
		engine := session.Engine
		if engine == "" && session.Live != nil {
			engine = session.Live.Engine
		}
		fmt.Printf("%-10s  %-14s  %-9s  %-18s  %-11s  %-17s  %-10s  %s\n",
			session.Kind, emptyDash(session.Ticket), emptyDash(engine), emptyDash(model),
			emptyDash(liveState), target, context, active)
	}
	return nil
}

func runSessionResume(cmd *cobra.Command, args []string) error {
	id := args[0]
	discovered, err := telemetry.Discover("")
	if err != nil {
		return err
	}
	var matches []telemetry.Live
	for _, live := range discovered {
		if live.ProviderSessionID != id {
			continue
		}
		if sessionsResumeEngine != "" && !strings.EqualFold(sessionsResumeEngine, live.Engine) {
			continue
		}
		matches = append(matches, live)
	}
	if len(matches) == 0 {
		return fmt.Errorf("provider session %q was not found in recent local Claude or Codex metadata", id)
	}
	if len(matches) > 1 {
		return fmt.Errorf("provider session %q is ambiguous; pass --engine claude or --engine codex", id)
	}
	live := matches[0]
	if !sessionsResumeForce && (live.PID > 0 || strings.EqualFold(live.State, "working") || strings.EqualFold(live.State, "active")) {
		return fmt.Errorf("provider session %q still appears active (state %q, pid %d); use --force only after confirming it is safe to resume", id, live.State, live.PID)
	}
	cwd := live.CWD
	if sessionsResumeCWD != "" {
		cwd = sessionsResumeCWD
	}
	if cwd == "" {
		return fmt.Errorf("provider session %q has no recorded cwd; pass --cwd", id)
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return err
	}
	info, err := os.Stat(cwd)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("resume cwd %s is not an accessible directory", cwd)
	}
	binary, argv, err := telemetry.ResumeArgs(live.Engine, id, cwd)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath(binary); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", binary)
	}
	if sessionsResumeDry {
		fmt.Printf("cwd: %s\n", cwd)
		fmt.Printf("run: %s\n", displayCommand(append([]string{binary}, argv...)))
		return nil
	}
	resume := exec.Command(binary, argv...)
	resume.Dir = cwd
	resume.Stdin = os.Stdin
	resume.Stdout = os.Stdout
	resume.Stderr = os.Stderr
	return resume.Run()
}

func displayCommand(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = strconv.Quote(arg)
	}
	return strings.Join(quoted, " ")
}

func liveColumns(live *telemetry.Live) (model, state, context, active string) {
	context, active = "-", "-"
	if live == nil {
		return "", "", context, active
	}
	if live.ContextLimit > 0 {
		context = fmt.Sprintf("%d%%", live.ContextUsed*100/live.ContextLimit)
	} else if live.ContextUsed > 0 {
		context = compactTokens(live.ContextUsed)
	}
	if !live.LastActive.IsZero() {
		active = relativeTime(time.Now(), live.LastActive)
	}
	return live.Model, live.State, context, active
}

func compactTokens(tokens uint64) string {
	if tokens >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
	if tokens >= 1000 {
		return fmt.Sprintf("%.1fk", float64(tokens)/1000)
	}
	return fmt.Sprintf("%d", tokens)
}

func relativeTime(now, then time.Time) string {
	d := now.Sub(then)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func emptyDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
