package watch

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/searchmatch"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	terminalui "github.com/cengebretson/orc/internal/ui"
	"github.com/cengebretson/orc/internal/workspacesnapshot"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultInterval = 5 * time.Second
const watchAnimationInterval = 500 * time.Millisecond

type tickMsg struct {
	at    time.Time
	epoch uint64
}

type watchAnimationMsg struct {
	at    time.Time
	epoch uint64
}

type dataMsg struct {
	rows []row
	err  error
}

type attachDoneMsg struct {
	err error
}

type Options struct {
	Ticket    string
	Interval  time.Duration
	Wide      bool
	Mode      Mode
	PetLayout PetLayout
	Demo      bool
}

type workflowStep struct {
	name    string
	label   string
	advance string
}

type row struct {
	ticket          string
	name            string
	stage           string
	stageName       string
	workflow        string
	workflowLabel   string
	workflowSteps   []workflowStep
	worker          string
	status          string
	next            string
	session         string
	window          string
	pane            string
	tmuxState       string
	attention       string
	context         contextpressure.Pressure
	room            string
	branch          string
	engine          string
	model           string
	providerID      string
	liveState       string
	lastActive      time.Time
	contextTrend    []uint64
	flashUntil      time.Time
	celebrateUntil  time.Time
	demoCelebration bool
	history         []historyRow
	loadErr         error
	search          []string
}

type historyRow struct {
	at     string
	stage  string
	worker string
	result string
}

type Model struct {
	root     string
	ticket   string
	interval time.Duration
	wide     bool
	mode     Mode
	demo     bool
	inactive bool
	epoch    uint64
	help     bool
	uiFrame  int
	now      time.Time

	allRows    []row
	rows       []row
	cursor     int
	width      int
	height     int
	lastLoad   time.Time
	loadErr    error
	message    string
	petFrame   int
	petTicking bool
	petLayout  PetLayout

	preview   bool
	viewport  viewport.Model
	searching bool
	searchBox textinput.Model
}

// New constructs the watch model without starting a Bubble Tea program so it
// can run either standalone or as the Live section of the shared dashboard.
func New(root string, opts Options) (Model, error) {
	interval := opts.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	mode, err := ParseMode(string(opts.Mode))
	if err != nil {
		return Model{}, err
	}
	petLayout, err := ParsePetLayout(string(opts.PetLayout))
	if err != nil {
		return Model{}, err
	}
	searchBox := textinput.New()
	searchBox.Placeholder = "filter sessions..."
	searchBox.Prompt = "/ "
	searchBox.CharLimit = 96
	return Model{
		root:       root,
		ticket:     opts.Ticket,
		interval:   interval,
		wide:       opts.Wide,
		mode:       mode,
		demo:       opts.Demo,
		petLayout:  petLayout,
		petTicking: mode == ModePet,
		searchBox:  searchBox,
	}, nil
}

// CanSwitchSection reports whether dashboard-level navigation can safely
// consume a section-switch or help key without stealing search input.
func (m Model) CanSwitchSection() bool {
	return !m.searching
}

// SetActive controls background refresh and animation work while watch is
// embedded in the shared dashboard.
func (m Model) SetActive(active bool) Model {
	if active == !m.inactive {
		return m
	}
	m.epoch++
	m.inactive = !active
	if !active {
		m.petTicking = false
	}
	return m
}

func (m Model) IsActive() bool {
	return !m.inactive
}

