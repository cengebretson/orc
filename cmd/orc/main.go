package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/orchestrator"
	"github.com/cengebretson/orc/internal/report"
	"github.com/cengebretson/orc/internal/resume"
	"github.com/cengebretson/orc/internal/runner"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/ticket"
	"github.com/cengebretson/orc/internal/ticketview"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/cengebretson/orc/internal/tui"
	"github.com/cengebretson/orc/internal/validate"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/cengebretson/orc/internal/workspace"
	"github.com/cengebretson/orc/internal/workspacectx"
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
	Use:               "orc",
	Short:             "orc — agentic workspace orchestrator",
	Long:              banner,
	CompletionOptions: cobra.CompletionOptions{HiddenDefaultCmd: true},
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
	initPacks     []string
	initListPacks bool
	initDryRun    bool
	initForce     bool
)

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

	initCmd.Flags().StringSliceVar(&initPacks, "pack", nil, "Pack(s) to install: a named bundle of workflow + workers + stages. Repeatable. Omit for 'default'; use 'none' for a base-only workspace")
	initCmd.Flags().BoolVar(&initListPacks, "list-packs", false, "List available packs and exit")
	initCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Print what would be created without writing files")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing generated files")

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

	rootCmd.AddCommand(initCmd)
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

func runInit(cmd *cobra.Command, args []string) error {
	if initListPacks {
		return printPacks()
	}

	fmt.Print(banner)

	interactive := isTTY()

	// Workspace path — prompt if not explicitly set and running interactively.
	if !cmd.Root().PersistentFlags().Changed("workspace") && interactive {
		cwd, _ := os.Getwd()
		ans := promptLine(fmt.Sprintf("Workspace path [%s]: ", cwd))
		if ans == "" {
			globalWorkspace = cwd
		} else {
			globalWorkspace = ans
		}
	}

	// Pack — prompt if not explicitly set and running interactively.
	if !cmd.Flags().Changed("pack") && interactive {
		ans := strings.TrimSpace(promptLine("Which pack? [default] (or 'none' for a base-only workspace): "))
		if ans != "" {
			initPacks = []string{ans}
		}
	}

	opts := workspace.InitOptions{
		Root:   globalWorkspace,
		Packs:  initPacks,
		DryRun: initDryRun,
		Force:  initForce,
	}

	return workspace.Init(opts)
}

// printPacks lists the available packs for `orc init --list-packs`.
func printPacks() error {
	packs, err := workspace.ListPacks()
	if err != nil {
		return err
	}
	fmt.Println("Available packs:")
	fmt.Println()
	for _, p := range packs {
		fmt.Printf("  %-12s %s\n", p.Name, p.Description)
		fmt.Printf("  %-12s engines: %s\n", "", strings.Join(p.Engines, ", "))
	}
	fmt.Println()
	fmt.Println("Install with: orc init --pack <name>   (repeatable; omit for 'default', 'none' for base only)")
	return nil
}

// isTTY returns true when stdin is an interactive terminal.
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

func runDoctor(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		featureDir, err := ticket.Resolve(root, args[0])
		if err != nil {
			return err
		}
		if doctorFix {
			removed, err := state.ClearStaleLock(featureDir)
			if err != nil {
				return err
			}
			if removed {
				fmt.Printf("✓ removed stale %s.lock\n\n", state.Filename)
			}
		}
		report := validate.Run(root, featureDir)
		validate.Print(report)
		if !report.OK() {
			return fmt.Errorf("validation failed")
		}
		return nil
	}

	report := doctor.RunWithOptions(root, doctor.Options{Fix: doctorFix})
	doctor.Print(report)
	if !report.OK() {
		return fmt.Errorf("doctor found problems")
	}
	return nil
}

