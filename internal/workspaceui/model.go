package workspaceui

import (
	"math/rand"
	"path/filepath"
	"time"

	"github.com/cengebretson/orc/internal/config"
	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/doctor"
	"github.com/cengebretson/orc/internal/searchmatch"
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

type viewerKind uint8

const (
	viewerNone viewerKind = iota
	viewerHealth
	viewerRepositories
	viewerWorker
)

// ── messages ─────────────────────────────────────────────────────

type tickMsg struct {
	at    time.Time
	epoch uint64
}
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
	err             error
	features        []*featureRow
	healthItems     []doctor.Check
	artifactPolicy  string
	workerNames     []string
	workerGroups    []workerGroup
	workflowGroups  []workflowGroup
	allWorkers      []*workers.Worker
	workflows       []workflowChain
	repos           []config.Repo
	routes          []config.RepoRoute
	sectionItems    map[sectionID][]sectionItem
	refreshInterval time.Duration
	quotes          []string
}

type routeStep struct {
	name              string
	label             string
	advance           string // "auto" or "manual"
	workerID          string
	requiredArtifacts []string
}

type repairLoop struct {
	name   string
	label  string
	target string // stage in main chain it loops back to
}

type repairStep struct {
	name              string
	label             string
	workerID          string
	advance           string
	repairs           string
	repairsLabel      string
	maxRetries        int
	requiredArtifacts []string
}

type workerGroup struct {
	name  string
	items []sectionItem
}

type workflowGroup struct {
	name  string
	items []sectionItem
}

type workflowChain struct {
	name        string
	label       string
	description string
	steps       []routeStep
	loops       []repairLoop
	repairSteps []repairStep
}

type sectionItem struct {
	label string
	id    string
	path  string
}

// ── data types ───────────────────────────────────────────────────

type featureRow struct {
	s                 *state.State
	featureDir        string
	workflow          string
	stage             string
	workflowLabel     string
	stageLabel        string
	stageLoopLabel    string
	workerID          string
	workerName        string
	engine            string
	attention         string
	context           contextpressure.Pressure
	tmuxLive          bool
	hasIssues         bool
	requiredArtifacts []string
	loadErr           error // non-nil when STATE.yaml could not be parsed; s is nil
}

// ticketID returns the ticket for display. Broken rows have no parsed state, so
// fall back to the feature directory name.
func (f *featureRow) ticketID() string {
	if f.s != nil && f.s.Ticket != "" {
		return f.s.Ticket
	}
	return filepath.Base(f.featureDir)
}

func (f *featureRow) searchFields() []string {
	fields := []string{
		f.ticketID(), f.featureDir, f.workflow, f.workflowLabel, f.stage,
		f.stageLabel, f.stageLoopLabel, f.workerID, f.workerName, f.engine,
		f.attention,
	}
	if f.s == nil {
		return fields
	}
	fields = append(fields, f.s.Ticket, f.s.Slug, f.s.Status, f.s.Workflow,
		f.s.Stage.Name, f.s.Stage.Worker)
	for name, repo := range f.s.Repos {
		fields = append(fields, name, repo.Main, repo.Worktree, repo.Branch)
	}
	return fields
}

// ── model ─────────────────────────────────────────────────────────

type Model struct {
	root            string
	view            viewState
	features        []*featureRow
	healthItems     []doctor.Check
	artifactPolicy  string
	workerNames     []string
	workerGroups    []workerGroup
	workflowGroups  []workflowGroup
	allWorkers      []*workers.Worker
	workflows       []workflowChain
	repos           []config.Repo
	routes          []config.RepoRoute
	expanded        map[sectionID]bool
	cursor          int
	showArchived    bool
	lastRefresh     time.Time
	refreshInterval time.Duration
	width           int
	height          int
	embedded        bool
	inactive        bool
	epoch           uint64
	loadErr         error

	// workflow detail drill-in
	wfDetailName   string
	wfDetailCursor int

	// section pane navigation
	focusedPane         paneID
	sectionFocus        sectionID
	sectionCursor       int
	sectionItems        map[sectionID][]sectionItem
	autoExpandedSection sectionID

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
	viewerKind    viewerKind
	viewerPath    string
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
		focusedPane:  paneFeatures,
		sectionItems: map[sectionID][]sectionItem{},
		expanded:     defaultSectionExpansion(),
		search:       ti,
	}
}

// NewEmbedded constructs the Workspace section for the shared dashboard and
// suppresses duplicated persistent navigation legends owned by the shell.
func NewEmbedded(root string) Model {
	m := New(root)
	m.embedded = true
	return m
}

// SetActive controls refresh work while Workspace is embedded but hidden.
func (m Model) SetActive(active bool) Model {
	if active == !m.inactive {
		return m
	}
	m.epoch++
	m.inactive = !active
	if !active {
		m.rainbowStep = 0
	}
	return m
}

func (m Model) IsActive() bool {
	return !m.inactive
}

// CanSwitchSection reports whether the shared dashboard shell can consume a
// section-switch or help key without stealing text from the ticket filter.
func (m Model) CanSwitchSection() bool {
	return !m.searching
}

// ── Init ─────────────────────────────────────────────────────────

const defaultRefreshInterval = 60 * time.Second

func (m Model) Init() tea.Cmd {
	if m.inactive {
		return nil
	}
	return tea.Batch(loadData(m.root), tickEvery(defaultRefreshInterval, m.epoch))
}

// ── section navigation ─────────────────────────────────────────────

func (m Model) navigableSections() []sectionID {
	out := make([]sectionID, 0, len(workspaceSections))
	for _, spec := range workspaceSections {
		if spec.alwaysNavigable || len(m.sectionItems[spec.id]) > 0 {
			out = append(out, spec.id)
		}
	}
	return out
}

// visibleFeatures filters features by the archive toggle and search query.
func (m Model) visibleFeatures() []*featureRow {
	query := m.search.Value()
	var out []*featureRow
	for _, f := range m.features {
		// Broken rows (unparseable state) always show — they need attention and
		// we can't tell whether they're archived.
		if f.s != nil && f.s.Status == "archived" && !m.showArchived {
			continue
		}
		if !searchmatch.Match(query, f.searchFields()...) {
			continue
		}
		out = append(out, f)
	}
	return out
}
