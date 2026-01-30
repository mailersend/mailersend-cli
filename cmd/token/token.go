package token

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "token",
	Short: "Manage API tokens",
	Long:  "List, view, create, update, and delete API tokens.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(updateStatusCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().Int("limit", 0, "maximum number of tokens to return (0 = all)")

	createCmd.Flags().String("name", "", "token name (required)")
	_ = createCmd.MarkFlagRequired("name")
	createCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = createCmd.MarkFlagRequired("domain-id")
	createCmd.Flags().StringSlice("scopes", nil, "token scopes (required)")
	_ = createCmd.MarkFlagRequired("scopes")

	updateCmd.Flags().String("name", "", "token name")

	updateStatusCmd.Flags().String("status", "", "token status: pause or unpause (required)")
	_ = updateStatusCmd.MarkFlagRequired("status")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List API tokens",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		items, err := client.GetPaginated("/v1/token", nil, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			CreatedAt string `json:"created_at"`
		}

		headers := []string{"ID", "NAME", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var t item
			if err := json.Unmarshal(raw, &t); err != nil {
				return fmt.Errorf("failed to parse token: %w", err)
			}
			rows = append(rows, []string{t.ID, t.Name, t.Status, t.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get API token details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/token/"+args[0], nil)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		var resp struct {
			Data struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Status    string `json:"status"`
				CreatedAt string `json:"created_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := resp.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Status", d.Status},
			{"Created At", d.CreatedAt},
		}
		output.Table(headers, rows)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API token",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		name, _ := c.Flags().GetString("name")
		domainID, _ := c.Flags().GetString("domain-id")
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
		scopes, _ := c.Flags().GetStringSlice("scopes")

		payload := map[string]interface{}{
			"name":      name,
			"domain_id": domainID,
			"scopes":    scopes,
		}

		respBody, _, err := client.Post("/v1/token", payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(respBody, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		var resp struct {
			Data struct {
				ID          string `json:"id"`
				AccessToken string `json:"accessToken"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		output.Success("Token created successfully. ID: " + resp.Data.ID)
		if resp.Data.AccessToken != "" {
			fmt.Printf("Access Token: %s\n", resp.Data.AccessToken)
		}
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an API token",
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

		respBody, err := client.Put("/v1/token/"+args[0], payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(respBody, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		output.Success("Token " + args[0] + " updated successfully.")
		return nil
	},
}

var updateStatusCmd = &cobra.Command{
	Use:   "update-status <id>",
	Short: "Update API token status (pause/unpause)",
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

		respBody, err := client.Put("/v1/token/"+args[0]+"/settings", payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(respBody, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		output.Success("Token " + args[0] + " status updated to " + status + ".")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/token/" + args[0])
		if err != nil {
			return err
		}

		output.Success("Token " + args[0] + " deleted successfully.")
		return nil
	},
}
