package watch

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/featurelist"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const defaultInterval = 5 * time.Second

type tickMsg time.Time

type dataMsg struct {
	rows []row
	err  error
}

type attachDoneMsg struct {
	err error
}

type Options struct {
	Ticket   string
	Interval time.Duration
	Wide     bool
}

type row struct {
	ticket    string
	stage     string
	worker    string
	status    string
	next      string
	session   string
	window    string
	pane      string
	tmuxState string
	attention string
	history   []historyRow
	archived  bool
	loadErr   error
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

	rows     []row
	cursor   int
	width    int
	height   int
	lastLoad time.Time
	loadErr  error
	message  string

	preview  bool
	viewport viewport.Model
}

func Run(root string, opts Options) error {
	interval := opts.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	m := Model{
		root:     root,
		ticket:   opts.Ticket,
		interval: interval,
		wide:     opts.Wide,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadData(m.root, m.ticket), tickEvery(m.interval))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = max(1, msg.Width-2)
		m.viewport.Height = max(1, msg.Height-2)
		m.refreshPreview()
		return m, nil
	case tickMsg:
		return m, tea.Batch(loadData(m.root, m.ticket), tickEvery(m.interval))
	case dataMsg:
		m.rows = msg.rows
		m.loadErr = msg.err
		m.lastLoad = time.Now()
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
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			if m.preview && msg.String() == "esc" {
				m.preview = false
				return m, nil
			}
			return m, tea.Quit
		case "j", "down":
			if !m.preview && m.cursor < m.itemCount()-1 {
				m.cursor++
			}
			return m, nil
		case "k", "up":
			if !m.preview && m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "enter", "n":
			m.preview = !m.preview
			m.refreshPreview()
			return m, nil
		case "r":
			m.message = ""
			return m, loadData(m.root, m.ticket)
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
	if m.preview {
		return m.renderPreview()
	}
	if m.wide || m.width >= 56 {
		return m.renderWide()
	}
	return m.renderRail()
}

func loadData(root, ticket string) tea.Cmd {
	return func() tea.Msg {
		rows, err := collectRows(root, ticket)
		return dataMsg{rows: rows, err: err}
	}
}

func collectRows(root, ticket string) ([]row, error) {
	features, err := featurelist.Collect(root, featurelist.Options{IncludeArchived: false})
	if err != nil {
		return nil, err
	}
	rows := make([]row, 0, len(features))
	for _, f := range features {
		r := rowFromFeature(f)
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

func rowFromFeature(f *featurelist.Feature) row {
	if f.LoadError != nil || f.State == nil {
		return row{
			ticket:  filepath.Base(f.FeatureDir),
			status:  "error",
			loadErr: f.LoadError,
		}
	}
	s := f.State
	worker := f.WorkerName
	if worker == "" {
		worker = f.WorkerID
	}
	if worker == "" {
		worker = s.Stage.Worker
	}
	return row{
		ticket:    s.Ticket,
		stage:     s.Stage.Name + f.StageLoopLabel,
		worker:    shortWorker(worker),
		status:    s.Status,
		next:      s.NextAction.Prompt,
		session:   tmuxSession(s),
		window:    s.Stage.Name,
		pane:      tmuxPane(s),
		tmuxState: tmuxState(s, f.TmuxLive),
		attention: f.Attention,
		history:   historyRows(s.History),
		archived:  f.Archived,
	}
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

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) attachSelected() (tea.Cmd, string) {
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

func (m Model) selectedWork() (row, bool) {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

func truncate(s string, width int) string {
	s = strings.TrimSpace(s)
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	if width == 1 {
		return s[:1]
	}
	return s[:width-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a6e3a1"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	sectionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#9399b2")).Bold(true)
	activeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa"))
	blockedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8"))
	inputStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387"))
	reviewStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7"))
	doneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	pendingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true)
)

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

func linef(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