func (m Model) Init() tea.Cmd {
	if m.inactive {
		return nil
	}
	commands := []tea.Cmd{loadData(m.root, m.ticket, m.demo), tickEvery(m.interval, m.epoch), watchAnimationTick(m.epoch)}
	if m.mode == ModePet {
		commands = append(commands, petTick(m.epoch))
	}
	return tea.Batch(commands...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = max(1, msg.Width-2)
		m.viewport.Height = max(1, msg.Height)
		m.searchBox.Width = max(8, msg.Width-6)
		m.refreshPreview()
		return m, nil
	case tickMsg:
		if m.inactive || msg.epoch != m.epoch {
			return m, nil
		}
		return m, tea.Batch(loadData(m.root, m.ticket, m.demo), tickEvery(m.interval, m.epoch))
	case watchAnimationMsg:
		if m.inactive || msg.epoch != m.epoch {
			return m, nil
		}
		m.uiFrame++
		m.now = msg.at
		return m, watchAnimationTick(m.epoch)
	case petTickMsg:
		if m.inactive || msg.epoch != m.epoch || m.mode != ModePet {
			m.petTicking = false
			return m, nil
		}
		m.petFrame++
		m.petTicking = true
		return m, petTick(m.epoch)
	case dataMsg:
		now := time.Now()
		selectedTicket := ""
		if selected, ok := m.selectedWork(); ok {
			selectedTicket = selected.ticket
		}
		m.allRows = mergeLiveVisuals(m.allRows, msg.rows, now)
		m.applyFilter(false)
		m.loadErr = msg.err
		m.lastLoad = now
		m.now = now
		if selectedTicket != "" {
			for i := range m.rows {
				if m.rows[i].ticket == selectedTicket {
					m.cursor = i
					break
				}
			}
		}
		if m.cursor >= m.itemCount() {
			m.cursor = max(0, m.itemCount()-1)
		}
		m.refreshPreview()
		return m, nil
	case attachDoneMsg:
		if msg.err != nil {
			m.message = "attach failed: " + msg.err.Error()
		}
		return m, nil
	case tea.KeyMsg:
		if m.help {
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "?", "esc":
				m.help = false
				return m, nil
			default:
				return m, nil
			}
		}
		if m.searching {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.searching = false
				m.searchBox.Blur()
				m.searchBox.SetValue("")
				m.applyFilter(true)
				return m, nil
			case "enter":
				m.searching = false
				m.searchBox.Blur()
				m.cursor = 0
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchBox, cmd = m.searchBox.Update(msg)
				m.applyFilter(true)
				return m, cmd
			}
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.preview {
				m.preview = false
				return m, nil
			}
			if m.searchBox.Value() != "" {
				m.searchBox.SetValue("")
				m.applyFilter(true)
				return m, nil
			}
			return m, tea.Quit
		case "/":
			if !m.preview {
				if m.allRows == nil {
					m.allRows = append([]row(nil), m.rows...)
				}
				m.searching = true
				m.searchBox.Focus()
				return m, textinput.Blink
			}
		case "?":
			m.help = true
			return m, nil
		case "v":
			if m.preview {
				return m, nil
			}
			if m.mode == ModePet {
				m.mode = ModeRail
				return m, nil
			}
			m.mode = ModePet
			if !m.petTicking {
				m.petTicking = true
				return m, petTick(m.epoch)
			}
			return m, nil
		case "l":
			if m.preview || m.mode != ModePet {
				return m, nil
			}
			if normalizePetLayout(m.petLayout) == PetLayoutColumn {
				m.petLayout = PetLayoutResponsive
			} else {
				m.petLayout = PetLayoutColumn
			}
			return m, nil
		case "j", "down":
			if m.preview {
				m.viewport.ScrollDown(1)
			} else if m.cursor < m.itemCount()-1 {
				m.cursor++
			}
			return m, nil
		case "k", "up":
			if m.preview {
				m.viewport.ScrollUp(1)
			} else if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "pgdown", "ctrl+d":
			if m.preview {
				m.viewport.HalfPageDown()
			}
			return m, nil
		case "pgup", "ctrl+u":
			if m.preview {
				m.viewport.HalfPageUp()
			}
			return m, nil
		case "g", "home":
			if m.preview {
				m.viewport.GotoTop()
			}
			return m, nil
		case "G", "end":
			if m.preview {
				m.viewport.GotoBottom()
			}
			return m, nil
		case "enter", "n":
			m.preview = !m.preview
			m.refreshPreview()
			return m, nil
		case "r":
			m.message = ""
			return m, loadData(m.root, m.ticket, m.demo)
		case "a":
			cmd, message := m.attachSelected()
			m.message = message
			return m, cmd
		case "i":
			cmd, message := m.focusNext()
			m.message = message
			return m, cmd
		}
		if m.preview {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return ""
	}
	if m.help {
		return m.renderHelp()
	}
	if m.preview {
		return m.renderPreview()
	}
	if m.mode == ModePet {
		return m.renderPets()
	}
	if m.wide || m.width >= 56 {
		return m.renderWide()
	}
	return m.renderRail()
}

