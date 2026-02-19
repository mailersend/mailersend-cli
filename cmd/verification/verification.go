package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "verification",
	Short: "Email verification commands",
	Long:  "Verify individual email addresses and manage email verification lists.",
}

// --- Subcommand group for list operations ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Manage verification lists",
	Long:  "List, create, verify, and inspect email verification lists.",
}

// --- init ---

func init() {
	// Single-email commands
	Cmd.AddCommand(verifyCmd)
	Cmd.AddCommand(verifyAsyncCmd)
	Cmd.AddCommand(statusCmd)

	// List subcommand group
	Cmd.AddCommand(listCmd)
	listCmd.AddCommand(listListCmd)
	listCmd.AddCommand(listGetCmd)
	listCmd.AddCommand(listCreateCmd)
	listCmd.AddCommand(listVerifyCmd)
	listCmd.AddCommand(listResultsCmd)

	// list list flags
	listListCmd.Flags().Int("limit", 0, "maximum number of lists to return (0 = all)")

	// list create flags
	listCreateCmd.Flags().String("name", "", "name for the verification list (required)")
	listCreateCmd.Flags().StringSlice("emails", nil, "comma-separated list of email addresses")
	listCreateCmd.Flags().String("emails-file", "", "path to file with one email per line")

	// list verify flags
	listVerifyCmd.Flags().Bool("wait", false, "poll until verification completes")

	// list results flags
	listResultsCmd.Flags().Int("limit", 0, "maximum number of results to return (0 = all)")
	listResultsCmd.Flags().String("status", "", "filter by status (valid, invalid, catch_all, mailbox_full, role, unknown)")
}

// --- Single-email commands ---

// verify -- uses raw HTTP because the SDK's VerifySingle only returns {status}
// but the API returns a richer response with email details.
var verifyCmd = &cobra.Command{
	Use:   "verify <email>",
	Short: "Verify a single email address",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		payload, _ := json.Marshal(map[string]string{"email": args[0]})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.mailersend.com/v1/email-verification/verify", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+ms.APIKey())
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

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
			return parseHTTPError(resp.StatusCode, body)
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		var respData struct {
			Data struct {
				Email  json.RawMessage `json:"email"`
				Status string          `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &respData); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"Email", args[0]},
			{"Status", respData.Data.Status},
		}

		// Try to extract additional email info
		if respData.Data.Email != nil {
			var emailInfo map[string]interface{}
			if err := json.Unmarshal(respData.Data.Email, &emailInfo); err == nil {
				for _, key := range []string{"local_part", "domain", "mx_found", "mx_record"} {
					if v, ok := emailInfo[key]; ok && v != nil {
						rows = append(rows, []string{key, fmt.Sprintf("%v", v)})
					}
				}
			}
		}

		output.Table(headers, rows)
		return nil
	},
}

// verify-async -- uses raw HTTP since the SDK doesn't have this endpoint.
var verifyAsyncCmd = &cobra.Command{
	Use:   "verify-async <email>",
	Short: "Verify a single email address asynchronously",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		payload, _ := json.Marshal(map[string]string{"email": args[0]})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.mailersend.com/v1/email-verification/verify-async", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+ms.APIKey())
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")

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
			return parseHTTPError(resp.StatusCode, body)
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		var respData struct {
			Data struct {
				ID      string `json:"id"`
				Address string `json:"address"`
				Status  string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &respData); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", respData.Data.ID},
			{"Address", respData.Data.Address},
			{"Status", respData.Data.Status},
		}

		output.Table(headers, rows)
		return nil
	},
}

// status -- uses raw HTTP since the SDK doesn't have this endpoint.
var statusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Get async email verification status",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.mailersend.com/v1/email-verification/verify-async/"+args[0], nil)
		if err != nil {
			return err
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
			return parseHTTPError(resp.StatusCode, body)
		}

		if cmdutil.JSONFlag(c) {
			var raw json.RawMessage
			if err := json.Unmarshal(body, &raw); err != nil {
				return err
			}
			return output.JSON(raw)
		}

		var respData struct {
			Data struct {
				ID      string          `json:"id"`
				Address string          `json:"address"`
				Status  string          `json:"status"`
				Result  json.RawMessage `json:"result"`
				Error   json.RawMessage `json:"error"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &respData); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", respData.Data.ID},
			{"Address", respData.Data.Address},
			{"Status", respData.Data.Status},
		}

		if respData.Data.Result != nil && string(respData.Data.Result) != "null" {
			rows = append(rows, []string{"Result", string(respData.Data.Result)})
		}
		if respData.Data.Error != nil && string(respData.Data.Error) != "null" {
			rows = append(rows, []string{"Error", string(respData.Data.Error)})
		}

		output.Table(headers, rows)
		return nil
	},
}

