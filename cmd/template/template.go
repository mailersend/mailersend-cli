package template

import (
	"context"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "template",
	Short: "Manage templates",
	Long:  "List, view, and delete email templates.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().Int("limit", 0, "maximum number of templates to return")
	listCmd.Flags().String("domain", "", "filter by domain name or ID")
}

// --- list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List templates",
	RunE:  runList,
}

func runList(c *cobra.Command, args []string) error {
	ms, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	limit, _ := c.Flags().GetInt("limit")
	domainID, _ := c.Flags().GetString("domain")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomainSDK(ms, domainID)
		if err != nil {
			return err
		}
	}

	ctx := context.Background()

	items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Template, bool, error) {
		root, _, err := ms.Template.List(ctx, &mailersend.ListTemplateOptions{
			DomainID: domainID,
			Page:     page,
			Limit:    perPage,
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

	headers := []string{"ID", "NAME", "TYPE", "CREATED AT"}
	var rows [][]string

	for _, t := range items {
		rows = append(rows, []string{
			t.ID,
			output.Truncate(t.Name, 40),
			t.Type,
			t.CreatedAt,
		})
	}

	output.Table(headers, rows)
	return nil
}

// --- get ---

var getCmd = &cobra.Command{
	Use:   "get <template_id>",
	Short: "Get template details",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func runGet(c *cobra.Command, args []string) error {
	ms, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, _, err := ms.Template.Get(ctx, args[0])
	if err != nil {
		return sdkclient.WrapError(err)
	}

	if cmdutil.JSONFlag(c) {
		return output.JSON(result)
	}

	d := result.Data

	fmt.Printf("ID:           %s\n", d.ID)
	fmt.Printf("Name:         %s\n", d.Name)
	fmt.Printf("Type:         %s\n", d.Type)
	fmt.Printf("Image Path:   %s\n", d.ImagePath)
	fmt.Printf("Created At:   %s\n", d.CreatedAt.Format("2006-01-02 15:04:05"))

	if d.Category != nil {
		if cat, ok := d.Category.(map[string]interface{}); ok {
			fmt.Printf("Category:     %v (%v)\n", cat["name"], cat["id"])
		} else {
			fmt.Printf("Category:     %v\n", d.Category)
		}
	} else {
		fmt.Printf("Category:     —\n")
	}

	if d.Domain.ID != "" {
		fmt.Printf("Domain:       %s (%s)\n", d.Domain.Name, d.Domain.ID)
	} else {
		fmt.Printf("Domain:       —\n")
	}

	fmt.Println()
	fmt.Println("Stats:")
	fmt.Printf("  Total:          %d\n", d.TemplateStats.Total)
	fmt.Printf("  Queued:         %d\n", d.TemplateStats.Queued)
	fmt.Printf("  Sent:           %d\n", d.TemplateStats.Sent)
	fmt.Printf("  Rejected:       %d\n", d.TemplateStats.Rejected)
	fmt.Printf("  Delivered:      %d\n", d.TemplateStats.Delivered)
	fmt.Printf("  Last Sent At:   %s\n", d.TemplateStats.LastEmailSentAt.Format("2006-01-02 15:04:05"))

	return nil
}

// --- delete ---

var deleteCmd = &cobra.Command{
	Use:   "delete <template_id>",
	Short: "Delete a template",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func runDelete(c *cobra.Command, args []string) error {
	ms, err := cmdutil.NewSDKClient(c)
	if err != nil {
		return err
	}

	ctx := context.Background()
	_, err = ms.Template.Delete(ctx, args[0])
	if err != nil {
		return sdkclient.WrapError(err)
	}

	output.Success("Template " + args[0] + " deleted successfully.")
	return nil
}
