package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cldixon/jernel/internal/persona"
	"github.com/cldixon/jernel/internal/store"
)

// View modes
type viewMode int

const (
	viewEntries viewMode = iota
	viewPersonas
)

// Color palette - blue/green theme
var (
	colorPrimary    = lipgloss.Color("#5FAFAF") // teal
	colorSecondary  = lipgloss.Color("#87D787") // soft green
	colorAccent     = lipgloss.Color("#5FD7FF") // sky blue
	colorMuted      = lipgloss.Color("#6C7086") // gray
	colorSubtle     = lipgloss.Color("#45475A") // dark gray
	colorText       = lipgloss.Color("#CDD6F4") // light text
	colorBrightText = lipgloss.Color("#FFFFFF") // white
	colorOverlay    = lipgloss.Color("#1E1E2E") // dark background
)

// Styles
var (
	// List panel
	listStyle = lipgloss.NewStyle().
			Padding(1, 1)

	listTitleStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	// Content viewport
	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(1, 2)

	viewportFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2)

	// Entry header (rendered with lipgloss, not glamour)
	entryTitleStyle = lipgloss.NewStyle().
			Foreground(colorBrightText).
			Bold(true).
			MarginBottom(1)

	personaTagStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Background(lipgloss.Color("#1E3A3A")).
			Padding(0, 1).
			MarginLeft(2)

	entryDateStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true).
			MarginBottom(1)

	dividerStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	// Metrics panel
	metricsPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSubtle).
				Padding(1, 2)

	metricsTitleStyle = lipgloss.NewStyle().
				Foreground(colorSecondary).
				Bold(true).
				MarginBottom(1)

	metricLabelStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Width(12)

	metricValueStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Bold(true)

	// Help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 2)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Scroll indicator
	scrollIndicatorStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	// Empty state
	emptyStateStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Align(lipgloss.Center)

	emptyStateTitleStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true).
				MarginBottom(1)

	// Modal overlay styles
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true).
			Padding(0, 1)

	personaFileStyle = lipgloss.NewStyle().
				Foreground(colorAccent)

	personaNameStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Bold(true)

	personaDescStyle = lipgloss.NewStyle().
				Foreground(colorMuted)
)

// entryItem wraps a store.Entry for the list
type entryItem struct {
	entry *store.Entry
}

func (i entryItem) Title() string {
	return fmt.Sprintf("#%d  %s", i.entry.ID, i.entry.Persona)
}

func (i entryItem) Description() string {
	relTime := formatRelativeTime(i.entry.CreatedAt)
	preview := getContentPreview(i.entry.Content, 40)
	return fmt.Sprintf("%s · %s", relTime, preview)
}

func (i entryItem) FilterValue() string {
	return i.entry.Persona + " " + i.entry.Content
}

// personaItem wraps a persona for the list
type personaItem struct {
	persona *persona.Persona
}

func (i personaItem) Title() string {
	return i.persona.Name + ".md"
}

func (i personaItem) Description() string {
	preview := getContentPreview(i.persona.Description, 50)
	return preview
}

func (i personaItem) FilterValue() string {
	return i.persona.Name + " " + i.persona.Description
}

// Model is the main TUI model
type Model struct {
	// Main view
	list         list.Model
	viewport     viewport.Model
	entries      []*store.Entry
	ready        bool
	width        int
	height       int
	renderer     *glamour.TermRenderer
	quitting     bool
	showMetrics  bool
	metricsWidth int

	// Persona modal
	viewMode        viewMode
	personaList     list.Model
	personaViewport viewport.Model
	personas        []*persona.Persona
}

// editorFinishedMsg is sent when the external editor closes
type editorFinishedMsg struct{ err error }

