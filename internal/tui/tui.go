package tui

import (
	"fmt"
	"strings"

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

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
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
	list     list.Model
	viewport viewport.Model
	entries  []*store.Entry
	ready    bool
	width    int
	height   int
	renderer *glamour.TermRenderer
	quitting bool
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
		list:     l,
		entries:  entries,
		renderer: renderer,
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
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		listWidth := msg.Width / 3
		viewportWidth := msg.Width - listWidth - 4

		m.list.SetSize(listWidth, msg.Height-2)

		headerHeight := 3
		m.viewport = viewport.New(viewportWidth, msg.Height-headerHeight-4)
		m.viewport.Style = viewportStyle

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

	// Build content with metadata header
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Entry #%d\n\n", entry.ID))
	content.WriteString(fmt.Sprintf("**Persona:** %s\n\n", entry.Persona))
	content.WriteString(fmt.Sprintf("**Date:** %s\n\n", entry.CreatedAt.Format("Monday, January 02, 2006 at 3:04 PM")))
	content.WriteString(fmt.Sprintf("**Model:** %s\n\n", entry.ModelID))

	if entry.MetricsSnapshot != nil {
		m := entry.MetricsSnapshot
		content.WriteString(fmt.Sprintf("**System:** CPU %.1f%% | Memory %.1f%% | Disk %.1f%%\n\n",
			m.CPUPercent, m.MemoryPercent, m.DiskPercent))
	}

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

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, contentView)
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
