package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cldixon/jernel/internal/config"
	"github.com/cldixon/jernel/internal/daemon"
	"github.com/cldixon/jernel/internal/entry"
	"github.com/cldixon/jernel/internal/persona"
	"github.com/cldixon/jernel/internal/store"
)

// Tab represents the main navigation tabs
type tab int

const (
	tabEntries tab = iota
	tabPersonas
	tabDaemon
	tabSettings
)

// Sub-modes for specific interactions
type subMode int

const (
	subModeNone subMode = iota
	subModeGenerating
	subModeDeleteConfirm
	subModeSelectPersona // for selecting persona when generating
	subModePersonaEditor // in-TUI persona editor (create/edit)
	subModeFirstPersona  // first-time persona creation wizard
	subModeError         // show error message
)

// Colors - minimal palette
var (
	colorFg       = lipgloss.Color("#cccccc")
	colorFgDim    = lipgloss.Color("#666666")
	colorFgBright = lipgloss.Color("#ffffff")
	colorAccent   = lipgloss.Color("#de4f5c") // cherry red
	colorBorder   = lipgloss.Color("#444444")
	colorError    = lipgloss.Color("#cc6666")
)

// Styles - minimal
var (
	tabStyle = lipgloss.NewStyle().
			Foreground(colorFgDim).
			Padding(0, 2)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(colorFgBright).
			Bold(true).
			Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(colorBorder)

	contentStyle = lipgloss.NewStyle().
			Padding(1, 2)

	listStyle = lipgloss.NewStyle().
			Padding(0, 1)

	viewportStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(colorBorder).
			Padding(0, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorFgBright).
			Bold(true)

	// Entry title matches the list selection highlight color
	entryTitleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(colorFgDim).
			Width(14)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorFg)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorFgDim).
			Padding(0, 2)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(colorAccent)

	logoStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			Padding(0, 2)

	statusRunning = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#66cc66"))

	statusStopped = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#cc6666"))
)

// entryItem wraps a store.Entry for the list
type entryItem struct {
	entry *store.Entry
}

func (i entryItem) Title() string {
	preview := getContentPreview(i.entry.Content, 30)
	return fmt.Sprintf("#%d  %s", i.entry.ID, preview)
}

func (i entryItem) Description() string {
	relTime := formatRelativeTime(i.entry.CreatedAt)
	return fmt.Sprintf("%s · %s", relTime, i.entry.Persona)
}

func (i entryItem) FilterValue() string {
	return i.entry.Persona + " " + i.entry.Content
}

// personaItem wraps a persona for the list
type personaItem struct {
	persona *persona.Persona
}

func (i personaItem) Title() string {
	return formatPersonaName(i.persona.Name)
}

func (i personaItem) Description() string {
	return getContentPreview(i.persona.Description, 40)
}

func (i personaItem) FilterValue() string {
	return i.persona.Name + " " + i.persona.Description
}

// Message types
type editorFinishedMsg struct{ err error }
type generateDoneMsg struct {
	entry *store.Entry
	err   error
}
type daemonStatusMsg struct {
	running bool
	state   *daemon.State
}

// Model is the main TUI model
type Model struct {
	// Layout
	activeTab tab
	subMode   subMode
	width     int
	height    int
	ready     bool
	quitting  bool
	version   string

	// Entries tab
	entryList    list.Model
	entryView    viewport.Model
	entries      []*store.Entry
	showMetrics  bool
	metricsWidth int

	// Personas tab
	personaList list.Model
	personaView viewport.Model
	personas    []*persona.Persona

	// Generation
	generating bool
	genSpinner spinner.Model
	genError   error
	genPersona string

	// Persona editor
	editorNameInput  textinput.Model
	editorDescInput  textarea.Model
	editorFocusName  bool   // true = name focused, false = desc focused
	editorIsNew      bool   // true = creating new, false = editing existing
	editorOrigName   string // original name when editing (for rename detection)
	deleteTarget     string
	deleteEntryCount int // number of entries that will be deleted with persona

	// Daemon tab
	daemonRunning bool
	daemonState   *daemon.State
	daemonSpinner spinner.Model

	// Settings tab
	cfg *config.Config

	// Shared
	renderer *glamour.TermRenderer
}

// New creates a new TUI model
func New(entries []*store.Entry, version string) (*Model, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(70),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	// Entry list
	entryItems := make([]list.Item, len(entries))
	for i, e := range entries {
		entryItems[i] = entryItem{entry: e}
	}
	entryList := createList(entryItems)

	// Spinner for generation
	genSpin := spinner.New()
	genSpin.Spinner = spinner.Dot
	genSpin.Style = spinnerStyle

	// Spinner for daemon (blinking dot)
	daemonSpin := spinner.New()
	daemonSpin.Spinner = spinner.Spinner{
		Frames: []string{"●", " "},
		FPS:    1, // Slow pulse - one cycle per second
	}
	daemonSpin.Style = statusRunning

	// Persona editor - name input
	nameInput := textinput.New()
	nameInput.Placeholder = "persona_name"
	nameInput.CharLimit = 32
	nameInput.Width = 40
	nameInput.Prompt = ""

	// Persona editor - description textarea
	descInput := textarea.New()
	descInput.Placeholder = "Describe the persona's voice, style, and personality..."
	descInput.CharLimit = 2000
	descInput.SetWidth(60)
	descInput.SetHeight(10)
	descInput.ShowLineNumbers = false

	// Load config
	cfg, _ := config.Load()

	return &Model{
		activeTab:       tabEntries,
		entries:         entries,
		entryList:       entryList,
		showMetrics:     true,
		metricsWidth:    28,
		genSpinner:      genSpin,
		daemonSpinner:   daemonSpin,
		editorNameInput: nameInput,
		editorDescInput: descInput,
		editorFocusName: true,
		cfg:             cfg,
		renderer:        renderer,
		version:         version,
	}, nil
}

