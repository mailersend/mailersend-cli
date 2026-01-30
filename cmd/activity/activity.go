package activity

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "activity",
	Short: "View and manage activity",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)

	f := listCmd.Flags()
	f.String("domain-id", "", "domain ID or name (required)")
	f.Int("limit", 0, "maximum number of results to return")
	f.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	f.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	f.StringSlice("event", nil, "event types to filter (queued, sent, delivered, soft_bounced, hard_bounced, opened, clicked, unsubscribed, spam_complaints)")
}

// --- list subcommand ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List activity for a domain",
	Long:  "List activity events for a domain via the MailerSend API.",
	RunE:  runList,
}

func runList(cobraCmd *cobra.Command, args []string) error {
	flags := cobraCmd.Flags()
	domainIDStr, _ := flags.GetString("domain-id")
	dateFromStr, _ := flags.GetString("date-from")
	dateToStr, _ := flags.GetString("date-to")

	if domainIDStr == "" {
		return fmt.Errorf("missing required flag: --domain-id\n\nExample:\n  mailersend-cli activity list --domain-id nikola.wtf --date-from 2025-01-01 --date-to 2025-01-30")
	}

	now := time.Now()
	dateFrom, dateTo, err := cmdutil.DefaultDateRange(dateFromStr, dateToStr, now)
	if err != nil {
		return err
	}

	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	domainID, err := cmdutil.ResolveDomain(client, domainIDStr)
	if err != nil {
		return err
	}
	limit, _ := flags.GetInt("limit")
	events, _ := flags.GetStringSlice("event")

	params := map[string]string{
		"date_from": strconv.FormatInt(dateFrom, 10),
		"date_to":   strconv.FormatInt(dateTo, 10),
	}

	for i, e := range events {
		params[fmt.Sprintf("event[%d]", i)] = e
	}

	path := fmt.Sprintf("/v1/activity/%s", domainID)

	items, err := client.GetPaginated(path, params, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(items)
	}

	headers := []string{"ID", "TYPE", "FROM", "SUBJECT", "CREATED AT"}
	var rows [][]string

	for _, raw := range items {
		var item map[string]interface{}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}

		id := stringVal(item, "id")
		typ := stringVal(item, "type")
		createdAt := stringVal(item, "created_at")

		var from, subject string
		if emailObj, ok := item["email"].(map[string]interface{}); ok {
			from = stringVal(emailObj, "from")
			subject = stringVal(emailObj, "subject")
		}

		rows = append(rows, []string{
			id,
			typ,
			from,
			output.Truncate(subject, 40),
			createdAt,
		})
	}

	output.Table(headers, rows)
	return nil
}

// --- get subcommand ---

var getCmd = &cobra.Command{
	Use:   "get <activity_id>",
	Short: "Get activity details",
	Long:  "Get detailed information about a specific activity.",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func runGet(cobraCmd *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	activityID := args[0]
	path := fmt.Sprintf("/v1/activities/%s", activityID)

	body, err := client.Get(path, nil)
	if err != nil {
		return err
	}

	var resp struct {
		Data struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
			Type      string `json:"type"`
			Email     struct {
				ID        string `json:"id"`
				From      string `json:"from"`
				Subject   string `json:"subject"`
				Text      string `json:"text"`
				HTML      string `json:"html"`
				Status    string `json:"status"`
				Tags      []string `json:"tags"`
				CreatedAt string `json:"created_at"`
				UpdatedAt string `json:"updated_at"`
				Recipient struct {
					ID        string `json:"id"`
					Email     string `json:"email"`
					CreatedAt string `json:"created_at"`
				} `json:"recipient"`
			} `json:"email"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(resp.Data)
	}

	d := resp.Data
	fmt.Printf("%-20s %s\n", "ID:", d.ID)
	fmt.Printf("%-20s %s\n", "Type:", d.Type)
	fmt.Printf("%-20s %s\n", "From:", d.Email.From)
	fmt.Printf("%-20s %s\n", "Subject:", d.Email.Subject)
	fmt.Printf("%-20s %s\n", "Status:", d.Email.Status)
	fmt.Printf("%-20s %s\n", "Recipient Email:", d.Email.Recipient.Email)
	fmt.Printf("%-20s %s\n", "Created At:", d.CreatedAt)

	return nil
}

// stringVal safely extracts a string value from a map.
func stringVal(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}
