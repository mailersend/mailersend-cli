package recipient

import (
	"context"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "recipient",
	Short: "Manage recipients",
	Long:  "List, view, and delete recipients.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().Int("limit", 0, "maximum number of recipients to return (0 = all)")
	listCmd.Flags().String("domain", "", "filter by domain name or ID")

}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recipients",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		limit, _ := c.Flags().GetInt("limit")

		// The MailerSend API /recipients endpoint does not support
		// domain_id filtering, so we resolve the domain to a name and
		// filter client-side by email suffix.
		domainName, _ := c.Flags().GetString("domain")
		if domainName != "" {
			domainName, err = cmdutil.ResolveDomainNameSDK(ms, domainName)
			if err != nil {
				return err
			}
		}
		suffix := ""
		if domainName != "" {
			suffix = "@" + strings.ToLower(domainName)
		}

		// When filtering by domain we must fetch all recipients and
		// filter client-side, so pass 0 as the limit to FetchAll and
		// trim afterward.
		fetchLimit := limit
		if suffix != "" {
			fetchLimit = 0
		}

		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.RecipientObject, bool, error) {
			root, _, err := ms.Recipient.List(ctx, &mailersend.ListRecipientOptions{
				Page:  page,
				Limit: perPage,
			})
			if err != nil {
				return nil, false, sdkclient.WrapError(err)
			}
			return root.Data, root.Links.Next != "", nil
		}, fetchLimit)
		if err != nil {
			return err
		}

		if suffix != "" {
			filtered := items[:0]
			for _, r := range items {
				if strings.HasSuffix(strings.ToLower(r.Email), suffix) {
					filtered = append(filtered, r)
				}
			}
			items = filtered
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "EMAIL", "CREATED AT"}
		var rows [][]string
		for _, r := range items {
			rows = append(rows, []string{r.ID, r.Email, r.CreatedAt})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <recipient_id>",
	Short: "Get recipient details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.Recipient.Get(ctx, args[0])
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
			{"Email", d.Email},
			{"Created At", d.CreatedAt},
			{"Updated At", d.UpdatedAt},
		}
		output.Table(headers, rows)
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <recipient_id>",
	Short: "Delete a recipient",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.Recipient.Delete(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		output.Success("Recipient " + args[0] + " deleted successfully.")
		return nil
	},
}
