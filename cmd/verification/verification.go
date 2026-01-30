package verification

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "verification",
	Short: "Email verification commands",
	Long:  "Verify individual email addresses and manage email verification lists.",
}

// --- Types ---

type verificationListDetail struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Total                int             `json:"total"`
	VerificationStarted  string          `json:"verification_started"`
	VerificationEnded    string          `json:"verification_ended"`
	CreatedAt            string          `json:"created_at"`
	UpdatedAt            string          `json:"updated_at"`
	Status               json.RawMessage `json:"status"`
	Source               string          `json:"source"`
	Statistics           json.RawMessage `json:"statistics"`
}

type statusObj struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type statistics struct {
	Valid           int `json:"valid"`
	CatchAll        int `json:"catch_all"`
	MailboxFull     int `json:"mailbox_full"`
	RoleBased       int `json:"role_based"`
	Unknown         int `json:"unknown"`
	SyntaxError     int `json:"syntax_error"`
	Typo            int `json:"typo"`
	MailboxNotFound int `json:"mailbox_not_found"`
	Disposable      int `json:"disposable"`
	MailboxBlocked  int `json:"mailbox_blocked"`
	Failed          int `json:"failed"`
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
	_ = listCreateCmd.MarkFlagRequired("name")

	// list verify flags
	listVerifyCmd.Flags().Bool("wait", false, "poll until verification completes")

	// list results flags
	listResultsCmd.Flags().Int("limit", 0, "maximum number of results to return (0 = all)")
	listResultsCmd.Flags().String("status", "", "filter by status (valid, invalid, catch_all, mailbox_full, role, unknown)")
}

// --- Single-email commands ---

// verify
var verifyCmd = &cobra.Command{
	Use:   "verify <email>",
	Short: "Verify a single email address",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		reqBody := map[string]string{
			"email": args[0],
		}

		body, _, err := client.Post("/v1/email-verification/verify", reqBody)
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
				Email  json.RawMessage `json:"email"`
				Status string          `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"Email", args[0]},
			{"Status", resp.Data.Status},
		}

		// Try to extract additional email info
		if resp.Data.Email != nil {
			var emailInfo map[string]interface{}
			if err := json.Unmarshal(resp.Data.Email, &emailInfo); err == nil {
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

// verify-async
var verifyAsyncCmd = &cobra.Command{
	Use:   "verify-async <email>",
	Short: "Verify a single email address asynchronously",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		reqBody := map[string]string{
			"email": args[0],
		}

		body, _, err := client.Post("/v1/email-verification/verify-async", reqBody)
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
				ID      string `json:"id"`
				Address string `json:"address"`
				Status  string `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", resp.Data.ID},
			{"Address", resp.Data.Address},
			{"Status", resp.Data.Status},
		}

		output.Table(headers, rows)
		return nil
	},
}

