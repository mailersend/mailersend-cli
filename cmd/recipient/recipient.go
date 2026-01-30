package recipient

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
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
	listCmd.Flags().String("domain-id", "", "filter by domain ID")

}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List recipients",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		params := map[string]string{}
		if domainID, _ := c.Flags().GetString("domain-id"); domainID != "" {
			domainID, err = cmdutil.ResolveDomain(client, domainID)
			if err != nil {
				return err
			}
			params["domain_id"] = domainID
		}

		items, err := client.GetPaginated("/v1/recipients", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			CreatedAt string `json:"created_at"`
		}

		headers := []string{"ID", "EMAIL", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var r item
			if err := json.Unmarshal(raw, &r); err != nil {
				return fmt.Errorf("failed to parse recipient: %w", err)
			}
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/recipients/"+args[0], nil)
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
				Email     string `json:"email"`
				CreatedAt string `json:"created_at"`
				UpdatedAt string `json:"updated_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := resp.Data
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/recipients/" + args[0])
		if err != nil {
			return err
		}

		output.Success("Recipient " + args[0] + " deleted successfully.")
		return nil
	},
}
