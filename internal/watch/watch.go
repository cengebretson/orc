package watch

import (
	"strings"
	"time"

	"github.com/cengebretson/orc/internal/contextpressure"
	"github.com/cengebretson/orc/internal/mux"
	"github.com/cengebretson/orc/internal/tmux"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
	rows    []row
	err     error
	warning error
}

type attachDoneMsg struct {
	err error
}

type promptDoneMsg struct {
	ticket string
	result mux.AgentControlResult
	err    error
}

type Options struct {
	Ticket    string
	Interval  time.Duration
	Wide      bool
	Mode      Mode
	PetLayout PetLayout
	Demo      bool
	Mux       mux.Backend
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
	parked          bool
	woken           bool
	wakeReason      string
	context         contextpressure.Pressure
	room            string
	branch          string
	engine          string
	model           string
	providerID      string
	backend         string
	agentID         string
	agentInstance   string
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
	mux      mux.Backend

	allRows        []row
	rows           []row
	cursor         int
	width          int
	height         int
	lastLoad       time.Time
	loadErr        error
	loadWarning    error
	message        string
	petFrame       int
	petTicking     bool
	petLayout      PetLayout
	parkedExpanded bool

	preview   bool
	viewport  viewport.Model
	searching bool
	searchBox textinput.Model

	prompting  bool
	confirming bool
	promptBox  textinput.Model
	promptRow  row
}

// New constructs the watch model without starting a Bubble Tea program so it
// can run either standalone or as the Live section of the shared dashboard.
func New(root string, opts Options) (Model, error) {
	backend := opts.Mux
	if backend == nil {
		backend = tmux.New()
	}
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
	promptBox := textinput.New()
	promptBox.Placeholder = "message for the selected agent..."
	promptBox.Prompt = "> "
	promptBox.CharLimit = mux.MaxAgentPromptBytes
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
		promptBox:  promptBox,
		mux:        backend,
	}, nil
}

// CanSwitchSection reports whether dashboard-level navigation can safely
// consume a section-switch or help key without stealing modal input.
func (m Model) CanSwitchSection() bool {
	return !m.searching && !m.prompting && !m.confirming
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
	commands := []tea.Cmd{loadDataWithMux(m.root, m.ticket, m.demo, m.mux), tickEvery(m.interval, m.epoch), watchAnimationTick(m.epoch)}
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
		m.promptBox.Width = max(8, msg.Width-6)
		m.refreshPreview()
		return m, nil
	case tickMsg:
		if m.inactive || msg.epoch != m.epoch {
			return m, nil
		}
		return m, tea.Batch(loadDataWithMux(m.root, m.ticket, m.demo, m.mux), tickEvery(m.interval, m.epoch))
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
		m.loadWarning = msg.warning
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
	case promptDoneMsg:
		if msg.err != nil {
			m.message = "prompt failed: " + msg.err.Error()
		} else {
			m.message = "prompt sent to " + msg.ticket
			if msg.result.Lifecycle != "" {
				m.message += " · " + msg.result.Lifecycle
			}
		}
		return m, loadDataWithMux(m.root, m.ticket, m.demo, m.mux)
	case tea.KeyMsg:
		if m.prompting {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.cancelPrompt()
				return m, nil
			case "enter":
				if strings.TrimSpace(m.promptBox.Value()) == "" {
					m.message = "prompt text is required"
					return m, nil
				}
				m.prompting = false
				m.confirming = true
				m.promptBox.Blur()
				m.message = ""
				return m, nil
			default:
				var cmd tea.Cmd
				m.promptBox, cmd = m.promptBox.Update(msg)
				return m, cmd
			}
		}
		if m.confirming {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "y", "Y":
				return m, m.sendConfirmedPrompt()
			case "n", "N", "esc":
				m.cancelPrompt()
				m.message = "prompt cancelled"
				return m, nil
			default:
				return m, nil
			}
		}
		if m.help {
			switch {
			case key.Matches(msg, keys.quit):
				return m, tea.Quit
			case key.Matches(msg, keys.help, keys.back):
				m.help = false
				return m, nil
			default:
				return m, nil
			}
		}
		if m.searching {
			switch {
			case key.Matches(msg, keys.quit):
				return m, tea.Quit
			case key.Matches(msg, keys.back):
				m.searching = false
				m.searchBox.Blur()
				m.searchBox.SetValue("")
				m.applyFilter(true)
				return m, nil
			case key.Matches(msg, keys.confirm):
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
		switch {
		case key.Matches(msg, keys.quit):
			return m, tea.Quit
		case key.Matches(msg, keys.back):
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
		case key.Matches(msg, keys.filter):
			if !m.preview {
				if m.allRows == nil {
					m.allRows = append([]row(nil), m.rows...)
				}
				m.searching = true
				m.searchBox.Focus()
				return m, textinput.Blink
			}
		case key.Matches(msg, keys.help):
			m.help = true
			return m, nil
		case key.Matches(msg, keys.view):
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
		case key.Matches(msg, keys.petLayout):
			if m.preview || m.mode != ModePet {
				return m, nil
			}
			if normalizePetLayout(m.petLayout) == PetLayoutColumn {
				m.petLayout = PetLayoutResponsive
			} else {
				m.petLayout = PetLayoutColumn
			}
			return m, nil
		case key.Matches(msg, keys.parking):
			if m.preview || m.parkedCount() == 0 {
				return m, nil
			}
			m.parkedExpanded = !m.parkedExpanded
			m.applyFilter(false)
			if m.cursor >= m.itemCount() {
				m.cursor = max(0, m.itemCount()-1)
			}
			return m, nil
		case key.Matches(msg, keys.down):
			if m.preview {
				m.viewport.ScrollDown(1)
			} else if m.cursor < m.itemCount()-1 {
				m.cursor++
			}
			return m, nil
		case key.Matches(msg, keys.up):
			if m.preview {
				m.viewport.ScrollUp(1)
			} else if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case key.Matches(msg, keys.pageDown):
			if m.preview {
				m.viewport.HalfPageDown()
			}
			return m, nil
		case key.Matches(msg, keys.pageUp):
			if m.preview {
				m.viewport.HalfPageUp()
			}
			return m, nil
		case key.Matches(msg, keys.top):
			if m.preview {
				m.viewport.GotoTop()
			}
			return m, nil
		case key.Matches(msg, keys.bottom):
			if m.preview {
				m.viewport.GotoBottom()
			}
			return m, nil
		case key.Matches(msg, keys.details):
			m.preview = !m.preview
			m.refreshPreview()
			return m, nil
		case key.Matches(msg, keys.refresh):
			m.message = ""
			return m, loadDataWithMux(m.root, m.ticket, m.demo, m.mux)
		case key.Matches(msg, keys.attach):
			cmd, message := m.attachSelected()
			m.message = message
			return m, cmd
		case key.Matches(msg, keys.attention):
			cmd, message := m.focusNext()
			m.message = message
			return m, cmd
		case key.Matches(msg, keys.sendPrompt):
			m.message = m.beginPrompt()
			if m.prompting {
				return m, textinput.Blink
			}
			return m, nil
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
	if m.prompting || m.confirming {
		return m.renderPromptAction()
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
