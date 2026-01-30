package smtp

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "smtp",
	Short: "Manage SMTP users",
	Long:  "List, view, create, update, and delete SMTP users for a domain.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = listCmd.MarkFlagRequired("domain-id")
	listCmd.Flags().Int("limit", 0, "maximum number of SMTP users to return (0 = all)")

	getCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = getCmd.MarkFlagRequired("domain-id")

	createCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = createCmd.MarkFlagRequired("domain-id")
	createCmd.Flags().String("name", "", "SMTP user name (required)")
	_ = createCmd.MarkFlagRequired("name")
	createCmd.Flags().Bool("enabled", true, "whether the SMTP user is enabled")

	updateCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = updateCmd.MarkFlagRequired("domain-id")
	updateCmd.Flags().String("name", "", "SMTP user name")
	updateCmd.Flags().Bool("enabled", true, "whether the SMTP user is enabled")

	deleteCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = deleteCmd.MarkFlagRequired("domain-id")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMTP users",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain-id")
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
		limit, _ := c.Flags().GetInt("limit")

		items, err := client.GetPaginated("/v1/domains/"+domainID+"/smtp-users", nil, limit)
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
			var s item
			if err := json.Unmarshal(raw, &s); err != nil {
				return fmt.Errorf("failed to parse SMTP user: %w", err)
			}
			enabled := "No"
			if s.Enabled {
				enabled = "Yes"
			}
			rows = append(rows, []string{s.ID, s.Name, enabled})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMTP user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain-id")
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/domains/"+domainID+"/smtp-users/"+args[0], nil)
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
				ID      string `json:"id"`
				Name    string `json:"name"`
				Enabled bool   `json:"enabled"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := resp.Data
		enabled := "No"
		if d.Enabled {
			enabled = "Yes"
		}
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Enabled", enabled},
		}
		output.Table(headers, rows)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an SMTP user",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain-id")
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
		name, _ := c.Flags().GetString("name")

		payload := map[string]interface{}{
			"name": name,
		}

		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			payload["enabled"] = v
		}

		respBody, _, err := client.Post("/v1/domains/"+domainID+"/smtp-users", payload)
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
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		output.Success("SMTP user created successfully. ID: " + resp.Data.ID)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMTP user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain-id")
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{}

		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			payload["name"] = v
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			payload["enabled"] = v
		}

		respBody, err := client.Put("/v1/domains/"+domainID+"/smtp-users/"+args[0], payload)
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

		output.Success("SMTP user " + args[0] + " updated successfully.")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMTP user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain-id")
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/domains/" + domainID + "/smtp-users/" + args[0])
		if err != nil {
			return err
		}

		output.Success("SMTP user " + args[0] + " deleted successfully.")
		return nil
	},
}
