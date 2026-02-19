package sms

import (
	"context"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var messageCmd = &cobra.Command{
	Use:   "message",
	Short: "Manage SMS messages",
}

func init() {
	messageCmd.AddCommand(messageListCmd)
	messageCmd.AddCommand(messageGetCmd)

	messageListCmd.Flags().Int("limit", 0, "maximum number of messages to return (0 = all)")
}

var messageListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS messages",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.SmsMessageData, bool, error) {
			root, _, err := ms.SmsMessage.List(ctx, &mailersend.ListSmsMessageOptions{
				Page:  page,
				Limit: perPage,
			})
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

		headers := []string{"ID", "FROM", "TO", "CREATED AT"}
		var rows [][]string
		for _, m := range items {
			createdAt := ""
			if !m.CreatedAt.IsZero() {
				createdAt = m.CreatedAt.Format("2006-01-02 15:04:05")
			}
			toStr := strings.Join(m.To, ", ")
			rows = append(rows, []string{m.Id, m.From, toStr, createdAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var messageGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS message details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmsMessage.Get(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		createdAt := ""
		if !d.CreatedAt.IsZero() {
			createdAt = d.CreatedAt.Format("2006-01-02 15:04:05")
		}
		toStr := strings.Join(d.To, ", ")
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.Id},
			{"From", d.From},
			{"To", toStr},
			{"Text", d.Text},
			{"Created At", createdAt},
		}
		output.Table(headers, rows)
		return nil
	},
}
