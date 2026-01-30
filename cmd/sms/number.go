package sms

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var numberCmd = &cobra.Command{
	Use:   "number",
	Short: "Manage SMS phone numbers",
}

func init() {
	numberCmd.AddCommand(numberListCmd)
	numberCmd.AddCommand(numberGetCmd)
	numberCmd.AddCommand(numberUpdateCmd)
	numberCmd.AddCommand(numberDeleteCmd)

	numberListCmd.Flags().Int("limit", 0, "maximum number of numbers to return (0 = all)")
	numberListCmd.Flags().Bool("paused", false, "filter by paused status")

	numberUpdateCmd.Flags().Bool("paused", false, "whether the number is paused")
}

var numberListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS phone numbers",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		params := map[string]string{}

		if c.Flags().Changed("paused") {
			v, _ := c.Flags().GetBool("paused")
			params["paused"] = boolStr(v)
		}

		items, err := client.GetPaginated("/v1/sms-numbers", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID          string `json:"id"`
			TelephoneNumber string `json:"telephone_number"`
			Paused      bool   `json:"paused"`
			CreatedAt   string `json:"created_at"`
		}

		headers := []string{"ID", "NUMBER", "PAUSED", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var n item
			if err := json.Unmarshal(raw, &n); err != nil {
				return fmt.Errorf("failed to parse SMS number: %w", err)
			}
			rows = append(rows, []string{n.ID, n.TelephoneNumber, boolYesNo(n.Paused), n.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var numberGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS phone number details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/sms-numbers/"+args[0], nil)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(body)
		}

		var d struct {
			ID              string `json:"id"`
			TelephoneNumber string `json:"telephone_number"`
			Paused          bool   `json:"paused"`
			CreatedAt       string `json:"created_at"`
		}
		if err := parseDataField(body, &d); err != nil {
			return err
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Number", d.TelephoneNumber},
			{"Paused", boolYesNo(d.Paused)},
			{"Created At", d.CreatedAt},
		}
		output.Table(headers, rows)
		return nil
	},
}

var numberUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS phone number",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{}

		if c.Flags().Changed("paused") {
			v, _ := c.Flags().GetBool("paused")
			payload["paused"] = v
		}

		respBody, err := client.Put("/v1/sms-numbers/"+args[0], payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(respBody)
		}

		output.Success("SMS number " + args[0] + " updated successfully.")
		return nil
	},
}

var numberDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMS phone number",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/sms-numbers/" + args[0])
		if err != nil {
			return err
		}

		output.Success("SMS number " + args[0] + " deleted successfully.")
		return nil
	},
}
