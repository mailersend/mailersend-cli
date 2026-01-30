package template

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
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
	listCmd.Flags().String("domain-id", "", "filter by domain ID or name")
}

// --- list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List templates",
	RunE:  runList,
}

func runList(c *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	limit, _ := c.Flags().GetInt("limit")
	domainID, _ := c.Flags().GetString("domain-id")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
	}

	params := map[string]string{}
	if domainID != "" {
		params["domain_id"] = domainID
	}

	items, err := client.GetPaginated("/v1/templates", params, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(c) {
		return output.JSON(items)
	}

	type templateItem struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		CreatedAt string `json:"created_at"`
	}

	headers := []string{"ID", "NAME", "TYPE", "CREATED AT"}
	var rows [][]string

	for _, raw := range items {
		var t templateItem
		if err := json.Unmarshal(raw, &t); err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}
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
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	body, err := client.Get("/v1/templates/"+args[0], nil)
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
		Data struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Type      string `json:"type"`
			ImagePath string `json:"image_path"`
			CreatedAt string `json:"created_at"`
			Category  *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"category"`
			Domain *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"domain"`
			TemplateStats struct {
				Total         int    `json:"total"`
				Queued        int    `json:"queued"`
				Sent          int    `json:"sent"`
				Rejected      int    `json:"rejected"`
				Delivered     int    `json:"delivered"`
				LastEmailSent string `json:"last_email_sent_at"`
			} `json:"template_stats"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	d := resp.Data

	fmt.Printf("ID:           %s\n", d.ID)
	fmt.Printf("Name:         %s\n", d.Name)
	fmt.Printf("Type:         %s\n", d.Type)
	fmt.Printf("Image Path:   %s\n", d.ImagePath)
	fmt.Printf("Created At:   %s\n", d.CreatedAt)

	if d.Category != nil {
		fmt.Printf("Category:     %s (%s)\n", d.Category.Name, d.Category.ID)
	} else {
		fmt.Printf("Category:     —\n")
	}

	if d.Domain != nil {
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
	fmt.Printf("  Last Sent At:   %s\n", d.TemplateStats.LastEmailSent)

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
	client, err := cmdutil.NewClient(c)
	if err != nil {
		return err
	}

	_, err = client.Delete("/v1/templates/" + args[0])
	if err != nil {
		return err
	}

	output.Success("Template " + args[0] + " deleted successfully.")
	return nil
}
