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
	terminalui "github.com/cengebretson/orc/internal/ui"
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

// pickNextQuote picks a quote different from current when possible, so the
// idle empty-Features rotation doesn't visibly repeat itself back to back.
func pickNextQuote(custom []string, current string) string {
	if len(custom) < 2 {
		return pickQuote(custom)
	}
	next := custom[rand.Intn(len(custom)-1)]
	if next == current {
		next = custom[len(custom)-1]
	}
	return next
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
type liveTickMsg struct {
	at    time.Time
	epoch uint64
}
type quoteRotateTickMsg struct {
	epoch uint64
}

// quoteRotateInterval is how often the idle empty-Features quote changes.
const quoteRotateInterval = 8 * time.Second

type rainbowTickMsg struct{}

const rainbowSteps = terminalui.RainbowSteps // 4 cycles × 12 colors ≈ 3.8s

func rainbowTick() tea.Cmd {
	return tea.Tick(terminalui.RainbowInterval, func(time.Time) tea.Msg { return rainbowTickMsg{} })
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
	config          *config.Config
}

type liveDataMsg struct {
	err      error
	features []*featureRow
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
	root     string
	view     viewState
	width    int
	height   int
	embedded bool

	data       workspaceData
	lifecycle  lifecycleState
	navigation navigationState
	detail     detailState
	viewer     viewerState
	filter     searchState
	effects    effectsState
}

type workspaceData struct {
	config         *config.Config
	features       []*featureRow
	healthItems    []doctor.Check
	artifactPolicy string
	quotes         []string
	workerNames    []string
	workerGroups   []workerGroup
	workflowGroups []workflowGroup
	allWorkers     []*workers.Worker
	workflows      []workflowChain
	repos          []config.Repo
	routes         []config.RepoRoute
}

type lifecycleState struct {
	lastRefresh     time.Time
	lastLiveRefresh time.Time
	refreshInterval time.Duration
	inactive        bool
	epoch           uint64
	loadErr         error
}

type navigationState struct {
	expanded       map[sectionID]bool
	featureCursor  int
	showArchived   bool
	sectionCursors map[sectionID]int

	workflowName   string
	workflowCursor int

	pane          paneID
	section       sectionID
	sectionCursor int
	items         map[sectionID][]sectionItem
	autoExpanded  sectionID
}

type detailState struct {
	feature   *featureRow
	files     []detailFile
	fileIndex int
	scroll    int // saved viewport offset, restored when returning from a file
}

type viewerState struct {
	viewport   viewport.Model
	title      string
	context    string // label shown in file viewer title bar
	returnView viewState
	kind       viewerKind
	path       string
	// render regenerates the viewer's content at a given width so the file
	// viewer re-flows on resize. Set for every viewFile, file-backed or synthetic.
	render func(width int) string
}

type searchState struct {
	input  textinput.Model
	active bool
}

type effectsState struct {
	quote string

	// easter egg: type "orc" on the dashboard to trigger rainbow logo
	keyBuffer   [3]string
	rainbowStep int // 0=off, counts down from rainbowSteps

	// easter egg: press "!" on a focused worker to open Bard's Tale character sheet
	charSheetWorker *workers.Worker
	charSheetReturn viewState

	// contextHistory holds each feature's recent context-pressure percentages
	// (most recent last), keyed by ticket, for the Features table sparkline.
	// Populated on the 2s live refresh rather than the full data reload, so it
	// survives workspaceData being replaced wholesale on every full refresh.
	contextHistory map[string][]uint64
}

// contextHistoryLimit caps how many samples the sparkline keeps per feature —
// enough to show a short trend without the history growing unbounded.
const contextHistoryLimit = 8

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
		root: root,
		lifecycle: lifecycleState{
			lastRefresh: time.Now(),
		},
		navigation: navigationState{
			pane:           paneFeatures,
			items:          map[sectionID][]sectionItem{},
			expanded:       defaultSectionExpansion(),
			sectionCursors: map[sectionID]int{},
		},
		filter: searchState{input: ti},
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
	if active == !m.lifecycle.inactive {
		return m
	}
	m.lifecycle.epoch++
	m.lifecycle.inactive = !active
	if !active {
		m.effects.rainbowStep = 0
	}
	return m
}

func (m Model) IsActive() bool {
	return !m.lifecycle.inactive
}

// HealthIssueCount returns the warnings and failures surfaced by the Health
// destination so the shared shell can badge its tab.
func (m Model) HealthIssueCount() int {
	count := 0
	for _, item := range m.data.healthItems {
		if item.Status != doctor.OK {
			count++
		}
	}
	return count
}

// CanSwitchSection reports whether the shared dashboard shell can consume a
// section-switch or help key without stealing text from the ticket filter.
func (m Model) CanSwitchSection() bool {
	return !m.filter.active
}

// SetDestination selects a top-level Workspace view without rebuilding the
// model, preserving filters, cursors, and loaded data.
func (m Model) SetDestination(destination Destination) Model {
	m.view = viewDashboard
	section := sectionForDestination(destination)
	if section == sectionNone {
		m.blurSectionFocus()
		return m
	}
	m.focusSection(section)
	switch destination {
	case DestinationHealth:
		m.openHealthReport(viewFile)
	case DestinationRepositories:
		m.openRepositoryReport(viewFile)
	case DestinationWorkers:
		m.openWorkerReport(viewFile)
	case DestinationWorkflows:
		m.openDefaultWorkflowDetail()
	}
	return m
}

// ── Init ─────────────────────────────────────────────────────────

const (
	defaultRefreshInterval     = 60 * time.Second
	defaultLiveRefreshInterval = 2 * time.Second
)

func (m Model) Init() tea.Cmd {
	if m.lifecycle.inactive {
		return nil
	}
	return tea.Batch(
		loadData(m.root),
		tickEvery(defaultRefreshInterval, m.lifecycle.epoch),
		liveTickEvery(defaultLiveRefreshInterval, m.lifecycle.epoch),
		quoteRotateTick(m.lifecycle.epoch),
	)
}

// ── section navigation ─────────────────────────────────────────────

func (m Model) navigableSections() []sectionID {
	out := make([]sectionID, 0, len(workspaceSections))
	for _, spec := range workspaceSections {
		if spec.alwaysNavigable || len(m.navigation.items[spec.id]) > 0 {
			out = append(out, spec.id)
		}
	}
	return out
}

// visibleFeatures filters features by the archive toggle and search query.
func (m Model) visibleFeatures() []*featureRow {
	query := m.filter.input.Value()
	var out []*featureRow
	for _, f := range m.data.features {
		// Broken rows (unparseable state) always show — they need attention and
		// we can't tell whether they're archived.
		if f.s != nil && f.s.Status == "archived" && !m.navigation.showArchived {
			continue
		}
		if !searchmatch.Match(query, f.searchFields()...) {
			continue
		}
		out = append(out, f)
	}
	return out
}
