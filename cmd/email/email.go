package email

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "email",
	Short: "Send and manage emails",
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send an email",
	Long:  "Send an email via the MailerSend API.",
	RunE:  runSend,
}

func init() {
	Cmd.AddCommand(sendCmd)
	f := sendCmd.Flags()
	f.String("from", "", "sender email address")
	f.String("from-name", "", "sender name")
	f.String("to", "", "recipient email address (required)")
	f.String("to-name", "", "recipient name")
	f.String("cc", "", "CC email address")
	f.String("bcc", "", "BCC email address")
	f.String("reply-to", "", "reply-to email address")
	f.String("subject", "", "email subject")
	f.String("text", "", "plain text body")
	f.String("html", "", "HTML body")
	f.String("html-file", "", "path to file containing HTML body")
	f.String("text-file", "", "path to file containing plain text body")
	f.String("template-id", "", "template ID to use")
	f.StringSlice("tags", nil, "email tags")
	f.Int64("send-at", 0, "unix timestamp for scheduled sending")
	f.Bool("track-clicks", false, "enable click tracking")
	f.Bool("track-opens", false, "enable open tracking")
	f.Bool("track-content", false, "enable content tracking")
}

func runSend(cobraCmd *cobra.Command, args []string) error {
	client, err := cmdutil.NewClient(cobraCmd)
	if err != nil {
		return err
	}

	flags := cobraCmd.Flags()

	from, _ := flags.GetString("from")
	fromName, _ := flags.GetString("from-name")
	to, _ := flags.GetString("to")
	toName, _ := flags.GetString("to-name")
	cc, _ := flags.GetString("cc")
	bcc, _ := flags.GetString("bcc")
	replyTo, _ := flags.GetString("reply-to")
	subject, _ := flags.GetString("subject")
	text, _ := flags.GetString("text")
	html, _ := flags.GetString("html")
	htmlFile, _ := flags.GetString("html-file")
	textFile, _ := flags.GetString("text-file")
	templateID, _ := flags.GetString("template-id")
	tags, _ := flags.GetStringSlice("tags")
	sendAt, _ := flags.GetInt64("send-at")
	trackClicks, _ := flags.GetBool("track-clicks")
	trackOpens, _ := flags.GetBool("track-opens")
	trackContent, _ := flags.GetBool("track-content")

	// Interactive prompts for required fields
	to, err = prompt.RequireArg(to, "to", "Recipient email address")
	if err != nil {
		return err
	}

	if from == "" && prompt.IsInteractive() {
		from, err = prompt.Input("Sender email address", "")
		if err != nil {
			return err
		}
	}

	if subject == "" && prompt.IsInteractive() {
		subject, err = prompt.Input("Subject", "")
		if err != nil {
			return err
		}
	}

	// Read HTML from file if --html-file is set
	if htmlFile != "" {
		data, err := os.ReadFile(htmlFile)
		if err != nil {
			return fmt.Errorf("failed to read HTML file: %w", err)
		}
		html = string(data)
	}

	// Read text from file if --text-file is set
	if textFile != "" {
		data, err := os.ReadFile(textFile)
		if err != nil {
			return fmt.Errorf("failed to read text file: %w", err)
		}
		text = string(data)
	}

	// Read stdin as HTML body if no body or template provided and stdin is piped
	if html == "" && text == "" && templateID == "" && !prompt.IsInteractive() {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read stdin: %w", err)
		}
		if len(data) > 0 {
			html = string(data)
		}
	}

	// Build request body
	body := map[string]interface{}{}

	// From
	if from != "" {
		fromObj := map[string]string{"email": from}
		if fromName != "" {
			fromObj["name"] = fromName
		}
		body["from"] = fromObj
	}

	// To
	toObj := map[string]string{"email": to}
	if toName != "" {
		toObj["name"] = toName
	}
	body["to"] = []map[string]string{toObj}

	// CC
	if cc != "" {
		body["cc"] = []map[string]string{{"email": cc}}
	}

	// BCC
	if bcc != "" {
		body["bcc"] = []map[string]string{{"email": bcc}}
	}

	// Reply-To
	if replyTo != "" {
		body["reply_to"] = map[string]string{"email": replyTo}
	}

	// Subject
	if subject != "" {
		body["subject"] = subject
	}

	// Text
	if text != "" {
		body["text"] = text
	}

	// HTML
	if html != "" {
		body["html"] = html
	}

	// Template ID
	if templateID != "" {
		body["template_id"] = templateID
	}

	// Tags
	if len(tags) > 0 {
		body["tags"] = tags
	}

	// Send at
	if sendAt != 0 {
		body["send_at"] = sendAt
	}

	// Settings
	if trackClicks || trackOpens || trackContent {
		settings := map[string]bool{}
		if trackClicks {
			settings["track_clicks"] = true
		}
		if trackOpens {
			settings["track_opens"] = true
		}
		if trackContent {
			settings["track_content"] = true
		}
		body["settings"] = settings
	}

	// Send the request
	respBody, headers, err := client.Post("/v1/email", body)
	if err != nil {
		return err
	}

	// JSON output
	if cmdutil.JSONFlag(cobraCmd) {
		if len(respBody) > 0 {
			var parsed interface{}
			if jsonErr := json.Unmarshal(respBody, &parsed); jsonErr == nil {
				return output.JSON(parsed)
			}
		}
		return output.JSON(map[string]string{"status": "sent"})
	}

	// Default output: show message ID from headers
	messageID := headers.Get("x-message-id")
	if messageID != "" {
		output.Success(fmt.Sprintf("Email queued successfully. Message ID: %s", messageID))
	} else {
		output.Success("Email queued successfully.")
	}

	return nil
}