func runNext(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State

	if nextJSON {
		plan, err := runner.Compute(root, featureDir, nextWorker)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{
			"ticket":       plan.Ticket,
			"status":       s.Status,
			"workflow":     plan.Workflow,
			"stage":        plan.Stage,
			"stage_worker": s.Stage.Worker,
			"cwd":          plan.CWD,
			"prompt":       plan.Prompt,
			"worker":       plan.Worker.ID,
			"product":      plan.Worker.Engine,
			"model":        plan.Worker.Model,
			"launch":       plan.LaunchCommand,
		})
	}

	fmt.Printf("Ticket:   %s\n", s.Ticket)
	fmt.Printf("Status:   %s\n", s.Status)
	fmt.Printf("Workflow: %s\n", resolveWorkflow(root, s.Workflow))
	fmt.Printf("Stage:    %s\n", s.Stage.Name)
	fmt.Printf("Worker:   %s\n", s.Stage.Worker)

	interactive := isTTY()
	useResume := false

	switch s.Status {
	case "pending":
		// --dry must not mutate state; starting the ticket (pending → active +
		// a "started" history entry) is a real write, so skip it when previewing.
		if !nextDry {
			if err := state.Start(featureDir); err != nil {
				return err
			}
		}

	case "active":
		sessionActive := s.Runtime.Tmux != nil && tmux.Available() && tmux.SessionExists(s.Runtime.Tmux.Session)
		if sessionActive {
			fmt.Println()
			fmt.Printf("⚠ tmux session %q is already running.\n", s.Runtime.Tmux.Session)
			if interactive {
				ans := promptLine("  Attach to existing session? [Y/n]: ")
				ans = strings.ToLower(strings.TrimSpace(ans))
				if ans == "" || ans == "y" || ans == "yes" {
					return tmux.Attach(s.Runtime.Tmux.Session)
				}
				fmt.Println("Cancelled.")
				return nil
			}
			return tmux.Attach(s.Runtime.Tmux.Session)
		} else {
			fmt.Println()
			fmt.Println("⚠ Ticket is active but no session found — likely interrupted.")
			if interactive {
				ans := promptLine("  Launch with recovery context? [Y/n]: ")
				ans = strings.ToLower(strings.TrimSpace(ans))
				if ans == "" || ans == "y" || ans == "yes" {
					useResume = true
				}
			} else {
				useResume = true
			}
		}

	case "paused":
		reason := s.NextAction.Prompt
		if len(s.History) > 0 {
			reason = s.History[len(s.History)-1].Result
		}
		fmt.Println()
		fmt.Printf("⚠ Ticket is paused:\n  %s\n", reason)
		if interactive {
			ans := promptLine("  Launch with recovery context? [Y/n]: ")
			ans = strings.ToLower(strings.TrimSpace(ans))
			if ans != "" && ans != "y" && ans != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}
		useResume = true
	}
	fmt.Println()

	plan, err := runner.Compute(root, featureDir, nextWorker)
	if err != nil {
		return err
	}

	if useResume {
		ctx, err := resume.Build(root, featureDir)
		if err != nil {
			return fmt.Errorf("building resume prompt: %w", err)
		}
		plan.Prompt = ctx.Prompt
		fmt.Println("Using recovery context.")
		fmt.Println()
	}

	if nextDry {
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
		fmt.Printf("Override worker: orc next %s --worker <worker-id>\n", s.Ticket)
		return nil
	}

	return launchPlan(root, featureDir, s, plan)
}

func launchPlan(root, featureDir string, s *state.State, plan *runner.Plan) error {
	launcher := orchestrator.NewLauncher()
	result, err := launcher.Launch(orchestrator.LaunchOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
		Plan:       plan,
		In:         os.Stdin,
		Out:        os.Stdout,
		Err:        os.Stderr,
		OnFallback: func(message string) {
			if strings.HasPrefix(message, "warning:") {
				fmt.Println(message)
			} else {
				fmt.Printf("%s — running in foreground\n", message)
			}
		},
		OnHistoryWarning: func(message string) {
			fmt.Println(message)
		},
		OnTmuxSend: func(session, window string) {
			fmt.Printf("Sending to tmux session %s:%s...\n", session, window)
		},
		OnForeground: func() {
			fmt.Printf("Launching %s (%s)...\n", plan.Worker.Name, plan.Worker.Engine)
		},
	})
	if err != nil {
		return err
	}

	if result.Mode == orchestrator.LaunchModeTmux {
		fmt.Printf("Agent launched in background.\n")
		fmt.Printf("Attach:  %s\n", result.AttachHint)
	}
	return nil
}

