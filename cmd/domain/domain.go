package domain

import (
	"context"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage domains",
	Long:  "List, create, update, verify, and delete domains in your MailerSend account.",
}

// --- Helpers ---

func boolYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func boolCheck(b bool) string {
	if b {
		return "\u2713"
	}
	return "\u2717"
}

// --- Subcommands ---

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(addCmd)
	Cmd.AddCommand(deleteCmd)
	Cmd.AddCommand(updateSettingsCmd)
	Cmd.AddCommand(dnsCmd)
	Cmd.AddCommand(verifyCmd)

	// list flags
	listCmd.Flags().Int("limit", 0, "maximum number of domains to return (0 = all)")
	listCmd.Flags().Bool("verified", false, "filter by verified status")

	// add flags
	addCmd.Flags().String("name", "", "domain name (required)")
	addCmd.Flags().String("return-path-subdomain", "", "custom return path subdomain")
	addCmd.Flags().String("custom-tracking-subdomain", "", "custom tracking subdomain")

	// update-settings flags
	updateSettingsCmd.Flags().Bool("send-paused", false, "pause sending")
	updateSettingsCmd.Flags().Bool("track-clicks", false, "track clicks")
	updateSettingsCmd.Flags().Bool("track-opens", false, "track opens")
	updateSettingsCmd.Flags().Bool("track-unsubscribe", false, "track unsubscribes")
	updateSettingsCmd.Flags().Bool("track-content", false, "track content")
	updateSettingsCmd.Flags().Bool("custom-tracking-enabled", false, "enable custom tracking")
	updateSettingsCmd.Flags().String("custom-tracking-subdomain", "", "custom tracking subdomain")
	updateSettingsCmd.Flags().Bool("precedence-bulk", false, "set precedence bulk header")
	updateSettingsCmd.Flags().Bool("ignore-duplicated-recipients", false, "ignore duplicated recipients")
}

// list
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List domains",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		verified, _ := c.Flags().GetBool("verified")

		ctx := context.Background()

		var verifiedFilter *bool
		if c.Flags().Changed("verified") {
			verifiedFilter = mailersend.Bool(verified)
		}

		domains, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Domain, bool, error) {
			root, _, err := ms.Domain.List(ctx, &mailersend.ListDomainOptions{
				Page:     page,
				Limit:    perPage,
				Verified: verifiedFilter,
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
			return output.JSON(domains)
		}

		headers := []string{"ID", "NAME", "VERIFIED", "DNS ACTIVE", "CREATED"}
		var rows [][]string
		for _, d := range domains {
			rows = append(rows, []string{
				d.ID,
				d.Name,
				boolYesNo(d.IsVerified),
				boolYesNo(d.IsDNSActive),
				d.CreatedAt,
			})
		}

		output.Table(headers, rows)
		return nil
	},
}

// get
var getCmd = &cobra.Command{
	Use:   "get <domain_id_or_name>",
	Short: "Get domain details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomainSDK(ms, transport, args[0])
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.Domain.Get(ctx, domainID)
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
			{"Verified", boolYesNo(d.IsVerified)},
			{"SPF", boolYesNo(d.Spf)},
			{"DKIM", boolYesNo(d.Dkim)},
			{"Tracking", boolYesNo(d.Tracking)},
			{"DNS Active", boolYesNo(d.IsDNSActive)},
			{"Created", d.CreatedAt},
			{"Updated", d.UpdatedAt},
		}

		output.Table(headers, rows)
		return nil
	},
}

// add
var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new domain",
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		name, _ := c.Flags().GetString("name")
		name, err = prompt.RequireArg(name, "name", "Domain name")
		if err != nil {
			return err
		}
		returnPath, _ := c.Flags().GetString("return-path-subdomain")
		customTracking, _ := c.Flags().GetString("custom-tracking-subdomain")

		opts := &mailersend.CreateDomainOptions{
			Name: name,
		}
		if returnPath != "" {
			opts.ReturnPathSubdomain = returnPath
		}
		if customTracking != "" {
			opts.CustomTrackingSubdomain = customTracking
		}

		ctx := context.Background()
		result, _, err := ms.Domain.Create(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		output.Success(fmt.Sprintf("Domain created successfully: %s (ID: %s)", d.Name, d.ID))
		return nil
	},
}

