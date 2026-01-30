package sms

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var inboundCmd = &cobra.Command{
	Use:   "inbound",
	Short: "Manage SMS inbound routes",
}

func init() {
	inboundCmd.AddCommand(inboundListCmd)
	inboundCmd.AddCommand(inboundGetCmd)
	inboundCmd.AddCommand(inboundCreateCmd)
	inboundCmd.AddCommand(inboundUpdateCmd)
	inboundCmd.AddCommand(inboundDeleteCmd)

	inboundListCmd.Flags().Int("limit", 0, "maximum number of routes to return (0 = all)")
	inboundListCmd.Flags().String("sms-number-id", "", "filter by SMS number ID")
	inboundListCmd.Flags().Bool("enabled", false, "filter by enabled status")

	inboundCreateCmd.Flags().String("sms-number-id", "", "SMS number ID (required)")
	_ = inboundCreateCmd.MarkFlagRequired("sms-number-id")
	inboundCreateCmd.Flags().String("name", "", "route name (required)")
	_ = inboundCreateCmd.MarkFlagRequired("name")
	inboundCreateCmd.Flags().String("forward-url", "", "forward URL (required)")
	_ = inboundCreateCmd.MarkFlagRequired("forward-url")
	inboundCreateCmd.Flags().String("filter-comparer", "", "filter comparer")
	inboundCreateCmd.Flags().String("filter-value", "", "filter value")
	inboundCreateCmd.Flags().Bool("enabled", true, "whether the route is enabled")

	inboundUpdateCmd.Flags().String("sms-number-id", "", "SMS number ID")
	inboundUpdateCmd.Flags().String("name", "", "route name")
	inboundUpdateCmd.Flags().String("forward-url", "", "forward URL")
	inboundUpdateCmd.Flags().String("filter-comparer", "", "filter comparer")
	inboundUpdateCmd.Flags().String("filter-value", "", "filter value")
	inboundUpdateCmd.Flags().Bool("enabled", true, "whether the route is enabled")
}

var inboundListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS inbound routes",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		params := map[string]string{}

		if v, _ := c.Flags().GetString("sms-number-id"); v != "" {
			params["sms_number_id"] = v
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			params["enabled"] = boolStr(v)
		}

		items, err := client.GetPaginated("/v1/sms-inbounds", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Enabled bool   `json:"enabled"`
		}

		headers := []string{"ID", "NAME", "ENABLED"}
		var rows [][]string
		for _, raw := range items {
			var r item
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("failed to parse SMS inbound route: %w", err)
			}
			rows = append(rows, []string{r.ID, r.Name, boolYesNo(r.Enabled)})
		}

		output.Table(headers, rows)
		return nil
	},
}

var inboundGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS inbound route details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/sms-inbounds/"+args[0], nil)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(body)
		}

		var d struct {
			ID             string `json:"id"`
			Name           string `json:"name"`
			ForwardURL     string `json:"forward_url"`
			FilterComparer string `json:"filter_comparer"`
			FilterValue    string `json:"filter_value"`
			Enabled        bool   `json:"enabled"`
		}
		if err := parseDataField(body, &d); err != nil {
			return err
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Forward URL", d.ForwardURL},
			{"Filter Comparer", d.FilterComparer},
			{"Filter Value", d.FilterValue},
			{"Enabled", boolYesNo(d.Enabled)},
		}
		output.Table(headers, rows)
		return nil
	},
}

var inboundCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an SMS inbound route",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		smsNumberID, _ := c.Flags().GetString("sms-number-id")
		name, _ := c.Flags().GetString("name")
		forwardURL, _ := c.Flags().GetString("forward-url")

		payload := map[string]interface{}{
			"sms_number_id": smsNumberID,
			"name":          name,
			"forward_url":   forwardURL,
		}

		if v, _ := c.Flags().GetString("filter-comparer"); v != "" {
			payload["filter"] = map[string]interface{}{
				"comparer": v,
				"value":    func() string { s, _ := c.Flags().GetString("filter-value"); return s }(),
			}
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			payload["enabled"] = v
		}

		respBody, _, err := client.Post("/v1/sms-inbounds", payload)
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

		output.Success("SMS inbound route created successfully. ID: " + resp.Data.ID)
		return nil
	},
}

var inboundUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS inbound route",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{}

		if c.Flags().Changed("sms-number-id") {
			v, _ := c.Flags().GetString("sms-number-id")
			payload["sms_number_id"] = v
		}
		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			payload["name"] = v
		}
		if c.Flags().Changed("forward-url") {
			v, _ := c.Flags().GetString("forward-url")
			payload["forward_url"] = v
		}
		if c.Flags().Changed("filter-comparer") || c.Flags().Changed("filter-value") {
			filter := map[string]interface{}{}
			if v, _ := c.Flags().GetString("filter-comparer"); v != "" {
				filter["comparer"] = v
			}
			if v, _ := c.Flags().GetString("filter-value"); v != "" {
				filter["value"] = v
			}
			payload["filter"] = filter
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			payload["enabled"] = v
		}

		respBody, err := client.Put("/v1/sms-inbounds/"+args[0], payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return printRawJSON(respBody)
		}

		output.Success("SMS inbound route " + args[0] + " updated successfully.")
		return nil
	},
}

var inboundDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMS inbound route",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/sms-inbounds/" + args[0])
		if err != nil {
			return err
		}

		output.Success("SMS inbound route " + args[0] + " deleted successfully.")
		return nil
	},
}