func createList(items []list.Item) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(colorAccent).
		BorderLeftForeground(colorAccent)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(colorFgDim).
		BorderLeftForeground(colorAccent)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(colorFg)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(colorFgDim)

	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(colorAccent)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorAccent)

	return l
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()
		if !m.ready {
			m.ready = true
			// Initialize content on first render
			m.updateEntryView()
		}

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case editorFinishedMsg:
		if m.activeTab == tabPersonas {
			m.loadPersonas()
			m.updatePersonaView()
		}
		return m, nil

	case generateDoneMsg:
		m.generating = false
		if msg.err != nil {
			m.genError = msg.err
			m.subMode = subModeError
		} else {
			m.subMode = subModeNone
			m.entries = append([]*store.Entry{msg.entry}, m.entries...)
			m.refreshEntryList()
			m.updateEntryView()
		}
		return m, nil

	case daemonStatusMsg:
		m.daemonRunning = msg.running
		m.daemonState = msg.state
		return m, nil

	case spinner.TickMsg:
		var cmds []tea.Cmd
		if m.generating {
			var cmd tea.Cmd
			m.genSpinner, cmd = m.genSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		if m.daemonRunning && m.activeTab == tabDaemon {
			var cmd tea.Cmd
			m.daemonSpinner, cmd = m.daemonSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
	}

	return m, nil
}

func (m *Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit
	if msg.String() == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	// Handle sub-modes first
	switch m.subMode {
	case subModeGenerating:
		return m, nil
	case subModeDeleteConfirm:
		return m.handleDeleteConfirm(msg)
	case subModeSelectPersona:
		return m.handleSelectPersona(msg)
	case subModePersonaEditor, subModeFirstPersona:
		return m.handlePersonaEditor(msg)
	case subModeError:
		// Any key dismisses the error
		m.subMode = subModeNone
		m.genError = nil
		return m, nil
	}

	// Tab navigation
	switch msg.String() {
	case "tab":
		m.activeTab = (m.activeTab + 1) % 4
		return m, m.onTabChange()
	case "shift+tab":
		m.activeTab = (m.activeTab + 3) % 4
		return m, m.onTabChange()
	case "1":
		m.activeTab = tabEntries
		return m, m.onTabChange()
	case "2":
		m.activeTab = tabPersonas
		return m, m.onTabChange()
	case "3":
		m.activeTab = tabDaemon
		return m, m.onTabChange()
	case "4":
		m.activeTab = tabSettings
		return m, m.onTabChange()
	case "q":
		m.quitting = true
		return m, tea.Quit
	}

	// Tab-specific handling
	switch m.activeTab {
	case tabEntries:
		return m.handleEntriesTab(msg)
	case tabPersonas:
		return m.handlePersonasTab(msg)
	case tabDaemon:
		return m.handleDaemonTab(msg)
	case tabSettings:
		return m.handleSettingsTab(msg)
	}

	return m, nil
}

func (m *Model) onTabChange() tea.Cmd {
	switch m.activeTab {
	case tabEntries:
		m.updateEntryView()
	case tabPersonas:
		if len(m.personas) == 0 {
			m.loadPersonas()
		}
		m.updatePersonaView()
	case tabDaemon:
		// Refresh daemon status when switching to tab
		running, _, _ := daemon.IsRunning()
		m.daemonRunning = running
		if running {
			m.daemonState, _ = daemon.LoadState()
			return m.daemonSpinner.Tick
		}
	case tabSettings:
		m.cfg, _ = config.Load()
	}
	return nil
}

func (m *Model) handleEntriesTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "n":
		m.loadPersonas()
		if len(m.personas) == 0 {
			// No personas exist - show first persona wizard with example
			examples, _ := persona.ListExamples()
			if len(examples) > 0 {
				if example, err := persona.GetExample(examples[0]); err == nil {
					m.initPersonaEditor(true, "", example.Name, example.Description)
				} else {
					m.initPersonaEditor(true, "", "", "")
				}
			} else {
				m.initPersonaEditor(true, "", "", "")
			}
			m.subMode = subModeFirstPersona
		} else {
			m.subMode = subModeSelectPersona
		}
		return m, nil
	case "m":
		m.showMetrics = !m.showMetrics
		m.recalculateLayout()
		m.updateEntryView()
		return m, nil
	}

	// Pass to list
	var cmd tea.Cmd
	prevIdx := m.entryList.Index()
	m.entryList, cmd = m.entryList.Update(msg)
	if prevIdx != m.entryList.Index() {
		m.updateEntryView()
	}

	// Pass to viewport for scrolling
	var vpCmd tea.Cmd
	m.entryView, vpCmd = m.entryView.Update(msg)

	return m, tea.Batch(cmd, vpCmd)
}

