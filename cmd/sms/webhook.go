package sms

import (
	"context"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Manage SMS webhooks",
}

func init() {
	webhookCmd.AddCommand(webhookListCmd)
	webhookCmd.AddCommand(webhookGetCmd)
	webhookCmd.AddCommand(webhookCreateCmd)
	webhookCmd.AddCommand(webhookUpdateCmd)
	webhookCmd.AddCommand(webhookDeleteCmd)

	webhookListCmd.Flags().String("sms-number-id", "", "SMS number ID (required)")

	webhookCreateCmd.Flags().String("sms-number-id", "", "SMS number ID (required)")
	webhookCreateCmd.Flags().String("name", "", "webhook name (required)")
	webhookCreateCmd.Flags().String("url", "", "webhook URL (required)")
	webhookCreateCmd.Flags().StringSlice("events", nil, "webhook events (required)")
	webhookCreateCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")

	webhookUpdateCmd.Flags().String("name", "", "webhook name")
	webhookUpdateCmd.Flags().String("url", "", "webhook URL")
	webhookUpdateCmd.Flags().StringSlice("events", nil, "webhook events")
	webhookUpdateCmd.Flags().Bool("enabled", true, "whether the webhook is enabled")
}

var webhookListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS webhooks",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		smsNumberID, _ := c.Flags().GetString("sms-number-id")
		smsNumberID, err = prompt.RequireArg(smsNumberID, "sms-number-id", "SMS number ID")
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmsWebhook.List(ctx, &mailersend.ListSmsWebhookOptions{
			SmsNumberId: smsNumberID,
		})
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		headers := []string{"ID", "NAME", "URL", "ENABLED"}
		var rows [][]string
		for _, w := range result.Data {
			rows = append(rows, []string{w.Id, w.Name, output.Truncate(w.Url, 50), boolYesNo(w.Enabled)})
		}

		output.Table(headers, rows)
		return nil
	},
}

var webhookGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS webhook details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmsWebhook.Get(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.Id},
			{"Name", d.Name},
			{"URL", d.Url},
			{"Enabled", boolYesNo(d.Enabled)},
		}
		output.Table(headers, rows)

		if len(d.Events) > 0 {
			fmt.Println("\nEvents:")
			for _, e := range d.Events {
				fmt.Printf("  - %s\n", e)
			}
		}
		return nil
	},
}

var webhookCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an SMS webhook",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		smsNumberID, _ := c.Flags().GetString("sms-number-id")
		smsNumberID, err = prompt.RequireArg(smsNumberID, "sms-number-id", "SMS number ID")
		if err != nil {
			return err
		}
		name, _ := c.Flags().GetString("name")
		name, err = prompt.RequireArg(name, "name", "Webhook name")
		if err != nil {
			return err
		}
		webhookURL, _ := c.Flags().GetString("url")
		webhookURL, err = prompt.RequireArg(webhookURL, "url", "Webhook URL")
		if err != nil {
			return err
		}
		events, _ := c.Flags().GetStringSlice("events")
		events, err = prompt.RequireSliceArg(events, "events", "Webhook events")
		if err != nil {
			return err
		}
		enabled, _ := c.Flags().GetBool("enabled")

		ctx := context.Background()
		result, _, err := ms.SmsWebhook.Create(ctx, &mailersend.CreateSmsWebhookOptions{
			SmsNumberId: smsNumberID,
			Name:        name,
			URL:         webhookURL,
			Events:      events,
			Enabled:     mailersend.Bool(enabled),
		})
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMS webhook created successfully. ID: " + result.Data.Id)
		return nil
	},
}

var webhookUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		opts := &mailersend.UpdateSmsWebhookOptions{
			Id: args[0],
		}

		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			opts.Name = v
		}
		if c.Flags().Changed("url") {
			v, _ := c.Flags().GetString("url")
			opts.URL = v
		}
		if c.Flags().Changed("events") {
			v, _ := c.Flags().GetStringSlice("events")
			opts.Events = v
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			opts.Enabled = mailersend.Bool(v)
		}

		ctx := context.Background()
		result, _, err := ms.SmsWebhook.Update(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMS webhook " + args[0] + " updated successfully.")
		return nil
	},
}

var webhookDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMS webhook",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.SmsWebhook.Delete(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		output.Success("SMS webhook " + args[0] + " deleted successfully.")
		return nil
	},
}
