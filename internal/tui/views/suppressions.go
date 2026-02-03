package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-cli/internal/tui/components"
	"github.com/mailersend/mailersend-cli/internal/tui/theme"
	"github.com/mailersend/mailersend-cli/internal/tui/types"
	"github.com/mailersend/mailersend-go"
)

// SuppressionType represents the type of suppression list.
type SuppressionType int

const (
	SuppressionBlocklist SuppressionType = iota
	SuppressionBounces
	SuppressionSpam
	SuppressionUnsubscribes
)

func (s SuppressionType) String() string {
	switch s {
	case SuppressionBlocklist:
		return "Blocklist"
	case SuppressionBounces:
		return "Hard Bounces"
	case SuppressionSpam:
		return "Spam Complaints"
	case SuppressionUnsubscribes:
		return "Unsubscribes"
	default:
		return "Unknown"
	}
}

var (
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2)

	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary).
			Background(theme.BgSelected).
			Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(theme.Muted).
			MarginBottom(1)
)

// SuppressionsView displays suppression lists.
type SuppressionsView struct {
	client        *mailersend.Mailersend
	transport     *sdkclient.CLITransport
	table         components.Table
	detail        components.DetailPanel
	items         []types.SuppressionItem
	loading       bool
	err           error
	width         int
	height        int
	focused       bool
	activeTab     SuppressionType
	tabFocused    bool
	showingDetail bool
}

// NewSuppressionsView creates a new suppressions view.
func NewSuppressionsView(client *mailersend.Mailersend, transport *sdkclient.CLITransport) SuppressionsView {
	columns := []components.Column{
		{Title: "EMAIL/PATTERN", Width: 35},
		{Title: "TYPE/REASON", Width: 25},
		{Title: "CREATED", Width: 19},
	}
	table := components.NewTable(columns)
	table.SetEmptyMessage("No suppression entries found.")

	return SuppressionsView{
		client:    client,
		transport: transport,
		table:     table,
		loading:   true,
		activeTab: SuppressionBlocklist,
	}
}

// SetClient updates the SDK client.
func (v *SuppressionsView) SetClient(client *mailersend.Mailersend, transport *sdkclient.CLITransport) {
	v.client = client
	v.transport = transport
}

// SetSize sets the view dimensions.
func (v *SuppressionsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	// Reserve space for tab bar
	v.table.SetSize(width, height-4)
}

// SetFocused sets whether this view is focused.
func (v *SuppressionsView) SetFocused(focused bool) {
	v.focused = focused
	v.table.SetFocused(focused && !v.tabFocused)
}

// Loading returns whether the view is loading.
func (v SuppressionsView) Loading() bool {
	return v.loading
}

// Error returns any error.
func (v SuppressionsView) Error() error {
	return v.err
}

// ItemCount returns the number of items.
func (v SuppressionsView) ItemCount() int {
	return len(v.items)
}

// Fetch returns a command to fetch suppressions based on active tab.
func (v SuppressionsView) Fetch() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return types.SuppressionsLoadedMsg{Err: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var items []types.SuppressionItem
		var err error

		switch v.activeTab {
		case SuppressionBlocklist:
			items, err = v.fetchBlocklist(ctx)
		case SuppressionBounces:
			items, err = v.fetchBounces(ctx)
		case SuppressionSpam:
			items, err = v.fetchSpam(ctx)
		case SuppressionUnsubscribes:
			items, err = v.fetchUnsubscribes(ctx)
		}

		return types.SuppressionsLoadedMsg{
			Items: items,
			Type:  v.activeTab.String(),
			Err:   err,
		}
	}
}

func (v *SuppressionsView) fetchBlocklist(ctx context.Context) ([]types.SuppressionItem, error) {
	result, _, err := v.client.Suppression.ListBlockList(ctx, nil)
	if err != nil {
		return nil, sdkclient.WrapError(v.transport, err)
	}

	var items []types.SuppressionItem
	for _, b := range result.Data {
		items = append(items, types.SuppressionItem{
			ID:        b.ID,
			Pattern:   b.Pattern,
			Type:      b.Type,
			CreatedAt: b.CreatedAt.Format(time.RFC3339),
		})
	}
	return items, nil
}

func (v *SuppressionsView) fetchBounces(ctx context.Context) ([]types.SuppressionItem, error) {
	result, _, err := v.client.Suppression.ListHardBounces(ctx, nil)
	if err != nil {
		return nil, sdkclient.WrapError(v.transport, err)
	}

	var items []types.SuppressionItem
	for _, b := range result.Data {
		items = append(items, types.SuppressionItem{
			ID:        b.ID,
			Pattern:   b.Recipient.Email,
			Reason:    b.Reason,
			CreatedAt: b.CreatedAt.Format(time.RFC3339),
		})
	}
	return items, nil
}

