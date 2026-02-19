package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/mailersend/mailersend-cli/internal/tui/components"
	"github.com/mailersend/mailersend-cli/internal/tui/theme"
	"github.com/mailersend/mailersend-cli/internal/tui/types"
	"github.com/mailersend/mailersend-cli/internal/tui/views"
	"github.com/mailersend/mailersend-go"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary).
			Padding(0, 1)

	headerBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(theme.Muted)

	contentStyle = lipgloss.NewStyle().
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(theme.Error)
)

// FocusArea represents which area of the UI is focused.
type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusContent
)

// App is the main TUI application model.
type App struct {
	// SDK
	client  *mailersend.Mailersend
	profile string

	// Components
	sidebar   components.Sidebar
	statusbar components.StatusBar
	spinner   components.Spinner
	help      components.Help
	keys      KeyMap

	// Views
	domains      views.DomainsView
	activity     views.ActivityView
	analytics    views.AnalyticsView
	messages     views.MessagesView
	suppressions views.SuppressionsView

	// State
	activeView  types.ViewType
	focus       FocusArea
	width       int
	height      int
	showHelp    bool
	err         error
	initialized bool
}

// NewApp creates a new TUI application.
func NewApp(client *mailersend.Mailersend, profile string) *App {
	keys := DefaultKeyMap()

	app := &App{
		client: client,

		profile:   profile,
		keys:      keys,
		sidebar:   components.NewSidebar(),
		statusbar: components.NewStatusBar(),
		spinner:   components.NewSpinner("Loading..."),
		help:      components.NewHelp(keys.HelpBindings()),
		focus:     FocusContent,
	}

	// Initialize views
	app.domains = views.NewDomainsView(client)
	app.activity = views.NewActivityView(client)
	app.analytics = views.NewAnalyticsView(client)
	app.messages = views.NewMessagesView(client)
	app.suppressions = views.NewSuppressionsView(client)

	// Set initial focus
	app.sidebar.SetFocused(false)
	app.domains.SetFocused(true)

	return app
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Init(),
		a.fetchCurrentView(),
	)
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateLayout()
		if !a.initialized {
			a.initialized = true
		}

	case tea.KeyMsg:
		// Handle help overlay first
		if a.showHelp {
			if key.Matches(msg, a.keys.Help) || key.Matches(msg, a.keys.Back) {
				a.showHelp = false
				return a, nil
			}
			return a, nil
		}

		// Global keys
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.Help):
			a.showHelp = true
			return a, nil
		case key.Matches(msg, a.keys.Tab):
			a.toggleFocus()
			return a, nil
		case key.Matches(msg, a.keys.View1):
			return a, a.switchView(types.ViewDomains)
		case key.Matches(msg, a.keys.View2):
			return a, a.switchView(types.ViewActivity)
		case key.Matches(msg, a.keys.View3):
			return a, a.switchView(types.ViewAnalytics)
		case key.Matches(msg, a.keys.View4):
			return a, a.switchView(types.ViewMessages)
		case key.Matches(msg, a.keys.View5):
			return a, a.switchView(types.ViewSuppressions)
		}

		// Focus-specific keys
		if a.focus == FocusSidebar {
			cmd := a.handleSidebarKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			cmd := a.handleContentKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	// Handle data loaded messages
	case types.DomainsLoadedMsg:
		a.domains, _ = a.domains.Update(msg)
		a.updateStatusBar()
	case views.ActivityDomainsLoadedMsg:
		var cmd tea.Cmd
		a.activity, cmd = a.activity.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		a.updateStatusBar()
	case types.ActivityLoadedMsg:
		a.activity, _ = a.activity.Update(msg)
		a.updateStatusBar()
	case types.AnalyticsLoadedMsg:
		a.analytics, _ = a.analytics.Update(msg)
		a.updateStatusBar()
	case types.MessagesLoadedMsg:
		a.messages, _ = a.messages.Update(msg)
		a.updateStatusBar()
	case types.MessageDetailLoadedMsg:
		a.messages, _ = a.messages.Update(msg)
	case types.SuppressionsLoadedMsg:
		a.suppressions, _ = a.suppressions.Update(msg)
		a.updateStatusBar()

	case types.ErrorMsg:
		a.err = msg.Err
	}

	// Update spinner
	var spinnerCmd tea.Cmd
	a.spinner, spinnerCmd = a.spinner.Update(msg)
	if spinnerCmd != nil {
		cmds = append(cmds, spinnerCmd)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) toggleFocus() {
	if a.focus == FocusSidebar {
		a.focus = FocusContent
		a.sidebar.SetFocused(false)
		a.setCurrentViewFocused(true)
	} else {
		a.focus = FocusSidebar
		a.sidebar.SetFocused(true)
		a.setCurrentViewFocused(false)
	}
}

func (a *App) handleSidebarKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, a.keys.Down):
		a.sidebar.Next()
		return a.switchView(a.sidebar.Active())
	case key.Matches(msg, a.keys.Up):
		a.sidebar.Prev()
		return a.switchView(a.sidebar.Active())
	case key.Matches(msg, a.keys.Enter), key.Matches(msg, a.keys.Right):
		a.focus = FocusContent
		a.sidebar.SetFocused(false)
		a.setCurrentViewFocused(true)
	}
	return nil
}

