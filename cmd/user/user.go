package user

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "user",
	Short: "Manage account users and invites",
	Long:  "List, view, invite, update, and delete account users. Manage invites.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(inviteCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().Int("limit", 0, "maximum number of users to return (0 = all)")

	inviteCmd.AddCommand(inviteCreateCmd)
	inviteCmd.AddCommand(inviteListCmd)
	inviteCmd.AddCommand(inviteGetCmd)
	inviteCmd.AddCommand(inviteResendCmd)
	inviteCmd.AddCommand(inviteCancelCmd)

	inviteCreateCmd.Flags().String("email", "", "email address (required)")
	_ = inviteCreateCmd.MarkFlagRequired("email")
	inviteCreateCmd.Flags().String("role", "", "user role (required)")
	_ = inviteCreateCmd.MarkFlagRequired("role")
	inviteCreateCmd.Flags().StringSlice("permissions", nil, "permissions")
	inviteCreateCmd.Flags().StringSlice("templates", nil, "template IDs")
	inviteCreateCmd.Flags().StringSlice("domains", nil, "domain IDs")

	inviteListCmd.Flags().Int("limit", 0, "maximum number of invites to return (0 = all)")

	updateCmd.Flags().String("role", "", "user role")
	updateCmd.Flags().StringSlice("permissions", nil, "permissions")
	updateCmd.Flags().StringSlice("templates", nil, "template IDs")
	updateCmd.Flags().StringSlice("domains", nil, "domain IDs")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List account users",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		items, err := client.GetPaginated("/v1/users", nil, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		}

		headers := []string{"ID", "EMAIL", "ROLE"}
		var rows [][]string
		for _, raw := range items {
			var u item
			if err := json.Unmarshal(raw, &u); err != nil {
				return fmt.Errorf("failed to parse user: %w", err)
			}
			rows = append(rows, []string{u.ID, u.Email, u.Role})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/users/"+args[0], nil)
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
				ID    string `json:"id"`
				Email string `json:"email"`
				Role  string `json:"role"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := resp.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Email", d.Email},
			{"Role", d.Role},
		}
		output.Table(headers, rows)
		return nil
	},
}

var inviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Manage user invitations",
}

var inviteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Invite a new user",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		email, _ := c.Flags().GetString("email")
		role, _ := c.Flags().GetString("role")

		payload := map[string]interface{}{
			"email": email,
			"role":  role,
		}

		if perms, _ := c.Flags().GetStringSlice("permissions"); len(perms) > 0 {
			payload["permissions"] = perms
		}
		if templates, _ := c.Flags().GetStringSlice("templates"); len(templates) > 0 {
			payload["templates"] = templates
		}
		if domains, _ := c.Flags().GetStringSlice("domains"); len(domains) > 0 {
			payload["domains"] = domains
		}

		respBody, _, err := client.Post("/v1/users", payload)
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

		output.Success("User invitation sent to " + email + ".")
		return nil
	},
}

var inviteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending invites",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		items, err := client.GetPaginated("/v1/invites", nil, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Role  string `json:"role"`
		}

		headers := []string{"ID", "EMAIL", "ROLE"}
		var rows [][]string
		for _, raw := range items {
			var i item
			if err := json.Unmarshal(raw, &i); err != nil {
				return fmt.Errorf("failed to parse invite: %w", err)
			}
			rows = append(rows, []string{i.ID, i.Email, i.Role})
		}

		output.Table(headers, rows)
		return nil
	},
}

var inviteGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get invite details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/invites/"+args[0], nil)
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
				ID    string `json:"id"`
				Email string `json:"email"`
				Role  string `json:"role"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := resp.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Email", d.Email},
			{"Role", d.Role},
		}
		output.Table(headers, rows)
		return nil
	},
}

var inviteResendCmd = &cobra.Command{
	Use:   "resend <id>",
	Short: "Resend an invite",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, _, err = client.Post("/v1/invites/"+args[0]+"/resend", nil)
		if err != nil {
			return err
		}

		output.Success("Invite " + args[0] + " resent successfully.")
		return nil
	},
}

var inviteCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel an invite",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/invites/" + args[0])
		if err != nil {
			return err
		}

		output.Success("Invite " + args[0] + " cancelled successfully.")
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{}

		if c.Flags().Changed("role") {
			v, _ := c.Flags().GetString("role")
			payload["role"] = v
		}
		if c.Flags().Changed("permissions") {
			v, _ := c.Flags().GetStringSlice("permissions")
			payload["permissions"] = v
		}
		if c.Flags().Changed("templates") {
			v, _ := c.Flags().GetStringSlice("templates")
			payload["templates"] = v
		}
		if c.Flags().Changed("domains") {
			v, _ := c.Flags().GetStringSlice("domains")
			payload["domains"] = v
		}

		respBody, err := client.Put("/v1/users/"+args[0], payload)
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

		output.Success("User " + args[0] + " updated successfully.")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/users/" + args[0])
		if err != nil {
			return err
		}

		output.Success("User " + args[0] + " deleted successfully.")
		return nil
	},
}
