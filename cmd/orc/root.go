package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/spf13/cobra"
)

const banner = `
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢀⡀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢠⣿⣿⡄⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⣀⣤⣶⣧⣄⣉⣉⣠⣼⣶⣤⣀⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⢰⣿⣿⣿⣿⡿⣿⣿⣿⣿⢿⣿⣿⣿⣿⡆⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⣼⣤⣤⣈⠙⠳⢄⣉⣋⡡⠞⠋⣁⣤⣤⣧⠀⠀⠀⠀⠀⠀⠀
⠀⢲⣶⣤⣄⡀⢀⣿⣄⠙⠿⣿⣦⣤⡿⢿⣤⣴⣿⠿⠋⣠⣿⠀⢀⣠⣤⣶⡖⠀
⠀⠀⠙⣿⠛⠇⢸⣿⣿⡟⠀⡄⢉⠉⢀⡀⠉⡉⢠⠀⢻⣿⣿⡇⠸⠛⣿⠋⠀⠀
⠀⠀⠀⠘⣷⠀⢸⡏⠻⣿⣤⣤⠂⣠⣿⣿⣄⠑⣤⣤⣿⠟⢹⡇⠀⣾⠃⠀⠀⠀
⠀⠀⠀⠀⠘⠀⢸⣿⡀⢀⠙⠻⢦⣌⣉⣉⣡⡴⠟⠋⡀⢀⣿⡇⠀⠃⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⢸⣿⣧⠈⠛⠂⠀⠉⠛⠛⠉⠀⠐⠛⠁⣼⣿⡇⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠸⣏⠀⣤⡶⠖⠛⠋⠉⠉⠙⠛⠲⢶⣤⠀⣹⠇⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⢹⣿⣶⣿⣿⣿⣿⣿⣿⣶⣿⡏⠀⠀⠀⠀⠀⠀⠀⠀⠀
⠀⠀⠀⠀⠀⠀⠀⠀⠀⠈⠉⠉⠉⠛⠛⠛⠛⠉⠉⠉⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀

orc · workspace orchestrator
`

var version = "dev"

var globalWorkspace string

var rootCmd = &cobra.Command{
	Use:   "orc",
	Short: "orc — agentic workspace orchestrator",
	Long:  banner,
	// Runs after flag/arg validation, so usage still prints for misuse
	// but not for errors returned by the command itself.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the orc version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(version)
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new orc workspace — asks questions interactively when run in a terminal",
	RunE:  runInit,
}

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

var reportCmd = &cobra.Command{
	Use:   "report [ticket]",
	Short: "Show time-in-stage derived from ticket history",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runReport,
}

