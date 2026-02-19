package user

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
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
	inviteCreateCmd.Flags().String("role", "", "user role (required)")
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
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.User, bool, error) {
			root, _, err := ms.User.List(ctx, &mailersend.ListUserOptions{
				Page:  page,
				Limit: perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(err)
			}
			return root.Data, root.Links.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "EMAIL", "ROLE"}
		var rows [][]string
		for _, u := range items {
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
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.User.Get(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
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

// inviteCreateCmd uses raw HTTP because the SDK's InviteUserOptions only supports
// email and role, but the API also accepts permissions, templates, and domains.
var inviteCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Invite a new user",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		email, _ := c.Flags().GetString("email")
		email, err = prompt.RequireArg(email, "email", "Email address")
		if err != nil {
			return err
		}
		role, _ := c.Flags().GetString("role")
		role, err = prompt.RequireArg(role, "role", "User role")
		if err != nil {
			return err
		}

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

		ctx := context.Background()
		body, err := doRawRequest(ms, ctx, http.MethodPost, "https://api.mailersend.com/v1/users", payload)
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

		output.Success("User invitation sent to " + email + ".")
		return nil
	},
}

// inviteListCmd uses raw HTTP since the SDK doesn't have invite list methods.
var inviteListCmd = &cobra.Command{
	Use:   "list",
	Short: "List pending invites",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]inviteItem, bool, error) {
			url := fmt.Sprintf("https://api.mailersend.com/v1/invites?page=%d&limit=%d", page, perPage)
			body, err := doRawRequest(ms, ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, false, err
			}

			var resp struct {
				Data  []inviteItem `json:"data"`
				Links struct {
					Next string `json:"next"`
				} `json:"links"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return nil, false, fmt.Errorf("failed to parse response: %w", err)
			}
			return resp.Data, resp.Links.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "EMAIL", "ROLE"}
		var rows [][]string
		for _, i := range items {
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
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		body, err := doRawRequest(ms, ctx, http.MethodGet, "https://api.mailersend.com/v1/invites/"+args[0], nil)
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
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = doRawRequest(ms, ctx, http.MethodPost, "https://api.mailersend.com/v1/invites/"+args[0]+"/resend", nil)
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
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = doRawRequest(ms, ctx, http.MethodDelete, "https://api.mailersend.com/v1/invites/"+args[0], nil)
		if err != nil {
			return err
		}

		output.Success("Invite " + args[0] + " cancelled successfully.")
		return nil
	},
}

// updateCmd uses raw HTTP because the SDK's UpdateUserOptions only supports role,
// but the API also accepts permissions, templates, and domains.
var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
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

		ctx := context.Background()
		body, err := doRawRequest(ms, ctx, http.MethodPut, "https://api.mailersend.com/v1/users/"+args[0], payload)
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

		output.Success("User " + args[0] + " updated successfully.")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.User.Delete(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		output.Success("User " + args[0] + " deleted successfully.")
		return nil
	},
}

// --- Helpers ---

type inviteItem struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// doRawRequest performs an HTTP request using the SDK's transport-equipped client.
func doRawRequest(ms *mailersend.Mailersend, ctx context.Context, method, url string, payload interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+ms.APIKey())
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ms.Client().Do(req)
	if err != nil {
		return nil, sdkclient.WrapError(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, parseHTTPError(resp.StatusCode, body)
	}

	return body, nil
}

// parseHTTPError creates a CLIError from a raw HTTP error response.
func parseHTTPError(statusCode int, body []byte) error {
	cliErr := &sdkclient.CLIError{
		StatusCode: statusCode,
	}
	if len(body) > 0 {
		var parsed struct {
			Message string              `json:"message"`
			Errors  map[string][]string `json:"errors"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			cliErr.Message = parsed.Message
			if len(parsed.Errors) > 0 {
				cliErr.Errors = parsed.Errors
			}
		}
		if cliErr.Message == "" {
			cliErr.Message = string(body)
		}
	}
	return cliErr
}
