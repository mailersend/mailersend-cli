package inbound

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

// parseForwards converts "type:value" strings into ForwardsFilter structs.
// If no colon is present, defaults to type "webhook".
func parseForwards(raw []string) []mailersend.ForwardsFilter {
	out := make([]mailersend.ForwardsFilter, 0, len(raw))
	for _, s := range raw {
		typ := "webhook"
		val := s
		if idx := strings.Index(s, ":"); idx > 0 && !strings.HasPrefix(s, "http") {
			typ = s[:idx]
			val = s[idx+1:]
		}
		out = append(out, mailersend.ForwardsFilter{Type: typ, Value: val})
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
	listCmd.Flags().String("domain", "", "domain name or ID (required)")

	createCmd.Flags().String("domain", "", "domain name or ID (required)")
	createCmd.Flags().String("name", "", "route name (required)")
	createCmd.Flags().Bool("domain-enabled", true, "whether the domain is enabled")
	createCmd.Flags().String("inbound-domain", "", "inbound domain (required when domain-enabled is true)")
	createCmd.Flags().Int("inbound-priority", 0, "inbound priority (required when domain-enabled is true)")
	createCmd.Flags().String("catch-type", "", "catch type (catch_recipient, catch_all)")
	createCmd.Flags().String("catch-filter-type", "", "catch filter type (required when domain-enabled, e.g. catch_all, catch_recipient)")
	createCmd.Flags().String("match-filter-type", "", "match filter type (required, e.g. match_all, match_recipient)")
	createCmd.Flags().StringSlice("forwards", nil, "forward URLs as type:value pairs, e.g. 'webhook:https://example.com' (required)")

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
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		domainID, _ := c.Flags().GetString("domain")
		domainID, err = prompt.RequireArg(domainID, "domain", "Domain name or ID")
		if err != nil {
			return err
		}
		domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
		if err != nil {
			return err
		}

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Inbound, bool, error) {
			root, _, err := ms.Inbound.List(ctx, &mailersend.ListInboundOptions{
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

		headers := []string{"ID", "NAME"}
		var rows [][]string
		for _, item := range items {
			rows = append(rows, []string{item.ID, item.Name})
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
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.Inbound.Get(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		enabled := "No"
		if d.Enabled {
			enabled = "Yes"
		}
		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Domain Enabled", enabled},
			{"Inbound Domain", d.Domain},
		}
		output.Table(headers, rows)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an inbound route",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

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
		name, err = prompt.RequireArg(name, "name", "Route name")
		if err != nil {
			return err
		}

		domainEnabled, _ := c.Flags().GetBool("domain-enabled")
		matchFilterType, _ := c.Flags().GetString("match-filter-type")
		matchFilterType, err = prompt.RequireArg(matchFilterType, "match-filter-type", "Match filter type (e.g. match_all, match_recipient)")
		if err != nil {
			return err
		}
		forwards, _ := c.Flags().GetStringSlice("forwards")
		forwards, err = prompt.RequireSliceArg(forwards, "forwards", "Forward URLs (type:value pairs)")
		if err != nil {
			return err
		}

		opts := &mailersend.CreateInboundOptions{
			DomainID:      domainID,
			Name:          name,
			DomainEnabled: domainEnabled,
			MatchFilter:   &mailersend.MatchFilter{Type: matchFilterType},
			Forwards:      parseForwards(forwards),
		}

		if v, _ := c.Flags().GetString("inbound-domain"); v != "" {
			opts.InboundDomain = v
		}
		if c.Flags().Changed("inbound-priority") {
			v, _ := c.Flags().GetInt("inbound-priority")
			if v > 0 {
				opts.InboundPriority = v
			}
		}
		// The SDK omits InboundPriority when 0 (json omitempty), but the API
		// requires it when domain_enabled is true. Default to 100 if not set.
		if domainEnabled && opts.InboundPriority == 0 {
			opts.InboundPriority = 100
		}
		catchFilterType, _ := c.Flags().GetString("catch-filter-type")
		if catchFilterType != "" {
			opts.CatchFilter = &mailersend.CatchFilter{
				Type:    catchFilterType,
				Filters: []mailersend.Filter{},
			}
		}

		ctx := context.Background()
		result, _, err := ms.Inbound.Create(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success("Inbound route created successfully. ID: " + result.Data.ID)
		return nil
	},
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an inbound route",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()

		// Fetch current route -- the API requires all fields on PUT.
		current, _, err := ms.Inbound.Get(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to fetch current route: %w", sdkclient.WrapError(transport, err))
		}
		d := current.Data

		// Start with current values.
		name := d.Name
		domainEnabled := d.Enabled
		inboundDomain := d.Domain
		inboundPriority := d.Priority

		// Build match_filter and catch_filter from existing filters.
		var matchFilter *mailersend.MatchFilter
		var catchFilter *mailersend.CatchFilter
		for _, f := range d.Filters {
			switch f.Type {
			case "match_all", "match_sender", "match_domain", "match_recipient":
				matchFilter = &mailersend.MatchFilter{Type: f.Type}
			case "catch_all", "catch_recipient":
				catchFilter = &mailersend.CatchFilter{Type: f.Type, Filters: []mailersend.Filter{}}
			}
		}
		if matchFilter == nil {
			matchFilter = &mailersend.MatchFilter{Type: "match_all"}
		}
		if catchFilter == nil {
			catchFilter = &mailersend.CatchFilter{Type: "catch_all", Filters: []mailersend.Filter{}}
		}

		// Build forwards from current.
		fwds := make([]mailersend.ForwardsFilter, 0, len(d.Forwards))
		for _, fw := range d.Forwards {
			fwds = append(fwds, mailersend.ForwardsFilter{Type: fw.Type, Value: fw.Value})
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
			catchFilter = &mailersend.CatchFilter{Type: v, Filters: []mailersend.Filter{}}
		}
		if c.Flags().Changed("match-filter-type") {
			v, _ := c.Flags().GetString("match-filter-type")
			matchFilter = &mailersend.MatchFilter{Type: v}
		}
		if c.Flags().Changed("forwards") {
			v, _ := c.Flags().GetStringSlice("forwards")
			fwds = parseForwards(v)
		}

		opts := &mailersend.UpdateInboundOptions{
			Name:            name,
			DomainEnabled:   domainEnabled,
			InboundDomain:   inboundDomain,
			InboundPriority: inboundPriority,
			MatchFilter:     matchFilter,
			CatchFilter:     catchFilter,
			Forwards:        fwds,
		}

		result, _, err := ms.Inbound.Update(ctx, args[0], opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
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
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.Inbound.Delete(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		output.Success("Inbound route " + args[0] + " deleted successfully.")
		return nil
	},
}