func loadData(root, ticket string, demo bool) tea.Cmd {
	return func() tea.Msg {
		if demo {
			return dataMsg{rows: demoRows(ticket)}
		}
		rows, err := collectRows(root, ticket)
		return dataMsg{rows: rows, err: err}
	}
}

func collectRows(root, ticket string) ([]row, error) {
	snapshot, err := workspacesnapshot.Load(root)
	if err != nil {
		return nil, err
	}
	thresholds := snapshot.Config.ContextPressureThresholds()
	rows := make([]row, 0, len(snapshot.Features))
	for _, f := range snapshot.Features {
		if f.Archived {
			continue
		}
		r := rowFromFeature(f, snapshot.Config)
		if live, ok := snapshot.Telemetry[filepath.Clean(f.FeatureDir)]; ok {
			r.context = contextpressure.Evaluate(live.ContextUsed, live.ContextLimit, thresholds)
			r.providerID = live.ProviderSessionID
			r.liveState = live.State
			r.model = live.Model
			r.lastActive = live.LastActive
			if live.Engine != "" {
				r.engine = live.Engine
			}
		}
		if ticket != "" && !strings.EqualFold(r.ticket, ticket) {
			continue
		}
		rows = append(rows, r)
	}
	sortRows(rows)
	return rows, nil
}

func sortRows(rows []row) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rowPriority(rows[i]), rowPriority(rows[j])
		if left != right {
			return left < right
		}
		return rows[i].ticket < rows[j].ticket
	})
}

func mergeLiveVisuals(previous, next []row, now time.Time) []row {
	byTicket := make(map[string]row, len(previous))
	for _, r := range previous {
		byTicket[r.ticket] = r
	}
	for i := range next {
		current := &next[i]
		old, ok := byTicket[current.ticket]
		if ok {
			current.contextTrend = append([]uint64(nil), old.contextTrend...)
			current.flashUntil = old.flashUntil
			current.celebrateUntil = old.celebrateUntil
			_, oldState := displayState(old)
			_, newState := displayState(*current)
			if oldState != newState {
				current.flashUntil = now.Add(2 * time.Second)
				if newState == "done" {
					current.celebrateUntil = now.Add(4 * time.Second)
				}
			}
		}
		if current.context.Observed && current.context.Available {
			current.contextTrend = appendContextSample(current.contextTrend, current.context.Percent, 10)
		}
	}
	return next
}

func appendContextSample(samples []uint64, value uint64, limit int) []uint64 {
	samples = append(samples, value)
	if len(samples) > limit {
		samples = append([]uint64(nil), samples[len(samples)-limit:]...)
	}
	return samples
}