func runWork(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	result, err := workspace.Work(workspace.WorkOptions{
		Root:     root,
		Ticket:   args[0],
		Slug:     workSlug,
		Workflow: workWorkflow,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Created: features/%s/\n\n", result.Slug)

	cfg, _ := config.Load(root)

	useTmux := workTmux
	if !useTmux && cfg != nil {
		useTmux = cfg.Settings.AutoTmux
	}
	if useTmux {
		if err := state.SetRuntime(result.FeatureDir, result.Slug); err != nil {
			fmt.Printf("warning: could not write tmux runtime to STATE.yaml: %v\n", err)
		}
	}

	plan, err := runner.Compute(root, result.FeatureDir, "")
	if err != nil {
		return err
	}

	useNext := workNext || (cfg != nil && cfg.Settings.AutoNext)
	if useNext {
		s, err := state.Load(result.FeatureDir)
		if err != nil {
			return err
		}
		return launchPlan(root, result.FeatureDir, s, plan)
	}

	printDryRun(plan, result.Slug)
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		t, err := ticket.Load(root, args[0])
		if err != nil {
			return err
		}
		if statusJSON {
			return printJSON(t.State)
		}
		if err := printShow(root, t.FeatureDir, t.State); err != nil {
			return err
		}
		if !validate.Run(root, t.FeatureDir).OK() {
			fmt.Printf("\n⚠  state has problems — run `orc doctor %s` for details\n", t.State.Ticket)
		}
		return nil
	}

	type row struct {
		ticket   string
		status   string
		workflow string
		worker   string
		next     string
		session  string
	}

	showTmux := tmux.Available()
	sessionNames := []string{}
	if showTmux {
		sessionNames = tmux.ListSessions()
	}

	statusCfg, _ := config.Load(root)
	features, err := featurelist.Collect(root, featurelist.Options{
		IncludeArchived: true,
		TmuxAvailable: func() bool {
			return showTmux
		},
		ListSessions: func() []string {
			return sessionNames
		},
	})
	if err != nil {
		return err
	}

	collectRows := func(archived bool) []row {
		var rows []row
		for _, f := range features {
			if f.Archived != archived {
				continue
			}
			if f.LoadError != nil {
				rows = append(rows, row{ticket: filepath.Base(f.FeatureDir), status: "error", next: f.LoadError.Error()})
				continue
			}
			s := f.State
			next := s.NextAction.Prompt
			if len(next) > 40 {
				next = next[:40] + "…"
			}
			session := "-"
			if s.Runtime.Tmux != nil {
				if f.TmuxLive {
					session = "✓"
				} else {
					session = "✗" // configured but not running
				}
			}
			rowPname := resolveWorkflow(root, s.Workflow)
			stageLabel := rowPname + " · " + s.Stage.Name + loopCountSuffix(statusCfg, rowPname, s.Stage.Name, s)
			if s.Runtime.JIT != nil {
				stageLabel += " + jit"
			}
			rows = append(rows, row{
				ticket:   s.Ticket,
				status:   s.Status,
				workflow: stageLabel,
				worker:   f.WorkerID,
				next:     next,
				session:  session,
			})
		}
		return rows
	}

	printTable := func(rows []row) {
		if showTmux {
			fmt.Printf("%-16s  %-16s  %-28s  %-20s  %-6s  %s\n", "Ticket", "Status", "Workflow", "Worker", "Tmux", "Next")
			fmt.Printf("%-16s  %-16s  %-28s  %-20s  %-6s  %s\n", "------", "------", "--------", "-----", "----", "----")
			for _, r := range rows {
				fmt.Printf("%-16s  %-16s  %-28s  %-20s  %-6s  %s\n", r.ticket, r.status, r.workflow, r.worker, r.session, r.next)
			}
		} else {
			fmt.Printf("%-16s  %-16s  %-28s  %-20s  %s\n", "Ticket", "Status", "Workflow", "Worker", "Next")
			fmt.Printf("%-16s  %-16s  %-28s  %-20s  %s\n", "------", "------", "--------", "-----", "----")
			for _, r := range rows {
				fmt.Printf("%-16s  %-16s  %-28s  %-20s  %s\n", r.ticket, r.status, r.workflow, r.worker, r.next)
			}
		}
	}

	if statusJSON {
		collectStates := func(archived bool) []*state.State {
			var out []*state.State
			for _, f := range features {
				if f.Archived != archived || f.LoadError != nil {
					continue
				}
				out = append(out, f.State)
			}
			return out
		}
		return printJSON(map[string]any{
			"active":   collectStates(false),
			"archived": collectStates(true),
		})
	}

	active := collectRows(false)
	archived := collectRows(true)

	if len(active) == 0 && len(archived) == 0 {
		fmt.Println("No features found. Start one with `orc work <ticket>`.")
		return nil
	}

	if len(active) > 0 {
		fmt.Printf("Active (%d)\n\n", len(active))
		printTable(active)
	}

	if len(archived) > 0 {
		if len(active) > 0 {
			fmt.Println()
		}
		fmt.Printf("Archived (%d)\n\n", len(archived))
		printTable(archived)
	}

	return nil
}

func runReport(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	now := time.Now()

	if len(args) == 1 {
		t, err := ticket.LoadWithArchive(root, args[0])
		if err != nil {
			return err
		}
		rep := report.Compute(t.State, now)
		if reportJSON {
			return printJSON(ticketReportJSON(rep, t.State.Stage.Name))
		}
		printTicketReport(rep, t.State.Stage.Name)
		return nil
	}

	features, err := featurelist.Collect(root, featurelist.Options{IncludeArchived: reportArchived})
	if err != nil {
		return err
	}
	var reports []report.Report
	for _, f := range features {
		if f.LoadError != nil {
			continue
		}
		reports = append(reports, report.Compute(f.State, now))
	}
	aggs := report.Aggregate(reports)
	if reportJSON {
		return printJSON(aggregateReportJSON(aggs, len(reports)))
	}
	printAggregateReport(aggs, len(reports))
	return nil
}

func printTicketReport(rep report.Report, currentStage string) {
	status := "complete"
	if rep.Open {
		status = "in progress"
	}
	fmt.Printf("%s · %s\n\n", rep.Ticket, status)
	if len(rep.Stages) == 0 {
		fmt.Println("No timing history yet.")
		return
	}
	fmt.Printf("%-18s  %-10s  %-10s  %-6s\n", "Stage", "Active", "Wall", "Visits")
	fmt.Printf("%-18s  %-10s  %-10s  %-6s\n", "-----", "------", "----", "------")
	for _, st := range rep.Stages {
		marker := ""
		if rep.Open && st.Stage == currentStage {
			marker = "  (current)"
		}
		fmt.Printf("%-18s  %-10s  %-10s  %-6d%s\n",
			st.Stage, report.Humanize(st.Active), report.Humanize(st.Wall), st.Visits, marker)
	}
	fmt.Printf("%-18s  %-10s  %-10s\n", "-----", "------", "----")
	fmt.Printf("%-18s  %-10s  %-10s\n", "Total", report.Humanize(rep.Active), report.Humanize(rep.Wall))
}

func printAggregateReport(aggs []report.StageAgg, tickets int) {
	if tickets == 0 {
		fmt.Println("No features found. Start one with `orc work <ticket>`.")
		return
	}
	fmt.Printf("Stage timing across %d ticket(s)\n\n", tickets)
	fmt.Printf("%-18s  %-8s  %-10s  %-10s  %-6s\n", "Stage", "Tickets", "Avg active", "Med active", "Visits")
	fmt.Printf("%-18s  %-8s  %-10s  %-10s  %-6s\n", "-----", "-------", "----------", "----------", "------")
	for _, a := range aggs {
		fmt.Printf("%-18s  %-8d  %-10s  %-10s  %-6d\n",
			a.Stage, a.Tickets, report.Humanize(a.AvgActive), report.Humanize(a.MedActive), a.Visits)
	}
}

// ── report JSON shapes (durations as whole seconds, plus a human string) ──

func ticketReportJSON(rep report.Report, currentStage string) map[string]any {
	stages := make([]map[string]any, 0, len(rep.Stages))
	for _, st := range rep.Stages {
		stages = append(stages, map[string]any{
			"stage":          st.Stage,
			"active_seconds": int64(st.Active.Seconds()),
			"wall_seconds":   int64(st.Wall.Seconds()),
			"active":         report.Humanize(st.Active),
			"wall":           report.Humanize(st.Wall),
			"visits":         st.Visits,
			"current":        rep.Open && st.Stage == currentStage,
		})
	}
	return map[string]any{
		"ticket":               rep.Ticket,
		"open":                 rep.Open,
		"stages":               stages,
		"total_active_seconds": int64(rep.Active.Seconds()),
		"total_wall_seconds":   int64(rep.Wall.Seconds()),
	}
}

func aggregateReportJSON(aggs []report.StageAgg, tickets int) map[string]any {
	stages := make([]map[string]any, 0, len(aggs))
	for _, a := range aggs {
		stages = append(stages, map[string]any{
			"stage":              a.Stage,
			"tickets":            a.Tickets,
			"avg_active_seconds": int64(a.AvgActive.Seconds()),
			"med_active_seconds": int64(a.MedActive.Seconds()),
			"avg_active":         report.Humanize(a.AvgActive),
			"med_active":         report.Humanize(a.MedActive),
			"visits":             a.Visits,
		})
	}
	return map[string]any{
		"tickets": tickets,
		"stages":  stages,
	}
}

// loopCountSuffix returns " (N/M)" when stageName is an active loop stage with a max defined.
func loopCountSuffix(cfg *config.Config, workflow, stageName string, s *state.State) string {
	if cfg == nil || !cfg.IsLoopStage(workflow, stageName) {
		return ""
	}
	owner, ok := cfg.OwnerStage(workflow, stageName)
	if !ok {
		return ""
	}
	loopDef, ok := cfg.LoopConfig(workflow, owner)
	if !ok || loopDef.Max <= 0 {
		return ""
	}
	count := s.StageCounts[stageName]
	if count == 0 {
		return ""
	}
	return fmt.Sprintf(" (%d/%d)", count, loopDef.Max)
}

func printShow(root, featureDir string, s *state.State) error {
	summary := ticketview.Build(root, featureDir, s, ticketview.Options{})

	fmt.Printf("Ticket:   %s\n", s.Ticket)
	fmt.Printf("Slug:     %s\n", s.Slug)
	fmt.Printf("Status:   %s\n", s.Status)
	if summary.TmuxConfigured {
		if summary.TmuxLive {
			fmt.Printf("Session:  %s\n", summary.TmuxAttachHint)
		} else {
			fmt.Printf("Session:  %s  (not running — %s)\n", summary.TmuxSession, summary.TmuxRestart)
		}
	}
	if summary.JIT != nil {
		fmt.Println()
		fmt.Println("JIT")
		fmt.Printf("  Worker:   %s\n", summary.JIT.Worker)
		fmt.Printf("  Task:     %s\n", summary.JIT.Task)
		fmt.Printf("  Started:  %s\n", summary.JIT.StartedAt)
	}

	fmt.Println()
	fmt.Println("Stage")
	fmt.Printf("  Stage:     %s · %s%s\n", summary.Workflow, summary.Stage, summary.StageLoopLabel)
	fmt.Printf("  Worker:    %s\n", summary.WorkerID)
	if summary.NextStage != "" {
		fmt.Printf("  Next:      %s  (%s)\n", summary.NextStage, summary.NextAdvance)
	}

	if len(s.Repos) > 0 {
		fmt.Println()
		fmt.Println("Repos")
		for name, r := range s.Repos {
			fmt.Printf("  %s\n", name)
			if r.Main != "" {
				fmt.Printf("    main:     %s\n", r.Main)
			}
			if r.Worktree != "" {
				fmt.Printf("    worktree: %s\n", r.Worktree)
				fmt.Printf("    branch:   %s\n", r.Branch)
			}
		}
	}

	if len(s.Inputs.Ready)+len(s.Inputs.Required)+len(s.Inputs.Completed) > 0 {
		fmt.Println()
		fmt.Println("Inputs")
		for _, f := range s.Inputs.Ready {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Inputs.Required {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Inputs.Completed {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
	}

	if len(s.Outputs.Ready)+len(s.Outputs.Required)+len(s.Outputs.Completed) > 0 {
		fmt.Println()
		fmt.Println("Outputs")
		for _, f := range s.Outputs.Ready {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Outputs.Required {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
		for _, f := range s.Outputs.Completed {
			fmt.Printf("  %s  %s\n", fileCheck(featureDir, f), f)
		}
	}

	fmt.Println()
	fmt.Println("Next")
	switch s.Status {
	case "paused":
		fmt.Printf("  Paused:  %s\n", summary.PausedReason)
		fmt.Println("  Run `orc next` after resolving to continue.")
	default:
		if summary.WorkerID != "" {
			if summary.WorkerFound {
				fmt.Printf("  Worker:  %s (%s)\n", summary.WorkerName, summary.WorkerEngine)
				if summary.WorkerModel != "" {
					fmt.Printf("  Model:   %s\n", summary.WorkerModel)
				}
			} else {
				fmt.Printf("  Worker:  %s (not found in workers/)\n", summary.WorkerID)
			}
		} else {
			fmt.Println("  Worker:  none assigned — set worker: in orc.yaml")
		}
		fmt.Println("  Run `orc next` to launch.")
	}

	if len(s.History) > 0 {
		fmt.Println()
		fmt.Println("History")
		for _, h := range s.History {
			ts := h.At
			if t, err := time.Parse(time.RFC3339, h.At); err == nil {
				ts = t.Format("2006-01-02 15:04")
			}
			fmt.Printf("  %-16s  %-20s  %-20s  %s\n", ts, h.Stage, h.Worker, h.Result)
		}
	}

	return nil
}

func fileCheck(featureDir, relPath string) string {
	if _, err := os.Stat(filepath.Join(featureDir, relPath)); err == nil {
		return "✓"
	}
	return "✗"
}

func runMark(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	ticketArg := args[0]
	action := strings.ToLower(args[1])

	// jit can target any status including archived; all other actions use features/ only.
	var t *ticket.Ticket
	if action == "jit" {
		t, err = ticket.LoadWithArchive(root, ticketArg)
	} else {
		t, err = ticket.Load(root, ticketArg)
	}
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State

	if action == "pause" && len(args) < 3 {
		return fmt.Errorf("orc mark %s pause requires a reason — e.g. orc mark %s pause \"<reason>\"", ticketArg, ticketArg)
	}

	switch action {
	case "start":
		if !oneOf(s.Status, "pending", "ready") {
			return fmt.Errorf("cannot mark %s start from status %q — use `orc mark %s resume` to continue a paused ticket", s.Ticket, s.Status, s.Ticket)
		}
		if err := state.ValidateRepos(s, root); err != nil {
			return err
		}
		if err := state.Start(featureDir); err != nil {
			return err
		}
		fmt.Printf("Ticket:  %s\n", s.Ticket)
		fmt.Printf("Status:  active\n")
		return nil

	case "resume":
		if s.Status != "paused" {
			return fmt.Errorf("cannot mark %s resume from status %q — resume is only valid from paused", s.Ticket, s.Status)
		}
		if err := state.ValidateRepos(s, root); err != nil {
			return err
		}
		if err := state.Resume(featureDir); err != nil {
			return err
		}
		fmt.Printf("Ticket:  %s\n", s.Ticket)
		fmt.Printf("Status:  active\n")
		return nil

	case "pause":
		if oneOf(s.Status, "done", "archived") {
			return fmt.Errorf("cannot pause %s from status %q", s.Ticket, s.Status)
		}
		if err := state.ValidateRepos(s, root); err != nil {
			return err
		}
		reason := strings.Join(args[2:], " ")
		if err := state.Pause(featureDir, reason); err != nil {
			return err
		}
		fmt.Printf("Ticket:  %s\n", s.Ticket)
		fmt.Printf("Status:  paused\n")
		fmt.Printf("Reason:  %s\n", reason)
		fmt.Printf("\nRun `orc next %s` to resume once resolved.\n", s.Ticket)
		return nil

	case "next":
		return runMarkNext(root, featureDir)

	case "done":
		if !oneOf(s.Status, "active", "ready", "paused") {
			return fmt.Errorf("cannot mark %s done from status %q", s.Ticket, s.Status)
		}
		result := markResult
		if result == "" {
			result = "done"
		}
		if err := state.Done(featureDir, result); err != nil {
			return err
		}
		fmt.Printf("Ticket:  %s\n", s.Ticket)
		fmt.Printf("Status:  done\n")
		return nil

	case "jit":
		if len(args) < 3 {
			return fmt.Errorf("orc mark %s jit requires a summary", ticketArg)
		}
		summary := strings.Join(args[2:], " ")
		workerID := ""
		if s.Runtime.JIT != nil {
			workerID = s.Runtime.JIT.Worker
		}
		if err := state.AppendHistory(featureDir, "jit", workerID, summary); err != nil {
			return err
		}
		if err := state.ClearJIT(featureDir); err != nil {
			return err
		}
		fmt.Printf("Done: jit task recorded for %s\n", s.Ticket)
		return nil

	default:
		return fmt.Errorf("unknown action %q — use: start | resume | next [--result] [--stage] [--worker] | pause <reason> | done [--result] | jit <summary>", action)
	}
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func runMarkNext(root, featureDir string) error {
	result, err := orchestrator.Advance(orchestrator.AdvanceOptions{
		Root:       root,
		FeatureDir: featureDir,
		Stage:      markStage,
		Worker:     markWorker,
		Result:     markResult,
	})
	if err != nil {
		return err
	}

	if result.Outcome == orchestrator.AdvanceOutcomePaused {
		fmt.Printf("Loop limit reached %s. Pausing for human review.\n", strings.TrimPrefix(result.Reason, "loop limit reached "))
		return nil
	}

	fmt.Printf("Ticket:   %s\n", result.Ticket)
	if result.Next != "" && result.Next != result.Previous {
		fmt.Printf("Stage:    %s → %s\n", result.Previous, result.Next)
	} else if result.Next == "" {
		fmt.Printf("Stage:    %s  (final)\n", result.Previous)
		fmt.Printf("Status:   done\n")
	} else {
		fmt.Printf("Stage:    %s  (unchanged)\n", result.Previous)
	}
	if result.Worker != "" {
		fmt.Printf("Worker:   %s\n", result.Worker)
	}

	if result.Outcome == orchestrator.AdvanceOutcomeDone {
		if result.AutoArchive {
			fmt.Println()
			s, err := state.Load(featureDir)
			if err == nil {
				return archiveFeature(root, featureDir, s)
			}
			return err
		}
		return nil
	}

	fmt.Printf("\nRun `orc next %s` to launch the next worker.\n", result.Ticket)
	fmt.Println()

	plan, err := runner.Compute(root, featureDir, "")
	if err != nil {
		return err
	}
	printDryRun(plan, result.Ticket)
	return nil
}

func runArchive(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}

	return archiveFeature(root, t.FeatureDir, t.State)
}

func archiveFeature(root, featureDir string, s *state.State) error {
	result, err := orchestrator.Archive(orchestrator.ArchiveOptions{
		Root:       root,
		FeatureDir: featureDir,
		State:      s,
	})
	if err != nil {
		return err
	}

	for _, wt := range result.Worktrees {
		fmt.Printf("Removing worktree: %s\n", wt.WorktreeRel)
		if wt.Warning != "" {
			fmt.Printf("  warning: %v\n", wt.Warning)
			fmt.Printf("  you may need to run: git -C %q worktree remove %q --force\n", wt.Main, wt.WorktreePath)
		} else {
			fmt.Printf("  removed %s (%s)\n", wt.Name, wt.Branch)
		}
	}

	if result.TmuxKillWarn != "" {
		fmt.Printf("warning: %s\n", result.TmuxKillWarn)
	} else if result.KilledTmux {
		fmt.Printf("Killed tmux session: %s\n", result.TmuxSession)
	}
	if result.RuntimeClearWarn != "" {
		fmt.Printf("warning: %s\n", result.RuntimeClearWarn)
	}

	fmt.Printf("Archived: features/_archive/%s/\n", result.Slug)
	return nil
}

func runDelete(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	t, err := ticket.LoadWithArchive(root, args[0])
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State

	if s.Status != "done" && s.Status != "archived" {
		return fmt.Errorf("cannot delete %q: status is %q (must be done or archived)", s.Slug, s.Status)
	}

	rel, _ := filepath.Rel(root, featureDir)
	if isTTY() {
		ans := promptLine(fmt.Sprintf("Permanently delete %s? [y/N]: ", rel))
		ans = strings.ToLower(strings.TrimSpace(ans))
		if ans != "y" && ans != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if err := os.RemoveAll(featureDir); err != nil {
		return fmt.Errorf("deleting feature folder: %w", err)
	}

	fmt.Printf("Deleted: %s/\n", rel)
	return nil
}

func runJIT(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}

	ticketArg := args[0]
	instruction := args[1]

	t, err := ticket.LoadWithArchive(root, ticketArg)
	if err != nil {
		return err
	}
	featureDir := t.FeatureDir
	s := t.State

	if s.Runtime.JIT != nil && !jitDry {
		return fmt.Errorf("jit task already running for %s (worker: %s, started: %s)\nRun `orc mark %s jit \"<summary>\"` to close it first",
			s.Ticket, s.Runtime.JIT.Worker, s.Runtime.JIT.StartedAt, s.Ticket)
	}

	ctx, err := workspacectx.Load(root)
	if err != nil {
		return err
	}
	w := workers.FindByID(ctx.Workers, jitWorker)
	if w == nil {
		return fmt.Errorf("worker %q not found in workers/", jitWorker)
	}

	timestamp := time.Now().Format("20060102-150405")
	outputDir := filepath.Join(featureDir, "jit", timestamp)
	prompt := buildJITPrompt(s, instruction, outputDir)
	launchArgv := workers.LaunchArgs(w, root, featureDir, prompt)
	launchCommand := workers.LaunchCommand(w, root, featureDir, prompt)

	if jitDry {
		fmt.Printf("Worker:  %s (%s)\n", w.Name, w.Engine)
		if w.Model != "" {
			fmt.Printf("Model:   %s\n", w.Model)
		}
		fmt.Printf("Output:  jit/%s/\n", timestamp)
		fmt.Println()
		fmt.Println("Would run:")
		fmt.Printf("  %s\n", launchCommand)
		fmt.Println()
		fmt.Println("Prompt:")
		fmt.Println(prompt)
		return nil
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating jit output dir: %w", err)
	}
	if err := state.SetJIT(featureDir, jitWorker, instruction); err != nil {
		return fmt.Errorf("writing runtime.jit: %w", err)
	}

	fmt.Printf("Ticket:  %s\n", s.Ticket)
	fmt.Printf("Worker:  %s (%s)\n", w.Name, w.Engine)
	fmt.Printf("Output:  jit/%s/\n", timestamp)
	fmt.Println()

	plan := &runner.Plan{
		Ticket:        s.Ticket,
		Stage:         "jit",
		Worker:        w,
		Prompt:        prompt,
		LaunchCommand: launchCommand,
		LaunchArgv:    launchArgv,
		CWD:           featureDir,
	}
	launcher := orchestrator.NewLauncher()
	result, err := launcher.Launch(orchestrator.LaunchOptions{
		Root:                root,
		FeatureDir:          featureDir,
		State:               s,
		Plan:                plan,
		Window:              "jit",
		In:                  os.Stdin,
		Out:                 os.Stdout,
		Err:                 os.Stderr,
		DisableTmux:         !jitTmux,
		RequireExistingTmux: true,
		OnFallback: func(message string) {
			fmt.Printf("%s — running in foreground\n", message)
		},
		OnHistoryWarning: func(message string) {
			fmt.Println(message)
		},
		OnForeground: func() {
			fmt.Printf("Launching %s (%s)...\n", w.Name, w.Engine)
		},
	})
	if err != nil {
		return err
	}
	if result.Mode == orchestrator.LaunchModeTmux {
		fmt.Printf("Agent launched in tmux session %s:%s\n", result.Session, result.Window)
		fmt.Printf("Attach:  %s\n", result.AttachHint)
	}
	return nil
}

func buildJITPrompt(s *state.State, instruction, outputDir string) string {
	return fmt.Sprintf(`Before starting: read AGENTS.md and ORC.md.

## JIT task: %s

%s

## Context

Start in features/%s/ and orient yourself by reading:
- STATE.yaml — current state and history
- TICKET.md — original ticket
- SPEC.md — scope and requirements (if present)
- DECISIONS.md — decisions made so far (if present)

Current pipeline stage: %s (do not advance — this is a one-off task outside the pipeline)

Write any output or notes to %s

When you are done, run:
  orc mark %s jit "<summary of what you did>"`,
		s.Ticket, instruction, s.Slug, s.Stage.Name, outputDir, s.Ticket)
}

func runTui(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	return tui.Run(root)
}

func runAttach(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot(globalWorkspace)
	if err != nil {
		return err
	}
	if !tmux.Available() {
		return fmt.Errorf("tmux is not installed or not in PATH")
	}

	t, err := ticket.Load(root, args[0])
	if err != nil {
		return err
	}
	s := t.State

	session := s.Slug
	if s.Runtime.Tmux != nil && s.Runtime.Tmux.Session != "" {
		session = s.Runtime.Tmux.Session
	}

	if !tmux.SessionExists(session) {
		return fmt.Errorf("no tmux session for %s — run `orc next %s` to start one", s.Ticket, s.Ticket)
	}

	return tmux.Attach(session + ":" + s.Stage.Name)
}

// resolveWorkflow returns the ticket's workflow name for display purposes.
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