// New creates a new TUI model with the given entries
func New(entries []*store.Entry) (*Model, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(70),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	// Convert entries to list items
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = entryItem{entry: e}
	}

	// Create entry list with custom delegate
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(colorSecondary).
		BorderLeftForeground(colorSecondary)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(colorMuted).
		BorderLeftForeground(colorSecondary)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(colorText)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(colorSubtle)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.
		Foreground(colorSubtle)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.
		Foreground(colorSubtle)

	l := list.New(items, delegate, 0, 0)
	l.Title = fmt.Sprintf("📓 jernel (%d entries)", len(entries))
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = listTitleStyle
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(colorAccent)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorSecondary)

	return &Model{
		list:         l,
		entries:      entries,
		renderer:     renderer,
		showMetrics:  true,
		metricsWidth: 32,
		viewMode:     viewEntries,
	}, nil
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case editorFinishedMsg:
		// Reload personas after editing
		if m.viewMode == viewPersonas {
			m.loadPersonas()
			m.updatePersonaViewportContent()
		}
		return m, nil

	case tea.KeyMsg:
		// Handle view-specific keys
		if m.viewMode == viewPersonas {
			return m.updatePersonaView(msg)
		}
		return m.updateEntriesView(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()
		m.ready = true
		m.updateViewportContent()
		if m.viewMode == viewPersonas {
			m.recalculatePersonaLayout()
			m.updatePersonaViewportContent()
		}
	}

	// Update list
	var listCmd tea.Cmd
	prevIndex := m.list.Index()
	m.list, listCmd = m.list.Update(msg)
	cmds = append(cmds, listCmd)

	if prevIndex != m.list.Index() {
		m.updateViewportContent()
	}

	// Update viewport
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// updateEntriesView handles input for the main entries view
func (m *Model) updateEntriesView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "m":
		m.showMetrics = !m.showMetrics
		m.recalculateLayout()
		m.updateViewportContent()
		return m, nil
	case "p":
		m.viewMode = viewPersonas
		m.loadPersonas()
		m.recalculatePersonaLayout()
		m.updatePersonaViewportContent()
		return m, nil
	}
	return m, nil
}

// updatePersonaView handles input for the persona modal
func (m *Model) updatePersonaView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "p", "esc":
		m.viewMode = viewEntries
		return m, nil
	case "e":
		// Open selected persona in editor
		return m, m.openPersonaInEditor()
	}

	// Update persona list
	var cmd tea.Cmd
	prevIndex := m.personaList.Index()
	m.personaList, cmd = m.personaList.Update(msg)

	if prevIndex != m.personaList.Index() {
		m.updatePersonaViewportContent()
	}

	// Update persona viewport for scrolling
	var vpCmd tea.Cmd
	m.personaViewport, vpCmd = m.personaViewport.Update(msg)

	return m, tea.Batch(cmd, vpCmd)
}

// loadPersonas loads all personas from disk
func (m *Model) loadPersonas() {
	names, err := persona.List()
	if err != nil {
		return
	}

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

	// Create persona list
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(colorSecondary).
		BorderLeftForeground(colorSecondary)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(colorMuted).
		BorderLeftForeground(colorSecondary)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(colorText)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(colorSubtle)

	m.personaList = list.New(items, delegate, 0, 0)
	m.personaList.Title = "📁 personas/"
	m.personaList.SetShowStatusBar(false)
	m.personaList.SetFilteringEnabled(true)
	m.personaList.Styles.Title = modalTitleStyle
	m.personaList.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(colorAccent)
	m.personaList.Styles.FilterCursor = lipgloss.NewStyle().Foreground(colorSecondary)
}

// recalculatePersonaLayout sets up the persona modal layout
func (m *Model) recalculatePersonaLayout() {
	modalWidth := m.width * 3 / 4
	modalHeight := m.height * 3 / 4
	listWidth := modalWidth / 3

	m.personaList.SetSize(listWidth, modalHeight-6)
	m.personaViewport = viewport.New(modalWidth-listWidth-8, modalHeight-6)
	m.personaViewport.Style = lipgloss.NewStyle().Padding(0, 1)
}

// updatePersonaViewportContent renders the selected persona
func (m *Model) updatePersonaViewportContent() {
	if len(m.personas) == 0 {
		m.personaViewport.SetContent(
			lipgloss.NewStyle().Foreground(colorMuted).Render("No personas found"))
		return
	}

	selected := m.personaList.SelectedItem()
	if selected == nil {
		return
	}

	item := selected.(personaItem)
	p := item.persona

	var content strings.Builder

	// File name header
	content.WriteString(personaFileStyle.Render(p.Name+".md") + "\n\n")

	// Frontmatter
	content.WriteString(dividerStyle.Render("---") + "\n")
	content.WriteString(personaNameStyle.Render("name: ") + lipgloss.NewStyle().Foreground(colorText).Render(p.Name) + "\n")
	content.WriteString(dividerStyle.Render("---") + "\n\n")

	// Description
	content.WriteString(personaDescStyle.Render(p.Description))

	m.personaViewport.SetContent(content.String())
}

// openPersonaInEditor opens the selected persona file in $EDITOR
func (m *Model) openPersonaInEditor() tea.Cmd {
	selected := m.personaList.SelectedItem()
	if selected == nil {
		return nil
	}

	item := selected.(personaItem)
	p := item.persona

	// Get persona file path
	dir, err := persona.Dir()
	if err != nil {
		return nil
	}
	filePath := dir + "/" + p.Name + ".md"

	// Get editor from environment
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim" // fallback
	}

	c := exec.Command(editor, filePath)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err}
	})
}

