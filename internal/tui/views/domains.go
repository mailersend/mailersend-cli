package views

import (
	"context"
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
	checkStyle = lipgloss.NewStyle().Foreground(theme.Success)
	crossStyle = lipgloss.NewStyle().Foreground(theme.Error)
)

func check(ok bool) string {
	if ok {
		return checkStyle.Render("✓")
	}
	return crossStyle.Render("✗")
}

// DomainsView displays the list of domains.
type DomainsView struct {
	client        *mailersend.Mailersend
	transport     *sdkclient.CLITransport
	table         components.Table
	detail        components.DetailPanel
	domains       []mailersend.Domain
	loading       bool
	err           error
	width         int
	height        int
	focused       bool
	showingDetail bool
}

// NewDomainsView creates a new domains view.
func NewDomainsView(client *mailersend.Mailersend, transport *sdkclient.CLITransport) DomainsView {
	columns := []components.Column{
		{Title: "NAME", Width: 30},
		{Title: "VERIFIED", Width: 10},
		{Title: "DNS", Width: 8},
		{Title: "TRACKING", Width: 10},
		{Title: "CREATED", Width: 12},
	}
	table := components.NewTable(columns)
	table.SetEmptyMessage("No domains found. Add a domain to get started.")

	return DomainsView{
		client:    client,
		transport: transport,
		table:     table,
		loading:   true,
	}
}

// SetClient updates the SDK client (for profile switching).
func (v *DomainsView) SetClient(client *mailersend.Mailersend, transport *sdkclient.CLITransport) {
	v.client = client
	v.transport = transport
}

// SetSize sets the view dimensions.
func (v *DomainsView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height)
}

// SetFocused sets whether this view is focused.
func (v *DomainsView) SetFocused(focused bool) {
	v.focused = focused
	v.table.SetFocused(focused)
}

// Loading returns whether the view is loading.
func (v DomainsView) Loading() bool {
	return v.loading
}

// Error returns any error.
func (v DomainsView) Error() error {
	return v.err
}

// ItemCount returns the number of items.
func (v DomainsView) ItemCount() int {
	return len(v.domains)
}

// SelectedDomain returns the currently selected domain.
func (v DomainsView) SelectedDomain() *mailersend.Domain {
	idx := v.table.Cursor()
	if idx >= 0 && idx < len(v.domains) {
		return &v.domains[idx]
	}
	return nil
}

// Fetch returns a command to fetch domains.
func (v DomainsView) Fetch() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return types.DomainsLoadedMsg{Err: nil}
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

		return types.DomainsLoadedMsg{
			Domains: domains,
			Err:     err,
		}
	}
}

// Update handles messages for this view.
func (v DomainsView) Update(msg tea.Msg) (DomainsView, tea.Cmd) {
	switch msg := msg.(type) {
	case types.DomainsLoadedMsg:
		v.loading = false
		v.err = msg.Err
		if msg.Err == nil {
			v.domains = msg.Domains
			v.updateTable()
		}
	}
	return v, nil
}

// HandleKey handles key events when this view is active.
func (v *DomainsView) HandleKey(msg tea.KeyMsg) tea.Cmd {
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
	case "enter":
		v.showDetail()
	case "r":
		v.loading = true
		v.table.SetLoading(true)
		return v.Fetch()
	}
	return nil
}

func (v *DomainsView) showDetail() {
	domain := v.SelectedDomain()
	if domain == nil {
		return
	}

	v.detail.SetTitle("Domain: " + domain.Name)

	created := domain.CreatedAt
	if t, err := time.Parse(time.RFC3339, domain.CreatedAt); err == nil {
		created = t.Format("2006-01-02 15:04:05")
	}

	verified := "No"
	if domain.IsVerified {
		verified = "Yes"
	}

	dnsActive := "No"
	if domain.IsDNSActive {
		dnsActive = "Yes"
	}

	tracking := "Disabled"
	if domain.Tracking {
		tracking = "Enabled"
	}

	v.detail.SetRows([]components.DetailRow{
		{Label: "ID", Value: domain.ID},
		{Label: "Name", Value: domain.Name},
		{Label: "Verified", Value: verified},
		{Label: "DNS Active", Value: dnsActive},
		{Label: "Tracking", Value: tracking},
		{Label: "Created", Value: created},
	})
	v.detail.SetSize(v.width, v.height)
	v.showingDetail = true
}

func (v *DomainsView) updateTable() {
	var rows [][]string
	for _, d := range v.domains {
		created := ""
		if t, err := time.Parse(time.RFC3339, d.CreatedAt); err == nil {
			created = t.Format("2006-01-02")
		}

		rows = append(rows, []string{
			d.Name,
			check(d.IsVerified),
			check(d.IsDNSActive),
			check(d.Tracking),
			created,
		})
	}
	v.table.SetRows(rows)
	v.table.SetLoading(false)
}

// View renders the domains view.
func (v DomainsView) View() string {
	if v.showingDetail {
		return v.detail.View()
	}
	return v.table.View()
}

// ShowingDetail returns whether the detail view is active.
func (v DomainsView) ShowingDetail() bool {
	return v.showingDetail
}
