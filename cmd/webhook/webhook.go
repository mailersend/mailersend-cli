package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var webhookEvents = []string{
	"activity.sent",
	"activity.delivered",
	"activity.soft_bounced",
	"activity.hard_bounced",
	"activity.opened",
	"activity.opened_unique",
	"activity.clicked",
	"activity.clicked_unique",
	"activity.unsubscribed",
	"activity.spam_complaint",
	"activity.survey_opened",
	"activity.survey_submitted",
	"maintenance.start",
	"maintenance.end",
	"email_single.verified",
	"email_list.verified",
	"bulk_email.completed",
}

var Cmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage webhooks",
	Long:  "List, view, create, update, and delete webhooks.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deleteCmd)

	// list flags
	listCmd.Flags().String("domain-id", "", "domain ID or name (required)")
	_ = listCmd.MarkFlagRequired("domain-id")
	listCmd.Flags().Int("limit", 0, "maximum number of webhooks to return")

	// create flags
	createCmd.Flags().String("name", "", "webhook name (required)")
	_ = createCmd.MarkFlagRequired("name")
	createCmd.Flags().String("url", "", "webhook URL (required)")
	_ = createCmd.MarkFlagRequired("url")
	createCmd.Flags().String("domain-id", "", "domain ID or name (required)")
	_ = createCmd.MarkFlagRequired("domain-id")
	createCmd.Flags().StringSlice("events", nil, "webhook events (required)")
	_ = createCmd.MarkFlagRequired("events")
	createCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")

	// update flags
	updateCmd.Flags().String("name", "", "webhook name")
	updateCmd.Flags().String("url", "", "webhook URL")
	updateCmd.Flags().StringSlice("events", nil, "webhook events")
	updateCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")
}

// --- list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List webhooks for a domain",
	RunE:  runList,
}

func runList(c *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	limit, _ := c.Flags().GetInt("limit")
	domainID, _ := c.Flags().GetString("domain-id")
	domainID, err = cmdutil.ResolveDomain(client, domainID)
	if err != nil {
		return err
	}

	params := map[string]string{
		"domain_id": domainID,
	}

	items, err := client.GetPaginated("/v1/webhooks", params, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(c) {
		return output.JSON(items)
	}

	type webhookItem struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		URL       string `json:"url"`
		Enabled   bool   `json:"enabled"`
		CreatedAt string `json:"created_at"`
	}

	headers := []string{"ID", "NAME", "URL", "ENABLED", "CREATED AT"}
	var rows [][]string

	for _, raw := range items {
		var w webhookItem
		if err := json.Unmarshal(raw, &w); err != nil {
			return fmt.Errorf("failed to parse webhook: %w", err)
		}
		enabled := "No"
		if w.Enabled {
			enabled = "Yes"
		}
		rows = append(rows, []string{
			w.ID,
			output.Truncate(w.Name, 40),
			output.Truncate(w.URL, 50),
			enabled,
			w.CreatedAt,
		})
	}

	output.Table(headers, rows)
	return nil
}

// --- get ---

var getCmd = &cobra.Command{
	Use:   "get <webhook_id>",
	Short: "Get webhook details",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func runGet(c *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	body, err := client.Get("/v1/webhooks/"+args[0], nil)
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
			ID        string   `json:"id"`
			Name      string   `json:"name"`
			URL       string   `json:"url"`
			Events    []string `json:"events"`
			Enabled   bool     `json:"enabled"`
			CreatedAt string   `json:"created_at"`
			UpdatedAt string   `json:"updated_at"`
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

	fmt.Printf("ID:           %s\n", d.ID)
	fmt.Printf("Name:         %s\n", d.Name)
	fmt.Printf("URL:          %s\n", d.URL)
	fmt.Printf("Enabled:      %s\n", enabled)
	fmt.Printf("Created At:   %s\n", d.CreatedAt)
	fmt.Printf("Updated At:   %s\n", d.UpdatedAt)

	fmt.Println()
	fmt.Println("Events:")
	for _, e := range d.Events {
		fmt.Printf("  - %s\n", e)
	}

	return nil
}

// --- create ---

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a webhook",
	Long:  "Create a new webhook.\n\nValid events: " + strings.Join(webhookEvents, ", "),
	RunE:  runCreate,
}

func runCreate(c *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	name, _ := c.Flags().GetString("name")
	url, _ := c.Flags().GetString("url")
	domainID, _ := c.Flags().GetString("domain-id")
	domainID, err = cmdutil.ResolveDomain(client, domainID)
	if err != nil {
		return err
	}
	events, _ := c.Flags().GetStringSlice("events")
	enabled, _ := c.Flags().GetBool("enabled")

	payload := map[string]interface{}{
		"name":      name,
		"url":       url,
		"domain_id": domainID,
		"events":    events,
		"enabled":   enabled,
	}

	respBody, _, err := client.Post("/v1/webhooks", payload)
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

	output.Success("Webhook created successfully. ID: " + resp.Data.ID)
	return nil
}

// --- update ---

var updateCmd = &cobra.Command{
	Use:   "update <webhook_id>",
	Short: "Update a webhook",
	Long:  "Update an existing webhook.\n\nValid events: " + strings.Join(webhookEvents, ", "),
	Args:  cobra.ExactArgs(1),
	RunE:  runUpdate,
}

func runUpdate(c *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	payload := map[string]interface{}{}

	if c.Flags().Changed("name") {
		name, _ := c.Flags().GetString("name")
		payload["name"] = name
	}
	if c.Flags().Changed("url") {
		url, _ := c.Flags().GetString("url")
		payload["url"] = url
	}
	if c.Flags().Changed("events") {
		events, _ := c.Flags().GetStringSlice("events")
		payload["events"] = events
	}
	if c.Flags().Changed("enabled") {
		enabled, _ := c.Flags().GetBool("enabled")
		payload["enabled"] = enabled
	}

	respBody, err := client.Put("/v1/webhooks/"+args[0], payload)
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

	output.Success("Webhook " + args[0] + " updated successfully.")
	return nil
}

// --- delete ---

var deleteCmd = &cobra.Command{
	Use:   "delete <webhook_id>",
	Short: "Delete a webhook",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func runDelete(c *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	_, err = client.Delete("/v1/webhooks/" + args[0])
	if err != nil {
		return err
	}

	output.Success("Webhook " + args[0] + " deleted successfully.")
	return nil
}
