package smtp

import (
	"context"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "smtp",
	Short: "Manage SMTP users",
	Long:  "List, view, create, update, and delete SMTP users for a domain.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().String("domain", "", "domain name or ID (required)")
	listCmd.Flags().Int("limit", 0, "maximum number of SMTP users to return (0 = all)")

	getCmd.Flags().String("domain", "", "domain name or ID (required)")

	createCmd.Flags().String("domain", "", "domain name or ID (required)")
	createCmd.Flags().String("name", "", "SMTP user name (required)")
	createCmd.Flags().Bool("enabled", true, "whether the SMTP user is enabled")

	updateCmd.Flags().String("domain", "", "domain name or ID (required)")
	updateCmd.Flags().String("name", "", "SMTP user name")
	updateCmd.Flags().Bool("enabled", true, "whether the SMTP user is enabled")

	deleteCmd.Flags().String("domain", "", "domain name or ID (required)")
}

func boolYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List SMTP users",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
		limit, _ := c.Flags().GetInt("limit")

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.SmtpUser, bool, error) {
			root, _, err := ms.SmtpUser.List(ctx, domainID, &mailersend.ListSmtpUserOptions{
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

		headers := []string{"ID", "NAME", "ENABLED"}
		var rows [][]string
		for _, s := range items {
			rows = append(rows, []string{s.ID, s.Name, boolYesNo(s.Enabled)})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get SMTP user details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.SmtpUser.Get(ctx, domainID, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Enabled", boolYesNo(d.Enabled)},
		}
		output.Table(headers, rows)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an SMTP user",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
		name, _ := c.Flags().GetString("name")
		name, err = prompt.RequireArg(name, "name", "SMTP user name")
		if err != nil {
			return err
		}

		opts := &mailersend.CreateSmtpUserOptions{
			Name: name,
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			opts.Enabled = mailersend.Bool(v)
		}

		ctx := context.Background()
		result, _, err := ms.SmtpUser.Create(ctx, domainID, opts)
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMTP user created successfully. ID: " + result.Data.ID)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an SMTP user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}

		opts := &mailersend.UpdateSmtpUserOptions{}

		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			opts.Name = v
		}
		if c.Flags().Changed("enabled") {
			v, _ := c.Flags().GetBool("enabled")
			opts.Enabled = mailersend.Bool(v)
		}

		ctx := context.Background()
		result, _, err := ms.SmtpUser.Update(ctx, domainID, args[0], opts)
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("SMTP user " + args[0] + " updated successfully.")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an SMTP user",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.SmtpUser.Delete(ctx, domainID, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		output.Success("SMTP user " + args[0] + " deleted successfully.")
		return nil
	},
}