func (m *Model) handlePersonasTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "c":
		// Create new persona
		m.initPersonaEditor(true, "", "", "")
		m.subMode = subModePersonaEditor
		return m, nil
	case "e":
		// Edit selected persona
		if sel := m.personaList.SelectedItem(); sel != nil {
			p := sel.(personaItem).persona
			m.initPersonaEditor(false, p.Name, p.Name, p.Description)
			m.subMode = subModePersonaEditor
		}
		return m, nil
	case "d":
		if sel := m.personaList.SelectedItem(); sel != nil {
			m.deleteTarget = sel.(personaItem).persona.Name
			// Get entry count for this persona
			m.deleteEntryCount = 0
			if db, err := store.Open(); err == nil {
				if count, err := db.CountByPersona(m.deleteTarget); err == nil {
					m.deleteEntryCount = count
				}
				db.Close()
			}
			m.subMode = subModeDeleteConfirm
		}
		return m, nil
	}

	// Pass to list
	var cmd tea.Cmd
	prevIdx := m.personaList.Index()
	m.personaList, cmd = m.personaList.Update(msg)
	if prevIdx != m.personaList.Index() {
		m.updatePersonaView()
	}

	// Pass to viewport for scrolling
	var vpCmd tea.Cmd
	m.personaView, vpCmd = m.personaView.Update(msg)

	return m, tea.Batch(cmd, vpCmd)
}

func (m *Model) handleDaemonTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s":
		if m.daemonRunning {
			return m, m.stopDaemon()
		}
		return m, m.startDaemon()
	case "r":
		running, _, _ := daemon.IsRunning()
		m.daemonRunning = running
		if running {
			m.daemonState, _ = daemon.LoadState()
		} else {
			m.daemonState = nil
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) handleSettingsTab(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Settings is read-only, no special handling
	return m, nil
}

func (m *Model) handleSelectPersona(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.subMode = subModeNone
		return m, nil
	case "enter":
		if sel := m.personaList.SelectedItem(); sel != nil {
			m.genPersona = sel.(personaItem).persona.Name
			m.subMode = subModeGenerating
			m.generating = true
			m.genError = nil
			return m, tea.Batch(m.genSpinner.Tick, m.generateEntry())
		}
		return m, nil
	}

	// Pass to persona list
	var cmd tea.Cmd
	m.personaList, cmd = m.personaList.Update(msg)
	return m, cmd
}

// initPersonaEditor sets up the persona editor with initial values
func (m *Model) initPersonaEditor(isNew bool, origName, name, desc string) {
	m.editorIsNew = isNew
	m.editorOrigName = origName
	m.editorFocusName = true

	m.editorNameInput.SetValue(name)
	m.editorNameInput.Focus()

	m.editorDescInput.SetValue(desc)
	m.editorDescInput.Blur()

	// Resize for current window
	m.editorDescInput.SetWidth(m.width - 20)
	m.editorDescInput.SetHeight(m.height - 15)
}

// handlePersonaEditor handles input for the in-TUI persona editor
func (m *Model) handlePersonaEditor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.subMode = subModeNone
		return m, nil

	case "tab":
		// Toggle focus between name and description
		m.editorFocusName = !m.editorFocusName
		if m.editorFocusName {
			m.editorNameInput.Focus()
			m.editorDescInput.Blur()
		} else {
			m.editorNameInput.Blur()
			m.editorDescInput.Focus()
		}
		return m, nil

	case "ctrl+s":
		// Save the persona
		return m.savePersonaFromEditor()
	}

	// Pass input to the focused component
	var cmd tea.Cmd
	if m.editorFocusName {
		m.editorNameInput, cmd = m.editorNameInput.Update(msg)
	} else {
		m.editorDescInput, cmd = m.editorDescInput.Update(msg)
	}
	return m, cmd
}

// savePersonaFromEditor saves the persona and returns to previous view
func (m *Model) savePersonaFromEditor() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.editorNameInput.Value())
	desc := strings.TrimSpace(m.editorDescInput.Value())

	// Validate
	if name == "" {
		return m, nil // Don't save without a name
	}

	// Convert spaces to underscores for filename
	fileName := strings.ReplaceAll(name, " ", "_")
	fileName = strings.ToLower(fileName)

	// Create/update the persona
	p := &persona.Persona{
		Name:        fileName,
		Description: desc,
	}

	// If editing and name changed, delete old file
	if !m.editorIsNew && m.editorOrigName != "" && m.editorOrigName != fileName {
		persona.Delete(m.editorOrigName)
	}

	if err := persona.Save(p); err != nil {
		return m, nil // Could show error, for now just don't save
	}

	// Reload personas
	m.loadPersonas()
	m.updatePersonaView()

	// If this was the first persona wizard, proceed to generate entry
	if m.subMode == subModeFirstPersona {
		m.genPersona = fileName
		m.subMode = subModeGenerating
		m.generating = true
		m.genError = nil
		return m, tea.Batch(m.genSpinner.Tick, m.generateEntry())
	}

	m.subMode = subModeNone
	return m, nil
}

