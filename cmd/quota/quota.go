package quota

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "quota",
	Short: "View API quota",
	Long:  "Display your current API quota usage.",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/api-quota", nil)
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
			Remaining int `json:"remaining"`
			Total     int `json:"total"`
			Used      int `json:"used"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"Total", fmt.Sprintf("%d", resp.Total)},
			{"Used", fmt.Sprintf("%d", resp.Used)},
			{"Remaining", fmt.Sprintf("%d", resp.Remaining)},
		}
		output.Table(headers, rows)
		return nil
	},
}
