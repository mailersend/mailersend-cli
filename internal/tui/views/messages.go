package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-cli/internal/tui/components"
	"github.com/mailersend/mailersend-cli/internal/tui/types"
	"github.com/mailersend/mailersend-go"
)

// MessagesView displays sent messages.
type MessagesView struct {
	client        *mailersend.Mailersend
	transport     *sdkclient.CLITransport
	table         components.Table
	detail        components.DetailPanel
	items         []types.MessageItem
	loading       bool
	loadingDetail bool
	err           error
	width         int
	height        int
	focused       bool
	showingDetail bool
}

// NewMessagesView creates a new messages view.
func NewMessagesView(client *mailersend.Mailersend, transport *sdkclient.CLITransport) MessagesView {
	columns := []components.Column{
		{Title: "MESSAGE ID", Width: 28},
		{Title: "CREATED", Width: 19},
		{Title: "UPDATED", Width: 19},
	}
	table := components.NewTable(columns)
	table.SetEmptyMessage("No messages found.")
	table.SetLoading(true)

	return MessagesView{
		client:    client,
		transport: transport,
		table:     table,
		loading:   true,
	}
}

// SetClient updates the SDK client.
func (v *MessagesView) SetClient(client *mailersend.Mailersend, transport *sdkclient.CLITransport) {
	v.client = client
	v.transport = transport
}

// SetSize sets the view dimensions.
func (v *MessagesView) SetSize(width, height int) {
	v.width = width
	v.height = height
	v.table.SetSize(width, height)
}

// SetFocused sets whether this view is focused.
func (v *MessagesView) SetFocused(focused bool) {
	v.focused = focused
	v.table.SetFocused(focused)
}

// Loading returns whether the view is loading.
func (v MessagesView) Loading() bool {
	return v.loading
}

// Error returns any error.
func (v MessagesView) Error() error {
	return v.err
}

// ItemCount returns the number of items.
func (v MessagesView) ItemCount() int {
	return len(v.items)
}

// Fetch returns a command to fetch messages.
func (v MessagesView) Fetch() tea.Cmd {
	return func() tea.Msg {
		if v.client == nil {
			return types.MessagesLoadedMsg{Err: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]types.MessageItem, bool, error) {
			result, _, err := v.client.Message.List(ctx, &mailersend.ListMessageOptions{
				Page:  page,
				Limit: perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(v.transport, err)
			}

			var out []types.MessageItem
			for _, m := range result.Data {
				out = append(out, types.MessageItem{
					ID:        m.ID,
					CreatedAt: m.CreatedAt.Format(time.RFC3339),
					UpdatedAt: m.UpdatedAt.Format(time.RFC3339),
				})
			}
			return out, result.Links.Next != "", nil
		}, 0)

		if err != nil {
			return types.MessagesLoadedMsg{Err: err}
		}

		// API returns oldest first; reverse so newest appear at the top.
		for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
			items[i], items[j] = items[j], items[i]
		}

		return types.MessagesLoadedMsg{
			Messages: items,
			Err:      nil,
		}
	}
}

// Update handles messages for this view.
func (v MessagesView) Update(msg tea.Msg) (MessagesView, tea.Cmd) {
	switch msg := msg.(type) {
	case types.MessagesLoadedMsg:
		v.loading = false
		v.err = msg.Err
		if msg.Err == nil {
			v.items = msg.Messages
			v.updateTable()
		}
	case types.MessageDetailLoadedMsg:
		v.loadingDetail = false
		if msg.Err != nil {
			v.err = msg.Err
			return v, nil
		}
		v.populateDetail(msg.Detail)
	}
	return v, nil
}

// HandleKey handles key events when this view is active.
func (v *MessagesView) HandleKey(msg tea.KeyMsg) tea.Cmd {
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
		return v.enterDetail()
	case "r":
		v.loading = true
		v.table.SetLoading(true)
		return v.Fetch()
	}
	return nil
}

// SelectedItem returns the currently selected message item.
func (v MessagesView) SelectedItem() *types.MessageItem {
	idx := v.table.Cursor()
	if idx >= 0 && idx < len(v.items) {
		return &v.items[idx]
	}
	return nil
}