func (v *SuppressionsView) fetchSpam(ctx context.Context) ([]types.SuppressionItem, error) {
	result, _, err := v.client.Suppression.ListSpamComplaints(ctx, nil)
	if err != nil {
		return nil, sdkclient.WrapError(v.transport, err)
	}

	var items []types.SuppressionItem
	for _, s := range result.Data {
		items = append(items, types.SuppressionItem{
			ID:        s.ID,
			Pattern:   s.Recipient.Email,
			Reason:    "Spam complaint",
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}
	return items, nil
}

func (v *SuppressionsView) fetchUnsubscribes(ctx context.Context) ([]types.SuppressionItem, error) {
	result, _, err := v.client.Suppression.ListUnsubscribes(ctx, nil)
	if err != nil {
		return nil, sdkclient.WrapError(v.transport, err)
	}

	var items []types.SuppressionItem
	for _, u := range result.Data {
		items = append(items, types.SuppressionItem{
			ID:        u.ID,
			Pattern:   u.Recipient.Email,
			Reason:    u.ReadableReason,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}
	return items, nil
}

// Update handles messages for this view.
func (v SuppressionsView) Update(msg tea.Msg) (SuppressionsView, tea.Cmd) {
	switch msg := msg.(type) {
	case types.SuppressionsLoadedMsg:
		v.loading = false
		v.err = msg.Err
		if msg.Err == nil {
			v.items = msg.Items
			v.updateTable()
		}
	}
	return v, nil
}

// HandleKey handles key events when this view is active.
func (v *SuppressionsView) HandleKey(msg tea.KeyMsg) tea.Cmd {
	// Handle detail view navigation
	if v.showingDetail {
		switch msg.String() {
		case "esc", "backspace", "q":
			v.showingDetail = false
		}
		return nil
	}

	switch msg.String() {
	case "j", "down":
		v.table.MoveDown()
	case "k", "up":
		v.table.MoveUp()
	case "g":
		v.table.GotoTop()
	case "G":
		v.table.GotoBottom()
	case "h", "left":
		v.prevTab()
		v.loading = true
		return v.Fetch()
	case "l", "right":
		v.nextTab()
		v.loading = true
		return v.Fetch()
	case "enter":
		v.showDetail()
	case "r":
		v.loading = true
		v.table.SetLoading(true)
		return v.Fetch()
	}
	return nil
}

// SelectedItem returns the currently selected suppression item.
func (v SuppressionsView) SelectedItem() *types.SuppressionItem {
	idx := v.table.Cursor()
	if idx >= 0 && idx < len(v.items) {
		return &v.items[idx]
	}
	return nil
}

func (v *SuppressionsView) showDetail() {
	item := v.SelectedItem()
	if item == nil {
		return
	}

	v.detail.SetTitle("Suppression Entry")

	created := item.CreatedAt
	if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
		created = t.Format("2006-01-02 15:04:05")
	}

	rows := []components.DetailRow{
		{Label: "ID", Value: item.ID},
		{Label: "Email/Pattern", Value: item.Pattern},
		{Label: "List Type", Value: v.activeTab.String()},
	}

	if item.Type != "" {
		rows = append(rows, components.DetailRow{Label: "Type", Value: item.Type})
	}
	if item.Reason != "" {
		rows = append(rows, components.DetailRow{Label: "Reason", Value: item.Reason})
	}

	rows = append(rows, components.DetailRow{Label: "Created", Value: created})

	v.detail.SetRows(rows)
	v.detail.SetSize(v.width, v.height-4)
	v.showingDetail = true
}

func (v *SuppressionsView) nextTab() {
	v.activeTab = (v.activeTab + 1) % 4
}

func (v *SuppressionsView) prevTab() {
	v.activeTab--
	if v.activeTab < 0 {
		v.activeTab = 3
	}
}

func (v *SuppressionsView) updateTable() {
	var rows [][]string
	for _, item := range v.items {
		created := item.CreatedAt
		if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
			created = t.Format("2006-01-02 15:04:05")
		}

		typeOrReason := item.Type
		if item.Reason != "" {
			typeOrReason = item.Reason
		}

		rows = append(rows, []string{
			item.Pattern,
			typeOrReason,
			created,
		})
	}
	v.table.SetRows(rows)
	v.table.SetLoading(false)
}

// View renders the suppressions view.
func (v SuppressionsView) View() string {
	if v.showingDetail {
		return v.detail.View()
	}

	var b strings.Builder

	// Tab bar
	tabs := []SuppressionType{
		SuppressionBlocklist,
		SuppressionBounces,
		SuppressionSpam,
		SuppressionUnsubscribes,
	}

	var tabViews []string
	for _, tab := range tabs {
		style := tabStyle
		if tab == v.activeTab {
			style = activeTabStyle
		}
		tabViews = append(tabViews, style.Render(tab.String()))
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabViews...)
	b.WriteString(tabBarStyle.Render(tabBar))
	b.WriteString("\n")

	// Hint for tab navigation
	hint := fmt.Sprintf("← → to switch tabs | %d items", len(v.items))
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(hint))
	b.WriteString("\n\n")

	// Table
	b.WriteString(v.table.View())

	return b.String()
}

// ShowingDetail returns whether the detail view is active.
func (v SuppressionsView) ShowingDetail() bool {
	return v.showingDetail
}