func (m *Model) handleDeleteConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.deleteTarget != "" {
			// Delete entries first (cascade delete)
			if m.deleteEntryCount > 0 {
				if db, err := store.Open(); err == nil {
					db.DeleteByPersona(m.deleteTarget)
					db.Close()
					// Refresh entries list if we're on entries tab
					m.refreshEntriesFromDB()
				}
			}
			// Delete the persona file
			persona.Delete(m.deleteTarget)
			m.deleteTarget = ""
			m.deleteEntryCount = 0
			m.loadPersonas()
			m.updatePersonaView()
		}
		m.subMode = subModeNone
		return m, nil
	case "n", "N", "esc":
		m.deleteTarget = ""
		m.deleteEntryCount = 0
		m.subMode = subModeNone
		return m, nil
	}
	return m, nil
}

func (m *Model) generateEntry() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return generateDoneMsg{err: err}
		}
		result, err := entry.Generate(context.Background(), cfg, m.genPersona)
		if err != nil {
			return generateDoneMsg{err: err}
		}
		return generateDoneMsg{entry: result.Entry}
	}
}

func (m *Model) startDaemon() tea.Cmd {
	return func() tea.Msg {
		executable, err := os.Executable()
		if err != nil {
			return daemonStatusMsg{running: false}
		}

		cmd := exec.Command(executable, "daemon", "start")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := cmd.Start(); err != nil {
			return daemonStatusMsg{running: false}
		}

		go cmd.Wait()
		time.Sleep(500 * time.Millisecond)

		running, _, _ := daemon.IsRunning()
		var state *daemon.State
		if running {
			state, _ = daemon.LoadState()
		}
		return daemonStatusMsg{running: running, state: state}
	}
}

func (m *Model) stopDaemon() tea.Cmd {
	return func() tea.Msg {
		pid, err := daemon.ReadPID()
		if err != nil {
			return daemonStatusMsg{running: false}
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			return daemonStatusMsg{running: false}
		}

		process.Signal(syscall.SIGTERM)
		time.Sleep(500 * time.Millisecond)

		running, _, _ := daemon.IsRunning()
		return daemonStatusMsg{running: running}
	}
}

func (m *Model) openPersonaInEditor() tea.Cmd {
	sel := m.personaList.SelectedItem()
	if sel == nil {
		return nil
	}
	p := sel.(personaItem).persona
	dir, err := persona.Dir()
	if err != nil {
		return nil
	}
	return m.openFileInEditor(dir + "/" + p.Name + ".md")
}

func (m *Model) openFileInEditor(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

func (m *Model) loadPersonas() {
	names, _ := persona.List()
	m.personas = make([]*persona.Persona, 0, len(names))
	items := make([]list.Item, 0, len(names))

	for _, name := range names {
		p, err := persona.Get(name)
		if err != nil {
			continue
		}
		m.personas = append(m.personas, p)
		items = append(items, personaItem{persona: p})
	}

	m.personaList = createList(items)

	// Size the list appropriately
	contentHeight := m.height - 4
	listWidth := m.width / 3
	if listWidth < 30 {
		listWidth = 30
	}
	m.personaList.SetSize(listWidth, contentHeight-4)

	m.recalculateLayout()
}

func (m *Model) refreshEntryList() {
	items := make([]list.Item, len(m.entries))
	for i, e := range m.entries {
		items[i] = entryItem{entry: e}
	}
	m.entryList.SetItems(items)
}

func (m *Model) refreshEntriesFromDB() {
	db, err := store.Open()
	if err != nil {
		return
	}
	defer db.Close()

	entries, err := db.List(100)
	if err != nil {
		return
	}
	m.entries = entries
	m.refreshEntryList()
	m.updateEntryView()
}

func (m *Model) recalculateLayout() {
	contentHeight := m.height - 4 // tab bar + help bar

	switch m.activeTab {
	case tabEntries:
		listWidth := m.width / 4
		if listWidth < 25 {
			listWidth = 25
		}
		metricsWidth := 0
		if m.showMetrics {
			metricsWidth = m.metricsWidth
		}
		viewWidth := m.width - listWidth - metricsWidth - 4

		m.entryList.SetSize(listWidth, contentHeight-2)
		m.entryView = viewport.New(viewWidth, contentHeight-2)

	case tabPersonas:
		listWidth := m.width / 3
		if listWidth < 25 {
			listWidth = 25
		}
		viewWidth := m.width - listWidth - 4

		m.personaList.SetSize(listWidth, contentHeight-2)
		m.personaView = viewport.New(viewWidth, contentHeight-2)
	}
}

func (m *Model) updateEntryView() {
	if len(m.entries) == 0 {
		m.entryView.SetContent(m.renderEmptyEntries())
		return
	}

	sel := m.entryList.SelectedItem()
	if sel == nil {
		return
	}

	e := sel.(entryItem).entry
	var content strings.Builder

	content.WriteString(entryTitleStyle.Render(fmt.Sprintf("Entry #%d", e.ID)))
	content.WriteString("\n\n")
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgDim).Render(
		e.CreatedAt.Format("Monday, January 02, 2006 at 3:04 PM")))
	content.WriteString("\n\n")

	rendered, err := m.renderer.Render(e.Content)
	if err != nil {
		content.WriteString(e.Content)
	} else {
		content.WriteString(rendered)
	}

	m.entryView.SetContent(content.String())
}

