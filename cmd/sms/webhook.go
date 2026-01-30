package sms

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage SMS webhooks",
}

func init() {
	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookGetCmd)
	webhookCmd.AddCommand(webhookCreateCmd)
	webhookCmd.AddCommand(webhookUpdateCmd)
	webhookCmd.AddCommand(webhookDeleteCmd)

	webhookListCmd.Flags().String("sms-number-id", "", "SMS number ID (required)")
	_ = webhookListCmd.MarkFlagRequired("sms-number-id")

	webhookCreateCmd.Flags().String("sms-number-id", "", "SMS number ID (required)")
	_ = webhookCreateCmd.MarkFlagRequired("sms-number-id")
	webhookCreateCmd.Flags().String("name", "", "webhook name (required)")
	_ = webhookCreateCmd.MarkFlagRequired("name")
	webhookCreateCmd.Flags().String("url", "", "webhook URL (required)")
	_ = webhookCreateCmd.MarkFlagRequired("url")
	webhookCreateCmd.Flags().StringSlice("events", nil, "webhook events (required)")
	_ = webhookCreateCmd.MarkFlagRequired("events")
	webhookCreateCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")

	webhookUpdateCmd.Flags().String("name", "", "webhook name")
	webhookUpdateCmd.Flags().String("url", "", "webhook URL")
	webhookUpdateCmd.Flags().StringSlice("events", nil, "webhook events")
	webhookUpdateCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")
}

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS webhooks",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		smsNumberID, _ := c.Flags().GetString("sms-number-id")

		body, err := client.Get("/v1/sms-webhooks", map[string]string{
			"sms_number_id": smsNumberID,
		})
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(body)
		}

		var resp struct {
			Data []struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				URL     string `json:"url"`
				Enabled bool   `json:"enabled"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"ID", "NAME", "URL", "ENABLED"}
		var rows [][]string
		for _, w := range resp.Data {
			rows = append(rows, []string{w.ID, w.Name, output.Truncate(w.URL, 50), boolYesNo(w.Enabled)})
		}

		output.Table(headers, rows)
		return nil
	},
}

var webhookGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS webhook details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/sms-webhooks/"+args[0], nil)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(body)
		}

		var d struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			URL     string   `json:"url"`
			Events  []string `json:"events"`
			Enabled bool     `json:"enabled"`
		}
		if err := parseDataField(body, &d); err != nil {
			return err
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"URL", d.URL},
			{"Enabled", boolYesNo(d.Enabled)},
		}
		output.Table(headers, rows)

		if len(d.Events) > 0 {
			fmt.Println("\nEvents:")
			for _, e := range d.Events {
				fmt.Printf("  - %s\n", e)
			}
		}
		return nil
	},
}

var webhookCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an SMS webhook",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		smsNumberID, _ := c.Flags().GetString("sms-number-id")
		name, _ := c.Flags().GetString("name")
		url, _ := c.Flags().GetString("url")
		events, _ := c.Flags().GetStringSlice("events")
		enabled, _ := c.Flags().GetBool("enabled")

		payload := map[string]interface{}{
			"sms_number_id": smsNumberID,
			"name":          name,
			"url":           url,
			"events":        events,
			"enabled":       enabled,
		}

		respBody, _, err := client.Post("/v1/sms-webhooks", payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(respBody)
		}

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		output.Success("SMS webhook created successfully. ID: " + resp.Data.ID)
		return nil
	},
}

var webhookUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{}

		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			payload["name"] = v
		}
		if c.Flags().Changed("url") {
			v, _ := c.Flags().GetString("url")
			payload["url"] = v
		}
		if c.Flags().Changed("events") {
			v, _ := c.Flags().GetStringSlice("events")
			payload["events"] = v
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			payload["enabled"] = v
		}

		respBody, err := client.Put("/v1/sms-webhooks/"+args[0], payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(respBody)
		}

		output.Success("SMS webhook " + args[0] + " updated successfully.")
		return nil
	},
}

var webhookDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMS webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/sms-webhooks/" + args[0])
		if err != nil {
			return err
		}

		output.Success("SMS webhook " + args[0] + " deleted successfully.")
		return nil
	},
}
