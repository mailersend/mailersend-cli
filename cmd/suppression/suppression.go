package suppression

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
	Use:   "suppression",
	Short: "Manage suppressions",
	Long:  "Manage blocklist, hard bounces, spam complaints, unsubscribes, and on-hold list.",
}

func init() {
	Cmd.AddCommand(blocklistCmd)
	Cmd.AddCommand(hardBouncesCmd)
	Cmd.AddCommand(spamComplaintsCmd)
	Cmd.AddCommand(unsubscribesCmd)
	Cmd.AddCommand(onHoldCmd)
}

// --- helpers ---

// suppressionItem is a generic representation for table display across all suppression types.
type suppressionItem struct {
	ID           string
	Type         string
	PatternEmail string
	CreatedAt    string
}

func addListFlags(cmd *cobra.Command) {
	cmd.Flags().Int("limit", 0, "maximum number of items to return (0 = all)")
	cmd.Flags().String("domain", "", "filter by domain name or ID")
}

func addDeleteFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("ids", nil, "IDs to delete")
	cmd.Flags().Bool("all", false, "delete all entries")
	cmd.Flags().String("domain", "", "domain name or ID")
}

func suppressionDeleteRun(suppressionType string) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		ids, _ := c.Flags().GetStringSlice("ids")
		all, _ := c.Flags().GetBool("all")

		if len(ids) == 0 && !all {
			return fmt.Errorf("provide --ids or --all")
		}

		var domainID string
		if c.Flags().Changed("domain") {
			domainID, _ = c.Flags().GetString("domain")
			domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
			if err != nil {
				return err
			}
		}

		if all {
			_, err = ms.Suppression.DeleteAll(ctx, domainID, suppressionType)
		} else {
			_, err = ms.Suppression.Delete(ctx, &mailersend.DeleteSuppressionOptions{
				DomainID: domainID,
				Ids:      ids,
			}, suppressionType)
		}
		if err != nil {
			return sdkclient.WrapError(err)
		}

		output.Success("Suppression entries deleted successfully.")
		return nil
	}
}

// --- blocklist ---

var blocklistCmd = &cobra.Command{
	Use:   "blocklist",
	Short: "Manage blocklist suppressions",
}

var blocklistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List blocklist entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		domainID, _ := c.Flags().GetString("domain")
		if domainID != "" {
			domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
			if err != nil {
				return err
			}
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]suppressionItem, bool, error) {
			root, _, err := ms.Suppression.ListBlockList(ctx, &mailersend.SuppressionOptions{
				DomainID: domainID,
				Page:     page,
				Limit:    perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(err)
			}
			var out []suppressionItem
			for _, d := range root.Data {
				out = append(out, suppressionItem{
					ID:           d.ID,
					Type:         d.Type,
					PatternEmail: d.Pattern,
					CreatedAt:    d.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			return out, root.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"}
		var rows [][]string
		for _, i := range items {
			rows = append(rows, []string{i.ID, i.Type, i.PatternEmail, i.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var blocklistAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add entries to the blocklist",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
		recipients, _ := c.Flags().GetStringSlice("recipients")
		patterns, _ := c.Flags().GetStringSlice("patterns")

		result, _, err := ms.Suppression.CreateBlock(ctx, &mailersend.CreateSuppressionBlockOptions{
			DomainID:   domainID,
			Recipients: recipients,
			Patterns:   patterns,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Blocklist entries added successfully.")
		return nil
	},
}

var blocklistDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete blocklist entries",
	RunE:  suppressionDeleteRun(mailersend.BlockList),
}

func init() {
	blocklistCmd.AddCommand(blocklistListCmd)
	blocklistCmd.AddCommand(blocklistAddCmd)
	blocklistCmd.AddCommand(blocklistDeleteCmd)

	addListFlags(blocklistListCmd)

	blocklistAddCmd.Flags().String("domain", "", "domain name or ID (required)")
	blocklistAddCmd.Flags().StringSlice("recipients", nil, "recipient emails to block")
	blocklistAddCmd.Flags().StringSlice("patterns", nil, "patterns to block")

	addDeleteFlags(blocklistDeleteCmd)
}

// --- hard-bounces ---

var hardBouncesCmd = &cobra.Command{
	Use:   "hard-bounces",
	Short: "Manage hard bounce suppressions",
}

var hardBouncesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List hard bounce entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		domainID, _ := c.Flags().GetString("domain")
		if domainID != "" {
			domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
			if err != nil {
				return err
			}
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]suppressionItem, bool, error) {
			root, _, err := ms.Suppression.ListHardBounces(ctx, &mailersend.SuppressionOptions{
				DomainID: domainID,
				Page:     page,
				Limit:    perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(err)
			}
			var out []suppressionItem
			for _, d := range root.Data {
				out = append(out, suppressionItem{
					ID:           d.ID,
					PatternEmail: d.Recipient.Email,
					CreatedAt:    d.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			return out, root.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"}
		var rows [][]string
		for _, i := range items {
			rows = append(rows, []string{i.ID, i.Type, i.PatternEmail, i.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var hardBouncesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add hard bounce entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
		recipients, _ := c.Flags().GetStringSlice("recipients")

		result, _, err := ms.Suppression.CreateHardBounce(ctx, &mailersend.CreateSuppressionOptions{
			DomainID:   domainID,
			Recipients: recipients,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Hard bounce entries added successfully.")
		return nil
	},
}

var hardBouncesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete hard bounce entries",
	RunE:  suppressionDeleteRun(mailersend.HardBounces),
}

func init() {
	hardBouncesCmd.AddCommand(hardBouncesListCmd)
	hardBouncesCmd.AddCommand(hardBouncesAddCmd)
	hardBouncesCmd.AddCommand(hardBouncesDeleteCmd)

	addListFlags(hardBouncesListCmd)

	hardBouncesAddCmd.Flags().String("domain", "", "domain name or ID (required)")
	hardBouncesAddCmd.Flags().StringSlice("recipients", nil, "recipient emails")

	addDeleteFlags(hardBouncesDeleteCmd)
}

// --- spam-complaints ---

var spamComplaintsCmd = &cobra.Command{
	Use:   "spam-complaints",
	Short: "Manage spam complaint suppressions",
}

var spamComplaintsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List spam complaint entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		domainID, _ := c.Flags().GetString("domain")
		if domainID != "" {
			domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
			if err != nil {
				return err
			}
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]suppressionItem, bool, error) {
			root, _, err := ms.Suppression.ListSpamComplaints(ctx, &mailersend.SuppressionOptions{
				DomainID: domainID,
				Page:     page,
				Limit:    perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(err)
			}
			var out []suppressionItem
			for _, d := range root.Data {
				out = append(out, suppressionItem{
					ID:           d.ID,
					PatternEmail: d.Recipient.Email,
					CreatedAt:    d.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			return out, root.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"}
		var rows [][]string
		for _, i := range items {
			rows = append(rows, []string{i.ID, i.Type, i.PatternEmail, i.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var spamComplaintsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add spam complaint entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
		recipients, _ := c.Flags().GetStringSlice("recipients")

		result, _, err := ms.Suppression.CreateSpamComplaint(ctx, &mailersend.CreateSuppressionOptions{
			DomainID:   domainID,
			Recipients: recipients,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Spam complaint entries added successfully.")
		return nil
	},
}

var spamComplaintsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete spam complaint entries",
	RunE:  suppressionDeleteRun(mailersend.SpamComplaints),
}

func init() {
	spamComplaintsCmd.AddCommand(spamComplaintsListCmd)
	spamComplaintsCmd.AddCommand(spamComplaintsAddCmd)
	spamComplaintsCmd.AddCommand(spamComplaintsDeleteCmd)

	addListFlags(spamComplaintsListCmd)

	spamComplaintsAddCmd.Flags().String("domain", "", "domain name or ID (required)")
	spamComplaintsAddCmd.Flags().StringSlice("recipients", nil, "recipient emails")

	addDeleteFlags(spamComplaintsDeleteCmd)
}

// --- unsubscribes ---

var unsubscribesCmd = &cobra.Command{
	Use:   "unsubscribes",
	Short: "Manage unsubscribe suppressions",
}

var unsubscribesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List unsubscribe entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		domainID, _ := c.Flags().GetString("domain")
		if domainID != "" {
			domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
			if err != nil {
				return err
			}
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]suppressionItem, bool, error) {
			root, _, err := ms.Suppression.ListUnsubscribes(ctx, &mailersend.SuppressionOptions{
				DomainID: domainID,
				Page:     page,
				Limit:    perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(err)
			}
			var out []suppressionItem
			for _, d := range root.Data {
				out = append(out, suppressionItem{
					ID:           d.ID,
					PatternEmail: d.Recipient.Email,
					CreatedAt:    d.CreatedAt.Format("2006-01-02 15:04:05"),
				})
			}
			return out, root.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"}
		var rows [][]string
		for _, i := range items {
			rows = append(rows, []string{i.ID, i.Type, i.PatternEmail, i.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var unsubscribesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add unsubscribe entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
		recipients, _ := c.Flags().GetStringSlice("recipients")

		result, _, err := ms.Suppression.CreateUnsubscribe(ctx, &mailersend.CreateSuppressionOptions{
			DomainID:   domainID,
			Recipients: recipients,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Unsubscribe entries added successfully.")
		return nil
	},
}

var unsubscribesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete unsubscribe entries",
	RunE:  suppressionDeleteRun(mailersend.Unsubscribes),
}

func init() {
	unsubscribesCmd.AddCommand(unsubscribesListCmd)
	unsubscribesCmd.AddCommand(unsubscribesAddCmd)
	unsubscribesCmd.AddCommand(unsubscribesDeleteCmd)

	addListFlags(unsubscribesListCmd)

	unsubscribesAddCmd.Flags().String("domain", "", "domain name or ID (required)")
	unsubscribesAddCmd.Flags().StringSlice("recipients", nil, "recipient emails")

	addDeleteFlags(unsubscribesDeleteCmd)
}

// --- on-hold ---
// The SDK does not have dedicated on-hold endpoints, so we use raw HTTP
// requests via the SDK's HTTP client (which includes the CLI transport for
// retries, verbose logging, and base URL rewrite).

var onHoldCmd = &cobra.Command{
	Use:   "on-hold",
	Short: "Manage on-hold list",
}

var onHoldListCmd = &cobra.Command{
	Use:   "list",
	Short: "List on-hold entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		domainID, _ := c.Flags().GetString("domain")
		if domainID != "" {
			domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
			if err != nil {
				return err
			}
		}

		type rawItem struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Pattern   string `json:"pattern"`
			Recipient struct {
				Email string `json:"email"`
			} `json:"recipient"`
			CreatedAt string `json:"created_at"`
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]rawItem, bool, error) {
			url := fmt.Sprintf("https://api.mailersend.com/v1/suppressions/on-hold-list?page=%d&limit=%d", page, perPage)
			if domainID != "" {
				url += "&domain_id=" + domainID
			}
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
				Data  []rawItem `json:"data"`
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

		headers := []string{"ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"}
		var rows [][]string
		for _, i := range items {
			value := i.Pattern
			if value == "" {
				value = i.Recipient.Email
			}
			rows = append(rows, []string{i.ID, i.Type, value, i.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var onHoldDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete on-hold entries",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		payload := map[string]interface{}{}

		if ids, _ := c.Flags().GetStringSlice("ids"); len(ids) > 0 {
			payload["ids"] = ids
		}
		if all, _ := c.Flags().GetBool("all"); all {
			payload["all"] = true
		}

		if len(payload) == 0 {
			return fmt.Errorf("provide --ids or --all")
		}

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "DELETE", "https://api.mailersend.com/v1/suppressions/on-hold-list", bytes.NewReader(bodyBytes))
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

		if resp.StatusCode >= 400 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		output.Success("On-hold entries deleted successfully.")
		return nil
	},
}

func init() {
	onHoldCmd.AddCommand(onHoldListCmd)
	onHoldCmd.AddCommand(onHoldDeleteCmd)

	onHoldListCmd.Flags().Int("limit", 0, "maximum number of items to return (0 = all)")
	onHoldListCmd.Flags().String("domain", "", "filter by domain name or ID")

	onHoldDeleteCmd.Flags().StringSlice("ids", nil, "IDs to delete")
	onHoldDeleteCmd.Flags().Bool("all", false, "delete all entries")
}
