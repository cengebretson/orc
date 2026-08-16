package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	initSkipDefaultPack bool
	initDryRun          bool
	initForce           bool
)

var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Inspect and manage workflow packs",
}

var packListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed packs and what they install",
	RunE:  runPackList,
}

var packAvailableCmd = &cobra.Command{
	Use:   "available",
	Short: "List built-in packs available for install",
	Args:  cobra.NoArgs,
	RunE:  runPackAvailable,
}

var packShowCmd = &cobra.Command{
	Use:   "show <pack>",
	Short: "Show one installed pack and what it installs",
	Args:  cobra.ExactArgs(1),
	RunE:  runPackShow,
}

var packInspectCmd = &cobra.Command{
	Use:   "inspect <path>",
	Short: "Validate and preview a local pack without installing it",
	Args:  cobra.ExactArgs(1),
	RunE:  runPackInspect,
}

var packInstallCmd = &cobra.Command{
	Use:   "install <pack>",
	Short: "Install a pack into the current workspace",
	Args:  cobra.ExactArgs(1),
	RunE:  runPackInstall,
}

var packInspectJSON bool

var doctorCmd = &cobra.Command{
	Use:   "doctor [ticket]",
	Short: "Check workspace and local tool readiness, or validate a ticket's state when a ticket ID is given",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDoctor,
}

var doctorFix bool
var doctorSystem bool

var nextCmd = &cobra.Command{
	Use:   "next <ticket>",
	Short: "Launch the next agent for a ticket — use --dry to preview without running",
	Args:  cobra.ExactArgs(1),
	RunE:  runNext,
}

var (
	nextJSON   bool
	nextDry    bool
	nextWorker string
)

var statusCmd = &cobra.Command{
	Use:   "status [ticket]",
	Short: "Show all features and their current stage, or full details for a specific ticket",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runStatus,
}

var statusJSON bool
var statusLabels []string

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List managed, orphaned, and optional unmanaged agent sessions",
	Args:  cobra.NoArgs,
	RunE:  runSessions,
}

var (
	sessionsJSON   bool
	sessionsAll    bool
	sessionsLabels []string
)

var sessionsResumeCmd = &cobra.Command{
	Use:   "resume [provider-session-id]",
	Short: "Resume a discovered Claude or Codex session in the foreground",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runSessionResume,
}

var (
	sessionsResumeEngine string
	sessionsResumeCWD    string
	sessionsResumeDry    bool
	sessionsResumeForce  bool
)

var sessionsParkCmd = &cobra.Command{
	Use:   "park",
	Short: "Snapshot and stop resumable Orc-managed tmux sessions",
	Args:  cobra.NoArgs,
	RunE:  runSessionsPark,
}

var sessionsUnparkCmd = &cobra.Command{
	Use:   "unpark",
	Short: "Recreate sessions from the last Orc parking snapshot",
	Args:  cobra.NoArgs,
	RunE:  runSessionsUnpark,
}

var (
	sessionsParkDry   bool
	sessionsParkYes   bool
	sessionsUnparkDry bool
	sessionsUnparkYes bool
)

var reportCmd = &cobra.Command{
	Use:   "report [ticket]",
	Short: "Show time-in-stage derived from ticket history",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runReport,
}

var (
	reportJSON     bool
	reportArchived bool
	reportByWorker bool
)

var artifactsCmd = &cobra.Command{
	Use:   "artifacts <ticket>",
	Short: "Check required feature artifacts for a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runArtifacts,
}

var (
	artifactsAll  bool
	artifactsJSON bool
)

var workCmd = &cobra.Command{
	Use:   "work <ticket>",
	Short: "Start work on a ticket — creates the feature folder and STATE.yaml",
	Args:  cobra.ExactArgs(1),
	RunE:  runWork,
}

var (
	workSlug     string
	workTmux     bool
	workNext     bool
	workWorkflow string
)

var runCmd = &cobra.Command{
	Use:   "run \"<instruction>\"",
	Short: "Create and launch a standalone local feature",
	Args:  cobra.ExactArgs(1),
	RunE:  runLocal,
}

var (
	runSlug       string
	runWorker     string
	runRepo       string
	runTmux       bool
	runAutoAttach bool
)

var markCmd = &cobra.Command{
	Use:   "mark <ticket> <start|resume|next|pause|done|jit> [reason]",
	Short: "Update ticket state — start | resume | next [--result] [--stage] [--worker] | pause <reason> | done [--result] | jit <summary>",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runMark,

	Hidden: true,
}

var (
	markWorker  string
	markResult  string
	markStage   string
	markForce   bool
	markConfirm bool
	markText    bool
	markChoices []string
)

var archiveCmd = &cobra.Command{
	Use:   "archive <ticket>",
	Short: "Archive a completed feature — removes worktrees and moves folder to features/_archive/",
	Args:  cobra.ExactArgs(1),
	RunE:  runArchive,
}

var deleteCmd = &cobra.Command{
	Use:   "delete <ticket>",
	Short: "Permanently delete a feature folder (only allowed when status is done or archived)",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

var jitCmd = &cobra.Command{
	Use:   "jit <ticket> --worker <id> \"<instruction>\"",
	Short: "Run a one-off agent task outside the pipeline",
	Args:  cobra.ExactArgs(2),
	RunE:  runJIT,
}

var (
	jitWorker  string
	jitDry     bool
	jitTmux    bool
	jitConsult bool
)

var attachCmd = &cobra.Command{
	Use:   "attach <ticket>",
	Short: "Attach to the tmux session for a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runAttach,
}

var focusCmd = &cobra.Command{
	Use:   "focus",
	Short: "Attach to the highest-priority live session that needs attention",
	Args:  cobra.NoArgs,
	RunE:  runFocus,
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Open the unified workspace and live-operations dashboard",
	Args:  cobra.NoArgs,
	RunE:  runDashboard,
}

var watchCmd = &cobra.Command{
	Use:   "watch [ticket]",
	Short: "Open the compact watch rail for active agent work",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runWatch,
}

var (
	watchInterval string
	watchWide     bool
	watchDemo     bool
)

var helpAllCmd = &cobra.Command{
	Use:   "help-all",
	Short: "List all commands with human and agent commands separated",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var human, agent []*cobra.Command
		for _, c := range rootCmd.Commands() {
			if c.Name() == "help" || c.Name() == "help-all" || c.Name() == "completion" {
				continue
			}
			if c.Hidden {
				agent = append(agent, c)
			} else {
				human = append(human, c)
			}
		}

		colWidth := func(cmds []*cobra.Command) int {
			w := len("COMMAND")
			for _, c := range cmds {
				if n := len(c.UseLine()); n > w {
					w = n
				}
			}
			return w
		}
		printSection := func(title string, cmds []*cobra.Command) {
			w := colWidth(cmds)
			fmt.Println(title)
			fmt.Println()
			fmt.Printf("  %-*s  %s\n", w, "COMMAND", "DESCRIPTION")
			fmt.Printf("  %-*s  %s\n", w, strings.Repeat("-", w), strings.Repeat("-", len("DESCRIPTION")))
			for _, c := range cmds {
				fmt.Printf("  %-*s  %s\n", w, c.UseLine(), c.Short)
			}
		}

		printSection("Human commands:", human)
		fmt.Println()
		printSection("Agent commands  (called by agents, hidden from orc --help):", agent)
		fmt.Println()
		fmt.Println("Read commands  (human commands agents also use):")
		fmt.Println()
		fmt.Println("  orc status <ticket> --json    read current state as JSON")
		fmt.Println("  orc artifacts <ticket> --json read required artifact readiness as JSON")
		fmt.Println()
	},
}
