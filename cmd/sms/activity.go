package sms

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		params := map[string]string{}

		if v, _ := c.Flags().GetString("sms-number-id"); v != "" {
			params["sms_number_id"] = v
		}
		if v, _ := c.Flags().GetString("date-from"); v != "" {
			ts, err := cmdutil.ParseDate(v)
			if err != nil {
				return err
			}
			params["date_from"] = fmt.Sprintf("%d", ts)
		}
		if v, _ := c.Flags().GetString("date-to"); v != "" {
			ts, err := cmdutil.ParseDate(v)
			if err != nil {
				return err
			}
			params["date_to"] = fmt.Sprintf("%d", ts)
		}
		if statuses, _ := c.Flags().GetStringSlice("status"); len(statuses) > 0 {
			for i, s := range statuses {
				params[fmt.Sprintf("status[%d]", i)] = s
			}
		}

		items, err := client.GetPaginated("/v1/sms-activity", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID        string `json:"id"`
			Status    string `json:"status"`
			From      string `json:"from"`
			To        string `json:"to"`
			CreatedAt string `json:"created_at"`
		}

		headers := []string{"ID", "FROM", "TO", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var a item
			if err := json.Unmarshal(raw, &a); err != nil {
				return fmt.Errorf("failed to parse SMS activity: %w", err)
			}
			rows = append(rows, []string{a.ID, a.From, a.To, a.Status, a.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}
