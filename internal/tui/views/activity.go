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
	domainTabStyle = lipgloss.NewStyle().
			Padding(0, 2)

	activeDomainTabStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(theme.Primary).
				Background(theme.BgSelected).
				Padding(0, 2)

	domainTabBarStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(theme.Muted).
				MarginBottom(1)
)

// ActivityView displays the activity log.
type ActivityView struct {
	client          *mailersend.Mailersend
	transport       *sdkclient.CLITransport
	table           components.Table
	detail          components.DetailPanel
	items           []types.ActivityItem
	domains         []mailersend.Domain
	activeDomainIdx int
	loading         bool
	loadingDomains  bool
	err             error
	width           int
	height          int
	focused         bool
	showingDetail   bool
}

// NewActivityView creates a new activity view.
func NewActivityView(client *mailersend.Mailersend, transport *sdkclient.CLITransport) ActivityView {
	columns := []components.Column{
		{Title: "TIME", Width: 19},
		{Title: "EVENT", Width: 14},
		{Title: "RECIPIENT", Width: 28},
		{Title: "SUBJECT", Width: 30},
	}
	table := components.NewTable(columns)
	table.SetEmptyMessage("Select a domain to view activity.")

	return ActivityView{
		client:         client,
		transport:      transport,
		table:          table,
		loading:        true,
		loadingDomains: true,
	}
}

// SetClient updates the SDK client.
func (v *ActivityView) SetClient(client *mailersend.Mailersend, transport *sdkclient.CLITransport) {
	v.client = client
	v.transport = transport
}

// SetSize sets the view dimensions.
func (v *ActivityView) SetSize(width, height int) {
	v.width = width
	v.height = height
	// Reserve space for domain bar
	v.table.SetSize(width, height-4)
}

// SetFocused sets whether this view is focused.
func (v *ActivityView) SetFocused(focused bool) {
	v.focused = focused
	v.table.SetFocused(focused)
}

// Loading returns whether the view is loading.
func (v ActivityView) Loading() bool {
	return v.loading
}

// Error returns any error.
func (v ActivityView) Error() error {
	return v.err
}

// ItemCount returns the number of items.
func (v ActivityView) ItemCount() int {
	return len(v.items)
}

// Fetch returns a command to fetch domains first, then activity.
func (v ActivityView) Fetch() tea.Cmd {
	return v.fetchDomains()
}

func (v ActivityView) fetchDomains() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return types.ActivityLoadedMsg{Err: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		domains, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Domain, bool, error) {
			root, _, err := v.client.Domain.List(ctx, &mailersend.ListDomainOptions{
				Page:  page,
				Limit: perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(v.transport, err)
			}
			return root.Data, root.Links.Next != "", nil
		}, 100)

		if err != nil {
			return ActivityDomainsLoadedMsg{Err: err}
		}

		return ActivityDomainsLoadedMsg{Domains: domains}
	}
}

// ActivityDomainsLoadedMsg is sent when domains are loaded for the activity view.
type ActivityDomainsLoadedMsg struct {
	Domains []mailersend.Domain
	Err     error
}

func (v ActivityView) fetchActivity() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil || len(v.domains) == 0 {
			return types.ActivityLoadedMsg{Err: nil}
		}

		domainID := v.domains[v.activeDomainIdx].ID

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use last 30 days
		now := time.Now()
		dateFrom := now.AddDate(0, 0, -30).Unix()
		dateTo := now.Unix()

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]types.ActivityItem, bool, error) {
			root, _, err := v.client.Activity.List(ctx, &mailersend.ActivityOptions{
				DomainID: domainID,
				Page:     page,
				Limit:    perPage,
				DateFrom: dateFrom,
				DateTo:   dateTo,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(v.transport, err)
			}

			var items []types.ActivityItem
			for _, d := range root.Data {
				item := types.ActivityItem{
					ID:        d.ID,
					CreatedAt: d.CreatedAt,
					Type:      d.Type,
				}
				item.Email.From = d.Email.From
				item.Email.To = d.Email.Recipient.Email
				item.Email.Subject = d.Email.Subject
				items = append(items, item)
			}

			return items, root.Links.Next != "", nil
		}, 100)

		return types.ActivityLoadedMsg{
			Items: items,
			Err:   err,
		}
	}
}