var (
	reportJSON     bool
	reportArchived bool
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

var markCmd = &cobra.Command{
	Use:   "mark <ticket> <start|resume|next|pause|done> [reason]",
	Short: "Update ticket state — start | resume | next [--result] [--stage] [--worker] | pause <reason> | done [--result]",
	Args:  cobra.MinimumNArgs(2),
	RunE:  runMark,

	Hidden: true,
}

var (
	markWorker string
	markResult string
	markStage  string
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
	jitWorker string
	jitDry    bool
	jitTmux   bool
)

var attachCmd = &cobra.Command{
	Use:   "attach <ticket>",
	Short: "Attach to the tmux session for a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runAttach,
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the interactive dashboard",
	Args:  cobra.NoArgs,
	RunE:  runTui,
}

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
		fmt.Println()
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&globalWorkspace, "workspace", ".", "Workspace root (default: current directory)")

	initCmd.Flags().BoolVar(&initSkipDefaultPack, "skip-default-pack", false, "Create the base workspace without installing the default pack")
	initCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Print what would be created without writing files")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing generated files")
	packInspectCmd.Flags().BoolVar(&packInspectJSON, "json", false, "Output as JSON")

	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Remove provably-stale state locks (dead PID or old without a valid PID); live locks are never touched")
	nextCmd.Flags().BoolVar(&nextJSON, "json", false, "Output as JSON")
	nextCmd.Flags().BoolVar(&nextDry, "dry", false, "Print the launch command without executing it")
	nextCmd.Flags().StringVar(&nextWorker, "worker", "", "Override the workflow's default worker (worker ID)")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "Output as JSON")
	reportCmd.Flags().BoolVar(&reportArchived, "archived", false, "Include archived tickets in the aggregate (no-arg) report")
	workCmd.Flags().StringVar(&workSlug, "slug", "", "Optional slug suffix (e.g. add-user-export → TICKET-123-add-user-export)")
	workCmd.Flags().BoolVar(&workTmux, "tmux", false, "Enable tmux session for this ticket — session created automatically on first orc next")
	workCmd.Flags().BoolVar(&workNext, "next", false, "Immediately launch the first stage after creating the feature")
	workCmd.Flags().StringVar(&workWorkflow, "workflow", "", "Workflow to use (default: settings.default_workflow in orc.yaml)")
	markCmd.Flags().StringVar(&markWorker, "worker", "", "Worker ID that owns the new stage (next only)")
	markCmd.Flags().StringVar(&markResult, "result", "", "Summary of what was accomplished (next/done only)")
	markCmd.Flags().StringVar(&markStage, "stage", "", "New stage name (next only — required when crossing workflow boundaries)")
	jitCmd.Flags().StringVar(&jitWorker, "worker", "", "Worker ID to run the task (required)")
	_ = jitCmd.MarkFlagRequired("worker")
	jitCmd.Flags().BoolVar(&jitDry, "dry", false, "Print resolved worker and prompt without launching")
	jitCmd.Flags().BoolVar(&jitTmux, "tmux", false, "Send to the ticket's existing tmux session instead of foreground")

	nextCmd.ValidArgsFunction = ticketCompleter([]string{"pending", "active", "paused"}, false)
	statusCmd.ValidArgsFunction = ticketCompleter(nil, true)
	reportCmd.ValidArgsFunction = ticketCompleter(nil, true)
	markCmd.ValidArgsFunction = ticketCompleter([]string{"pending", "active", "paused"}, false)
	attachCmd.ValidArgsFunction = ticketCompleter([]string{"active"}, false)
	archiveCmd.ValidArgsFunction = ticketCompleter([]string{"done"}, false)
	deleteCmd.ValidArgsFunction = ticketCompleter([]string{"done", "archived"}, true)
	jitCmd.ValidArgsFunction = ticketCompleter(nil, true)
	doctorCmd.ValidArgsFunction = ticketCompleter(nil, false)

	packCmd.AddCommand(packListCmd)
	packCmd.AddCommand(packAvailableCmd)
	packCmd.AddCommand(packShowCmd)
	packCmd.AddCommand(packInspectCmd)
	packCmd.AddCommand(packInstallCmd)

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(packCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(nextCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(workCmd)
	rootCmd.AddCommand(markCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(jitCmd)
	rootCmd.AddCommand(attachCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(helpAllCmd)
	rootCmd.AddCommand(versionCmd)
}

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

func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// promptLine prints the prompt and reads one line from stdin.
func promptLine(prompt string) string {
	fmt.Print(prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func resolveWorkflow(root, ticketWorkflow string) string {
	if ticketWorkflow != "" {
		return ticketWorkflow
	}
	cfg, _ := config.Load(root)
	if cfg != nil && cfg.Settings.DefaultWorkflow != "" {
		return cfg.Settings.DefaultWorkflow
	}
	return ""
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printDryRun(plan *runner.Plan, ticket string) {
	fmt.Printf("Worker:  %s  (%s)\n", plan.Worker.Name, plan.WorkerReason)
	fmt.Printf("Engine: %s\n", plan.Worker.Engine)
	if plan.Worker.Model != "" {
		fmt.Printf("Model:   %s\n", plan.Worker.Model)
	}
	fmt.Printf("cwd:     %s\n", plan.CWD)
	fmt.Println()
	fmt.Println("Would run:")
	fmt.Printf("  %s\n", plan.LaunchCommand)
	fmt.Println()
	fmt.Printf("Override worker: orc next %s --worker <worker-id>\n", ticket)
}

func resolveRoot(path string) (string, error) {
	if path == "." {
		return os.Getwd()
	}
	return path, nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		// cobra already printed the error; just set the exit code
		os.Exit(1)
	}
}