// recalculateLayout adjusts component sizes based on current window and panel visibility
func (m *Model) recalculateLayout() {
	listWidth := m.width / 4
	if listWidth < 30 {
		listWidth = 30
	}
	metricsWidth := 0
	if m.showMetrics {
		metricsWidth = m.metricsWidth
	}
	viewportWidth := m.width - listWidth - metricsWidth - 6

	m.list.SetSize(listWidth, m.height-3)

	m.viewport = viewport.New(viewportWidth, m.height-5)
	m.viewport.Style = viewportStyle
}

// renderMetricsPanel creates the metrics panel content for the selected entry
func (m *Model) renderMetricsPanel() string {
	panelWidth := m.metricsWidth - 4
	panelHeight := m.height - 6

	if len(m.entries) == 0 {
		return metricsPanelStyle.
			Width(panelWidth).
			Height(panelHeight).
			Render(metricsTitleStyle.Render("System Metrics") + "\n\n" +
				lipgloss.NewStyle().Foreground(colorMuted).Render("No entry selected"))
	}

	selected := m.list.SelectedItem()
	if selected == nil {
		return metricsPanelStyle.
			Width(panelWidth).
			Height(panelHeight).
			Render(metricsTitleStyle.Render("System Metrics") + "\n\n" +
				lipgloss.NewStyle().Foreground(colorMuted).Render("No entry selected"))
	}

	item := selected.(entryItem)
	entry := item.entry
	snapshot := entry.MetricsSnapshot

	if snapshot == nil {
		return metricsPanelStyle.
			Width(panelWidth).
			Height(panelHeight).
			Render(metricsTitleStyle.Render("System Metrics") + "\n\n" +
				lipgloss.NewStyle().Foreground(colorMuted).Render("No metrics data"))
	}

	var content strings.Builder
	content.WriteString(metricsTitleStyle.Render("System Metrics"))
	content.WriteString("\n\n")

	addMetric := func(label, value string) {
		content.WriteString(metricLabelStyle.Render(label))
		content.WriteString(metricValueStyle.Render(value))
		content.WriteString("\n")
	}

	addMetric("Uptime", formatDuration(snapshot.Uptime))
	addMetric("CPU", fmt.Sprintf("%.1f%%", snapshot.CPUPercent))
	addMetric("Memory", fmt.Sprintf("%.1f%%", snapshot.MemoryPercent))
	addMetric("Disk", fmt.Sprintf("%.1f%%", snapshot.DiskPercent))

	if snapshot.LoadAverages != nil {
		content.WriteString("\n")
		addMetric("Load (1m)", fmt.Sprintf("%.2f", snapshot.LoadAverages.Load1))
		addMetric("Load (5m)", fmt.Sprintf("%.2f", snapshot.LoadAverages.Load5))
		addMetric("Load (15m)", fmt.Sprintf("%.2f", snapshot.LoadAverages.Load15))
	}

	if snapshot.SwapPercent != nil {
		addMetric("Swap", fmt.Sprintf("%.1f%%", *snapshot.SwapPercent))
	}

	if snapshot.ProcessCount != nil {
		addMetric("Processes", fmt.Sprintf("%d", *snapshot.ProcessCount))
	}

	if snapshot.NetworkIO != nil {
		content.WriteString("\n")
		sentGB := float64(snapshot.NetworkIO.BytesSent) / 1024 / 1024 / 1024
		recvGB := float64(snapshot.NetworkIO.BytesRecv) / 1024 / 1024 / 1024
		addMetric("Net ↑", fmt.Sprintf("%.2f GB", sentGB))
		addMetric("Net ↓", fmt.Sprintf("%.2f GB", recvGB))
	}

	if snapshot.Battery != nil {
		content.WriteString("\n")
		status := fmt.Sprintf("%.0f%%", snapshot.Battery.Percent)
		if snapshot.Battery.Charging {
			status += " ⚡"
		}
		addMetric("Battery", status)
	}

	return metricsPanelStyle.
		Width(panelWidth).
		Height(panelHeight).
		Render(content.String())
}

// renderEntryHeader creates a styled header for the entry
func (m *Model) renderEntryHeader(entry *store.Entry) string {
	var header strings.Builder

	title := entryTitleStyle.Render(fmt.Sprintf("Entry #%d", entry.ID))
	tag := personaTagStyle.Render(entry.Persona)
	header.WriteString(title + tag + "\n")

	date := entryDateStyle.Render(entry.CreatedAt.Format("Monday, January 02, 2006 at 3:04 PM"))
	header.WriteString(date + "\n\n")

	dividerWidth := 50
	divider := dividerStyle.Render(strings.Repeat("─", dividerWidth))
	header.WriteString(divider + "\n\n")

	return header.String()
}

