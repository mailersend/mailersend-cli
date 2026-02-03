package identity

import (
	"context"
	"fmt"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "identity",
	Short: "Manage sender identities",
	Long:  "List, view, create, update, and delete sender identities.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().Int("limit", 0, "maximum number of identities to return (0 = all)")
	listCmd.Flags().String("domain", "", "filter by domain name or ID")

	createCmd.Flags().String("domain", "", "domain name or ID (required)")
	createCmd.Flags().String("name", "", "sender name (required)")
	createCmd.Flags().String("email", "", "sender email (required)")
	createCmd.Flags().String("reply-to-email", "", "reply-to email")
	createCmd.Flags().String("reply-to-name", "", "reply-to name")
	createCmd.Flags().Bool("add-note", false, "add personal note")
	createCmd.Flags().String("personal-note", "", "personal note text")

	updateCmd.Flags().String("name", "", "sender name")
	updateCmd.Flags().String("reply-to-email", "", "reply-to email")
	updateCmd.Flags().String("reply-to-name", "", "reply-to name")
	updateCmd.Flags().Bool("add-note", false, "add personal note")
	updateCmd.Flags().String("personal-note", "", "personal note text")
}

func ifaceStr(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List sender identities",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		domainID, _ := c.Flags().GetString("domain")
		if domainID != "" {
			domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
			if err != nil {
				return err
			}
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Identity, bool, error) {
			root, _, err := ms.Identity.List(ctx, &mailersend.ListIdentityOptions{
				DomainID: domainID,
				Page:     page,
				Limit:    perPage,
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

		headers := []string{"ID", "NAME", "EMAIL"}
		var rows [][]string
		for _, i := range items {
			rows = append(rows, []string{i.ID, i.Name, i.Email})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <id_or_email>",
	Short: "Get sender identity details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		var result *mailersend.SingleIdentityRoot
		if strings.Contains(args[0], "@") {
			result, _, err = ms.Identity.GetByEmail(ctx, args[0])
		} else {
			result, _, err = ms.Identity.Get(ctx, args[0])
		}
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Email", d.Email},
			{"Reply-To Email", ifaceStr(d.ReplyToEmail)},
			{"Reply-To Name", ifaceStr(d.ReplyToName)},
			{"Personal Note", ifaceStr(d.PersonalNote)},
		}
		output.Table(headers, rows)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a sender identity",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
		if err != nil {
			return err
		}
		name, _ := c.Flags().GetString("name")
		name, err = prompt.RequireArg(name, "name", "Sender name")
		if err != nil {
			return err
		}
		email, _ := c.Flags().GetString("email")
		email, err = prompt.RequireArg(email, "email", "Sender email")
		if err != nil {
			return err
		}

		opts := &mailersend.CreateIdentityOptions{
			DomainID: domainID,
			Name:     name,
			Email:    email,
		}

		if v, _ := c.Flags().GetString("reply-to-email"); v != "" {
			opts.ReplyToEmail = v
		}
		if v, _ := c.Flags().GetString("reply-to-name"); v != "" {
			opts.ReplyToName = v
		}
		if c.Flags().Changed("add-note") {
			v, _ := c.Flags().GetBool("add-note")
			opts.AddNote = v
		}
		if v, _ := c.Flags().GetString("personal-note"); v != "" {
			opts.PersonalNote = v
		}

		result, _, err := ms.Identity.Create(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Identity created successfully. ID: " + result.Data.ID)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id_or_email>",
	Short: "Update a sender identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		opts := &mailersend.UpdateIdentityOptions{}

		if c.Flags().Changed("name") {
			v, _ := c.Flags().GetString("name")
			opts.Name = v
		}
		if c.Flags().Changed("reply-to-email") {
			v, _ := c.Flags().GetString("reply-to-email")
			opts.ReplyToEmail = v
		}
		if c.Flags().Changed("reply-to-name") {
			v, _ := c.Flags().GetString("reply-to-name")
			opts.ReplyToName = v
		}
		if c.Flags().Changed("add-note") {
			v, _ := c.Flags().GetBool("add-note")
			opts.AddNote = v
		}
		if c.Flags().Changed("personal-note") {
			v, _ := c.Flags().GetString("personal-note")
			opts.PersonalNote = v
		}

		var result *mailersend.SingleIdentityRoot
		if strings.Contains(args[0], "@") {
			result, _, err = ms.Identity.UpdateByEmail(ctx, args[0], opts)
		} else {
			result, _, err = ms.Identity.Update(ctx, args[0], opts)
		}
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Identity " + args[0] + " updated successfully.")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id_or_email>",
	Short: "Delete a sender identity",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		if strings.Contains(args[0], "@") {
			_, err = ms.Identity.DeleteByEmail(ctx, args[0])
		} else {
			_, err = ms.Identity.Delete(ctx, args[0])
		}
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		output.Success("Identity " + args[0] + " deleted successfully.")
		return nil
	},
}
