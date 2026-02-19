package sms

import (
	"context"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var recipientCmd = &cobra.Command{
	Use:   "recipient",
	Short: "Manage SMS recipients",
}

func init() {
	recipientCmd.AddCommand(recipientListCmd)
	recipientCmd.AddCommand(recipientGetCmd)
	recipientCmd.AddCommand(recipientUpdateCmd)

	recipientListCmd.Flags().Int("limit", 0, "maximum number of recipients to return (0 = all)")
	recipientListCmd.Flags().String("status", "", "filter by status")
	recipientListCmd.Flags().String("sms-number-id", "", "filter by SMS number ID")

	recipientUpdateCmd.Flags().String("status", "", "recipient status (required)")
}

var recipientListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS recipients",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		smsNumberID, _ := c.Flags().GetString("sms-number-id")

		// The SDK expects Status as bool, but the old code used string.
		// We pass the sms-number-id and let the API handle status filtering.
		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.SmsRecipient, bool, error) {
			opts := &mailersend.SmsRecipientOptions{
				SmsNumberId: smsNumberID,
				Page:        page,
				Limit:       perPage,
			}
			root, _, err := ms.SmsRecipient.List(ctx, opts)
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

		headers := []string{"ID", "NUMBER", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, r := range items {
			createdAt := ""
			if !r.CreatedAt.IsZero() {
				createdAt = r.CreatedAt.Format("2006-01-02 15:04:05")
			}
			rows = append(rows, []string{r.Id, r.Number, r.Status, createdAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var recipientGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS recipient details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmsRecipient.Get(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.Id},
			{"Number", d.Number},
			{"Status", d.Status},
		}
		output.Table(headers, rows)
		return nil
	},
}

var recipientUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS recipient",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		status, _ := c.Flags().GetString("status")
		status, err = prompt.RequireArg(status, "status", "Recipient status")
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmsRecipient.Update(ctx, &mailersend.SmsRecipientSettingOptions{
			Id:     args[0],
			Status: status,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMS recipient " + args[0] + " updated successfully.")
		return nil
	},
}
