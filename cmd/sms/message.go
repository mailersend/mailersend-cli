package sms

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Manage SMS messages",
}

func init() {
	messageCmd.AddCommand(messageListCmd)
	messageCmd.AddCommand(messageGetCmd)

	messageListCmd.Flags().Int("limit", 0, "maximum number of messages to return (0 = all)")
}

var messageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS messages",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		items, err := client.GetPaginated("/v1/sms-messages", nil, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID        string `json:"id"`
			From      string `json:"from"`
			To        string `json:"to"`
			Text      string `json:"text"`
			Status    string `json:"status"`
			CreatedAt string `json:"created_at"`
		}

		headers := []string{"ID", "FROM", "TO", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var m item
			if err := json.Unmarshal(raw, &m); err != nil {
				return fmt.Errorf("failed to parse SMS message: %w", err)
			}
			rows = append(rows, []string{m.ID, m.From, m.To, m.Status, m.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var messageGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS message details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/sms-messages/"+args[0], nil)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(body)
		}

		var d struct {
			ID        string `json:"id"`
			From      string `json:"from"`
			To        string `json:"to"`
			Text      string `json:"text"`
			Status    string `json:"status"`
			CreatedAt string `json:"created_at"`
		}
		if err := parseDataField(body, &d); err != nil {
			return err
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"From", d.From},
			{"To", d.To},
			{"Text", d.Text},
			{"Status", d.Status},
			{"Created At", d.CreatedAt},
		}
		output.Table(headers, rows)
		return nil
	},
}
