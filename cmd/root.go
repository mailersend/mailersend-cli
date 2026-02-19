package cmd

import (
	"github.com/mailersend/mailersend-cli/cmd/activity"
	"github.com/mailersend/mailersend-cli/cmd/analytics"
	"github.com/mailersend/mailersend-cli/cmd/auth"
	"github.com/mailersend/mailersend-cli/cmd/bulkemail"
	"github.com/mailersend/mailersend-cli/cmd/completion"
	"github.com/mailersend/mailersend-cli/cmd/dashboard"
	"github.com/mailersend/mailersend-cli/cmd/domain"
	"github.com/mailersend/mailersend-cli/cmd/email"
	"github.com/mailersend/mailersend-cli/cmd/identity"
	"github.com/mailersend/mailersend-cli/cmd/inbound"
	"github.com/mailersend/mailersend-cli/cmd/message"
	"github.com/mailersend/mailersend-cli/cmd/profile"
	"github.com/mailersend/mailersend-cli/cmd/quota"
	"github.com/mailersend/mailersend-cli/cmd/recipient"
	"github.com/mailersend/mailersend-cli/cmd/sms"
	"github.com/mailersend/mailersend-cli/cmd/smtp"
	"github.com/mailersend/mailersend-cli/cmd/suppression"
	"github.com/mailersend/mailersend-cli/cmd/template"
	"github.com/mailersend/mailersend-cli/cmd/token"
	"github.com/mailersend/mailersend-cli/cmd/user"
	"github.com/mailersend/mailersend-cli/cmd/verification"
	"github.com/mailersend/mailersend-cli/cmd/webhook"
	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "mailersend",
	Short:         "MailerSend CLI — manage your email infrastructure from the terminal",
	Long:          "A command-line interface for the MailerSend API. Send emails, manage domains, templates, webhooks, and more.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.Version = version
	cmdutil.SetVersion(version)
	rootCmd.PersistentFlags().String("profile", "", "config profile to use")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "show HTTP request/response details")
	rootCmd.PersistentFlags().Bool("json", false, "output as JSON")

	rootCmd.AddCommand(dashboard.Cmd)
	rootCmd.AddCommand(email.Cmd)
	rootCmd.AddCommand(domain.Cmd)
	rootCmd.AddCommand(message.Cmd)
	rootCmd.AddCommand(template.Cmd)
	rootCmd.AddCommand(analytics.Cmd)
	rootCmd.AddCommand(activity.Cmd)
	rootCmd.AddCommand(webhook.Cmd)
	rootCmd.AddCommand(verification.Cmd)
	rootCmd.AddCommand(auth.Cmd)
	rootCmd.AddCommand(profile.Cmd)
	rootCmd.AddCommand(completion.Cmd)
	rootCmd.AddCommand(recipient.Cmd)
	rootCmd.AddCommand(identity.Cmd)
	rootCmd.AddCommand(suppression.Cmd)
	rootCmd.AddCommand(inbound.Cmd)
	rootCmd.AddCommand(token.Cmd)
	rootCmd.AddCommand(user.Cmd)
	rootCmd.AddCommand(smtp.Cmd)
	rootCmd.AddCommand(quota.Cmd)
	rootCmd.AddCommand(bulkemail.Cmd)
	rootCmd.AddCommand(sms.Cmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func IsJSON() bool {
	return cmdutil.JSONFlag(rootCmd)
}
