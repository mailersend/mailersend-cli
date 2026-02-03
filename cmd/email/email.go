package email

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
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
	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
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

	// Interactive prompt for body/template when none provided
	if html == "" && text == "" && htmlFile == "" && textFile == "" && templateID == "" && prompt.IsInteractive() {
		contentType, err := prompt.Select("Email content type", []string{"text", "html", "template-id"})
		if err != nil {
			return err
		}
		switch contentType {
		case "text":
			text, err = prompt.Input("Plain text body", "")
			if err != nil {
				return err
			}
		case "html":
			html, err = prompt.Input("HTML body", "")
			if err != nil {
				return err
			}
		case "template-id":
			templateID, err = prompt.Input("Template ID", "")
			if err != nil {
				return err
			}
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

	// Build message using SDK
	message := ms.Email.NewMessage()

	// From
	if from != "" {
		message.SetFrom(mailersend.From{
			Email: from,
			Name:  fromName,
		})
	}

	// To
	recipient := mailersend.Recipient{
		Email: to,
		Name:  toName,
	}
	message.SetRecipients([]mailersend.Recipient{recipient})

	// CC
	if cc != "" {
		message.SetCc([]mailersend.Recipient{{Email: cc}})
	}

	// BCC
	if bcc != "" {
		message.SetBcc([]mailersend.Recipient{{Email: bcc}})
	}

	// Reply-To
	if replyTo != "" {
		message.SetReplyTo(mailersend.ReplyTo{Email: replyTo})
	}

	// Subject
	if subject != "" {
		message.SetSubject(subject)
	}

	// HTML
	if html != "" {
		message.SetHTML(html)
	}

	// Text
	if text != "" {
		message.SetText(text)
	}

	// Template ID
	if templateID != "" {
		message.SetTemplateID(templateID)
	}

	// Tags
	if len(tags) > 0 {
		message.SetTags(tags)
	}

	// Send at
	if sendAt != 0 {
		message.SetSendAt(sendAt)
	}

	// Settings
	if trackClicks || trackOpens || trackContent {
		message.SetSettings(mailersend.Settings{
			TrackClicks:  trackClicks,
			TrackOpens:   trackOpens,
			TrackContent: trackContent,
		})
	}

	// Send the email
	ctx := context.Background()
	resp, err := ms.Email.Send(ctx, message)
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	// JSON output
	if cmdutil.JSONFlag(cobraCmd) {
		result := map[string]string{"status": "sent"}
		if resp != nil && resp.Header.Get("x-message-id") != "" {
			result["message_id"] = resp.Header.Get("x-message-id")
		}
		return output.JSON(result)
	}

	// Default output: show message ID from headers
	if resp != nil {
		messageID := resp.Header.Get("x-message-id")
		if messageID != "" {
			output.Success(fmt.Sprintf("Email queued successfully. Message ID: %s", messageID))
			return nil
		}
	}
	output.Success("Email queued successfully.")

	return nil
}
