package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "activity",
	Short: "View and manage activity",
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)

	f := listCmd.Flags()
	f.String("domain", "", "domain name or ID (required)")
	f.Int("limit", 0, "maximum number of results to return")
	f.String("date-from", "", "start date as YYYY-MM-DD or unix timestamp (required)")
	f.String("date-to", "", "end date as YYYY-MM-DD or unix timestamp (required)")
	f.StringSlice("event", nil, "event types to filter (queued, sent, delivered, soft_bounced, hard_bounced, opened, clicked, unsubscribed, spam_complaints)")
}

// --- list subcommand ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List activity for a domain",
	Long:  "List activity events for a domain via the MailerSend API.",
	RunE:  runList,
}

func runList(cobraCmd *cobra.Command, args []string) error {
	flags := cobraCmd.Flags()
	domainIDStr, _ := flags.GetString("domain")
	dateFromStr, _ := flags.GetString("date-from")
	dateToStr, _ := flags.GetString("date-to")

	var err error
	domainIDStr, err = prompt.RequireArg(domainIDStr, "domain", "Domain name or ID")
	if err != nil {
		return err
	}

	now := time.Now()
	dateFrom, dateTo, err := cmdutil.DefaultDateRange(dateFromStr, dateToStr, now)
	if err != nil {
		return err
	}

	ms, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	domainID, err := cmdutil.ResolveDomainSDK(ms, domainIDStr)
	if err != nil {
		return err
	}
	limit, _ := flags.GetInt("limit")
	events, _ := flags.GetStringSlice("event")

	ctx := context.Background()

	items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.ActivityData, bool, error) {
		root, _, err := ms.Activity.List(ctx, &mailersend.ActivityOptions{
			DomainID: domainID,
			Page:     page,
			DateFrom: dateFrom,
			DateTo:   dateTo,
			Limit:    perPage,
			Event:    events,
		})
		if err != nil {
			return nil, false, sdkclient.WrapError(err)
		}
		return root.Data, root.Links.Next != "", nil
	}, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(items)
	}

	headers := []string{"ID", "TYPE", "FROM", "SUBJECT", "CREATED AT"}
	var rows [][]string

	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.Type,
			item.Email.From,
			output.Truncate(item.Email.Subject, 40),
			item.CreatedAt,
		})
	}

	output.Table(headers, rows)
	return nil
}

// --- get subcommand ---

var getCmd = &cobra.Command{
	Use:   "get <activity_id>",
	Short: "Get activity details",
	Long:  "Get detailed information about a specific activity.",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func runGet(cobraCmd *cobra.Command, args []string) error {
	ms, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	activityID := args[0]

	// The SDK does not have a Get method for individual activities, so we
	// make a direct HTTP request using the SDK's HTTP client (which includes
	// the CLI transport for retries, verbose logging, etc.).
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.mailersend.com/v1/activities/"+activityID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+ms.APIKey())
	req.Header.Set("Accept", "application/json")

	resp, err := ms.Client().Do(req)
	if err != nil {
		return sdkclient.WrapError(err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		// Parse as error
		cliErr := &sdkclient.CLIError{StatusCode: resp.StatusCode}
		var parsed struct {
			Message string              `json:"message"`
			Errors  map[string][]string `json:"errors"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			cliErr.Message = parsed.Message
			cliErr.Errors = parsed.Errors
		}
		if cliErr.Message == "" {
			cliErr.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return cliErr
	}

	var data struct {
		Data struct {
			ID        string `json:"id"`
			CreatedAt string `json:"created_at"`
			UpdatedAt string `json:"updated_at"`
			Type      string `json:"type"`
			Email     struct {
				ID        string   `json:"id"`
				From      string   `json:"from"`
				Subject   string   `json:"subject"`
				Text      string   `json:"text"`
				HTML      string   `json:"html"`
				Status    string   `json:"status"`
				Tags      []string `json:"tags"`
				CreatedAt string   `json:"created_at"`
				UpdatedAt string   `json:"updated_at"`
				Recipient struct {
					ID        string `json:"id"`
					Email     string `json:"email"`
					CreatedAt string `json:"created_at"`
				} `json:"recipient"`
			} `json:"email"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(data.Data)
	}

	d := data.Data
	fmt.Printf("%-20s %s\n", "ID:", d.ID)
	fmt.Printf("%-20s %s\n", "Type:", d.Type)
	fmt.Printf("%-20s %s\n", "From:", d.Email.From)
	fmt.Printf("%-20s %s\n", "Subject:", d.Email.Subject)
	fmt.Printf("%-20s %s\n", "Status:", d.Email.Status)
	fmt.Printf("%-20s %s\n", "Recipient Email:", d.Email.Recipient.Email)
	fmt.Printf("%-20s %s\n", "Created At:", d.CreatedAt)

	return nil
}
