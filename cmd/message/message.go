package message

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "message",
	Short: "Manage messages and scheduled messages",
}

// --- message list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List messages",
	RunE:  runList,
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(scheduledCmd)

	scheduledCmd.AddCommand(scheduledListCmd)
	scheduledCmd.AddCommand(scheduledGetCmd)
	scheduledCmd.AddCommand(scheduledDeleteCmd)

	f := listCmd.Flags()
	f.Int("limit", 25, "maximum number of results to return")
	f.String("status", "", "filter by status (queued|sent|delivered|failed)")
	f.String("domain-id", "", "filter by domain ID or name")
	f.String("date-from", "", "filter from date (YYYY-MM-DD or unix timestamp)")
	f.String("date-to", "", "filter to date (YYYY-MM-DD or unix timestamp)")

	sf := scheduledListCmd.Flags()
	sf.Int("limit", 25, "maximum number of results to return")
	sf.String("status", "", "filter by status (scheduled|sending|sent|error)")
	sf.String("domain-id", "", "filter by domain ID or name")
}

func runList(cobraCmd *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	flags := cobraCmd.Flags()
	limit, _ := flags.GetInt("limit")
	status, _ := flags.GetString("status")
	domainID, _ := flags.GetString("domain-id")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
	}
	dateFromStr, _ := flags.GetString("date-from")
	dateToStr, _ := flags.GetString("date-to")

	params := map[string]string{}
	if status != "" {
		params["status"] = status
	}
	if domainID != "" {
		params["domain_id"] = domainID
	}
	if dateFromStr != "" {
		dateFrom, err := cmdutil.ParseDate(dateFromStr)
		if err != nil {
			return err
		}
		params["date_from"] = fmt.Sprintf("%d", dateFrom)
	}
	if dateToStr != "" {
		dateTo, err := cmdutil.ParseDate(dateToStr)
		if err != nil {
			return err
		}
		params["date_to"] = fmt.Sprintf("%d", dateTo)
	}

	items, err := client.GetPaginated("/v1/messages", params, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(items)
	}

	headers := []string{"ID", "CREATED AT", "UPDATED AT"}
	var rows [][]string
	for _, raw := range items {
		var item struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		rows = append(rows, []string{item.ID, item.CreatedAt, item.UpdatedAt})
	}

	output.Table(headers, rows)
	return nil
}

// --- message get ---

var getCmd = &cobra.Command{
	Use:   "get <message_id>",
	Short: "Get message details",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func runGet(cobraCmd *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	messageID := args[0]
	body, err := client.Get(fmt.Sprintf("/v1/messages/%s", messageID), nil)
	if err != nil {
		return err
	}

	var resp struct {
		Data struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
			Emails    []struct {
				ID         string `json:"id"`
				From       string `json:"from"`
				Subject    string `json:"subject"`
				Status     string `json:"status"`
				TemplateID string `json:"template_id"`
				CreatedAt  string `json:"created_at"`
			} `json:"emails"`
			Domain struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"domain"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		var parsed interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return err
		}
		return output.JSON(parsed)
	}

	d := resp.Data
	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.ID},
		{"Created At", d.CreatedAt},
		{"Updated At", d.UpdatedAt},
		{"Domain", d.Domain.Name},
	}

	if len(d.Emails) > 0 {
		e := d.Emails[0]
		rows = append(rows,
			[]string{"Subject", e.Subject},
			[]string{"From", e.From},
			[]string{"Status", e.Status},
		)
		if e.TemplateID != "" {
			rows = append(rows, []string{"Template ID", e.TemplateID})
		}
	}

	if len(d.Emails) > 1 {
		rows = append(rows, []string{"Email Count", fmt.Sprintf("%d", len(d.Emails))})
	}

	output.Table(headers, rows)
	return nil
}

// --- message scheduled (subcommand group) ---

var scheduledCmd = &cobra.Command{
	Use:   "scheduled",
	Short: "Manage scheduled messages",
}

// --- message scheduled list ---

var scheduledListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled messages",
	RunE:  runScheduledList,
}

func runScheduledList(cobraCmd *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	flags := cobraCmd.Flags()
	limit, _ := flags.GetInt("limit")
	status, _ := flags.GetString("status")
	domainID, _ := flags.GetString("domain-id")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
	}

	params := map[string]string{}
	if status != "" {
		params["status"] = status
	}
	if domainID != "" {
		params["domain_id"] = domainID
	}

	items, err := client.GetPaginated("/v1/message-schedules", params, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(items)
	}

	headers := []string{"MESSAGE ID", "SUBJECT", "SEND AT", "STATUS", "CREATED AT"}
	var rows [][]string
	for _, raw := range items {
		var item struct {
			MessageID string `json:"message_id"`
			Subject   string `json:"subject"`
			SendAt    string `json:"send_at"`
			Status    string `json:"status"`
			CreatedAt string `json:"created_at"`
		}
		if err := json.Unmarshal(raw, &item); err != nil {
			continue
		}
		rows = append(rows, []string{
			item.MessageID,
			output.Truncate(item.Subject, 40),
			item.SendAt,
			item.Status,
			item.CreatedAt,
		})
	}

	output.Table(headers, rows)
	return nil
}

// --- message scheduled get ---

var scheduledGetCmd = &cobra.Command{
	Use:   "get <message_id>",
	Short: "Get scheduled message details",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduledGet,
}

func runScheduledGet(cobraCmd *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	messageID := args[0]
	body, err := client.Get(fmt.Sprintf("/v1/message-schedules/%s", messageID), nil)
	if err != nil {
		return err
	}

	var resp struct {
		Data struct {
			MessageID     string `json:"message_id"`
			Subject       string `json:"subject"`
			SendAt        string `json:"send_at"`
			Status        string `json:"status"`
			StatusMessage string `json:"status_message"`
			CreatedAt     string `json:"created_at"`
			Domain        struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"domain"`
			Message struct {
				ID string `json:"id"`
			} `json:"message"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		var parsed interface{}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return err
		}
		return output.JSON(parsed)
	}

	d := resp.Data
	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Message ID", d.MessageID},
		{"Subject", d.Subject},
		{"Send At", d.SendAt},
		{"Status", d.Status},
		{"Status Message", d.StatusMessage},
		{"Created At", d.CreatedAt},
		{"Domain", d.Domain.Name},
		{"Domain ID", d.Domain.ID},
		{"Related Message ID", d.Message.ID},
	}

	output.Table(headers, rows)
	return nil
}

// --- message scheduled delete ---

var scheduledDeleteCmd = &cobra.Command{
	Use:   "delete <message_id>",
	Short: "Delete a scheduled message",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduledDelete,
}

func runScheduledDelete(cobraCmd *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	messageID := args[0]
	_, err = client.Delete(fmt.Sprintf("/v1/message-schedules/%s", messageID))
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(map[string]string{"status": "deleted", "message_id": messageID})
	}

	output.Success(fmt.Sprintf("Scheduled message %s deleted successfully.", messageID))
	return nil
}

// --- wire up subcommands (merged into init above) ---
