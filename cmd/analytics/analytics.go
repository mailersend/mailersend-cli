package analytics

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
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
	df.String("domain-id", "", "filter by domain ID or name")
	df.String("group-by", "", "group by: days, weeks, months, years")
	df.StringSlice("tags", nil, "filter by tags")
	df.StringSlice("event", nil, "event types to retrieve (required, min 1): queued, sent, delivered, soft_bounced, hard_bounced, opened, clicked, unsubscribed, spam_complaints")

	// country flags
	cf := countryCmd.Flags()
	cf.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	cf.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	cf.String("domain-id", "", "filter by domain ID or name")
	cf.StringSlice("tags", nil, "filter by tags")

	// ua-name flags
	uf := uaNameCmd.Flags()
	uf.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	uf.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	uf.String("domain-id", "", "filter by domain ID or name")
	uf.StringSlice("tags", nil, "filter by tags")

	// ua-type flags
	tf := uaTypeCmd.Flags()
	tf.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	tf.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	tf.String("domain-id", "", "filter by domain ID or name")
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

	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	domainID, _ := flags.GetString("domain-id")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
	}
	groupBy, _ := flags.GetString("group-by")
	tags, _ := flags.GetStringSlice("tags")

	params := map[string]string{
		"date_from": strconv.FormatInt(dateFrom, 10),
		"date_to":   strconv.FormatInt(dateTo, 10),
	}
	if domainID != "" {
		params["domain_id"] = domainID
	}
	if groupBy != "" {
		params["group_by"] = groupBy
	}
	for i, t := range tags {
		params[fmt.Sprintf("tags[%d]", i)] = t
	}
	for i, e := range events {
		params[fmt.Sprintf("event[%d]", i)] = e
	}

	body, err := client.Get("/v1/analytics/date", params)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		var parsed interface{}
		if jsonErr := json.Unmarshal(body, &parsed); jsonErr == nil {
			return output.JSON(parsed)
		}
		return output.JSON(json.RawMessage(body))
	}

	var resp struct {
		Data struct {
			DateFrom interface{}              `json:"date_from"`
			DateTo   interface{}              `json:"date_to"`
			GroupBy  string                   `json:"group_by"`
			Stats    []map[string]interface{} `json:"stats"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	headers := []string{"DATE"}
	for _, e := range events {
		headers = append(headers, strings.ToUpper(e))
	}

	var rows [][]string
	for _, stat := range resp.Data.Stats {
		row := []string{fmt.Sprintf("%v", stat["date"])}
		for _, e := range events {
			val, ok := stat[e]
			if ok {
				row = append(row, fmt.Sprintf("%v", val))
			} else {
				row = append(row, "0")
			}
		}
		rows = append(rows, row)
	}

	output.Table(headers, rows)
	return nil
}

// --- analytics country ---

var countryCmd = &cobra.Command{
	Use:   "country",
	Short: "Get analytics grouped by country",
	RunE:  runCountry,
}

func runCountry(cobraCmd *cobra.Command, args []string) error {
	return runCountLike(cobraCmd, "/v1/analytics/country", "COUNTRY", "COUNT")
}

// --- analytics ua-name ---

var uaNameCmd = &cobra.Command{
	Use:   "ua-name",
	Short: "Get analytics grouped by user agent name",
	RunE:  runUAName,
}

func runUAName(cobraCmd *cobra.Command, args []string) error {
	return runCountLike(cobraCmd, "/v1/analytics/ua-name", "USER AGENT", "COUNT")
}

// --- analytics ua-type ---

var uaTypeCmd = &cobra.Command{
	Use:   "ua-type",
	Short: "Get analytics grouped by user agent type",
	RunE:  runUAType,
}

func runUAType(cobraCmd *cobra.Command, args []string) error {
	return runCountLike(cobraCmd, "/v1/analytics/ua-type", "TYPE", "COUNT")
}

// --- shared helper for country / ua-name / ua-type ---

func runCountLike(cobraCmd *cobra.Command, path string, nameHeader string, countHeader string) error {
	flags := cobraCmd.Flags()
	dateFromStr, _ := flags.GetString("date-from")
	dateToStr, _ := flags.GetString("date-to")

	now := time.Now()
	dateFrom, dateTo, err := cmdutil.DefaultDateRange(dateFromStr, dateToStr, now)
	if err != nil {
		return err
	}

	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	domainID, _ := flags.GetString("domain-id")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomain(client, domainID)
		if err != nil {
			return err
		}
	}
	tags, _ := flags.GetStringSlice("tags")

	params := map[string]string{
		"date_from": strconv.FormatInt(dateFrom, 10),
		"date_to":   strconv.FormatInt(dateTo, 10),
	}
	if domainID != "" {
		params["domain_id"] = domainID
	}
	for i, t := range tags {
		params[fmt.Sprintf("tags[%d]", i)] = t
	}

	body, err := client.Get(path, params)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		var parsed interface{}
		if jsonErr := json.Unmarshal(body, &parsed); jsonErr == nil {
			return output.JSON(parsed)
		}
		return output.JSON(json.RawMessage(body))
	}

	var resp struct {
		Data struct {
			Stats []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"stats"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	headers := []string{nameHeader, countHeader}
	var rows [][]string
	for _, stat := range resp.Data.Stats {
		rows = append(rows, []string{stat.Name, strconv.Itoa(stat.Count)})
	}

	output.Table(headers, rows)
	return nil
}
