package sms

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
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
	_ = sendCmd.MarkFlagRequired("from")
	sendCmd.Flags().StringSlice("to", nil, "recipient phone numbers (required)")
	_ = sendCmd.MarkFlagRequired("to")
	sendCmd.Flags().String("text", "", "message text (required)")
	_ = sendCmd.MarkFlagRequired("text")
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send an SMS",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		from, _ := c.Flags().GetString("from")
		to, _ := c.Flags().GetStringSlice("to")
		text, _ := c.Flags().GetString("text")

		payload := map[string]interface{}{
			"from": from,
			"to":   to,
			"text": text,
		}

		respBody, _, err := client.Post("/v1/sms", payload)
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

// boolStr converts a bool flag to a query parameter string.
func boolStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

// printRawJSON is a helper to output raw JSON for --json flag.
func printRawJSON(body []byte) error {
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return err
	}
	return output.JSON(raw)
}

// parseDataField extracts the "data" wrapper from API responses.
func parseDataField(body []byte, target interface{}) error {
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return json.Unmarshal(wrapper.Data, target)
}