// delete
var deleteCmd = &cobra.Command{
	Use:   "delete <domain_id_or_name>",
	Short: "Delete a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomainSDK(ms, transport, args[0])
		if err != nil {
			return err
		}

		ctx := context.Background()
		_, err = ms.Domain.Delete(ctx, domainID)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		output.Success(fmt.Sprintf("Domain %s deleted successfully.", args[0]))
		return nil
	},
}

// update-settings
var updateSettingsCmd = &cobra.Command{
	Use:   "update-settings <domain_id_or_name>",
	Short: "Update domain settings",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomainSDK(ms, transport, args[0])
		if err != nil {
			return err
		}

		opts := &mailersend.DomainSettingOptions{
			DomainID: domainID,
		}

		changed := false

		if c.Flags().Changed("send-paused") {
			val, _ := c.Flags().GetBool("send-paused")
			opts.SendPaused = mailersend.Bool(val)
			changed = true
		}
		if c.Flags().Changed("track-clicks") {
			val, _ := c.Flags().GetBool("track-clicks")
			opts.TrackClicks = mailersend.Bool(val)
			changed = true
		}
		if c.Flags().Changed("track-opens") {
			val, _ := c.Flags().GetBool("track-opens")
			opts.TrackOpens = mailersend.Bool(val)
			changed = true
		}
		if c.Flags().Changed("track-unsubscribe") {
			val, _ := c.Flags().GetBool("track-unsubscribe")
			opts.TrackUnsubscribe = mailersend.Bool(val)
			changed = true
		}
		if c.Flags().Changed("track-content") {
			val, _ := c.Flags().GetBool("track-content")
			opts.TrackContent = mailersend.Bool(val)
			changed = true
		}
		if c.Flags().Changed("custom-tracking-enabled") {
			val, _ := c.Flags().GetBool("custom-tracking-enabled")
			opts.CustomTrackingEnabled = mailersend.Bool(val)
			changed = true
		}
		if c.Flags().Changed("custom-tracking-subdomain") {
			val, _ := c.Flags().GetString("custom-tracking-subdomain")
			opts.CustomTrackingSubdomain = val
			changed = true
		}
		if c.Flags().Changed("precedence-bulk") {
			val, _ := c.Flags().GetBool("precedence-bulk")
			opts.PrecedenceBulk = mailersend.Bool(val)
			changed = true
		}
		if c.Flags().Changed("ignore-duplicated-recipients") {
			val, _ := c.Flags().GetBool("ignore-duplicated-recipients")
			opts.IgnoreDuplicatedRecipients = mailersend.Bool(val)
			changed = true
		}

		if !changed {
			return fmt.Errorf("no settings flags provided; use --help to see available options")
		}

		ctx := context.Background()
		result, _, err := ms.Domain.Update(ctx, opts)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		output.Success(fmt.Sprintf("Domain settings updated for %s (ID: %s).", d.Name, d.ID))
		return nil
	},
}

// dns
var dnsCmd = &cobra.Command{
	Use:   "dns <domain_id_or_name>",
	Short: "Show DNS records for a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomainSDK(ms, transport, args[0])
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.Domain.GetDNS(ctx, domainID)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		dns := result.Data
		headers := []string{"RECORD", "HOSTNAME", "TYPE", "VALUE"}
		rows := [][]string{
			{"SPF", dns.Spf.Hostname, dns.Spf.Type, dns.Spf.Value},
			{"DKIM", dns.Dkim.Hostname, dns.Dkim.Type, dns.Dkim.Value},
			{"Return Path", dns.ReturnPath.Hostname, dns.ReturnPath.Type, dns.ReturnPath.Value},
			{"Custom Tracking", dns.CustomTracking.Hostname, dns.CustomTracking.Type, dns.CustomTracking.Value},
		}

		output.Table(headers, rows)
		return nil
	},
}

// verify
var verifyCmd = &cobra.Command{
	Use:   "verify <domain_id_or_name>",
	Short: "Verify a domain",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, transport, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomainSDK(ms, transport, args[0])
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.Domain.Verify(ctx, domainID)
		if err != nil {
			return sdkclient.WrapError(transport, err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		headers := []string{"RECORD", "STATUS"}
		rows := [][]string{
			{"DKIM", boolCheck(result.Data.Dkim)},
			{"SPF", boolCheck(result.Data.Spf)},
			{"MX", boolCheck(result.Data.Mx)},
			{"Tracking", boolCheck(result.Data.Tracking)},
			{"CNAME", boolCheck(result.Data.Cname)},
			{"Return Path CNAME", boolCheck(result.Data.RpCname)},
		}

		output.Table(headers, rows)
		return nil
	},
}