// --- List subcommands ---

// list list
var listListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all verification lists",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.EmailVerification, bool, error) {
			root, _, err := ms.EmailVerification.List(ctx, &mailersend.ListEmailVerificationOptions{
				Page:  page,
				Limit: perPage,
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

		headers := []string{"ID", "NAME", "TOTAL", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, item := range items {
			statusName := ""
			if item.Status.Name != "" {
				statusName = item.Status.Name
			}
			createdAt := ""
			if !item.CreatedAt.IsZero() {
				createdAt = item.CreatedAt.Format("2006-01-02 15:04:05")
			}

			rows = append(rows, []string{
				item.Id,
				item.Name,
				strconv.Itoa(item.Total),
				statusName,
				createdAt,
			})
		}

		output.Table(headers, rows)
		return nil
	},
}

// list get
var listGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get verification list details",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		ctx := context.Background()
		result, _, err := ms.EmailVerification.Get(ctx, args[0])
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		d := result.Data
		statusName := ""
		if d.Status.Name != "" {
			statusName = d.Status.Name
		}

		createdAt := ""
		if !d.CreatedAt.IsZero() {
			createdAt = d.CreatedAt.Format("2006-01-02 15:04:05")
		}
		updatedAt := ""
		if !d.UpdatedAt.IsZero() {
			updatedAt = d.UpdatedAt.Format("2006-01-02 15:04:05")
		}

		verificationStarted := fmt.Sprintf("%v", d.VerificationStarted)
		verificationEnded := fmt.Sprintf("%v", d.VerificationEnded)
		if verificationStarted == "<nil>" {
			verificationStarted = ""
		}
		if verificationEnded == "<nil>" {
			verificationEnded = ""
		}

		stats := d.Statistics

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.Id},
			{"Name", d.Name},
			{"Total", strconv.Itoa(d.Total)},
			{"Status", statusName},
			{"Source", d.Source},
			{"Verification Started", verificationStarted},
			{"Verification Ended", verificationEnded},
			{"Created At", createdAt},
			{"Updated At", updatedAt},
			{"", ""},
			{"--- Statistics ---", ""},
			{"Valid", strconv.Itoa(stats.Valid)},
			{"Catch All", strconv.Itoa(stats.CatchAll)},
			{"Mailbox Full", strconv.Itoa(stats.MailboxFull)},
			{"Role Based", strconv.Itoa(stats.RoleBased)},
			{"Unknown", strconv.Itoa(stats.Unknown)},
			{"Syntax Error", strconv.Itoa(stats.SyntaxError)},
			{"Typo", strconv.Itoa(stats.Typo)},
			{"Mailbox Not Found", strconv.Itoa(stats.MailboxNotFound)},
			{"Disposable", strconv.Itoa(stats.Disposable)},
			{"Mailbox Blocked", strconv.Itoa(stats.MailboxBlocked)},
			{"Failed", strconv.Itoa(stats.Failed)},
		}

		output.Table(headers, rows)
		return nil
	},
}

