package bulkemail

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
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
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send bulk email",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		filePath, _ := c.Flags().GetString("file")
		filePath, err = prompt.RequireArg(filePath, "file", "Path to JSON file")
		if err != nil {
			return err
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		var messages []*mailersend.Message
		if err := json.Unmarshal(data, &messages); err != nil {
			return fmt.Errorf("invalid JSON in file: %w", err)
		}

		ctx := context.Background()
		result, _, err := ms.BulkEmail.Send(ctx, messages)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success(fmt.Sprintf("Bulk email sent. ID: %s", result.BulkEmailID))
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status <bulk_email_id>",
	Short: "Get bulk email status",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.BulkEmail.Status(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		createdAt := ""
		if !d.CreatedAt.IsZero() {
			createdAt = d.CreatedAt.Format("2006-01-02 15:04:05")
		}
		updatedAt := ""
		if !d.UpdatedAt.IsZero() {
			updatedAt = d.UpdatedAt.Format("2006-01-02 15:04:05")
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"State", d.State},
			{"Total Recipients", fmt.Sprintf("%d", d.TotalRecipientsCount)},
			{"Suppressed Recipients", fmt.Sprintf("%d", d.SuppressedRecipientsCount)},
			{"Validation Errors", fmt.Sprintf("%d", d.ValidationErrorsCount)},
			{"Messages", strings.Join(d.MessagesID, ", ")},
			{"Created At", createdAt},
			{"Updated At", updatedAt},
		}
		output.Table(headers, rows)
		return nil
	},
}
