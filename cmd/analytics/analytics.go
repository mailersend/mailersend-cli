package analytics

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "analytics",
	Short: "View email analytics",
}

func init() {
	Cmd.AddCommand(dateCmd)
	Cmd.AddCommand(countryCmd)
	Cmd.AddCommand(uaNameCmd)
	Cmd.AddCommand(uaTypeCmd)

	// date flags
	df := dateCmd.Flags()
	df.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	df.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	df.String("domain", "", "filter by domain name or ID")
	df.String("group-by", "", "group by: days, weeks, months, years")
	df.StringSlice("tags", nil, "filter by tags")
	df.StringSlice("event", nil, "event types to retrieve (required, min 1): queued, sent, delivered, soft_bounced, hard_bounced, opened, clicked, unsubscribed, spam_complaints")

	// country flags
	cf := countryCmd.Flags()
	cf.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	cf.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	cf.String("domain", "", "filter by domain name or ID")
	cf.StringSlice("tags", nil, "filter by tags")

	// ua-name flags
	uf := uaNameCmd.Flags()
	uf.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	uf.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	uf.String("domain", "", "filter by domain name or ID")
	uf.StringSlice("tags", nil, "filter by tags")

	// ua-type flags
	tf := uaTypeCmd.Flags()
	tf.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	tf.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	tf.String("domain", "", "filter by domain name or ID")
	tf.StringSlice("tags", nil, "filter by tags")
}

// --- analytics date ---

var dateCmd = &cobra.Command{
	Use:   "date",
	Short: "Get analytics grouped by date",
	RunE:  runDate,
}

func runDate(cobraCmd *cobra.Command, args []string) error {
	flags := cobraCmd.Flags()
	dateFromStr, _ := flags.GetString("date-from")
	dateToStr, _ := flags.GetString("date-to")
	events, _ := flags.GetStringSlice("event")

	if len(events) == 0 {
		return fmt.Errorf("missing required flag: --event\n\nExample:\n  mailersend-cli analytics date --date-from 2025-01-01 --date-to 2025-01-30 --event sent,delivered")
	}

	now := time.Now()
	dateFrom, dateTo, err := cmdutil.DefaultDateRange(dateFromStr, dateToStr, now)
	if err != nil {
		return err
	}

	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	domainID, _ := flags.GetString("domain")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
		if err != nil {
			return err
		}
	}
	groupBy, _ := flags.GetString("group-by")
	tags, _ := flags.GetStringSlice("tags")

	ctx := context.Background()
	result, _, err := ms.Analytics.GetActivityByDate(ctx, &mailersend.AnalyticsOptions{
		DomainID: domainID,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		GroupBy:  groupBy,
		Tags:     tags,
		Event:    events,
	})
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(result)
	}

	headers := []string{"DATE"}
	for _, e := range events {
		headers = append(headers, strings.ToUpper(e))
	}

	var rows [][]string
	for _, stat := range result.Data.Stats {
		row := []string{stat.Date}
		for _, e := range events {
			row = append(row, fmt.Sprintf("%d", statValue(stat, e)))
		}
		rows = append(rows, row)
	}

	output.Table(headers, rows)
	return nil
}

// statValue extracts a named stat field from AnalyticsStats by event name.
func statValue(s mailersend.AnalyticsStats, event string) int {
	switch event {
	case "queued":
		return s.Queued
	case "sent":
		return s.Sent
	case "delivered":
		return s.Delivered
	case "soft_bounced":
		return s.SoftBounced
	case "hard_bounced":
		return s.HardBounced
	case "junk":
		return s.Junk
	case "opened":
		return s.Opened
	case "clicked":
		return s.Clicked
	case "unsubscribed":
		return s.Unsubscribed
	case "spam_complaints":
		return s.SpamComplaints
	default:
		return 0
	}
}

// --- analytics country ---

var countryCmd = &cobra.Command{
	Use:   "country",
	Short: "Get analytics grouped by country",
	RunE:  runCountry,
}

func runCountry(cobraCmd *cobra.Command, args []string) error {
	opts, err := buildOpensOptions(cobraCmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, _, err := opts.ms.Analytics.GetOpensByCountry(ctx, opts.options)
	if err != nil {
		return sdkclient.WrapError(opts.transport, err)
	}

	return renderOpens(cobraCmd, result, "COUNTRY", "COUNT")
}

// --- analytics ua-name ---

var uaNameCmd = &cobra.Command{
	Use:   "ua-name",
	Short: "Get analytics grouped by user agent name",
	RunE:  runUAName,
}

func runUAName(cobraCmd *cobra.Command, args []string) error {
	opts, err := buildOpensOptions(cobraCmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, _, err := opts.ms.Analytics.GetOpensByUserAgent(ctx, opts.options)
	if err != nil {
		return sdkclient.WrapError(opts.transport, err)
	}

	return renderOpens(cobraCmd, result, "USER AGENT", "COUNT")
}

// --- analytics ua-type ---

var uaTypeCmd = &cobra.Command{
	Use:   "ua-type",
	Short: "Get analytics grouped by user agent type",
	RunE:  runUAType,
}

func runUAType(cobraCmd *cobra.Command, args []string) error {
	opts, err := buildOpensOptions(cobraCmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	result, _, err := opts.ms.Analytics.GetOpensByReadingEnvironment(ctx, opts.options)
	if err != nil {
		return sdkclient.WrapError(opts.transport, err)
	}

	return renderOpens(cobraCmd, result, "TYPE", "COUNT")
}

// --- shared helpers for country / ua-name / ua-type ---

type opensContext struct {
	ms        *mailersend.Mailersend
	transport *sdkclient.CLITransport
	options   *mailersend.AnalyticsOptions
}

func buildOpensOptions(cobraCmd *cobra.Command) (*opensContext, error) {
	flags := cobraCmd.Flags()
	dateFromStr, _ := flags.GetString("date-from")
	dateToStr, _ := flags.GetString("date-to")

	now := time.Now()
	dateFrom, dateTo, err := cmdutil.DefaultDateRange(dateFromStr, dateToStr, now)
	if err != nil {
		return nil, err
	}

	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return nil, err
	}

	domainID, _ := flags.GetString("domain")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
		if err != nil {
			return nil, err
		}
	}
	tags, _ := flags.GetStringSlice("tags")

	return &opensContext{
		ms:        ms,
		transport: transport,
		options: &mailersend.AnalyticsOptions{
			DomainID: domainID,
			DateFrom: dateFrom,
			DateTo:   dateTo,
			Tags:     tags,
		},
	}, nil
}

func renderOpens(cobraCmd *cobra.Command, result *mailersend.OpensRoot, nameHeader, countHeader string) error {
	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(result)
	}

	headers := []string{nameHeader, countHeader}
	var rows [][]string
	for _, stat := range result.Data.Stats {
		rows = append(rows, []string{stat.Name, strconv.Itoa(stat.Count)})
	}

	output.Table(headers, rows)
	return nil
}
