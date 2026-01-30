package suppression

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
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

func suppressionListRun(endpoint string) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
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

		items, err := client.GetPaginated(endpoint, params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID        string `json:"id"`
			Type      string `json:"type"`
			Pattern   string `json:"pattern"`
			Recipient struct {
				Email string `json:"email"`
			} `json:"recipient"`
			CreatedAt string `json:"created_at"`
		}

		headers := []string{"ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var i item
			if err := json.Unmarshal(raw, &i); err != nil {
				return fmt.Errorf("failed to parse suppression: %w", err)
			}
			value := i.Pattern
			if value == "" {
				value = i.Recipient.Email
			}
			rows = append(rows, []string{i.ID, i.Type, value, i.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	}
}

func suppressionDeleteRun(endpoint string) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		payload := map[string]interface{}{}

		if ids, _ := c.Flags().GetStringSlice("ids"); len(ids) > 0 {
			payload["ids"] = ids
		}
		if all, _ := c.Flags().GetBool("all"); all {
			payload["all"] = true
		}
		if c.Flags().Changed("domain-id") {
			domainID, _ := c.Flags().GetString("domain-id")
			domainID, err = cmdutil.ResolveDomain(client, domainID)
			if err != nil {
				return err
			}
			payload["domain_id"] = domainID
		}

		if len(payload) == 0 {
			return fmt.Errorf("provide --ids or --all")
		}

		_, err = client.DeleteWithBody(endpoint, payload)
		if err != nil {
			return err
		}

		output.Success("Suppression entries deleted successfully.")
		return nil
	}
}

func addListFlags(cmd *cobra.Command) {
	cmd.Flags().Int("limit", 0, "maximum number of items to return (0 = all)")
	cmd.Flags().String("domain-id", "", "filter by domain ID")
}

func addDeleteFlags(cmd *cobra.Command) {
	cmd.Flags().StringSlice("ids", nil, "IDs to delete")
	cmd.Flags().Bool("all", false, "delete all entries")
	cmd.Flags().String("domain-id", "", "domain ID")
}

// --- blocklist ---

var blocklistCmd = &cobra.Command{
	Use:   "blocklist",
	Short: "Manage blocklist suppressions",
}

var blocklistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List blocklist entries",
	RunE:  suppressionListRun("/v1/suppressions/blocklist"),
}

var blocklistAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add entries to the blocklist",
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
		recipients, _ := c.Flags().GetStringSlice("recipients")
		patterns, _ := c.Flags().GetStringSlice("patterns")

		payload := map[string]interface{}{
			"domain_id": domainID,
		}
		if len(recipients) > 0 {
			payload["recipients"] = recipients
		}
		if len(patterns) > 0 {
			payload["patterns"] = patterns
		}

		respBody, _, err := client.Post("/v1/suppressions/blocklist", payload)
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

		output.Success("Blocklist entries added successfully.")
		return nil
	},
}

var blocklistDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete blocklist entries",
	RunE:  suppressionDeleteRun("/v1/suppressions/blocklist"),
}

func init() {
	blocklistCmd.AddCommand(blocklistListCmd)
	blocklistCmd.AddCommand(blocklistAddCmd)
	blocklistCmd.AddCommand(blocklistDeleteCmd)

	addListFlags(blocklistListCmd)

	blocklistAddCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = blocklistAddCmd.MarkFlagRequired("domain-id")
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
	RunE:  suppressionListRun("/v1/suppressions/hard-bounces"),
}

var hardBouncesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add hard bounce entries",
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
		recipients, _ := c.Flags().GetStringSlice("recipients")

		payload := map[string]interface{}{
			"domain_id":  domainID,
			"recipients": recipients,
		}

		respBody, _, err := client.Post("/v1/suppressions/hard-bounces", payload)
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

		output.Success("Hard bounce entries added successfully.")
		return nil
	},
}

var hardBouncesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete hard bounce entries",
	RunE:  suppressionDeleteRun("/v1/suppressions/hard-bounces"),
}

func init() {
	hardBouncesCmd.AddCommand(hardBouncesListCmd)
	hardBouncesCmd.AddCommand(hardBouncesAddCmd)
	hardBouncesCmd.AddCommand(hardBouncesDeleteCmd)

	addListFlags(hardBouncesListCmd)

	hardBouncesAddCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = hardBouncesAddCmd.MarkFlagRequired("domain-id")
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
	RunE:  suppressionListRun("/v1/suppressions/spam-complaints"),
}

var spamComplaintsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add spam complaint entries",
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
		recipients, _ := c.Flags().GetStringSlice("recipients")

		payload := map[string]interface{}{
			"domain_id":  domainID,
			"recipients": recipients,
		}

		respBody, _, err := client.Post("/v1/suppressions/spam-complaints", payload)
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

		output.Success("Spam complaint entries added successfully.")
		return nil
	},
}

var spamComplaintsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete spam complaint entries",
	RunE:  suppressionDeleteRun("/v1/suppressions/spam-complaints"),
}

func init() {
	spamComplaintsCmd.AddCommand(spamComplaintsListCmd)
	spamComplaintsCmd.AddCommand(spamComplaintsAddCmd)
	spamComplaintsCmd.AddCommand(spamComplaintsDeleteCmd)

	addListFlags(spamComplaintsListCmd)

	spamComplaintsAddCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = spamComplaintsAddCmd.MarkFlagRequired("domain-id")
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
	RunE:  suppressionListRun("/v1/suppressions/unsubscribes"),
}

var unsubscribesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add unsubscribe entries",
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
		recipients, _ := c.Flags().GetStringSlice("recipients")

		payload := map[string]interface{}{
			"domain_id":  domainID,
			"recipients": recipients,
		}

		respBody, _, err := client.Post("/v1/suppressions/unsubscribes", payload)
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

		output.Success("Unsubscribe entries added successfully.")
		return nil
	},
}

var unsubscribesDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete unsubscribe entries",
	RunE:  suppressionDeleteRun("/v1/suppressions/unsubscribes"),
}

func init() {
	unsubscribesCmd.AddCommand(unsubscribesListCmd)
	unsubscribesCmd.AddCommand(unsubscribesAddCmd)
	unsubscribesCmd.AddCommand(unsubscribesDeleteCmd)

	addListFlags(unsubscribesListCmd)

	unsubscribesAddCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = unsubscribesAddCmd.MarkFlagRequired("domain-id")
	unsubscribesAddCmd.Flags().StringSlice("recipients", nil, "recipient emails")

	addDeleteFlags(unsubscribesDeleteCmd)
}

// --- on-hold ---

var onHoldCmd = &cobra.Command{
	Use:   "on-hold",
	Short: "Manage on-hold list",
}

var onHoldListCmd = &cobra.Command{
	Use:   "list",
	Short: "List on-hold entries",
	RunE:  suppressionListRun("/v1/suppressions/on-hold-list"),
}

var onHoldDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete on-hold entries",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

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

		_, err = client.DeleteWithBody("/v1/suppressions/on-hold-list", payload)
		if err != nil {
			return err
		}

		output.Success("On-hold entries deleted successfully.")
		return nil
	},
}

func init() {
	onHoldCmd.AddCommand(onHoldListCmd)
	onHoldCmd.AddCommand(onHoldDeleteCmd)

	onHoldListCmd.Flags().Int("limit", 0, "maximum number of items to return (0 = all)")
	onHoldListCmd.Flags().String("domain-id", "", "filter by domain ID")

	onHoldDeleteCmd.Flags().StringSlice("ids", nil, "IDs to delete")
	onHoldDeleteCmd.Flags().Bool("all", false, "delete all entries")
}
