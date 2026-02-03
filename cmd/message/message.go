package message

import (
	"context"
	"fmt"

	"github.com/mailersend/mailersend-cli/internal/cmdutil"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "message",
	Short: "Manage messages and scheduled messages",
}

// --- message list ---

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List messages",
	RunE:  runList,
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(getCmd)
	Cmd.AddCommand(scheduledCmd)

	scheduledCmd.AddCommand(scheduledListCmd)
	scheduledCmd.AddCommand(scheduledGetCmd)
	scheduledCmd.AddCommand(scheduledDeleteCmd)

	f := listCmd.Flags()
	f.Int("limit", 25, "maximum number of results to return")
	f.String("status", "", "filter by status (queued|sent|delivered|failed)")
	f.String("domain", "", "filter by domain name or ID")
	f.String("date-from", "", "filter from date (YYYY-MM-DD or unix timestamp)")
	f.String("date-to", "", "filter to date (YYYY-MM-DD or unix timestamp)")

	sf := scheduledListCmd.Flags()
	sf.Int("limit", 25, "maximum number of results to return")
	sf.String("status", "", "filter by status (scheduled|sending|sent|error)")
	sf.String("domain", "", "filter by domain name or ID")
}

func runList(cobraCmd *cobra.Command, args []string) error {
	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	flags := cobraCmd.Flags()
	limit, _ := flags.GetInt("limit")

	// NOTE: The SDK's ListMessageOptions only supports Page and Limit.
	// The --status, --domain, --date-from, --date-to flags are kept for
	// CLI compatibility but are not passed through the SDK. This is a known
	// limitation to be addressed in a future SDK update.

	ctx := context.Background()

	items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.MessageData, bool, error) {
		root, _, err := ms.Message.List(ctx, &mailersend.ListMessageOptions{
			Page:  page,
			Limit: perPage,
		})
		if err != nil {
			return nil, false, sdkclient.WrapError(transport, err)
		}
		return root.Data, root.Links.Next != "", nil
	}, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(items)
	}

	headers := []string{"ID", "CREATED AT", "UPDATED AT"}
	var rows [][]string
	for _, item := range items {
		rows = append(rows, []string{
			item.ID,
			item.CreatedAt.Format("2006-01-02 15:04:05"),
			item.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	output.Table(headers, rows)
	return nil
}

// --- message get ---

var getCmd = &cobra.Command{
	Use:   "get <message_id>",
	Short: "Get message details",
	Args:  cobra.ExactArgs(1),
	RunE:  runGet,
}

func runGet(cobraCmd *cobra.Command, args []string) error {
	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	messageID := args[0]
	result, _, err := ms.Message.Get(ctx, messageID)
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(result)
	}

	d := result.Data
	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"ID", d.ID},
		{"Created At", d.CreatedAt.Format("2006-01-02 15:04:05")},
		{"Updated At", d.UpdatedAt.Format("2006-01-02 15:04:05")},
		{"Domain", d.Domain.Name},
	}

	if len(d.Emails) > 0 {
		e := d.Emails[0]
		rows = append(rows,
			[]string{"Subject", e.Subject},
			[]string{"From", e.From},
			[]string{"Status", e.Status},
		)
	}

	if len(d.Emails) > 1 {
		rows = append(rows, []string{"Email Count", fmt.Sprintf("%d", len(d.Emails))})
	}

	output.Table(headers, rows)
	return nil
}

// --- message scheduled (subcommand group) ---

var scheduledCmd = &cobra.Command{
	Use:   "scheduled",
	Short: "Manage scheduled messages",
}

// --- message scheduled list ---

var scheduledListCmd = &cobra.Command{
	Use:   "list",
	Short: "List scheduled messages",
	RunE:  runScheduledList,
}

func runScheduledList(cobraCmd *cobra.Command, args []string) error {
	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	flags := cobraCmd.Flags()
	limit, _ := flags.GetInt("limit")
	status, _ := flags.GetString("status")
	domainID, _ := flags.GetString("domain")
	if domainID != "" {
		domainID, err = cmdutil.ResolveDomainSDK(ms, transport, domainID)
		if err != nil {
			return err
		}
	}

	ctx := context.Background()

	items, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.ScheduleMessageData, bool, error) {
		root, _, err := ms.ScheduleMessage.List(ctx, &mailersend.ListScheduleMessageOptions{
			DomainID: domainID,
			Status:   status,
			Page:     page,
			Limit:    perPage,
		})
		if err != nil {
			return nil, false, sdkclient.WrapError(transport, err)
		}
		return root.Data, root.Links.Next != "", nil
	}, limit)
	if err != nil {
		return err
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(items)
	}

	headers := []string{"MESSAGE ID", "SUBJECT", "SEND AT", "STATUS", "CREATED AT"}
	var rows [][]string
	for _, item := range items {
		rows = append(rows, []string{
			item.MessageID,
			output.Truncate(item.Subject, 40),
			item.SendAt.Format("2006-01-02 15:04:05"),
			item.Status,
			item.CreatedAt,
		})
	}

	output.Table(headers, rows)
	return nil
}

// --- message scheduled get ---

var scheduledGetCmd = &cobra.Command{
	Use:   "get <message_id>",
	Short: "Get scheduled message details",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduledGet,
}

func runScheduledGet(cobraCmd *cobra.Command, args []string) error {
	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	messageID := args[0]
	result, _, err := ms.ScheduleMessage.Get(ctx, messageID)
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(result)
	}

	d := result.Data

	statusMsg := ""
	if d.StatusMessage != nil {
		statusMsg = fmt.Sprintf("%v", d.StatusMessage)
	}

	headers := []string{"FIELD", "VALUE"}
	rows := [][]string{
		{"Message ID", d.MessageID},
		{"Subject", d.Subject},
		{"Send At", d.SendAt.Format("2006-01-02 15:04:05")},
		{"Status", d.Status},
		{"Status Message", statusMsg},
		{"Created At", d.CreatedAt.Format("2006-01-02 15:04:05")},
		{"Domain", d.Domain.Name},
		{"Domain ID", d.Domain.ID},
		{"Related Message ID", d.Message.ID},
	}

	output.Table(headers, rows)
	return nil
}

// --- message scheduled delete ---

var scheduledDeleteCmd = &cobra.Command{
	Use:   "delete <message_id>",
	Short: "Delete a scheduled message",
	Args:  cobra.ExactArgs(1),
	RunE:  runScheduledDelete,
}

func runScheduledDelete(cobraCmd *cobra.Command, args []string) error {
	ms, transport, err := cmdutil.NewSDKClient(cobraCmd)
	if err != nil {
		return err
	}

	ctx := context.Background()
	messageID := args[0]
	_, err = ms.ScheduleMessage.Delete(ctx, messageID)
	if err != nil {
		return sdkclient.WrapError(transport, err)
	}

	if cmdutil.JSONFlag(cobraCmd) {
		return output.JSON(map[string]string{"status": "deleted", "message_id": messageID})
	}

	output.Success(fmt.Sprintf("Scheduled message %s deleted successfully.", messageID))
	return nil
}

// --- wire up subcommands (merged into init above) ---