func (m *Model) updatePersonaView() {
	if len(m.personas) == 0 {
		m.personaView.SetContent(lipgloss.NewStyle().Foreground(colorFgDim).Render(
			"No personas found.\n\nPersonas define the voice and style for journal entries.\n" +
				"Press 'c' to create your first persona."))
		return
	}

	sel := m.personaList.SelectedItem()
	if sel == nil {
		return
	}

	p := sel.(personaItem).persona
	var content strings.Builder

	// Info header
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgDim).Italic(true).Render(
		"Personas describe the characters writing jernel entries."))
	content.WriteString("\n\n")

	content.WriteString(titleStyle.Render("personas/" + p.Name + ".md"))
	content.WriteString("\n\n")

	rendered, err := m.renderer.Render(p.Description)
	if err != nil {
		content.WriteString(p.Description)
	} else {
		content.WriteString(rendered)
	}

	m.personaView.SetContent(content.String())
}

func (m *Model) renderEmptyEntries() string {
	return lipgloss.NewStyle().Foreground(colorFgDim).Render(
		"\n  No entries yet.\n\n  Press 'n' to create your first entry.")
}

// View implements tea.Model
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return "  Loading..."
	}

	var sections []string

	// Tab bar
	sections = append(sections, m.renderTabBar())

	// Content
	sections = append(sections, m.renderContent())

	// Help bar
	sections = append(sections, m.renderHelpBar())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (m *Model) renderTabBar() string {
	// Tab navigation
	tabs := []string{"Entries", "Personas", "Daemon", "Settings"}
	var rendered []string

	for i, t := range tabs {
		if tab(i) == m.activeTab {
			rendered = append(rendered, activeTabStyle.Render(t))
		} else {
			rendered = append(rendered, tabStyle.Render(t))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	return tabBarStyle.Width(m.width).Render(bar)
}

func (m *Model) renderContent() string {
	// Handle sub-modes that overlay content
	switch m.subMode {
	case subModeGenerating:
		return m.renderGenerating()
	case subModeDeleteConfirm:
		return m.renderDeleteConfirm()
	case subModeSelectPersona:
		return m.renderSelectPersona()
	case subModePersonaEditor:
		return m.renderPersonaEditor(false)
	case subModeFirstPersona:
		return m.renderPersonaEditor(true)
	case subModeError:
		return m.renderError()
	}

	switch m.activeTab {
	case tabEntries:
		return m.renderEntriesTab()
	case tabPersonas:
		return m.renderPersonasTab()
	case tabDaemon:
		return m.renderDaemonTab()
	case tabSettings:
		return m.renderSettingsTab()
	}

	return ""
}

func (m *Model) renderEntriesTab() string {
	listView := listStyle.Render(m.entryList.View())
	contentView := viewportStyle.Render(m.entryView.View())

	var panels []string
	panels = append(panels, listView, contentView)

	if m.showMetrics {
		panels = append(panels, m.renderMetricsPanel())
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, panels...)
}

func (m *Model) renderPersonasTab() string {
	listView := listStyle.Render(m.personaList.View())
	contentView := viewportStyle.Render(m.personaView.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)
}

func (m *Model) renderDaemonTab() string {
	var content strings.Builder
	contentHeight := m.height - 4

	content.WriteString("\n")
	content.WriteString(titleStyle.Render("Status"))
	content.WriteString("\n\n")

	// Status
	content.WriteString(labelStyle.Render("Status"))
	if m.daemonRunning {
		pid := 0
		if m.daemonState != nil {
			pid = m.daemonState.PID
		}
		content.WriteString(m.daemonSpinner.View() + " ")
		content.WriteString(statusRunning.Render(fmt.Sprintf("Running (PID %d)", pid)))
	} else {
		content.WriteString(statusStopped.Render("Stopped"))
	}
	content.WriteString("\n")

	if m.daemonRunning && m.daemonState != nil {
		content.WriteString(labelStyle.Render("Started"))
		content.WriteString(valueStyle.Render(formatRelativeTime(m.daemonState.StartedAt)))
		content.WriteString("\n")

		content.WriteString(labelStyle.Render("Next entry"))
		content.WriteString(valueStyle.Render(formatRelativeTime(m.daemonState.NextTrigger)))
		content.WriteString("\n")

		content.WriteString(labelStyle.Render("Generated"))
		content.WriteString(valueStyle.Render(fmt.Sprintf("%d entries", m.daemonState.EntriesGenerated)))
		content.WriteString("\n")
	}

	content.WriteString("\n")

	// Config
	content.WriteString(lipgloss.NewStyle().Foreground(colorAccent).Render("Configuration"))
	content.WriteString("\n")

	if m.cfg != nil && m.cfg.Daemon != nil {
		content.WriteString(labelStyle.Render("Rate"))
		content.WriteString(valueStyle.Render(fmt.Sprintf("%d per %s", m.cfg.Daemon.Rate, m.cfg.Daemon.RatePeriod)))
		content.WriteString("\n")

		personas := m.cfg.Daemon.Personas
		if len(personas) == 0 {
			personas = []string{m.cfg.DefaultPersona}
		}
		content.WriteString(labelStyle.Render("Personas"))
		content.WriteString(valueStyle.Render(strings.Join(personas, ", ")))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgDim).Render(
		"Edit ~/.config/jernel/config.yaml to change daemon settings."))

	return contentStyle.Height(contentHeight).Render(content.String())
}

func (m *Model) renderSettingsTab() string {
	var content strings.Builder
	contentHeight := m.height - 4

	content.WriteString("\n")
	content.WriteString(titleStyle.Render("Settings"))
	content.WriteString("\n\n")

	// Paths
	content.WriteString(lipgloss.NewStyle().Foreground(colorAccent).Render("Paths"))
	content.WriteString("\n")

	cfgPath, _ := config.Path()
	personaDir, _ := persona.Dir()
	homeDir, _ := os.UserHomeDir()
	dbPath := homeDir + "/.local/share/jernel/jernel.db"

	content.WriteString(labelStyle.Render("Config"))
	content.WriteString(valueStyle.Render(cfgPath))
	content.WriteString("\n")

	content.WriteString(labelStyle.Render("Personas"))
	content.WriteString(valueStyle.Render(personaDir + "/"))
	content.WriteString("\n")

	content.WriteString(labelStyle.Render("Database"))
	content.WriteString(valueStyle.Render(dbPath))
	content.WriteString("\n\n")

	// API
	content.WriteString(lipgloss.NewStyle().Foreground(colorAccent).Render("API"))
	content.WriteString("\n")

	if m.cfg != nil {
		content.WriteString(labelStyle.Render("Provider"))
		content.WriteString(valueStyle.Render(m.cfg.Provider))
		content.WriteString("\n")

		content.WriteString(labelStyle.Render("Model"))
		content.WriteString(valueStyle.Render(m.cfg.Model))
		content.WriteString("\n")

		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		keyStatus := "Not set"
		if apiKey != "" {
			if len(apiKey) > 8 {
				keyStatus = "****..." + apiKey[len(apiKey)-4:] + " (env)"
			} else {
				keyStatus = "**** (env)"
			}
		}
		content.WriteString(labelStyle.Render("API Key"))
		content.WriteString(valueStyle.Render(keyStatus))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(lipgloss.NewStyle().Foreground(colorFgDim).Render(
		"Edit ~/.config/jernel/config.yaml to change settings."))

	return contentStyle.Height(contentHeight).Render(content.String())
}

func (m *Model) renderMetricsPanel() string {
	panelWidth := m.metricsWidth
	panelHeight := m.height - 6

	var content strings.Builder
	content.WriteString(titleStyle.Render("System"))
	content.WriteString("\n\n")

	if len(m.entries) == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(colorFgDim).Render("No entry selected"))
		return lipgloss.NewStyle().
			Width(panelWidth).
			Height(panelHeight).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(colorBorder).
			Padding(0, 2).
			Render(content.String())
	}

	sel := m.entryList.SelectedItem()
	if sel == nil {
		return ""
	}

	e := sel.(entryItem).entry
	snap := e.MetricsSnapshot

	if snap == nil {
		content.WriteString(lipgloss.NewStyle().Foreground(colorFgDim).Render("No system metrics"))
		return lipgloss.NewStyle().
			Width(panelWidth).
			Height(panelHeight).
			BorderStyle(lipgloss.NormalBorder()).
			BorderLeft(true).
			BorderForeground(colorBorder).
			Padding(0, 2).
			Render(content.String())
	}

	addMetric := func(label, value string) {
		content.WriteString(labelStyle.Width(10).Render(label))
		content.WriteString(valueStyle.Render(value))
		content.WriteString("\n")
	}

	// Machine identity
	addMetric("Type", string(snap.MachineType))
	addMetric("Time", string(snap.TimeOfDay))

	content.WriteString("\n")

	// Core metrics
	addMetric("Uptime", formatDuration(snap.Uptime))
	addMetric("CPU", fmt.Sprintf("%.1f%%", snap.CPUPercent))

	if snap.Thermal != nil && snap.Thermal.CPUTemp != nil {
		addMetric("CPU Temp", fmt.Sprintf("%.0f°C", *snap.Thermal.CPUTemp))
	}

	addMetric("Memory", fmt.Sprintf("%.1f%%", snap.MemoryPercent))
	addMetric("Disk", fmt.Sprintf("%.1f%%", snap.DiskPercent))

	if snap.SwapPercent != nil {
		addMetric("Swap", fmt.Sprintf("%.1f%%", *snap.SwapPercent))
	}

	if snap.ProcessCount != nil {
		addMetric("Procs", fmt.Sprintf("%d", *snap.ProcessCount))
	}

	if snap.LoadAverages != nil {
		content.WriteString("\n")
		addMetric("Load 1m", fmt.Sprintf("%.2f", snap.LoadAverages.Load1))
		addMetric("Load 5m", fmt.Sprintf("%.2f", snap.LoadAverages.Load5))
		addMetric("Load 15m", fmt.Sprintf("%.2f", snap.LoadAverages.Load15))
	}

	if snap.NetworkIO != nil {
		content.WriteString("\n")
		addMetric("Net ↑", fmt.Sprintf("%.2f GB", float64(snap.NetworkIO.BytesSent)/1024/1024/1024))
		addMetric("Net ↓", fmt.Sprintf("%.2f GB", float64(snap.NetworkIO.BytesRecv)/1024/1024/1024))
	}

	if snap.Battery != nil {
		content.WriteString("\n")
		status := fmt.Sprintf("%.0f%%", snap.Battery.Percent)
		// if snap.Battery.Charging {
		// 	status += " ⚡"
		// }
		addMetric("Battery", status)
	}

	if snap.GPU != nil && snap.GPU.Usage != nil {
		content.WriteString("\n")
		addMetric("GPU", fmt.Sprintf("%.1f%%", *snap.GPU.Usage))
	}

	if snap.Thermal != nil && snap.Thermal.GPUTemp != nil {
		addMetric("GPU Temp", fmt.Sprintf("%.0f°C", *snap.Thermal.GPUTemp))
	}

	if len(snap.Fans) > 0 {
		// Show average fan speed
		var totalRPM float64
		for _, fan := range snap.Fans {
			totalRPM += fan.Speed
		}
		avgRPM := totalRPM / float64(len(snap.Fans))
		addMetric("Fan", fmt.Sprintf("%.0f RPM", avgRPM))
	}

	return lipgloss.NewStyle().
		Width(panelWidth).
		Height(panelHeight).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(colorBorder).
		Padding(0, 2).
		Render(content.String())
}

