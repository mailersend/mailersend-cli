package webhook

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
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
	listCmd.Flags().String("domain", "", "domain name or ID (required)")
	listCmd.Flags().Int("limit", 0, "maximum number of webhooks to return")

	// create flags
	createCmd.Flags().String("name", "", "webhook name (required)")
	createCmd.Flags().String("url", "", "webhook URL (required)")
	createCmd.Flags().String("domain", "", "domain name or ID (required)")
	createCmd.Flags().StringSlice("events", nil, "webhook events (required)")
	createCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")
	createCmd.Flags().Int("version", 2, "webhook payload version (1=legacy, 2=recommended)")

	// update flags
	updateCmd.Flags().String("name", "", "webhook name")
	updateCmd.Flags().String("url", "", "webhook URL")
	updateCmd.Flags().StringSlice("events", nil, "webhook events")
	updateCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")
	updateCmd.Flags().Int("version", 0, "webhook payload version (1 or 2)")
}

// --- list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List webhooks for a domain",
	RunE:  runList,
}

func runList(c *cobra.Command, args []string) error {
	ms, transport, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	limit, _ := c.Flags().GetInt("limit")
	domainID, _ := c.Flags().GetString("domain")
	domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
	if err != nil {
		return err
	}
	domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
	if err != nil {
		return err
	}

	ctx := context.Background()
	opts := &mailersend.ListWebhookOptions{
		DomainID: domainID,
		Limit:    limit,
	}

	result, _, err := ms.Webhook.List(ctx, opts)
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(c) {
		return output.JSON(result.Data)
	}

	headers := []string{"ID", "NAME", "URL", "ENABLED", "CREATED AT"}
	var rows [][]string

	for _, w := range result.Data {
		enabled := "No"
		if w.Enabled {
			enabled = "Yes"
		}
		rows = append(rows, []string{
			w.ID,
			output.Truncate(w.Name, 40),
			output.Truncate(w.URL, 50),
			enabled,
			w.CreatedAt.Format(time.RFC3339),
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
	ms, transport, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, _, err := ms.Webhook.Get(ctx, args[0])
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(c) {
		return output.JSON(result)
	}

	d := result.Data

	enabled := "No"
	if d.Enabled {
		enabled = "Yes"
	}

	fmt.Printf("ID:           %s\n", d.ID)
	fmt.Printf("Name:         %s\n", d.Name)
	fmt.Printf("URL:          %s\n", d.URL)
	fmt.Printf("Enabled:      %s\n", enabled)
	fmt.Printf("Created At:   %s\n", d.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated At:   %s\n", d.UpdatedAt.Format(time.RFC3339))

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
	ms, transport, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	name, _ := c.Flags().GetString("name")
	name, err = prompt.RequireArg(name, "name", "Webhook name")
	if err != nil {
		return err
	}
	url, _ := c.Flags().GetString("url")
	url, err = prompt.RequireArg(url, "url", "Webhook URL")
	if err != nil {
		return err
	}
	domainID, _ := c.Flags().GetString("domain")
	domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
	if err != nil {
		return err
	}
	domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
	if err != nil {
		return err
	}
	events, _ := c.Flags().GetStringSlice("events")
	events, err = prompt.RequireSliceArg(events, "events", "Webhook events")
	if err != nil {
		return err
	}
	enabled, _ := c.Flags().GetBool("enabled")
	version, _ := c.Flags().GetInt("version")

	ctx := context.Background()
	opts := &mailersend.CreateWebhookOptions{
		Name:     name,
		DomainID: domainID,
		URL:      url,
		Enabled:  mailersend.Bool(enabled),
		Events:   events,
		Version:  mailersend.Int(version),
	}

	result, _, err := ms.Webhook.Create(ctx, opts)
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(c) {
		return output.JSON(result)
	}

	output.Success("Webhook created successfully. ID: " + result.Data.ID)
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
	ms, transport, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	opts := &mailersend.UpdateWebhookOptions{
		WebhookID: args[0],
	}

	if c.Flags().Changed("name") {
		name, _ := c.Flags().GetString("name")
		opts.Name = name
	}
	if c.Flags().Changed("url") {
		url, _ := c.Flags().GetString("url")
		opts.URL = url
	}
	if c.Flags().Changed("events") {
		events, _ := c.Flags().GetStringSlice("events")
		opts.Events = events
	}
	if c.Flags().Changed("enabled") {
		enabled, _ := c.Flags().GetBool("enabled")
		opts.Enabled = mailersend.Bool(enabled)
	}
	if c.Flags().Changed("version") {
		version, _ := c.Flags().GetInt("version")
		opts.Version = mailersend.Int(version)
	}

	ctx := context.Background()
	result, _, err := ms.Webhook.Update(ctx, opts)
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(c) {
		return output.JSON(result)
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
	ms, transport, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = ms.Webhook.Delete(ctx, args[0])
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	output.Success("Webhook " + args[0] + " deleted successfully.")
	return nil
}
