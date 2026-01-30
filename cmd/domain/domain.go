package domain

import (
	"encoding/json"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "domain",
	Short: "Manage domains",
	Long:  "List, create, update, verify, and delete domains in your MailerSend account.",
}

// --- Types ---

type domain struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IsVerified  bool   `json:"is_verified"`
	IsDNSActive bool   `json:"is_dns_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	DKIM        bool   `json:"dkim"`
	SPF         bool   `json:"spf"`
	Tracking    bool   `json:"tracking"`
}

type dataWrapper struct {
	Data json.RawMessage `json:"data"`
}

type dnsRecord struct {
	Hostname string `json:"hostname"`
	Type     string `json:"type"`
	Value    string `json:"value"`
}

type dnsData struct {
	ID             string    `json:"id"`
	SPF            dnsRecord `json:"spf"`
	DKIM           dnsRecord `json:"dkim"`
	ReturnPath     dnsRecord `json:"return_path"`
	CustomTracking dnsRecord `json:"custom_tracking"`
}

type verifyData struct {
	DKIM     bool `json:"dkim"`
	SPF      bool `json:"spf"`
	MX       bool `json:"mx"`
	Tracking bool `json:"tracking"`
	CNAME    bool `json:"cname"`
	RPCname  bool `json:"rp_cname"`
}

type verifyResponse struct {
	Message string     `json:"message"`
	Data    verifyData `json:"data"`
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
	_ = addCmd.MarkFlagRequired("name")

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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")
		verified, _ := c.Flags().GetBool("verified")

		params := map[string]string{}
		if c.Flags().Changed("verified") {
			if verified {
				params["verified"] = "true"
			} else {
				params["verified"] = "false"
			}
		}

		items, err := client.GetPaginated("/v1/domains", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "NAME", "VERIFIED", "DNS ACTIVE", "CREATED"}
		var rows [][]string
		for _, raw := range items {
			var d domain
			if err := json.Unmarshal(raw, &d); err != nil {
				return fmt.Errorf("failed to parse domain: %w", err)
			}
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomain(client, args[0])
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/domains/"+domainID, nil)
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

		var wrapper dataWrapper
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		var d domain
		if err := json.Unmarshal(wrapper.Data, &d); err != nil {
			return fmt.Errorf("failed to parse domain: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Verified", boolYesNo(d.IsVerified)},
			{"SPF", boolYesNo(d.SPF)},
			{"DKIM", boolYesNo(d.DKIM)},
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		name, _ := c.Flags().GetString("name")
		returnPath, _ := c.Flags().GetString("return-path-subdomain")
		customTracking, _ := c.Flags().GetString("custom-tracking-subdomain")

		reqBody := map[string]string{
			"name": name,
		}
		if returnPath != "" {
			reqBody["return_path_subdomain"] = returnPath
		}
		if customTracking != "" {
			reqBody["custom_tracking_subdomain"] = customTracking
		}

		body, _, err := client.Post("/v1/domains", reqBody)
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

		var wrapper dataWrapper
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		var d domain
		if err := json.Unmarshal(wrapper.Data, &d); err != nil {
			return fmt.Errorf("failed to parse domain: %w", err)
		}

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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomain(client, args[0])
		if err != nil {
			return err
		}

		_, err = client.Delete("/v1/domains/" + domainID)
		if err != nil {
			return err
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomain(client, args[0])
		if err != nil {
			return err
		}
		reqBody := map[string]interface{}{}

		flagMap := map[string]string{
			"send-paused":                  "send_paused",
			"track-clicks":                 "track_clicks",
			"track-opens":                  "track_opens",
			"track-unsubscribe":            "track_unsubscribe",
			"track-content":                "track_content",
			"custom-tracking-enabled":      "custom_tracking_enabled",
			"precedence-bulk":              "precedence_bulk",
			"ignore-duplicated-recipients": "ignore_duplicated_recipients",
		}

		for flag, key := range flagMap {
			if c.Flags().Changed(flag) {
				val, _ := c.Flags().GetBool(flag)
				reqBody[key] = val
			}
		}

		if c.Flags().Changed("custom-tracking-subdomain") {
			val, _ := c.Flags().GetString("custom-tracking-subdomain")
			reqBody["custom_tracking_subdomain"] = val
		}

		if len(reqBody) == 0 {
			return fmt.Errorf("no settings flags provided; use --help to see available options")
		}

		body, err := client.Put("/v1/domains/"+domainID+"/settings", reqBody)
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

		var wrapper dataWrapper
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		var d domain
		if err := json.Unmarshal(wrapper.Data, &d); err != nil {
			return fmt.Errorf("failed to parse domain: %w", err)
		}

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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomain(client, args[0])
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/domains/"+domainID+"/dns-records", nil)
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

		var wrapper struct {
			Data dnsData `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		dns := wrapper.Data
		headers := []string{"RECORD", "HOSTNAME", "TYPE", "VALUE"}
		rows := [][]string{
			{"SPF", dns.SPF.Hostname, dns.SPF.Type, dns.SPF.Value},
			{"DKIM", dns.DKIM.Hostname, dns.DKIM.Type, dns.DKIM.Value},
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		domainID, err := cmdutil.ResolveDomain(client, args[0])
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/domains/"+domainID+"/verify", nil)
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

		var resp verifyResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"RECORD", "STATUS"}
		rows := [][]string{
			{"DKIM", boolCheck(resp.Data.DKIM)},
			{"SPF", boolCheck(resp.Data.SPF)},
			{"MX", boolCheck(resp.Data.MX)},
			{"Tracking", boolCheck(resp.Data.Tracking)},
			{"CNAME", boolCheck(resp.Data.CNAME)},
			{"Return Path CNAME", boolCheck(resp.Data.RPCname)},
		}

		output.Table(headers, rows)
		return nil
	},
}