func (m *Model) renderGenerating() string {
	contentHeight := m.height - 4

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		titleStyle.Render("Generating Entry"),
		"",
		m.genSpinner.View()+" Creating with persona "+m.genPersona+"...",
	)

	return lipgloss.Place(m.width, contentHeight,
		lipgloss.Center, lipgloss.Center,
		content)
}

func (m *Model) renderSelectPersona() string {
	contentHeight := m.height - 4

	// Title
	title := titleStyle.Render("Select Persona for New Entry")

	// Left side: persona list
	listView := listStyle.Render(m.personaList.View())

	// Right side: selected persona preview
	var previewContent string
	if sel := m.personaList.SelectedItem(); sel != nil {
		p := sel.(personaItem).persona
		var preview strings.Builder
		preview.WriteString(titleStyle.Render(p.Name))
		preview.WriteString("\n\n")
		// Truncate description for preview
		desc := p.Description
		if len(desc) > 300 {
			desc = desc[:300] + "..."
		}
		preview.WriteString(lipgloss.NewStyle().Foreground(colorFgDim).Render(desc))
		previewContent = preview.String()
	} else {
		previewContent = lipgloss.NewStyle().Foreground(colorFgDim).Render("No persona selected")
	}

	previewStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(m.width/2 - 4).
		Height(contentHeight - 6)

	preview := previewStyle.Render(previewContent)

	// Combine list and preview
	panels := lipgloss.JoinHorizontal(lipgloss.Top, listView, preview)

	content := lipgloss.JoinVertical(lipgloss.Left,
		"",
		title,
		"",
		panels,
	)

	return lipgloss.NewStyle().Height(contentHeight).Render(content)
}

