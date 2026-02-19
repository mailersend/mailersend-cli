package quota

import (
	"context"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "quota",
	Short: "View API quota",
	Long:  "Display your current API quota usage.",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.ApiQuota.Get(ctx)
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		used := result.Quota - result.Remaining

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"Total", fmt.Sprintf("%d", result.Quota)},
			{"Used", fmt.Sprintf("%d", used)},
			{"Remaining", fmt.Sprintf("%d", result.Remaining)},
		}
		output.Table(headers, rows)
		return nil
	},
}
