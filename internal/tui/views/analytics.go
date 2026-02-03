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

var (
	statBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Muted).
			Padding(0, 2).
			Margin(0, 1)

	statLabelStyle = lipgloss.NewStyle().
			Foreground(theme.Muted)

	statValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Primary)

	statGoodStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Success)

	statBadStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(theme.Error)
)

// AnalyticsView displays analytics data.
type AnalyticsView struct {
	client    *mailersend.Mailersend
	transport *sdkclient.CLITransport
	table     components.Table
	data      types.AnalyticsData
	loading   bool
	err       error
	width     int
	height    int
	focused   bool
	dateRange string // "7d", "30d", "90d"
}

// NewAnalyticsView creates a new analytics view.
func NewAnalyticsView(client *mailersend.Mailersend, transport *sdkclient.CLITransport) AnalyticsView {
	columns := []components.Column{
		{Title: "DATE", Width: 12},
		{Title: "SENT", Width: 10},
		{Title: "DELIVERED", Width: 10},
		{Title: "OPENS", Width: 10},
		{Title: "CLICKS", Width: 10},
		{Title: "BOUNCED", Width: 10},
	}
	table := components.NewTable(columns)
	table.SetEmptyMessage("No analytics data found.")

	return AnalyticsView{
		client:    client,
		transport: transport,
		table:     table,
		loading:   true,
		dateRange: "7d",
	}
}

// SetClient updates the SDK client.
func (v *AnalyticsView) SetClient(client *mailersend.Mailersend, transport *sdkclient.CLITransport) {
	v.client = client
	v.transport = transport
}

// SetSize sets the view dimensions.
func (v *AnalyticsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	// Reserve space for summary stats
	v.table.SetSize(width, height-6)
}

// SetFocused sets whether this view is focused.
func (v *AnalyticsView) SetFocused(focused bool) {
	v.focused = focused
	v.table.SetFocused(focused)
}

// Loading returns whether the view is loading.
func (v AnalyticsView) Loading() bool {
	return v.loading
}

// Error returns any error.
func (v AnalyticsView) Error() error {
	return v.err
}

// ItemCount returns the number of items.
func (v AnalyticsView) ItemCount() int {
	return len(v.data.Stats)
}

func (v AnalyticsView) daysFromRange() int {
	switch v.dateRange {
	case "30d":
		return 30
	case "90d":
		return 90
	default:
		return 7
	}
}

// Fetch returns a command to fetch analytics.
func (v AnalyticsView) Fetch() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return types.AnalyticsLoadedMsg{Err: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		now := time.Now()
		days := v.daysFromRange()
		dateFrom := now.AddDate(0, 0, -days).Unix()
		dateTo := now.Unix()

		events := []string{"sent", "delivered", "opened", "clicked", "hard_bounced", "soft_bounced"}

		result, _, err := v.client.Analytics.GetActivityByDate(ctx, &mailersend.AnalyticsOptions{
			DateFrom: dateFrom,
			DateTo:   dateTo,
			GroupBy:  "days",
			Event:    events,
		})

		if err != nil {
			return types.AnalyticsLoadedMsg{Err: sdkclient.WrapError(v.transport, err)}
		}

		// Calculate totals
		data := types.AnalyticsData{
			Stats:    result.Data.Stats,
			DateFrom: now.AddDate(0, 0, -days).Format("2006-01-02"),
			DateTo:   now.Format("2006-01-02"),
			GroupBy:  "days",
		}

		for _, stat := range result.Data.Stats {
			data.Sent += stat.Sent
			data.Delivered += stat.Delivered
			data.Opens += stat.Opened
			data.Clicks += stat.Clicked
			data.Bounced += stat.HardBounced + stat.SoftBounced
		}

		return types.AnalyticsLoadedMsg{
			Data: data,
			Err:  nil,
		}
	}
}

// Update handles messages for this view.
func (v AnalyticsView) Update(msg tea.Msg) (AnalyticsView, tea.Cmd) {
	switch msg := msg.(type) {
	case types.AnalyticsLoadedMsg:
		v.loading = false
		v.err = msg.Err
		if msg.Err == nil {
			v.data = msg.Data
			v.updateTable()
		}
	}
	return v, nil
}

// HandleKey handles key events when this view is active.
func (v *AnalyticsView) HandleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "j", "down":
		v.table.MoveDown()
	case "k", "up":
		v.table.MoveUp()
	case "g":
		v.table.GotoTop()
	case "G":
		v.table.GotoBottom()
	case "r":
		v.loading = true
		v.table.SetLoading(true)
		return v.Fetch()
	case "1":
		v.dateRange = "7d"
		v.loading = true
		return v.Fetch()
	case "2":
		v.dateRange = "30d"
		v.loading = true
		return v.Fetch()
	case "3":
		v.dateRange = "90d"
		v.loading = true
		return v.Fetch()
	}
	return nil
}

func (v *AnalyticsView) updateTable() {
	var rows [][]string
	for _, stat := range v.data.Stats {
		rows = append(rows, []string{
			stat.Date,
			fmt.Sprintf("%d", stat.Sent),
			fmt.Sprintf("%d", stat.Delivered),
			fmt.Sprintf("%d", stat.Opened),
			fmt.Sprintf("%d", stat.Clicked),
			fmt.Sprintf("%d", stat.HardBounced+stat.SoftBounced),
		})
	}
	v.table.SetRows(rows)
	v.table.SetLoading(false)
}

// View renders the analytics view.
func (v AnalyticsView) View() string {
	var b strings.Builder

	// Summary stats row
	stats := []struct {
		label string
		value int
		style lipgloss.Style
	}{
		{"Sent", v.data.Sent, statValueStyle},
		{"Delivered", v.data.Delivered, statGoodStyle},
		{"Opens", v.data.Opens, statValueStyle},
		{"Clicks", v.data.Clicks, statValueStyle},
		{"Bounced", v.data.Bounced, statBadStyle},
	}

	var boxes []string
	for _, s := range stats {
		box := statBoxStyle.Render(
			statLabelStyle.Render(s.label) + "\n" +
				s.style.Render(fmt.Sprintf("%d", s.value)),
		)
		boxes = append(boxes, box)
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, boxes...))
	b.WriteString("\n\n")

	// Date range indicator
	rangeText := fmt.Sprintf("Date range: %s to %s (%s)", v.data.DateFrom, v.data.DateTo, v.dateRange)
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(rangeText))
	b.WriteString("  ")
	b.WriteString(lipgloss.NewStyle().Foreground(theme.Key).Render("[1]7d [2]30d [3]90d"))
	b.WriteString("\n\n")

	// Table
	b.WriteString(v.table.View())

	return b.String()
}
