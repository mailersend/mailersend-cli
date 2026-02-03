package sms

import (
	"context"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var inboundCmd = &cobra.Command{
	Use:   "inbound",
	Short: "Manage SMS inbound routes",
}

func init() {
	inboundCmd.AddCommand(inboundListCmd)
	inboundCmd.AddCommand(inboundGetCmd)
	inboundCmd.AddCommand(inboundCreateCmd)
	inboundCmd.AddCommand(inboundUpdateCmd)
	inboundCmd.AddCommand(inboundDeleteCmd)

	inboundListCmd.Flags().Int("limit", 0, "maximum number of routes to return (0 = all)")
	inboundListCmd.Flags().String("sms-number-id", "", "filter by SMS number ID")
	inboundListCmd.Flags().Bool("enabled", false, "filter by enabled status")

	inboundCreateCmd.Flags().String("sms-number-id", "", "SMS number ID (required)")
	inboundCreateCmd.Flags().String("name", "", "route name (required)")
	inboundCreateCmd.Flags().String("forward-url", "", "forward URL (required)")
	inboundCreateCmd.Flags().String("filter-comparer", "", "filter comparer")
	inboundCreateCmd.Flags().String("filter-value", "", "filter value")
	inboundCreateCmd.Flags().Bool("enabled", true, "whether the route is enabled")

	inboundUpdateCmd.Flags().String("sms-number-id", "", "SMS number ID")
	inboundUpdateCmd.Flags().String("name", "", "route name")
	inboundUpdateCmd.Flags().String("forward-url", "", "forward URL")
	inboundUpdateCmd.Flags().String("filter-comparer", "", "filter comparer")
	inboundUpdateCmd.Flags().String("filter-value", "", "filter value")
	inboundUpdateCmd.Flags().Bool("enabled", true, "whether the route is enabled")
}

var inboundListCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMS inbound routes",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		smsNumberID, _ := c.Flags().GetString("sms-number-id")

		var enabled *bool
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			enabled = mailersend.Bool(v)
		}

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.SmsInbound, bool, error) {
			root, _, err := ms.SmsInbound.List(ctx, &mailersend.ListSmsInboundOptions{
				SmsNumberId: smsNumberID,
				Enabled:     enabled,
				Page:        page,
				Limit:       perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(transport, err)
			}
			return root.Data, root.Links.Next != "", nil
		}, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "NAME", "ENABLED"}
		var rows [][]string
		for _, r := range items {
			rows = append(rows, []string{r.Id, r.Name, boolYesNo(r.Enabled)})
		}

		output.Table(headers, rows)
		return nil
	},
}

var inboundGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMS inbound route details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmsInbound.Get(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.Id},
			{"Name", d.Name},
			{"Forward URL", d.ForwardUrl},
			{"Enabled", boolYesNo(d.Enabled)},
		}
		output.Table(headers, rows)
		return nil
	},
}

var inboundCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an SMS inbound route",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		smsNumberID, _ := c.Flags().GetString("sms-number-id")
		smsNumberID, err = prompt.RequireArg(smsNumberID, "sms-number-id", "SMS number ID")
		if err != nil {
			return err
		}
		name, _ := c.Flags().GetString("name")
		name, err = prompt.RequireArg(name, "name", "Route name")
		if err != nil {
			return err
		}
		forwardURL, _ := c.Flags().GetString("forward-url")
		forwardURL, err = prompt.RequireArg(forwardURL, "forward-url", "Forward URL")
		if err != nil {
			return err
		}

		enabled, _ := c.Flags().GetBool("enabled")
		opts := &mailersend.CreateSmsInboundOptions{
			SmsNumberId: smsNumberID,
			Name:        name,
			ForwardUrl:  forwardURL,
			Enabled:     mailersend.Bool(enabled),
		}

		comparer, _ := c.Flags().GetString("filter-comparer")
		filterValue, _ := c.Flags().GetString("filter-value")
		if comparer != "" {
			opts.Filter = mailersend.Filter{
				Comparer: comparer,
				Value:    filterValue,
			}
		}

		ctx := context.Background()
		result, _, err := ms.SmsInbound.Create(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMS inbound route created successfully. ID: " + result.Data.Id)
		return nil
	},
}

var inboundUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMS inbound route",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		opts := &mailersend.UpdateSmsInboundOptions{
			Id: args[0],
		}

		if c.Flags().Changed("sms-number-id") {
			v, _ := c.Flags().GetString("sms-number-id")
			opts.SmsNumberId = v
		}
		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			opts.Name = v
		}
		if c.Flags().Changed("forward-url") {
			v, _ := c.Flags().GetString("forward-url")
			opts.ForwardUrl = v
		}
		if c.Flags().Changed("filter-comparer") || c.Flags().Changed("filter-value") {
			comparer, _ := c.Flags().GetString("filter-comparer")
			filterValue, _ := c.Flags().GetString("filter-value")
			opts.Filter = mailersend.Filter{
				Comparer: comparer,
				Value:    filterValue,
			}
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			opts.Enabled = mailersend.Bool(v)
		}

		ctx := context.Background()
		result, _, err := ms.SmsInbound.Update(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMS inbound route " + args[0] + " updated successfully.")
		return nil
	},
}

var inboundDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMS inbound route",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.SmsInbound.Delete(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		output.Success("SMS inbound route " + args[0] + " deleted successfully.")
		return nil
	},
}
