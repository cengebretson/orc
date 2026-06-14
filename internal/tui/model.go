package tui

import (
	"math/rand"
	"path/filepath"
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/state"
	"github.com/cengebretson/orc/internal/workers"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func pickQuote(custom []string) string {
	if len(custom) == 0 {
		return ""
	}
	return custom[rand.Intn(len(custom))]
}

// ── view states ──────────────────────────────────────────────────

type viewState int

const (
	viewDashboard viewState = iota
	viewDetail
	viewFile
	viewWorkflowDetail
	viewCharacterSheet
)

// ── messages ─────────────────────────────────────────────────────

type tickMsg time.Time
type rainbowTickMsg struct{}

const rainbowSteps = 48 // 4 cycles × 12 colors at 80ms each ≈ 3.8s

var rainbowPalette = []string{
	"#cba6f7", // mauve
	"#f5c2e7", // pink
	"#f2cdcd", // flamingo
	"#f38ba8", // red
	"#fab387", // peach
	"#f9e2af", // yellow
	"#a6e3a1", // green
	"#94e2d5", // teal
	"#89dceb", // sky
	"#74c7ec", // sapphire
	"#89b4fa", // blue
	"#b4befe", // lavender
}

func rainbowTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return rainbowTickMsg{} })
}

type dataMsg struct {
	features        []*featureRow
	healthItems     []doctor.Check
	workerNames     []string
	allWorkers      []*workers.Worker
	workflows       []workflowChain
	repos           []config.Repo
	sectionItems    map[string][]sectionItem
	refreshInterval time.Duration
	quotes          []string
}

type routeStep struct {
	name     string
	advance  string // "auto" or "manual"
	workerID string
}

type repairLoop struct {
	name   string
	target string // stage in main chain it loops back to
}

type repairStep struct {
	name       string
	workerID   string
	advance    string
	repairs    string
	maxRetries int
}

type workflowChain struct {
	name        string
	description string
	steps       []routeStep
	loops       []repairLoop
	repairSteps []repairStep
}

type sectionItem struct {
	label string
	path  string
}

// ── data types ───────────────────────────────────────────────────

type featureRow struct {
	s              *state.State
	featureDir     string
	workflow       string
	stageLoopLabel string
	workerName     string
	tmuxLive       bool
	hasIssues      bool
	loadErr        error // non-nil when STATE.yaml could not be parsed; s is nil
}

// ticketID returns the ticket for display. Broken rows have no parsed state, so
// fall back to the feature directory name.
func (f *featureRow) ticketID() string {
	if f.s != nil && f.s.Ticket != "" {
		return f.s.Ticket
	}
	return filepath.Base(f.featureDir)
}

// ── model ─────────────────────────────────────────────────────────

type Model struct {
	root            string
	view            viewState
	features        []*featureRow
	healthItems     []doctor.Check
	workerNames     []string
	allWorkers      []*workers.Worker
	workflows       []workflowChain
	repos           []config.Repo
	expanded        map[string]bool
	cursor          int
	showArchived    bool
	lastRefresh     time.Time
	refreshInterval time.Duration
	width           int
	height          int

	// workflow detail drill-in
	wfDetailName   string
	wfDetailCursor int

	// section pane navigation
	focusedPane   string // "features" or "section"
	sectionFocus  string // "workflows" | "workers" | "routes"
	sectionCursor int
	sectionItems  map[string][]sectionItem

	// detail
	detail       *featureRow
	detailFiles  []detailFile
	fileIdx      int
	detailScroll int // saved viewport offset, restored when returning from a file

	// file viewer
	viewport      viewport.Model
	viewerTitle   string
	viewerContext string // label shown in file viewer title bar
	viewerReturn  viewState
	// viewerRender regenerates the viewer's content at a given width so the file
	// viewer re-flows on resize. Set for every viewFile, file-backed or synthetic.
	viewerRender func(width int) string

	// search
	search    textinput.Model
	searching bool

	quote string

	// easter egg: type "orc" on the dashboard to trigger rainbow logo
	keyBuffer   [3]string
	rainbowStep int // 0=off, counts down from rainbowSteps

	// easter egg: press "!" on a focused worker to open Bard's Tale character sheet
	charSheetWorker *workers.Worker
	charSheetReturn viewState
}

type detailFile struct {
	label string
	path  string
}

func New(root string) Model {
	ti := textinput.New()
	ti.Placeholder = "filter tickets..."
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(activeTheme.Palette.Mauve))
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(activeTheme.Palette.Text))
	ti.Prompt = "/ "
	ti.CharLimit = 64

	return Model{
		root:         root,
		lastRefresh:  time.Now(),
		focusedPane:  "features",
		sectionItems: map[string][]sectionItem{},
		expanded: map[string]bool{
			"health":    false,
			"workflows": false,
			"workers":   false,
			"routes":    true,
		},
		search: ti,
	}
}

func Run(root string) error {
	if cfg, err := config.Load(root); err == nil {
		_ = LoadTheme(cfg.Settings.Theme)
	}
	m := New(root)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// ── Init ─────────────────────────────────────────────────────────

const defaultRefreshInterval = 60 * time.Second

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadData(m.root), tickEvery(defaultRefreshInterval))
}

// ── section navigation ─────────────────────────────────────────────

func (m Model) navigableSections() []string {
	out := []string{"health"}
	for _, key := range []string{"workflows", "workers", "routes"} {
		if len(m.sectionItems[key]) > 0 {
			out = append(out, key)
		}
	}
	return out
}

func sectionLabel(key string) string {
	labels := map[string]string{
		"workflows": "Workflows",
		"workers":   "Workers",
		"routes":    "Routes",
	}
	if l, ok := labels[key]; ok {
		return l
	}
	return key
}

// visibleFeatures filters features by the archive toggle and search query.
func (m Model) visibleFeatures() []*featureRow {
	query := strings.ToLower(strings.TrimSpace(m.search.Value()))
	var out []*featureRow
	for _, f := range m.features {
		// Broken rows (unparseable state) always show — they need attention and
		// we can't tell whether they're archived.
		if f.s != nil && f.s.Status == "archived" && !m.showArchived {
			continue
		}
		if query != "" {
			haystack := strings.ToLower(f.ticketID())
			if f.s != nil {
				haystack = strings.ToLower(f.s.Ticket + " " + f.s.Slug)
			}
			if !strings.Contains(haystack, query) {
				continue
			}
		}
		out = append(out, f)
	}
	return out
}