func (v *MessagesView) enterDetail() tea.Cmd {
	item := v.SelectedItem()
	if item == nil {
		return nil
	}

	v.showingDetail = true
	v.loadingDetail = true

	// Show loading state with what we have
	created := item.CreatedAt
	if t, err := time.Parse(time.RFC3339, item.CreatedAt); err == nil {
		created = t.Format("2006-01-02 15:04:05")
	}

	v.detail.SetTitle("Message Details")
	v.detail.SetRows([]components.DetailRow{
		{Label: "Message ID", Value: item.ID},
		{Label: "Created", Value: created},
		{Label: "", Value: "Loading details..."},
	})
	v.detail.SetSize(v.width, v.height)

	return v.fetchDetail(item.ID)
}

func (v *MessagesView) fetchDetail(messageID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		result, _, err := v.client.Message.Get(ctx, messageID)
		if err != nil {
			return types.MessageDetailLoadedMsg{Err: sdkclient.WrapError(v.transport, err)}
		}

		detail := types.MessageDetail{
			ID:        result.Data.ID,
			CreatedAt: result.Data.CreatedAt.Format(time.RFC3339),
			UpdatedAt: result.Data.UpdatedAt.Format(time.RFC3339),
			Domain:    result.Data.Domain.Name,
		}

		for _, e := range result.Data.Emails {
			detail.Emails = append(detail.Emails, types.MessageEmail{
				ID:        e.ID,
				From:      e.From,
				Subject:   e.Subject,
				Status:    e.Status,
				CreatedAt: e.CreatedAt.Format(time.RFC3339),
				UpdatedAt: e.UpdatedAt.Format(time.RFC3339),
				Tags:      e.Tags,
			})
		}

		return types.MessageDetailLoadedMsg{Detail: detail}
	}
}

func (v *MessagesView) populateDetail(detail types.MessageDetail) {
	created := detail.CreatedAt
	if t, err := time.Parse(time.RFC3339, detail.CreatedAt); err == nil {
		created = t.Format("2006-01-02 15:04:05")
	}
	updated := detail.UpdatedAt
	if t, err := time.Parse(time.RFC3339, detail.UpdatedAt); err == nil {
		updated = t.Format("2006-01-02 15:04:05")
	}

	rows := []components.DetailRow{
		{Label: "Message ID", Value: detail.ID},
		{Label: "Domain", Value: detail.Domain},
		{Label: "Created", Value: created},
		{Label: "Updated", Value: updated},
	}

	for i, e := range detail.Emails {
		if i > 0 {
			rows = append(rows, components.DetailRow{Label: "", Value: ""})
		}
		rows = append(rows, components.DetailRow{Label: "", Value: ""})

		emailLabel := "Email"
		if len(detail.Emails) > 1 {
			emailLabel = fmt.Sprintf("Email %d", i+1)
		}
		rows = append(rows,
			components.DetailRow{Label: emailLabel, Value: e.ID},
			components.DetailRow{Label: "From", Value: e.From},
			components.DetailRow{Label: "Subject", Value: e.Subject},
			components.DetailRow{Label: "Status", Value: e.Status},
		)
		if len(e.Tags) > 0 {
			rows = append(rows, components.DetailRow{Label: "Tags", Value: strings.Join(e.Tags, ", ")})
		}
	}

	v.detail.SetTitle("Message Details")
	v.detail.SetRows(rows)
	v.detail.SetSize(v.width, v.height)
}

func (v *MessagesView) updateTable() {
	var rows [][]string
	for _, m := range v.items {
		created := m.CreatedAt
		if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
			created = t.Format("2006-01-02 15:04:05")
		}
		updated := m.UpdatedAt
		if t, err := time.Parse(time.RFC3339, m.UpdatedAt); err == nil {
			updated = t.Format("2006-01-02 15:04:05")
		}

		rows = append(rows, []string{
			m.ID,
			created,
			updated,
		})
	}
	v.table.SetRows(rows)
	v.table.SetLoading(false)
}

// View renders the messages view.
func (v MessagesView) View() string {
	if v.showingDetail {
		return v.detail.View()
	}
	return v.table.View()
}

// ShowingDetail returns whether the detail view is active.
func (v MessagesView) ShowingDetail() bool {
	return v.showingDetail
}
