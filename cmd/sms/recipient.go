package sms

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var recipientCmd = &cobra.Command{
	Use:   "recipient",
	Short: "Manage SMS recipients",
}

func init() {
	recipientCmd.AddCommand(recipientListCmd)
	recipientCmd.AddCommand(recipientGetCmd)
	recipientCmd.AddCommand(recipientUpdateCmd)

	recipientListCmd.Flags().Int("limit", 0, "maximum number of recipients to return (0 = all)")
	recipientListCmd.Flags().String("status", "", "filter by status")
	recipientListCmd.Flags().String("sms-number-id", "", "filter by SMS number ID")

	recipientUpdateCmd.Flags().String("status", "", "recipient status (required)")
	_ = recipientUpdateCmd.MarkFlagRequired("status")
}

var recipientListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS recipients",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		params := map[string]string{}

		if v, _ := c.Flags().GetString("status"); v != "" {
			params["status"] = v
		}
		if v, _ := c.Flags().GetString("sms-number-id"); v != "" {
			params["sms_number_id"] = v
		}

		items, err := client.GetPaginated("/v1/sms-recipients", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID              string `json:"id"`
			TelephoneNumber string `json:"number"`
			Status          string `json:"status"`
			CreatedAt       string `json:"created_at"`
		}

		headers := []string{"ID", "NUMBER", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var r item
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("failed to parse SMS recipient: %w", err)
			}
			rows = append(rows, []string{r.ID, r.TelephoneNumber, r.Status, r.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var recipientGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS recipient details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/sms-recipients/"+args[0], nil)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(body)
		}

		var d struct {
			ID     string `json:"id"`
			Number string `json:"number"`
			Status string `json:"status"`
		}
		if err := parseDataField(body, &d); err != nil {
			return err
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Number", d.Number},
			{"Status", d.Status},
		}
		output.Table(headers, rows)
		return nil
	},
}

var recipientUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS recipient",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		status, _ := c.Flags().GetString("status")

		payload := map[string]interface{}{
			"status": status,
		}

		respBody, err := client.Put("/v1/sms-recipients/"+args[0], payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(respBody)
		}

		output.Success("SMS recipient " + args[0] + " updated successfully.")
		return nil
	},
}