func (m *Model) renderPersonaEditor(isFirstTime bool) string {
	contentHeight := m.height - 4

	// Title
	var title string
	if isFirstTime {
		title = titleStyle.Render("Create Your First Persona")
	} else if m.editorIsNew {
		title = titleStyle.Render("Create Persona")
	} else {
		title = titleStyle.Render("Edit Persona")
	}

	// Instructions for first-time users
	var intro string
	if isFirstTime {
		intro = lipgloss.NewStyle().Foreground(colorFgDim).Width(60).Render(
			"Personas define the voice and personality for your journal entries. " +
				"Here's an example to get you started - feel free to edit it or create your own.")
	}

	// Name field
	nameLabel := "Name"
	if m.editorFocusName {
		nameLabel = lipgloss.NewStyle().Foreground(colorAccent).Render("Name")
	} else {
		nameLabel = lipgloss.NewStyle().Foreground(colorFgDim).Render("Name")
	}

	nameField := lipgloss.JoinVertical(lipgloss.Left,
		nameLabel,
		m.editorNameInput.View(),
	)

	// Description field
	descLabel := "Description"
	if !m.editorFocusName {
		descLabel = lipgloss.NewStyle().Foreground(colorAccent).Render("Description")
	} else {
		descLabel = lipgloss.NewStyle().Foreground(colorFgDim).Render("Description")
	}

	descField := lipgloss.JoinVertical(lipgloss.Left,
		descLabel,
		m.editorDescInput.View(),
	)

	// Help text
	help := lipgloss.NewStyle().Foreground(colorFgDim).Render(
		"Tab: switch field  Ctrl+S: save  Esc: cancel")

	// Combine all elements
	var elements []string
	elements = append(elements, "", title)
	if intro != "" {
		elements = append(elements, "", intro)
	}
	elements = append(elements,
		"",
		nameField,
		"",
		descField,
		"",
		help,
	)

	content := lipgloss.JoinVertical(lipgloss.Left, elements...)

	// Center the content
	return lipgloss.Place(m.width, contentHeight,
		lipgloss.Center, lipgloss.Center,
		content)
}

