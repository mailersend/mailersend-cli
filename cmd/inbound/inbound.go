package inbound

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

// parseForwards converts "type:value" strings into objects.
// If no colon is present, defaults to type "webhook".
func parseForwards(raw []string) []map[string]string {
	out := make([]map[string]string, 0, len(raw))
	for _, s := range raw {
		typ := "webhook"
		val := s
		if idx := strings.Index(s, ":"); idx > 0 && !strings.HasPrefix(s, "http") {
			typ = s[:idx]
			val = s[idx+1:]
		}
		out = append(out, map[string]string{"type": typ, "value": val})
	}
	return out
}

var Cmd = &cobra.Command{
	Use:   "inbound",
	Short: "Manage inbound routes",
	Long:  "List, view, create, update, and delete inbound routes.",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(updateCmd)
	Cmd.AddCommand(deleteCmd)

	listCmd.Flags().Int("limit", 0, "maximum number of routes to return (0 = all)")
	listCmd.Flags().String("domain-id", "", "filter by domain ID")

	createCmd.Flags().String("domain-id", "", "domain ID (required)")
	_ = createCmd.MarkFlagRequired("domain-id")
	createCmd.Flags().String("name", "", "route name (required)")
	_ = createCmd.MarkFlagRequired("name")
	createCmd.Flags().Bool("domain-enabled", true, "whether the domain is enabled")
	createCmd.Flags().String("inbound-domain", "", "inbound domain (required when domain-enabled is true)")
	createCmd.Flags().Int("inbound-priority", 0, "inbound priority (required when domain-enabled is true)")
	createCmd.Flags().String("catch-type", "", "catch type (catch_recipient, catch_all)")
	createCmd.Flags().String("catch-filter-type", "", "catch filter type (required when domain-enabled, e.g. catch_all, catch_recipient)")
	createCmd.Flags().String("match-filter-type", "", "match filter type (required, e.g. match_all, match_recipient)")
	_ = createCmd.MarkFlagRequired("match-filter-type")
	createCmd.Flags().StringSlice("forwards", nil, "forward URLs as type:value pairs, e.g. 'webhook:https://example.com' (required)")
	_ = createCmd.MarkFlagRequired("forwards")

	updateCmd.Flags().String("name", "", "route name")
	updateCmd.Flags().Bool("domain-enabled", true, "whether the domain is enabled")
	updateCmd.Flags().String("inbound-domain", "", "inbound domain")
	updateCmd.Flags().Int("inbound-priority", 0, "inbound priority")
	updateCmd.Flags().String("catch-type", "", "catch type")
	updateCmd.Flags().String("catch-filter-type", "", "catch filter type")
	updateCmd.Flags().String("match-filter-type", "", "match filter type")
	updateCmd.Flags().StringSlice("forwards", nil, "forward URLs as type:value pairs, e.g. 'webhook:https://example.com'")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List inbound routes",
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

		items, err := client.GetPaginated("/v1/inbound", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		type item struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}

		headers := []string{"ID", "NAME"}
		var rows [][]string
		for _, raw := range items {
			var i item
			if err := json.Unmarshal(raw, &i); err != nil {
				return fmt.Errorf("failed to parse inbound route: %w", err)
			}
			rows = append(rows, []string{i.ID, i.Name})
		}

		output.Table(headers, rows)
		return nil
	},
}

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get inbound route details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/inbound/"+args[0], nil)
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
				ID             string `json:"id"`
				Name           string `json:"name"`
				DomainEnabled  bool   `json:"domain_enabled"`
				InboundDomain  string `json:"inbound_domain"`
				CatchType      string `json:"catch_type"`
				MatchType      string `json:"match_type"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := resp.Data
		enabled := "No"
		if d.DomainEnabled {
			enabled = "Yes"
		}
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Domain Enabled", enabled},
			{"Inbound Domain", d.InboundDomain},
			{"Catch Type", d.CatchType},
			{"Match Type", d.MatchType},
		}
		output.Table(headers, rows)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an inbound route",
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, _ := c.Flags().GetString("domain-id")
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
		name, _ := c.Flags().GetString("name")

		domainEnabled, _ := c.Flags().GetBool("domain-enabled")
		matchFilterType, _ := c.Flags().GetString("match-filter-type")
		forwards, _ := c.Flags().GetStringSlice("forwards")

		payload := map[string]interface{}{
			"domain_id":      domainID,
			"name":           name,
			"domain_enabled": domainEnabled,
			"match_filter": map[string]interface{}{
				"type": matchFilterType,
			},
			"forwards": parseForwards(forwards),
		}

		if v, _ := c.Flags().GetString("inbound-domain"); v != "" {
			payload["inbound_domain"] = v
		}
		if c.Flags().Changed("inbound-priority") {
			v, _ := c.Flags().GetInt("inbound-priority")
			payload["inbound_priority"] = v
		}
		if v, _ := c.Flags().GetString("catch-type"); v != "" {
			payload["catch_type"] = v
		}
		catchFilterType, _ := c.Flags().GetString("catch-filter-type")
		if catchFilterType != "" {
			payload["catch_filter"] = map[string]interface{}{
				"type":    catchFilterType,
				"filters": []interface{}{},
			}
		}

		respBody, _, err := client.Post("/v1/inbound", payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(respBody, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		var resp struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		output.Success("Inbound route created successfully. ID: " + resp.Data.ID)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an inbound route",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		// Fetch current route — the API requires all fields on PUT.
		body, err := client.Get("/v1/inbound/"+args[0], nil)
		if err != nil {
			return fmt.Errorf("failed to fetch current route: %w", err)
		}

		var current struct {
			Data struct {
				Name          string `json:"name"`
				Domain        string `json:"domain"`
				DomainEnabled bool   `json:"domain_enabled"`
				InboundDomain string `json:"inbound_domain"`
				Priority      int    `json:"priority"`
				CatchType     string `json:"catch_type"`
				MatchType     string `json:"match_type"`
				Filters       []struct {
					Type     string  `json:"type"`
					Key      *string `json:"key"`
					Comparer *string `json:"comparer"`
					Value    *string `json:"value"`
				} `json:"filters"`
				Forwards []struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"forwards"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &current); err != nil {
			return fmt.Errorf("failed to parse current route: %w", err)
		}
		d := current.Data

		// Start with current values.
		name := d.Name
		domainEnabled := d.DomainEnabled
		inboundDomain := d.InboundDomain
		if inboundDomain == "" {
			inboundDomain = d.Domain
		}
		inboundPriority := d.Priority

		// Build match_filter and catch_filter from existing filters.
		var matchFilter, catchFilter map[string]interface{}
		for _, f := range d.Filters {
			switch f.Type {
			case "match_all", "match_sender", "match_domain", "match_recipient":
				matchFilter = map[string]interface{}{"type": f.Type}
			case "catch_all", "catch_recipient":
				catchFilter = map[string]interface{}{"type": f.Type, "filters": []interface{}{}}
			}
		}
		if matchFilter == nil {
			matchFilter = map[string]interface{}{"type": d.MatchType}
		}
		if catchFilter == nil {
			catchFilter = map[string]interface{}{"type": d.CatchType, "filters": []interface{}{}}
		}

		// Build forwards from current.
		forwards := make([]map[string]string, 0, len(d.Forwards))
		for _, fw := range d.Forwards {
			forwards = append(forwards, map[string]string{"type": fw.Type, "value": fw.Value})
		}

		// Override with user-provided flags.
		if c.Flags().Changed("name") {
			name, _ = c.Flags().GetString("name")
		}
		if c.Flags().Changed("domain-enabled") {
			domainEnabled, _ = c.Flags().GetBool("domain-enabled")
		}
		if c.Flags().Changed("inbound-domain") {
			inboundDomain, _ = c.Flags().GetString("inbound-domain")
		}
		if c.Flags().Changed("inbound-priority") {
			inboundPriority, _ = c.Flags().GetInt("inbound-priority")
		}
		if c.Flags().Changed("catch-filter-type") {
			v, _ := c.Flags().GetString("catch-filter-type")
			catchFilter = map[string]interface{}{"type": v, "filters": []interface{}{}}
		}
		if c.Flags().Changed("match-filter-type") {
			v, _ := c.Flags().GetString("match-filter-type")
			matchFilter = map[string]interface{}{"type": v}
		}
		if c.Flags().Changed("forwards") {
			v, _ := c.Flags().GetStringSlice("forwards")
			forwards = parseForwards(v)
		}

		payload := map[string]interface{}{
			"name":             name,
			"domain_enabled":   domainEnabled,
			"inbound_domain":   inboundDomain,
			"inbound_priority": inboundPriority,
			"match_filter":     matchFilter,
			"catch_filter":     catchFilter,
			"forwards":         forwards,
		}

		if c.Flags().Changed("catch-type") {
			v, _ := c.Flags().GetString("catch-type")
			payload["catch_type"] = v
		}

		respBody, err := client.Put("/v1/inbound/"+args[0], payload)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(respBody, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		output.Success("Inbound route " + args[0] + " updated successfully.")
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an inbound route",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/inbound/" + args[0])
		if err != nil {
			return err
		}

		output.Success("Inbound route " + args[0] + " deleted successfully.")
		return nil
	},
}