// status
var statusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Get async email verification status",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/email-verification/verify-async/"+args[0], nil)
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
				ID      string          `json:"id"`
				Address string          `json:"address"`
				Status  string          `json:"status"`
				Result  json.RawMessage `json:"result"`
				Error   json.RawMessage `json:"error"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", resp.Data.ID},
			{"Address", resp.Data.Address},
			{"Status", resp.Data.Status},
		}

		if resp.Data.Result != nil && string(resp.Data.Result) != "null" {
			rows = append(rows, []string{"Result", string(resp.Data.Result)})
		}
		if resp.Data.Error != nil && string(resp.Data.Error) != "null" {
			rows = append(rows, []string{"Error", string(resp.Data.Error)})
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		limit, _ := c.Flags().GetInt("limit")

		items, err := client.GetPaginated("/v1/email-verification", nil, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"ID", "NAME", "TOTAL", "STATUS", "CREATED AT"}
		var rows [][]string
		for _, raw := range items {
			var item map[string]interface{}
			if err := json.Unmarshal(raw, &item); err != nil {
				continue
			}

			id := stringVal(item, "id")
			name := stringVal(item, "name")
			total := fmt.Sprintf("%v", item["total"])
			createdAt := stringVal(item, "created_at")

			statusName := ""
			if s, ok := item["status"].(map[string]interface{}); ok {
				statusName = stringVal(s, "name")
			}

			rows = append(rows, []string{
				id,
				name,
				total,
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		body, err := client.Get("/v1/email-verification/"+args[0], nil)
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
			Data verificationListDetail `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		d := wrapper.Data

		// Parse status
		var st statusObj
		if d.Status != nil {
			_ = json.Unmarshal(d.Status, &st)
		}

		// Parse statistics
		var stats statistics
		if d.Statistics != nil {
			_ = json.Unmarshal(d.Statistics, &stats)
		}

		headers := []string{"FIELD", "VALUE"}
		rows := [][]string{
			{"ID", d.ID},
			{"Name", d.Name},
			{"Total", strconv.Itoa(d.Total)},
			{"Status", st.Name},
			{"Source", d.Source},
			{"Verification Started", d.VerificationStarted},
			{"Verification Ended", d.VerificationEnded},
			{"Created At", d.CreatedAt},
			{"Updated At", d.UpdatedAt},
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		name, _ := c.Flags().GetString("name")
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

		reqBody := map[string]interface{}{
			"name":   name,
			"emails": emails,
		}

		body, _, err := client.Post("/v1/email-verification", reqBody)
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
			Data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &wrapper); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}

		output.Success(fmt.Sprintf("Verification list created: %s (ID: %s)", wrapper.Data.Name, wrapper.Data.ID))
		return nil
	},
}

// list verify
var listVerifyCmd = &cobra.Command{
	Use:   "verify <id>",
	Short: "Start verification of a list",
	Args:  cobra.ExactArgs(1),
	RunE: func(c *cobra.Command, args []string) error {
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		id := args[0]
		body, err := client.Get("/v1/email-verification/"+id+"/verify", nil)
		if err != nil {
			return err
		}

		wait, _ := c.Flags().GetBool("wait")

		if !wait {
			if cmdutil.JSONFlag(c) {
				var raw json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					return err
				}
				return output.JSON(raw)
			}

			output.Success(fmt.Sprintf("Verification started for list %s.", id))
			return nil
		}

		// Poll until done
		for {
			time.Sleep(5 * time.Second)

			pollBody, err := client.Get("/v1/email-verification/"+id, nil)
			if err != nil {
				return err
			}

			var resp struct {
				Data struct {
					Status json.RawMessage `json:"status"`
				} `json:"data"`
			}
			if err := json.Unmarshal(pollBody, &resp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}

			var st statusObj
			if resp.Data.Status != nil {
				_ = json.Unmarshal(resp.Data.Status, &st)
			}

			fmt.Printf("Waiting... (status: %s)\n", st.Name)

			if st.Name == "verified" || st.Name == "failed" {
				if cmdutil.JSONFlag(c) {
					var raw json.RawMessage
					if err := json.Unmarshal(pollBody, &raw); err != nil {
						return err
					}
					return output.JSON(raw)
				}

				if st.Name == "verified" {
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
		client, err := cmdutil.NewClient(c)
		if err != nil {
			return err
		}

		id := args[0]
		limit, _ := c.Flags().GetInt("limit")
		status, _ := c.Flags().GetString("status")

		params := map[string]string{}
		if status != "" {
			params["status"] = status
		}

		items, err := client.GetPaginated("/v1/email-verification/"+id+"/results", params, limit)
		if err != nil {
			return err
		}

		if cmdutil.JSONFlag(c) {
			return output.JSON(items)
		}

		headers := []string{"EMAIL", "RESULT", "REASON"}
		var rows [][]string
		for _, raw := range items {
			var item map[string]interface{}
			if err := json.Unmarshal(raw, &item); err != nil {
				continue
			}

			email := stringVal(item, "email_address")
			if email == "" {
				email = stringVal(item, "email")
			}
			result := stringVal(item, "result")
			reason := stringVal(item, "reason")

			rows = append(rows, []string{email, result, reason})
		}

		output.Table(headers, rows)
		return nil
	},
}

// --- Helpers ---

func stringVal(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Sprintf("%v", v)
	}
	return s
}
