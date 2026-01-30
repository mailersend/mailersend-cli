package bulkemail

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "bulk-email",
	Short: "Manage bulk email",
	Long:  "Send bulk emails and check bulk email status.",
}

func init() {
	Cmd.AddCommand(sendCmd)
	Cmd.AddCommand(statusCmd)

	sendCmd.Flags().String("file", "", "path to JSON file with email array (required)")
	_ = sendCmd.MarkFlagRequired("file")
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send bulk email",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		filePath, _ := c.Flags().GetString("file")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var payload json.RawMessage
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("invalid JSON in file: %w", err)
		}

		respBody, _, err := client.Post("/v1/bulk-email", payload)
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
			BulkEmailID string `json:"bulk_email_id"`
			Message     string `json:"message"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		output.Success(fmt.Sprintf("Bulk email sent. ID: %s", resp.BulkEmailID))
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <bulk_email_id>",
	Short: "Get bulk email status",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/bulk-email/"+args[0], nil)
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
				ID                        string   `json:"id"`
				State                     string   `json:"state"`
				TotalRecipientsCount      int      `json:"total_recipients_count"`
				SuppressedRecipientsCount int      `json:"suppressed_recipients_count"`
				ValidationErrorsCount     int      `json:"validation_errors_count"`
				MessagesID                []string `json:"messages_id"`
				CreatedAt                 string   `json:"created_at"`
				UpdatedAt                 string   `json:"updated_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := resp.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"State", d.State},
			{"Total Recipients", fmt.Sprintf("%d", d.TotalRecipientsCount)},
			{"Suppressed Recipients", fmt.Sprintf("%d", d.SuppressedRecipientsCount)},
			{"Validation Errors", fmt.Sprintf("%d", d.ValidationErrorsCount)},
			{"Messages", strings.Join(d.MessagesID, ", ")},
			{"Created At", d.CreatedAt},
			{"Updated At", d.UpdatedAt},
		}
		output.Table(headers, rows)
		return nil
	},
}
