package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cldixon/jernel/internal/store"
)

// Styles
var (
	listStyle = lipgloss.NewStyle().
			Padding(1, 2)

	viewportStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

	metricsPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")).
				Padding(1, 2)

	metricsTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("63")).
				Bold(true).
				MarginBottom(1)

	metricLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Width(12)

	metricValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Bold(true)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)
)

// entryItem wraps a store.Entry for the list
type entryItem struct {
	entry *store.Entry
}

func (i entryItem) Title() string {
	return fmt.Sprintf("#%d %s", i.entry.ID, i.entry.Persona)
}

func (i entryItem) Description() string {
	return i.entry.CreatedAt.Format("Jan 02, 2006 3:04 PM")
}

func (i entryItem) FilterValue() string {
	return i.entry.Persona
}

// Model is the main TUI model
type Model struct {
	list         list.Model
	viewport     viewport.Model
	entries      []*store.Entry
	ready        bool
	width        int
	height       int
	renderer     *glamour.TermRenderer
	quitting     bool
	showMetrics  bool // toggle for metrics panel
	metricsWidth int  // width of metrics panel when visible
}

// New creates a new TUI model with the given entries
func New(entries []*store.Entry) (*Model, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create renderer: %w", err)
	}

	// Convert entries to list items
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = entryItem{entry: e}
	}

	// Create list with custom delegate
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("205")).
		BorderLeftForeground(lipgloss.Color("205"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("241")).
		BorderLeftForeground(lipgloss.Color("205"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "jernel entries"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle

	return &Model{
		list:         l,
		entries:      entries,
		renderer:     renderer,
		showMetrics:  true, // show metrics panel by default
		metricsWidth: 32,   // default width for metrics panel
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
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "m":
			m.showMetrics = !m.showMetrics
			m.recalculateLayout()
			m.updateViewportContent()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()
		m.ready = true
		m.updateViewportContent()
	}

	// Update list
	var listCmd tea.Cmd
	prevIndex := m.list.Index()
	m.list, listCmd = m.list.Update(msg)
	cmds = append(cmds, listCmd)

	// If selection changed, update viewport
	if prevIndex != m.list.Index() {
		m.updateViewportContent()
	}

	// Update viewport
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// recalculateLayout adjusts component sizes based on current window and panel visibility
func (m *Model) recalculateLayout() {
	listWidth := m.width / 4
	metricsWidth := 0
	if m.showMetrics {
		metricsWidth = m.metricsWidth
	}
	viewportWidth := m.width - listWidth - metricsWidth - 6

	m.list.SetSize(listWidth, m.height-2)

	headerHeight := 3
	m.viewport = viewport.New(viewportWidth, m.height-headerHeight-4)
	m.viewport.Style = viewportStyle
}

// renderMetricsPanel creates the metrics panel content for the selected entry
func (m *Model) renderMetricsPanel() string {
	if len(m.entries) == 0 {
		return metricsPanelStyle.Width(m.metricsWidth - 4).Render("No entry selected")
	}

	selected := m.list.SelectedItem()
	if selected == nil {
		return metricsPanelStyle.Width(m.metricsWidth - 4).Render("No entry selected")
	}

	item := selected.(entryItem)
	entry := item.entry
	snapshot := entry.MetricsSnapshot

	if snapshot == nil {
		return metricsPanelStyle.Width(m.metricsWidth - 4).Render("No metrics available")
	}

	var content strings.Builder
	content.WriteString(metricsTitleStyle.Render("System Metrics"))
	content.WriteString("\n")

	// Helper function to add a metric row
	addMetric := func(label, value string) {
		content.WriteString(metricLabelStyle.Render(label))
		content.WriteString(metricValueStyle.Render(value))
		content.WriteString("\n")
	}

	// Core metrics (always present)
	addMetric("Uptime", formatDuration(snapshot.Uptime))
	addMetric("CPU", fmt.Sprintf("%.1f%%", snapshot.CPUPercent))
	addMetric("Memory", fmt.Sprintf("%.1f%%", snapshot.MemoryPercent))
	addMetric("Disk", fmt.Sprintf("%.1f%%", snapshot.DiskPercent))

	// Optional metrics
	if snapshot.LoadAverages != nil {
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
		sentGB := float64(snapshot.NetworkIO.BytesSent) / 1024 / 1024 / 1024
		recvGB := float64(snapshot.NetworkIO.BytesRecv) / 1024 / 1024 / 1024
		addMetric("Net ↑", fmt.Sprintf("%.2f GB", sentGB))
		addMetric("Net ↓", fmt.Sprintf("%.2f GB", recvGB))
	}

	if snapshot.Battery != nil {
		status := fmt.Sprintf("%.0f%%", snapshot.Battery.Percent)
		if snapshot.Battery.Charging {
			status += " ⚡"
		}
		addMetric("Battery", status)
	}

	return metricsPanelStyle.
		Width(m.metricsWidth - 4).
		Height(m.height - 6).
		Render(content.String())
}

// formatDuration formats a duration in a human-readable way
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

// updateViewportContent renders the selected entry
func (m *Model) updateViewportContent() {
	if len(m.entries) == 0 {
		m.viewport.SetContent("No entries yet. Create one with 'jernel new'")
		return
	}

	selected := m.list.SelectedItem()
	if selected == nil {
		return
	}

	item := selected.(entryItem)
	entry := item.entry

	// Build content with minimal metadata header (metrics moved to side panel)
	var content strings.Builder

	// Option B: Inline pill with brackets
	content.WriteString(fmt.Sprintf("# Entry #%d  「%s」\n\n", entry.ID, entry.Persona))
	content.WriteString(fmt.Sprintf("%s\n\n", entry.CreatedAt.Format("Mon, Jan 02 2006 at 3:04 PM")))
	content.WriteString("---\n\n")
	content.WriteString(entry.Content)

	content.WriteString("\n\n---\n\n")

	// Option C: Colored badge style
	content.WriteString(fmt.Sprintf("# Entry #%d · ◆ %s ◆\n\n", entry.ID, entry.Persona))
	content.WriteString(fmt.Sprintf("%s\n\n", entry.CreatedAt.Format("Mon, Jan 02 2006 at 3:04 PM")))
	content.WriteString("---\n\n")
	content.WriteString(entry.Content)

	rendered, err := m.renderer.Render(content.String())
	if err != nil {
		m.viewport.SetContent(fmt.Sprintf("Error rendering: %v", err))
		return
	}

	m.viewport.SetContent(rendered)
}

// View implements tea.Model
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	if !m.ready {
		return "Loading..."
	}

	listView := listStyle.Render(m.list.View())
	contentView := m.viewport.View()

	// Build the main horizontal layout
	var mainView string
	if m.showMetrics {
		metricsView := m.renderMetricsPanel()
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView, metricsView)
	} else {
		mainView = lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)
	}

	// Add help text at the bottom
	toggleHint := "m: show metrics"
	if m.showMetrics {
		toggleHint = "m: hide metrics"
	}
	helpText := helpStyle.Render(fmt.Sprintf("  q: quit  •  %s  •  ↑/↓: navigate  •  /: filter", toggleHint))

	return lipgloss.JoinVertical(lipgloss.Left, mainView, helpText)
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