func (a *App) handleContentKey(msg tea.KeyMsg) tea.Cmd {
	switch a.activeView {
	case types.ViewDomains:
		return a.domains.HandleKey(msg)
	case types.ViewActivity:
		return a.activity.HandleKey(msg)
	case types.ViewAnalytics:
		return a.analytics.HandleKey(msg)
	case types.ViewMessages:
		return a.messages.HandleKey(msg)
	case types.ViewSuppressions:
		return a.suppressions.HandleKey(msg)
	}
	return nil
}

func (a *App) switchView(v types.ViewType) tea.Cmd {
	if a.activeView == v {
		return nil
	}

	a.setCurrentViewFocused(false)
	a.activeView = v
	a.sidebar.SetActive(v)
	a.setCurrentViewFocused(a.focus == FocusContent)
	a.updateStatusBar()

	return a.fetchCurrentView()
}

func (a *App) setCurrentViewFocused(focused bool) {
	switch a.activeView {
	case types.ViewDomains:
		a.domains.SetFocused(focused)
	case types.ViewActivity:
		a.activity.SetFocused(focused)
	case types.ViewAnalytics:
		a.analytics.SetFocused(focused)
	case types.ViewMessages:
		a.messages.SetFocused(focused)
	case types.ViewSuppressions:
		a.suppressions.SetFocused(focused)
	}
}

func (a *App) fetchCurrentView() tea.Cmd {
	a.spinner.Start()
	a.spinner.SetLabel("Loading " + a.activeView.String() + "...")

	switch a.activeView {
	case types.ViewDomains:
		return a.domains.Fetch()
	case types.ViewActivity:
		return a.activity.Fetch()
	case types.ViewAnalytics:
		return a.analytics.Fetch()
	case types.ViewMessages:
		return a.messages.Fetch()
	case types.ViewSuppressions:
		return a.suppressions.Fetch()
	}
	return nil
}

func (a *App) updateLayout() {
	// Header takes 2 lines, status bar takes 2 lines
	contentHeight := a.height - 4

	a.sidebar.SetHeight(contentHeight)
	a.statusbar.SetWidth(a.width)
	a.help.SetSize(a.width, a.height)

	// Content width is total minus sidebar
	contentWidth := a.width - a.sidebar.Width() - 2

	a.domains.SetSize(contentWidth, contentHeight)
	a.activity.SetSize(contentWidth, contentHeight)
	a.analytics.SetSize(contentWidth, contentHeight)
	a.messages.SetSize(contentWidth, contentHeight)
	a.suppressions.SetSize(contentWidth, contentHeight)

	a.updateStatusBar()
}

func (a *App) updateStatusBar() {
	a.statusbar.SetProfile(a.profile)

	// Get current view info
	viewName := a.activeView.String()
	itemCount := 0
	loading := false

	switch a.activeView {
	case types.ViewDomains:
		itemCount = a.domains.ItemCount()
		loading = a.domains.Loading()
	case types.ViewActivity:
		itemCount = a.activity.ItemCount()
		loading = a.activity.Loading()
	case types.ViewAnalytics:
		itemCount = a.analytics.ItemCount()
		loading = a.analytics.Loading()
	case types.ViewMessages:
		itemCount = a.messages.ItemCount()
		loading = a.messages.Loading()
	case types.ViewSuppressions:
		itemCount = a.suppressions.ItemCount()
		loading = a.suppressions.Loading()
	}

	if loading {
		a.statusbar.SetLeft(viewName)
		a.statusbar.SetLoading(true, "Loading...")
		a.spinner.Start()
	} else {
		a.statusbar.SetLeft(fmt.Sprintf("%s (%d)", viewName, itemCount))
		a.statusbar.SetLoading(false, "")
		a.spinner.Stop()
	}
}

// View implements tea.Model.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Initializing..."
	}

	var b strings.Builder

	// Header
	header := a.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Main content area
	mainContent := a.renderMainContent()
	b.WriteString(mainContent)

	// Status bar
	b.WriteString("\n")
	b.WriteString(a.statusbar.View())

	// Help overlay (rendered on top)
	if a.showHelp {
		// Clear and render help overlay
		return a.help.View()
	}

	return b.String()
}

func (a *App) renderHeader() string {
	title := headerStyle.Render("MailerSend Dashboard")
	profile := lipgloss.NewStyle().Foreground(theme.Muted).Render("profile: " + a.profile)

	// Calculate spacing
	gap := a.width - lipgloss.Width(title) - lipgloss.Width(profile) - 4
	if gap < 1 {
		gap = 1
	}

	content := title + strings.Repeat(" ", gap) + profile
	return headerBarStyle.Width(a.width).Render(content)
}

func (a *App) renderMainContent() string {
	sidebar := a.sidebar.View()

	// Render active view
	var content string
	switch a.activeView {
	case types.ViewDomains:
		content = a.domains.View()
	case types.ViewActivity:
		content = a.activity.View()
	case types.ViewAnalytics:
		content = a.analytics.View()
	case types.ViewMessages:
		content = a.messages.View()
	case types.ViewSuppressions:
		content = a.suppressions.View()
	}

	// Add error display if present
	if a.err != nil {
		content = errorStyle.Render("Error: "+a.err.Error()) + "\n\n" + content
	}

	contentWidth := a.width - a.sidebar.Width() - 4
	styledContent := contentStyle.Width(contentWidth).Render(content)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, styledContent)
}
