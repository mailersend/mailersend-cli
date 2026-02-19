package sms

import (
	"context"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var numberCmd = &cobra.Command{
	Use:   "number",
	Short: "Manage SMS phone numbers",
}

func init() {
	numberCmd.AddCommand(numberListCmd)
	numberCmd.AddCommand(numberGetCmd)
	numberCmd.AddCommand(numberUpdateCmd)
	numberCmd.AddCommand(numberDeleteCmd)

	numberListCmd.Flags().Int("limit", 0, "maximum number of numbers to return (0 = all)")
	numberListCmd.Flags().Bool("paused", false, "filter by paused status")

	numberUpdateCmd.Flags().Bool("paused", false, "whether the number is paused")
}

var numberListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS phone numbers",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		paused := false
		if c.Flags().Changed("paused") {
			paused, _ = c.Flags().GetBool("paused")
		}

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Number, bool, error) {
			root, _, err := ms.SmsNumber.List(ctx, &mailersend.SmsNumberOptions{
				Paused: paused,
				Page:   page,
				Limit:  perPage,
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

		headers := []string{"ID", "NUMBER", "PAUSED", "CREATED AT"}
		var rows [][]string
		for _, n := range items {
			createdAt := ""
			if !n.CreatedAt.IsZero() {
				createdAt = n.CreatedAt.Format("2006-01-02 15:04:05")
			}
			rows = append(rows, []string{n.Id, n.TelephoneNumber, boolYesNo(n.Paused), createdAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var numberGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS phone number details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmsNumber.Get(ctx, args[0])
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
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.Id},
			{"Number", d.TelephoneNumber},
			{"Paused", boolYesNo(d.Paused)},
			{"Created At", createdAt},
		}
		output.Table(headers, rows)
		return nil
	},
}

var numberUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS phone number",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		opts := &mailersend.SmsNumberSettingOptions{
			Id: args[0],
		}

		if c.Flags().Changed("paused") {
			v, _ := c.Flags().GetBool("paused")
			opts.Paused = mailersend.Bool(v)
		}

		ctx := context.Background()
		result, _, err := ms.SmsNumber.Update(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMS number " + args[0] + " updated successfully.")
		return nil
	},
}

var numberDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMS phone number",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.SmsNumber.Delete(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		output.Success("SMS number " + args[0] + " deleted successfully.")
		return nil
	},
}