// updateViewportContent renders the selected entry
func (m *Model) updateViewportContent() {
	if len(m.entries) == 0 {
		m.viewport.SetContent(m.renderEmptyState())
		return
	}

	selected := m.list.SelectedItem()
	if selected == nil {
		return
	}

	item := selected.(entryItem)
	entry := item.entry

	header := m.renderEntryHeader(entry)

	renderedContent, err := m.renderer.Render(entry.Content)
	if err != nil {
		m.viewport.SetContent(fmt.Sprintf("Error rendering: %v", err))
		return
	}

	m.viewport.SetContent(header + renderedContent)
}

// renderEmptyState creates a friendly empty state message
func (m *Model) renderEmptyState() string {
	art := `
    ╭──────────────────────╮
    │                      │
    │   📓  No entries     │
    │       yet!           │
    │                      │
    ╰──────────────────────╯
`
	title := emptyStateTitleStyle.Render("Your journal awaits...")
	hint := lipgloss.NewStyle().Foreground(colorMuted).Render("Create your first entry with:\n\n") +
		lipgloss.NewStyle().Foreground(colorAccent).Bold(true).Render("  jernel new")

	return emptyStateStyle.Render(art + "\n" + title + "\n\n" + hint)
}

// renderScrollIndicator shows scroll position if content overflows
func (m *Model) renderScrollIndicator() string {
	if m.viewport.TotalLineCount() <= m.viewport.VisibleLineCount() {
		return ""
	}

	percent := m.viewport.ScrollPercent() * 100
	return scrollIndicatorStyle.Render(fmt.Sprintf(" ↕ %.0f%%", percent))
}

// renderHelpBar creates the bottom help bar for entries view
func (m *Model) renderHelpBar() string {
	var keys []string

	addKey := func(key, desc string) {
		keys = append(keys, helpKeyStyle.Render(key)+helpDescStyle.Render(" "+desc))
	}

	addKey("q", "quit")
	addKey("p", "personas")
	if m.showMetrics {
		addKey("m", "hide metrics")
	} else {
		addKey("m", "show metrics")
	}
	addKey("↑↓", "navigate")
	addKey("/", "filter")

	scrollIndicator := m.renderScrollIndicator()

	helpContent := strings.Join(keys, helpDescStyle.Render("  •  "))
	return helpStyle.Render(helpContent + scrollIndicator)
}

// renderPersonaHelpBar creates the help bar for persona view
func (m *Model) renderPersonaHelpBar() string {
	var keys []string

	addKey := func(key, desc string) {
		keys = append(keys, helpKeyStyle.Render(key)+helpDescStyle.Render(" "+desc))
	}

	addKey("esc/p", "back")
	addKey("e", "edit")
	addKey("↑↓", "navigate")
	addKey("/", "filter")

	helpContent := strings.Join(keys, helpDescStyle.Render("  •  "))
	return helpStyle.Render(helpContent)
}

// renderPersonaModal renders the persona browser modal
func (m *Model) renderPersonaModal() string {
	modalWidth := m.width * 3 / 4
	modalHeight := m.height * 3 / 4

	// List pane
	listView := m.personaList.View()

	// Content pane
	contentView := m.personaViewport.View()

	// Combine list and content horizontally
	innerContent := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(modalWidth/3).Render(listView),
		lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colorSubtle).
			PaddingLeft(2).
			Render(contentView),
	)

	// Wrap in modal border
	modal := modalStyle.
		Width(modalWidth).
		Height(modalHeight).
		Render(innerContent)

	// Center the modal
	return lipgloss.Place(m.width, m.height-2,
		lipgloss.Center, lipgloss.Center,
		modal,
		lipgloss.WithWhitespaceChars(" "),
		lipgloss.WithWhitespaceForeground(colorOverlay),
	)
}

// View implements tea.Model
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return lipgloss.NewStyle().Foreground(colorPrimary).Render("  Loading...")
	}

	// Persona modal view
	if m.viewMode == viewPersonas {
		modalView := m.renderPersonaModal()
		helpBar := m.renderPersonaHelpBar()
		return lipgloss.JoinVertical(lipgloss.Left, modalView, helpBar)
	}

	// Main entries view
	listView := listStyle.Render(m.list.View())
	contentView := m.viewport.View()

	var mainView string
	if m.showMetrics {
		metricsView := m.renderMetricsPanel()
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView, metricsView)
	} else {
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)
	}

	helpBar := m.renderHelpBar()

	return lipgloss.JoinVertical(lipgloss.Left, mainView, helpBar)
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

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		mins := int(diff.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
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

// Run starts the TUI
func Run(entries []*store.Entry) error {
	m, err := New(entries)
	if err != nil {
		return err
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