func demoRows(ticket string) []row {
	now := time.Now().UTC()
	steps := []workflowStep{
		{name: "intake", label: "intake", advance: "auto"},
		{name: "develop", label: "develop", advance: "auto"},
		{name: "review", label: "review", advance: "manual"},
		{name: "ship", label: "ship", advance: "manual"},
	}
	thresholds := contextpressure.DefaultThresholds()
	rows := []row{
		{
			ticket: "ORC-DEMO-1", name: "live-dashboard", stage: "develop", stageName: "develop", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "builder", status: "active", next: "Finish the responsive watch layout and hand off for review.", session: "orc-demo-1", window: "develop", pane: "%21", tmuxState: "live",
			context: contextpressure.Evaluate(44, 100, thresholds), contextTrend: []uint64{12, 18, 27, 35, 44}, room: "api/feature-orc-demo-1", branch: "feature/orc-demo-1", engine: "codex", model: "gpt-5", liveState: "working", lastActive: now.Add(-35 * time.Second),
			history: []historyRow{{at: now.Add(-48 * time.Minute).Format(time.RFC3339), stage: "intake", worker: "planner", result: "Implementation plan approved."}, {at: now.Add(-31 * time.Minute).Format(time.RFC3339), stage: "develop", worker: "builder", result: "Started implementation."}},
		},
		{
			ticket: "ORC-DEMO-2", name: "review-input", stage: "review", stageName: "review", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "reviewer", status: "paused", next: "Choose whether the new watch demo mode should be documented as a user feature.", session: "orc-demo-2", window: "review", pane: "%22", tmuxState: "live", attention: "input",
			context: contextpressure.Evaluate(76, 100, thresholds), contextTrend: []uint64{42, 51, 61, 69, 76}, room: "web/feature-orc-demo-2", branch: "feature/orc-demo-2", engine: "claude", model: "opus", liveState: "waiting", lastActive: now.Add(-4 * time.Minute),
			history: []historyRow{{at: now.Add(-2 * time.Hour).Format(time.RFC3339), stage: "develop", worker: "builder", result: "Completed the first implementation pass."}, {at: now.Add(-18 * time.Minute).Format(time.RFC3339), stage: "review", worker: "reviewer", result: "Requested a product decision."}},
		},
		{
			ticket: "ORC-DEMO-3", name: "stopped-session", stage: "review", stageName: "review", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "qa", status: "active", next: "Resume the stopped tmux session before continuing QA.", tmuxState: "stopped",
			context: contextpressure.Evaluate(94, 100, thresholds), contextTrend: []uint64{63, 72, 81, 88, 94}, room: "cli/feature-orc-demo-3", branch: "feature/orc-demo-3", engine: "codex", model: "gpt-5", liveState: "stopped", lastActive: now.Add(-27 * time.Minute),
		},
		{
			ticket: "ORC-DEMO-4", name: "completed-release", stage: "ship", stageName: "ship", workflow: "default:standard", workflowLabel: "standard", workflowSteps: steps,
			worker: "release", status: "done", next: "", tmuxState: "stopped", room: "docs/feature-orc-demo-4", branch: "feature/orc-demo-4", lastActive: now.Add(-2 * time.Minute), demoCelebration: true,
			history: []historyRow{{at: now.Add(-2 * time.Minute).Format(time.RFC3339), stage: "ship", worker: "release", result: "Released successfully."}},
		},
	}
	filtered := make([]row, 0, len(rows))
	for i := range rows {
		rows[i].search = []string{rows[i].ticket, rows[i].name, rows[i].stage, rows[i].worker, rows[i].status, rows[i].room, rows[i].engine}
		if ticket == "" || strings.EqualFold(ticket, rows[i].ticket) {
			filtered = append(filtered, rows[i])
		}
	}
	sortRows(filtered)
	return filtered
}

func rowFromFeature(f *featurelist.Feature, cfg *config.Config) row {
	if f.LoadError != nil || f.State == nil {
		ticket := filepath.Base(f.FeatureDir)
		return row{
			ticket:  ticket,
			name:    ticket,
			status:  "error",
			loadErr: f.LoadError,
			search:  []string{ticket, f.FeatureDir, "error"},
		}
	}
	s := f.State
	workflowID := s.Workflow
	if workflowID == "" {
		workflowID = f.Workflow
	}
	workflowLabel := workflowID
	stageLabel := s.Stage.Name
	var workflowSteps []workflowStep
	if cfg != nil {
		workflowID = cfg.ResolveWorkflow(workflowID)
		workflowLabel = cfg.WorkflowDisplayName(workflowID)
		stageLabel = stageDisplayLabel(cfg, s.Stage.Name)
		for _, stage := range cfg.Stages(workflowID) {
			workflowSteps = append(workflowSteps, workflowStep{name: stage.Name, label: cfg.StageDisplayName(stage.Name), advance: stage.Advance})
			if stage.Loop != nil && stage.Loop.Via == s.Stage.Name {
				workflowSteps = append(workflowSteps, workflowStep{name: stage.Loop.Via, label: "↺ " + cfg.StageDisplayName(stage.Loop.Via), advance: "loop"})
			}
		}
	}
	room, branch := featureRoom(s)
	worker := f.WorkerName
	if worker == "" {
		worker = f.WorkerID
	}
	if worker == "" {
		worker = s.Stage.Worker
	}
	searchFields := []string{
		s.Ticket, s.Slug, s.Status, s.Workflow, s.Stage.Name, stageLabel, s.Stage.Worker,
		f.Workflow, f.WorkerID, f.WorkerName, f.Engine, f.Attention,
	}
	for name, repo := range s.Repos {
		searchFields = append(searchFields, name, repo.Main, repo.Worktree, repo.Branch)
	}
	return row{
		ticket:        s.Ticket,
		name:          s.Slug,
		stage:         stageLabel + f.StageLoopLabel,
		stageName:     s.Stage.Name,
		workflow:      workflowID,
		workflowLabel: workflowLabel,
		workflowSteps: workflowSteps,
		worker:        shortWorker(worker),
		status:        s.Status,
		next:          s.NextAction.Prompt,
		session:       tmuxSession(s),
		window:        s.Stage.Name,
		pane:          tmuxPane(s),
		tmuxState:     tmuxState(s, f.TmuxLive),
		attention:     f.Attention,
		room:          room,
		branch:        branch,
		engine:        f.Engine,
		history:       historyRows(s.History),
		search:        searchFields,
	}
}