func (m *Model) renderDeleteConfirm() string {
	contentHeight := m.height - 4

	// Build the confirmation message
	var message string
	if m.deleteEntryCount > 0 {
		entryWord := "entry"
		if m.deleteEntryCount > 1 {
			entryWord = "entries"
		}
		message = fmt.Sprintf("Delete \"%s\" and %d associated %s?", m.deleteTarget, m.deleteEntryCount, entryWord)
	} else {
		message = fmt.Sprintf("Delete \"%s\"?", m.deleteTarget)
	}

	// Warning text if entries will be deleted
	var warning string
	if m.deleteEntryCount > 0 {
		warning = errorStyle.Render("This action cannot be undone.")
	}

	elements := []string{
		"",
		titleStyle.Render("Delete Persona"),
		"",
		message,
	}
	if warning != "" {
		elements = append(elements, "", warning)
	}
	elements = append(elements,
		"",
		lipgloss.NewStyle().Foreground(colorFgDim).Render("Press y to confirm, n to cancel"),
	)

	content := lipgloss.JoinVertical(lipgloss.Center, elements...)

	return lipgloss.Place(m.width, contentHeight,
		lipgloss.Center, lipgloss.Center,
		content)
}

func (m *Model) renderError() string {
	contentHeight := m.height - 4

	errMsg := "An unknown error occurred"
	if m.genError != nil {
		errMsg = m.genError.Error()
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		errorStyle.Render("Error"),
		"",
		lipgloss.NewStyle().Foreground(colorFg).Width(60).Render(errMsg),
		"",
		lipgloss.NewStyle().Foreground(colorFgDim).Render("Press any key to dismiss"),
	)

	return lipgloss.Place(m.width, contentHeight,
		lipgloss.Center, lipgloss.Center,
		content)
}

func (m *Model) renderHelpBar() string {
	var keys []string
	add := func(key, desc string) {
		keys = append(keys, helpKeyStyle.Render(key)+" "+desc)
	}

	add("Tab", "switch")
	add("q", "quit")

	switch m.subMode {
	case subModeSelectPersona:
		keys = nil
		add("Enter", "select")
		add("Esc", "cancel")
		add("↑↓", "navigate")
	case subModePersonaEditor, subModeFirstPersona:
		// Help shown in editor view
		keys = nil
	case subModeDeleteConfirm, subModeError:
		// Help shown in modal
		keys = nil
	default:
		switch m.activeTab {
		case tabEntries:
			add("n", "new")
			add("s", "system")
			add("↑↓", "navigate")
		case tabPersonas:
			add("c", "create")
			add("e", "edit")
			add("d", "delete")
			add("↑↓", "navigate")
		case tabDaemon:
			if m.daemonRunning {
				add("s", "stop")
			} else {
				add("s", "start")
			}
			add("r", "refresh")
		case tabSettings:
			// No special keys
		}
	}

	if len(keys) == 0 {
		return ""
	}

	return helpStyle.Render(strings.Join(keys, "  "))
}

// Helper functions

func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

func formatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < 0 {
		diff = -diff
		switch {
		case diff < time.Minute:
			return "in a moment"
		case diff < time.Hour:
			mins := int(diff.Minutes())
			return fmt.Sprintf("in %d min", mins)
		case diff < 24*time.Hour:
			hours := int(diff.Hours())
			return fmt.Sprintf("in %d hours", hours)
		default:
			days := int(diff.Hours() / 24)
			return fmt.Sprintf("in %d days", days)
		}
	}

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d min ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 02")
	}
}

func getContentPreview(content string, maxLen int) string {
	preview := strings.ReplaceAll(content, "\n", " ")
	preview = strings.ReplaceAll(preview, "#", "")
	preview = strings.ReplaceAll(preview, "*", "")
	preview = strings.TrimSpace(preview)

	if len(preview) > maxLen {
		return preview[:maxLen-1] + "…"
	}
	return preview
}

// formatPersonaName converts a file name like "poor_charlie" to "Poor Charlie"
func formatPersonaName(name string) string {
	// Replace underscores with spaces
	name = strings.ReplaceAll(name, "_", " ")
	// Capitalize each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// truncate shortens a string to maxLen, adding ellipsis if needed
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return s[:maxLen-1] + "…"
}

// Run starts the TUI
func Run(entries []*store.Entry, version string) error {
	m, err := New(entries, version)
	if err != nil {
		return err
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
