package token

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
	createCmd.Flags().String("domain", "", "domain name or ID (required)")
	createCmd.Flags().StringSlice("scopes", nil, "token scopes (required)")

	updateCmd.Flags().String("name", "", "token name")

	updateStatusCmd.Flags().String("status", "", "token status: pause or unpause (required)")
}

// --- list ---
// The SDK does not have a List method for tokens, so we use raw HTTP.

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List API tokens",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		type tokenItem struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			CreatedAt string `json:"created_at"`
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]tokenItem, bool, error) {
			url := fmt.Sprintf("https://api.mailersend.com/v1/token?page=%d&limit=%d", page, perPage)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return nil, false, err
			}
			req.Header.Set("Authorization", "Bearer "+ms.APIKey())
			req.Header.Set("Accept", "application/json")

			resp, err := ms.Client().Do(req)
			if err != nil {
				return nil, false, err
			}
			defer resp.Body.Close() //nolint:errcheck

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, false, err
			}

			if resp.StatusCode >= 400 {
				return nil, false, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
			}

			var parsed struct {
				Data  []tokenItem `json:"data"`
				Links struct {
					Next string `json:"next"`
				} `json:"links"`
			}
			if err := json.Unmarshal(body, &parsed); err != nil {
				return nil, false, fmt.Errorf("failed to parse response: %w", err)
			}
			return parsed.Data, parsed.Links.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "NAME", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, t := range items {
			rows = append(rows, []string{t.ID, t.Name, t.Status, t.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

// --- get ---
// The SDK does not have a Get method for tokens, so we use raw HTTP.

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get API token details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		url := fmt.Sprintf("https://api.mailersend.com/v1/token/%s", args[0])
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+ms.APIKey())
		req.Header.Set("Accept", "application/json")

		resp, err := ms.Client().Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close() //nolint:errcheck

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		var parsed struct {
			Data struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Status    string `json:"status"`
				CreatedAt string `json:"created_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := parsed.Data
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

// --- create ---

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API token",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		name, _ := c.Flags().GetString("name")
		name, err = prompt.RequireArg(name, "name", "Token name")
		if err != nil {
			return err
		}
		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
		scopes, _ := c.Flags().GetStringSlice("scopes")
		scopes, err = prompt.RequireSliceArg(scopes, "scopes", "Token scopes")
		if err != nil {
			return err
		}

		result, _, err := ms.Token.Create(ctx, &mailersend.CreateTokenOptions{
			Name:     name,
			DomainID: domainID,
			Scopes:   scopes,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Token created successfully. ID: " + result.Data.ID)
		if result.Data.AccessToken != "" {
			fmt.Printf("Access Token: %s\n", result.Data.AccessToken)
		}
		return nil
	},
}

// --- update ---
// The SDK's Update only supports status changes (PUT /token/{id}/settings).
// For name updates via PUT /v1/token/{id}, we use raw HTTP.

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		payload := map[string]interface{}{}

		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			payload["name"] = v
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		url := fmt.Sprintf("https://api.mailersend.com/v1/token/%s", args[0])
		req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyBytes))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+ms.APIKey())
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := ms.Client().Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close() //nolint:errcheck

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if resp.StatusCode >= 400 {
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
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

// --- update-status ---

var updateStatusCmd = &cobra.Command{
	Use:   "update-status <id>",
	Short: "Update API token status (pause/unpause)",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		status, _ := c.Flags().GetString("status")
		status, err = prompt.RequireArg(status, "status", "Token status (pause or unpause)")
		if err != nil {
			return err
		}

		result, _, err := ms.Token.Update(ctx, &mailersend.UpdateTokenOptions{
			TokenID: args[0],
			Status:  status,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Token " + args[0] + " status updated to " + status + ".")
		return nil
	},
}

// --- delete ---

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.Token.Delete(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		output.Success("Token " + args[0] + " deleted successfully.")
		return nil
	},
}