func stageDisplayLabel(cfg *config.Config, stage string) string {
	if cfg == nil {
		return stage
	}
	return cfg.StageDisplayName(cfg.ResolveStage(stage))
}

func tmuxSession(s *state.State) string {
	if s.Runtime.Tmux == nil {
		return ""
	}
	return s.Runtime.Tmux.Session
}

func tmuxPane(s *state.State) string {
	if s.Runtime.Tmux == nil {
		return ""
	}
	return s.Runtime.Tmux.Pane
}

func historyRows(entries []state.HistoryEntry) []historyRow {
	rows := make([]historyRow, 0, len(entries))
	for _, h := range entries {
		rows = append(rows, historyRow{
			at:     h.At,
			stage:  h.Stage,
			worker: shortWorker(h.Worker),
			result: h.Result,
		})
	}
	return rows
}

func tmuxState(s *state.State, live bool) string {
	if s.Runtime.Tmux == nil {
		return "-"
	}
	if live {
		return "live"
	}
	return "stopped"
}

func displayState(r row) (string, string) {
	if r.loadErr != nil {
		return "!", "error"
	}
	switch r.status {
	case "paused":
		return "!", "blocked"
	case "done":
		return "✓", "done"
	case "active":
		if r.tmuxState == "stopped" {
			return "x", "stopped"
		}
		switch r.attention {
		case tmux.AttentionInput:
			return "!", "input"
		case tmux.AttentionBlocked:
			return "!", "blocked"
		case tmux.AttentionReview:
			return "◆", "review"
		case tmux.AttentionDone:
			return "✓", "done"
		}
		return "●", "active"
	case "ready":
		return "▶", "ready"
	case "pending":
		return "○", "pending"
	default:
		return "?", r.status
	}
}

func rowPriority(r row) int {
	_, label := displayState(r)
	switch label {
	case "error":
		return 0
	case "blocked":
		return 1
	case "input":
		return 2
	case "review":
		return 3
	case "stopped":
		return 4
	case "ready":
		return 5
	case "pending":
		return 6
	case "active":
		return 7
	case "done":
		return 8
	default:
		return 9
	}
}

func attentionNeeded(r row) bool {
	_, label := displayState(r)
	return label == "blocked" || label == "input" || label == "review"
}

func shortWorker(worker string) string {
	worker = strings.TrimSpace(worker)
	worker = strings.TrimSuffix(worker, "-developer")
	worker = strings.TrimSuffix(worker, "-reviewer")
	worker = strings.TrimSuffix(worker, "-qa")
	if worker == "" {
		return "-"
	}
	return worker
}

func tickEvery(d time.Duration, epoch uint64) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg{at: t, epoch: epoch}
	})
}

func watchAnimationTick(epoch uint64) tea.Cmd {
	return tea.Tick(watchAnimationInterval, func(t time.Time) tea.Msg {
		return watchAnimationMsg{at: t, epoch: epoch}
	})
}

func (m Model) attachSelected() (tea.Cmd, string) {
	if m.demo {
		return nil, "demo mode: attach is disabled"
	}
	r, ok := m.selectedWork()
	if !ok {
		return nil, "no session selected"
	}
	if r.session == "" || r.window == "" {
		return nil, "no tmux target for " + r.ticket
	}
	if r.tmuxState == "stopped" {
		return nil, "tmux session stopped for " + r.ticket
	}
	target := r.session + ":" + r.window
	if r.pane != "" {
		target = r.pane
	}
	cmd, err := newAttachCmd(r.session, r.window, r.pane)
	if err != nil {
		return nil, "attach failed: " + err.Error()
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return attachDoneMsg{err: err}
	}), "attaching " + target
}

func (m *Model) focusNext() (tea.Cmd, string) {
	if m.demo {
		return nil, "demo mode: focus is disabled"
	}
	if len(m.rows) == 0 {
		return nil, "no live session needs attention"
	}
	for offset := 1; offset <= len(m.rows); offset++ {
		idx := (m.cursor + offset) % len(m.rows)
		r := m.rows[idx]
		if !attentionNeeded(r) || r.tmuxState != "live" || r.session == "" || r.window == "" {
			continue
		}
		m.cursor = idx
		m.preview = false
		return m.attachSelected()
	}
	return nil, "no live session needs attention"
}

