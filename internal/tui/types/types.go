package types

import "github.com/mailersend/mailersend-go"

// ViewType represents the different views in the dashboard.
type ViewType int

const (
	ViewDomains ViewType = iota
	ViewActivity
	ViewAnalytics
	ViewMessages
	ViewSuppressions
)

func (v ViewType) String() string {
	switch v {
	case ViewDomains:
		return "Domains"
	case ViewActivity:
		return "Activity"
	case ViewAnalytics:
		return "Analytics"
	case ViewMessages:
		return "Messages"
	case ViewSuppressions:
		return "Suppressions"
	default:
		return "Unknown"
	}
}

// ViewInfo contains display information for a view.
type ViewInfo struct {
	Type  ViewType
	Label string
	Icon  string
}

// AllViews returns all available views.
func AllViews() []ViewInfo {
	return []ViewInfo{
		{ViewDomains, "Domains", "◉"},
		{ViewActivity, "Activity", "◈"},
		{ViewAnalytics, "Analytics", "◆"},
		{ViewMessages, "Messages", "◇"},
		{ViewSuppressions, "Suppressions", "◌"},
	}
}

// Data loading messages

// DomainsLoadedMsg is sent when domains are fetched.
type DomainsLoadedMsg struct {
	Domains []mailersend.Domain
	Err     error
}

// ActivityItem represents a single activity event.
type ActivityItem struct {
	ID        string
	CreatedAt string
	Type      string
	Email     struct {
		From    string
		To      string
		Subject string
	}
}

// ActivityLoadedMsg is sent when activity items are fetched.
type ActivityLoadedMsg struct {
	Items []ActivityItem
	Err   error
}

// AnalyticsData holds analytics stats.
type AnalyticsData struct {
	Stats     []mailersend.AnalyticsStats
	DateFrom  string
	DateTo    string
	GroupBy   string
	Sent      int
	Delivered int
	Opens     int
	Clicks    int
	Bounced   int
}

// AnalyticsLoadedMsg is sent when analytics are fetched.
type AnalyticsLoadedMsg struct {
	Data AnalyticsData
	Err  error
}

// MessageItem represents a sent message.
type MessageItem struct {
	ID        string
	CreatedAt string
	UpdatedAt string
}

// MessagesLoadedMsg is sent when messages are fetched.
type MessagesLoadedMsg struct {
	Messages []MessageItem
	Err      error
}

// MessageDetail holds full message details.
type MessageDetail struct {
	ID        string
	CreatedAt string
	UpdatedAt string
	Domain    string
	Emails    []MessageEmail
}

// MessageEmail holds email details within a message.
type MessageEmail struct {
	ID        string
	From      string
	Subject   string
	Status    string
	CreatedAt string
	UpdatedAt string
	Tags      []string
}

// MessageDetailLoadedMsg is sent when a single message detail is fetched.
type MessageDetailLoadedMsg struct {
	Detail MessageDetail
	Err    error
}

// SuppressionItem represents a suppression entry.
type SuppressionItem struct {
	ID        string
	Pattern   string
	Type      string
	Reason    string
	CreatedAt string
}

// SuppressionsLoadedMsg is sent when suppressions are fetched.
type SuppressionsLoadedMsg struct {
	Items []SuppressionItem
	Type  string
	Err   error
}

// Control messages

// RefreshMsg triggers a refresh of the current view.
type RefreshMsg struct{}

// ProfileChangedMsg is sent when the profile changes.
type ProfileChangedMsg struct {
	Profile string
}

// ErrorMsg wraps an error for display.
type ErrorMsg struct {
	Err error
}

func (e ErrorMsg) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return ""
}