// list create
var listCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a verification list",
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		name, _ := c.Flags().GetString("name")
		name, err = prompt.RequireArg(name, "name", "Verification list name")
		if err != nil {
			return err
		}
		emails, _ := c.Flags().GetStringSlice("emails")
		emailsFile, _ := c.Flags().GetString("emails-file")

		if emailsFile != "" {
			data, err := os.ReadFile(emailsFile)
			if err != nil {
				return fmt.Errorf("failed to read emails file: %w", err)
			}
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					emails = append(emails, trimmed)
				}
			}
		}

		if len(emails) == 0 {
			return fmt.Errorf("provide emails via --emails or --emails-file")
		}

		ctx := context.Background()
		result, _, err := ms.EmailVerification.Create(ctx, &mailersend.CreateEmailVerificationOptions{
			Name:   name,
			Emails: emails,
		})
		if err != nil {
			return sdkclient.WrapError(err)
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(result)
		}

		output.Success(fmt.Sprintf("Verification list created: %s (ID: %s)", result.Data.Name, result.Data.Id))
		return nil
	},
}

// list verify
var listVerifyCmd = &cobra.Command{
	Use:   "verify <id>",
	Short: "Start verification of a list",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		id := args[0]
		ctx := context.Background()

		result, _, err := ms.EmailVerification.Verify(ctx, id)
		if err != nil {
			return sdkclient.WrapError(err)
		}

		wait, _ := c.Flags().GetBool("wait")

		if !wait {
			if cmdutil.JSONFlag(c) {
				return output.JSON(result)
			}

			output.Success(fmt.Sprintf("Verification started for list %s.", id))
			return nil
		}

		// Poll until done
		for {
			time.Sleep(5 * time.Second)

			pollResult, _, err := ms.EmailVerification.Get(ctx, id)
			if err != nil {
				return sdkclient.WrapError(err)
			}

			statusName := ""
			if pollResult.Data.Status.Name != "" {
				statusName = pollResult.Data.Status.Name
			}

			fmt.Printf("Waiting... (status: %s)\n", statusName)

			if statusName == "verified" || statusName == "failed" {
				if cmdutil.JSONFlag(c) {
					return output.JSON(pollResult)
				}

				if statusName == "verified" {
					output.Success(fmt.Sprintf("Verification completed for list %s.", id))
				} else {
					output.Error(fmt.Sprintf("Verification failed for list %s.", id))
				}
				return nil
			}
		}
	},
}

// list results
var listResultsCmd = &cobra.Command{
	Use:   "results <id>",
	Short: "Get verification results for a list",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		ms, err := cmdutil.NewSDKClient(c)
		if err != nil {
			return err
		}

		id := args[0]
		limit, _ := c.Flags().GetInt("limit")

		ctx := context.Background()
		items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Result, bool, error) {
			root, _, err := ms.EmailVerification.GetResults(ctx, &mailersend.GetEmailVerificationOptions{
				EmailVerificationId: id,
				Page:                page,
				Limit:               perPage,
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

		headers := []string{"EMAIL", "RESULT", "REASON"}
		var rows [][]string
		for _, item := range items {
			email := item.Address
			resultStr := item.Result
			reason := ""
			rows = append(rows, []string{email, resultStr, reason})
		}

		output.Table(headers, rows)
		return nil
	},
}

// --- Helpers ---

// parseHTTPError creates a CLIError from a raw HTTP error response.
func parseHTTPError(statusCode int, body []byte) error {
	cliErr := &sdkclient.CLIError{
		StatusCode: statusCode,
	}
	if len(body) > 0 {
		var parsed struct {
			Message string              `json:"message"`
			Errors  map[string][]string `json:"errors"`
		}
		if json.Unmarshal(body, &parsed) == nil {
			cliErr.Message = parsed.Message
			if len(parsed.Errors) > 0 {
				cliErr.Errors = parsed.Errors
			}
		}
		if cliErr.Message == "" {
			cliErr.Message = string(body)
		}
	}
	return cliErr
}
