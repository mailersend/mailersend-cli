package identity

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "identity",
	Short: "Manage sender identities",
	Long:  "List, view, create, update, and delete sender identities.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().Int("limit", 0, "maximum number of identities to return (0 = all)")
	listCmd.Flags().String("domain-id", "", "filter by domain ID")

	createCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = createCmd.MarkFlagRequired("domain-id")
	createCmd.Flags().String("name", "", "sender name (required)")
	_ = createCmd.MarkFlagRequired("name")
	createCmd.Flags().String("email", "", "sender email (required)")
	_ = createCmd.MarkFlagRequired("email")
	createCmd.Flags().String("reply-to-email", "", "reply-to email")
	createCmd.Flags().String("reply-to-name", "", "reply-to name")
	createCmd.Flags().Bool("add-note", false, "add personal note")
	createCmd.Flags().String("personal-note", "", "personal note text")

	updateCmd.Flags().String("name", "", "sender name")
	updateCmd.Flags().String("reply-to-email", "", "reply-to email")
	updateCmd.Flags().String("reply-to-name", "", "reply-to name")
	updateCmd.Flags().Bool("add-note", false, "add personal note")
	updateCmd.Flags().String("personal-note", "", "personal note text")
}

func identityPath(idOrEmail string) string {
	if strings.Contains(idOrEmail, "@") {
		return "/v1/identities/email/" + idOrEmail
	}
	return "/v1/identities/" + idOrEmail
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List sender identities",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		params := map[string]string{}
		if domainID, _ := c.Flags().GetString("domain-id"); domainID != "" {
			domainID, err = cmdutil.ResolveDomain(client, domainID)
			if err != nil {
				return err
			}
			params["domain_id"] = domainID
		}

		items, err := client.GetPaginated("/v1/identities", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		headers := []string{"ID", "NAME", "EMAIL"}
		var rows [][]string
		for _, raw := range items {
			var i item
			if err := json.Unmarshal(raw, &i); err != nil {
				return fmt.Errorf("failed to parse identity: %w", err)
			}
			rows = append(rows, []string{i.ID, i.Name, i.Email})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <id_or_email>",
	Short: "Get sender identity details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get(identityPath(args[0]), nil)
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
				ID           string `json:"id"`
				Name         string `json:"name"`
				Email        string `json:"email"`
				ReplyToEmail string `json:"reply_to_email"`
				ReplyToName  string `json:"reply_to_name"`
				PersonalNote string `json:"personal_note"`
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
			{"Email", d.Email},
			{"Reply-To Email", d.ReplyToEmail},
			{"Reply-To Name", d.ReplyToName},
			{"Personal Note", d.PersonalNote},
		}
		output.Table(headers, rows)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a sender identity",
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
		email, _ := c.Flags().GetString("email")

		payload := map[string]interface{}{
			"domain_id": domainID,
			"name":      name,
			"email":     email,
		}

		if v, _ := c.Flags().GetString("reply-to-email"); v != "" {
			payload["reply_to_email"] = v
		}
		if v, _ := c.Flags().GetString("reply-to-name"); v != "" {
			payload["reply_to_name"] = v
		}
		if c.Flags().Changed("add-note") {
			v, _ := c.Flags().GetBool("add-note")
			payload["add_note"] = v
		}
		if v, _ := c.Flags().GetString("personal-note"); v != "" {
			payload["personal_note"] = v
		}

		respBody, _, err := client.Post("/v1/identities", payload)
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

		output.Success("Identity created successfully. ID: " + resp.Data.ID)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id_or_email>",
	Short: "Update a sender identity",
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
		if c.Flags().Changed("reply-to-email") {
			v, _ := c.Flags().GetString("reply-to-email")
			payload["reply_to_email"] = v
		}
		if c.Flags().Changed("reply-to-name") {
			v, _ := c.Flags().GetString("reply-to-name")
			payload["reply_to_name"] = v
		}
		if c.Flags().Changed("add-note") {
			v, _ := c.Flags().GetBool("add-note")
			payload["add_note"] = v
		}
		if c.Flags().Changed("personal-note") {
			v, _ := c.Flags().GetString("personal-note")
			payload["personal_note"] = v
		}

		respBody, err := client.Put(identityPath(args[0]), payload)
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

		output.Success("Identity " + args[0] + " updated successfully.")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id_or_email>",
	Short: "Delete a sender identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete(identityPath(args[0]))
		if err != nil {
			return err
		}

		output.Success("Identity " + args[0] + " deleted successfully.")
		return nil
	},
}