// Focus attaches to the highest-priority live Orc session that needs human attention.
func Focus(root string) error {
	rows, err := collectRows(root, "")
	if err != nil {
		return err
	}
	for _, r := range rows {
		if !attentionNeeded(r) || r.tmuxState != "live" || r.session == "" || r.window == "" {
			continue
		}
		return tmux.AttachTarget(r.session, r.window, r.pane)
	}
	return fmt.Errorf("no live session needs attention")
}

var newAttachCmd = func(session, window, pane string) (*exec.Cmd, error) {
	return tmux.AttachCommandTarget(session, window, pane)
}

func (m *Model) refreshPreview() {
	if !m.preview {
		return
	}
	m.viewport.SetContent(m.previewContent())
}

func (m Model) itemCount() int {
	return len(m.rows)
}

func (m *Model) applyFilter(resetCursor bool) {
	if m.allRows == nil {
		return
	}
	query := m.searchBox.Value()
	rows := make([]row, 0, len(m.allRows))
	for _, candidate := range m.allRows {
		if searchmatch.Match(query, candidate.search...) {
			rows = append(rows, candidate)
		}
	}
	m.rows = rows
	if resetCursor {
		m.cursor = 0
	}
}

func (m Model) selectedWork() (row, bool) {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func truncate(s string, width int) string {
	return terminalui.Truncate(strings.TrimSpace(s), width)
}

var (
	titleStyle         lipgloss.Style
	mutedStyle         lipgloss.Style
	sectionStyle       lipgloss.Style
	activeStyle        lipgloss.Style
	blockedStyle       lipgloss.Style
	inputStyle         lipgloss.Style
	reviewStyle        lipgloss.Style
	doneStyle          lipgloss.Style
	pendingStyle       lipgloss.Style
	selectedStyle      lipgloss.Style
	accentStyle        lipgloss.Style
	watchHeaderStyle   lipgloss.Style
	selectedRowStyle   lipgloss.Style
	transitionRowStyle lipgloss.Style
	doneFlashStyle     lipgloss.Style
	selectedCardStyle  lipgloss.Style
	contextGreenStyle  lipgloss.Style
	contextYellowStyle lipgloss.Style
	contextRedStyle    lipgloss.Style
)

func init() {
	SetTheme(terminalui.DefaultTheme())
}

// SetTheme applies the shared Orc palette to the Live presentation.
func SetTheme(theme terminalui.Theme) {
	p := theme.Palette
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.Green))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Overlay0))
	sectionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Subtext1)).Bold(true)
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Blue))
	blockedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))
	inputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Peach))
	reviewStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve))
	doneStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve)).Bold(true)
	accentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Mauve))
	watchHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Base)).Background(lipgloss.Color(p.Mauve)).Bold(true)
	selectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Text)).Background(lipgloss.Color(p.Surface0)).Bold(true)
	transitionRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Base)).Background(lipgloss.Color(p.Sky)).Bold(true)
	doneFlashStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Base)).Background(lipgloss.Color(p.Green)).Bold(true)
	selectedCardStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(p.Mauve)).Padding(0, 1)
	contextGreenStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Green))
	contextYellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Yellow))
	contextRedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Red))
}

func stateStyle(label string) lipgloss.Style {
	switch label {
	case "blocked", "error", "stopped":
		return blockedStyle
	case "input":
		return inputStyle
	case "review":
		return reviewStyle
	case "done":
		return doneStyle
	case "pending", "ready":
		return pendingStyle
	case "active":
		return activeStyle
	default:
		return mutedStyle
	}
}

func renderContextPressure(pressure contextpressure.Pressure) string {
	style := mutedStyle
	switch pressure.Level {
	case contextpressure.LevelGreen:
		style = contextGreenStyle
	case contextpressure.LevelYellow:
		style = contextYellowStyle
	case contextpressure.LevelRed:
		style = contextRedStyle
	}
	return style.Render(pressure.Label())
}

func padStyledRight(value string, width int) string {
	return terminalui.PadRight(value, width)
}
