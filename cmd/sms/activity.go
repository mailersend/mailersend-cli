package sms

import (
	"context"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var activityCmd = &cobra.Command{
	Use:   "activity",
	Short: "View SMS activity",
}

func init() {
	activityCmd.AddCommand(activityListCmd)

	activityListCmd.Flags().Int("limit", 0, "maximum number of items to return (0 = all)")
	activityListCmd.Flags().String("sms-number-id", "", "filter by SMS number ID")
	activityListCmd.Flags().String("date-from", "", "start date (YYYY-MM-DD or unix timestamp)")
	activityListCmd.Flags().String("date-to", "", "end date (YYYY-MM-DD or unix timestamp)")
	activityListCmd.Flags().StringSlice("status", nil, "filter by status")
}

var activityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS activity",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		smsNumberID, _ := c.Flags().GetString("sms-number-id")
		statuses, _ := c.Flags().GetStringSlice("status")

		var dateFrom, dateTo int64
		if v, _ := c.Flags().GetString("date-from"); v != "" {
			dateFrom, err = cmdutil.ParseDate(v)
			if err != nil {
				return err
			}
		}
		if v, _ := c.Flags().GetString("date-to"); v != "" {
			dateTo, err = cmdutil.ParseDate(v)
			if err != nil {
				return err
			}
		}

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.SmsActivityData, bool, error) {
			opts := &mailersend.SmsActivityOptions{
				SmsNumberId: smsNumberID,
				Status:      statuses,
				Page:        page,
				Limit:       perPage,
			}
			if dateFrom > 0 {
				opts.DateFrom = dateFrom
			}
			if dateTo > 0 {
				opts.DateTo = dateTo
			}
			root, _, err := ms.SmsActivity.List(ctx, opts)
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

		headers := []string{"ID", "FROM", "TO", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, a := range items {
			createdAt := ""
			if !a.CreatedAt.IsZero() {
				createdAt = a.CreatedAt.Format("2006-01-02 15:04:05")
			}
			id := a.SmsMessageId
			rows = append(rows, []string{id, a.From, a.To, a.Status, createdAt})
		}

		output.Table(headers, rows)
		return nil
	},
}