// Update handles messages for this view.
func (v ActivityView) Update(msg tea.Msg) (ActivityView, tea.Cmd) {
	switch msg := msg.(type) {
	case ActivityDomainsLoadedMsg:
		v.loadingDomains = false
		if msg.Err != nil {
			v.loading = false
			v.err = msg.Err
			return v, nil
		}
		v.domains = msg.Domains
		if len(v.domains) > 0 {
			// Fetch activity for first domain
			return v, v.fetchActivity()
		}
		v.loading = false
		v.table.SetEmptyMessage("No domains found. Add a domain first.")
	case types.ActivityLoadedMsg:
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
func (v *ActivityView) HandleKey(msg tea.KeyMsg) tea.Cmd {
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
		if len(v.domains) > 0 {
			v.activeDomainIdx--
			if v.activeDomainIdx < 0 {
				v.activeDomainIdx = len(v.domains) - 1
			}
			v.loading = true
			v.table.SetLoading(true)
			return v.fetchActivity()
		}
	case "l", "right":
		if len(v.domains) > 0 {
			v.activeDomainIdx = (v.activeDomainIdx + 1) % len(v.domains)
			v.loading = true
			v.table.SetLoading(true)
			return v.fetchActivity()
		}
	case "enter":
		v.showDetail()
	case "r":
		v.loading = true
		v.table.SetLoading(true)
		return v.fetchActivity()
	}
	return nil
}

// SelectedItem returns the currently selected activity item.
func (v ActivityView) SelectedItem() *types.ActivityItem {
	idx := v.table.Cursor()
	if idx >= 0 && idx < len(v.items) {
		return &v.items[idx]
	}
	return nil
}

func (v *ActivityView) showDetail() {
	item := v.SelectedItem()
	if item == nil {
		return
	}

	v.detail.SetTitle("Activity Event")

	created := item.CreatedAt
	if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
		created = t.Format("2006-01-02 15:04:05")
	}

	v.detail.SetRows([]components.DetailRow{
		{Label: "ID", Value: item.ID},
		{Label: "Event Type", Value: item.Type},
		{Label: "Time", Value: created},
		{Label: "From", Value: item.Email.From},
		{Label: "To", Value: item.Email.To},
		{Label: "Subject", Value: item.Email.Subject},
	})
	v.detail.SetSize(v.width, v.height)
	v.showingDetail = true
}

func (v *ActivityView) updateTable() {
	var rows [][]string
	for _, item := range v.items {
		created := item.CreatedAt
		if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
			created = t.Format("2006-01-02 15:04:05")
		}

		subject := item.Email.Subject
		if len(subject) > 30 {
			subject = subject[:27] + "..."
		}

		rows = append(rows, []string{
			created,
			item.Type,
			item.Email.To,
			subject,
		})
	}
	v.table.SetRows(rows)
	v.table.SetLoading(false)
}

// View renders the activity view.
func (v ActivityView) View() string {
	if v.showingDetail {
		return v.detail.View()
	}

	var b strings.Builder

	// Domain selector bar
	if len(v.domains) > 0 {
		var domainTabs []string
		for i, d := range v.domains {
			name := d.Name
			if len(name) > 20 {
				name = name[:17] + "..."
			}
			style := domainTabStyle
			if i == v.activeDomainIdx {
				style = activeDomainTabStyle
			}
			domainTabs = append(domainTabs, style.Render(name))
		}

		tabBar := lipgloss.JoinHorizontal(lipgloss.Top, domainTabs...)
		b.WriteString(domainTabBarStyle.Render(tabBar))
		b.WriteString("\n")

		// Hint
		hint := fmt.Sprintf("← → to switch domains | %d events (last 30 days)", len(v.items))
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render(hint))
		b.WriteString("\n\n")
	} else if v.loadingDomains {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Muted).Render("Loading domains..."))
		b.WriteString("\n\n")
	}

	// Table
	b.WriteString(v.table.View())

	return b.String()
}

// ShowingDetail returns whether the detail view is active.
func (v ActivityView) ShowingDetail() bool {
	return v.showingDetail
}
