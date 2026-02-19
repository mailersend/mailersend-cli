package sms

import (
	"context"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "sms",
	Short: "Manage SMS",
	Long:  "Send SMS, manage messages, activity, phone numbers, recipients, inbound routes, and webhooks.",
}

func init() {
	Cmd.AddCommand(sendCmd)
	Cmd.AddCommand(messageCmd)
	Cmd.AddCommand(activityCmd)
	Cmd.AddCommand(numberCmd)
	Cmd.AddCommand(recipientCmd)
	Cmd.AddCommand(inboundCmd)
	Cmd.AddCommand(webhookCmd)

	sendCmd.Flags().String("from", "", "sender phone number (required)")
	sendCmd.Flags().StringSlice("to", nil, "recipient phone numbers (required)")
	sendCmd.Flags().String("text", "", "message text (required)")
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send an SMS",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		from, _ := c.Flags().GetString("from")
		from, err = prompt.RequireArg(from, "from", "Sender phone number")
		if err != nil {
			return err
		}
		to, _ := c.Flags().GetStringSlice("to")
		to, err = prompt.RequireSliceArg(to, "to", "Recipient phone numbers")
		if err != nil {
			return err
		}
		text, _ := c.Flags().GetString("text")
		text, err = prompt.RequireArg(text, "text", "Message text")
		if err != nil {
			return err
		}

		smsMsg := ms.Sms.NewMessage()
		smsMsg.From = from
		smsMsg.To = to
		smsMsg.Text = text

		ctx := context.Background()
		_, err = ms.Sms.Send(ctx, smsMsg)
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(map[string]string{"status": "sent"})
		}

		output.Success("SMS sent successfully.")
		return nil
	},
}

// boolYesNo converts a bool to "Yes"/"No" string.
func boolYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
