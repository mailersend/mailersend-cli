package email

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// newRootCmd builds a minimal root command tree that mirrors the real one,
// so that persistent flags (--json, --profile, --verbose) are available.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "mailersend", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("profile", "", "config profile to use")
	root.PersistentFlags().BoolP("verbose", "v", false, "show HTTP request/response details")
	root.PersistentFlags().Bool("json", false, "output as JSON")
	root.AddCommand(Cmd)

	// Reset sendCmd flags to avoid state leaking between tests.
	sendCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Value.Type() == "stringSlice" {
			// StringSlice.Set appends, so we need to use the SliceValue interface.
			if sv, ok := f.Value.(pflag.SliceValue); ok {
				_ = sv.Replace(nil)
			}
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	})

	return root
}

// ---------- Flag registration ----------

func TestSendCmd_FlagsRegistered(t *testing.T) {
	expected := []string{
		"from", "from-name", "to", "to-name",
		"cc", "bcc", "reply-to",
		"subject", "text", "html",
		"html-file", "text-file",
		"template-id", "tags",
		"send-at",
		"track-clicks", "track-opens", "track-content",
	}

	for _, name := range expected {
		if sendCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q to be registered on send command", name)
		}
	}
}

func TestEmailCmd_HasSendSubcommand(t *testing.T) {
	found := false
	for _, sub := range Cmd.Commands() {
		if sub.Name() == "send" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected email command to have 'send' subcommand")
	}
}

// ---------- Mock-server integration ----------

func TestSendCmd_PostBody(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedAuth string
	var receivedMethod string
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)

		w.Header().Set("x-message-id", "msg-abc-123")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token-xyz")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{
		"email", "send",
		"--from", "sender@example.com",
		"--from-name", "Sender",
		"--to", "recipient@example.com",
		"--to-name", "Recipient",
		"--subject", "Hello",
		"--text", "plain body",
		"--html", "<b>bold</b>",
		"--cc", "cc@example.com",
		"--bcc", "bcc@example.com",
		"--reply-to", "reply@example.com",
		"--template-id", "tmpl-1",
		"--tags", "tag1,tag2",
		"--send-at", "1700000000",
		"--track-clicks",
		"--track-opens",
		"--track-content",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	// Verify HTTP method and path
	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedPath != "/v1/email" {
		t.Errorf("expected /v1/email, got %s", receivedPath)
	}

	// Verify auth header
	if receivedAuth != "Bearer test-token-xyz" {
		t.Errorf("expected Bearer test-token-xyz, got %s", receivedAuth)
	}

	// Verify from
	fromObj, ok := receivedBody["from"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'from' to be an object")
	}
	if fromObj["email"] != "sender@example.com" {
		t.Errorf("expected from email sender@example.com, got %v", fromObj["email"])
	}
	if fromObj["name"] != "Sender" {
		t.Errorf("expected from name Sender, got %v", fromObj["name"])
	}

	// Verify to
	toArr, ok := receivedBody["to"].([]interface{})
	if !ok || len(toArr) == 0 {
		t.Fatal("expected 'to' to be a non-empty array")
	}
	toObj, ok := toArr[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'to[0]' to be an object")
	}
	if toObj["email"] != "recipient@example.com" {
		t.Errorf("expected to email recipient@example.com, got %v", toObj["email"])
	}
	if toObj["name"] != "Recipient" {
		t.Errorf("expected to name Recipient, got %v", toObj["name"])
	}

	// Verify cc
	ccArr, ok := receivedBody["cc"].([]interface{})
	if !ok || len(ccArr) == 0 {
		t.Fatal("expected 'cc' to be a non-empty array")
	}

	// Verify bcc
	bccArr, ok := receivedBody["bcc"].([]interface{})
	if !ok || len(bccArr) == 0 {
		t.Fatal("expected 'bcc' to be a non-empty array")
	}

	// Verify reply_to
	replyObj, ok := receivedBody["reply_to"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'reply_to' to be an object")
	}
	if replyObj["email"] != "reply@example.com" {
		t.Errorf("expected reply_to email reply@example.com, got %v", replyObj["email"])
	}

	// Verify subject, text, html
	if receivedBody["subject"] != "Hello" {
		t.Errorf("expected subject Hello, got %v", receivedBody["subject"])
	}
	if receivedBody["text"] != "plain body" {
		t.Errorf("expected text 'plain body', got %v", receivedBody["text"])
	}
	if receivedBody["html"] != "<b>bold</b>" {
		t.Errorf("expected html '<b>bold</b>', got %v", receivedBody["html"])
	}

	// Verify template_id
	if receivedBody["template_id"] != "tmpl-1" {
		t.Errorf("expected template_id tmpl-1, got %v", receivedBody["template_id"])
	}

	// Verify tags
	tags, ok := receivedBody["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Errorf("expected tags array with 2 elements, got %v", receivedBody["tags"])
	}

	// Verify send_at (JSON numbers decode as float64)
	if sendAt, ok := receivedBody["send_at"].(float64); !ok || sendAt != 1700000000 {
		t.Errorf("expected send_at 1700000000, got %v", receivedBody["send_at"])
	}

	// Verify settings
	settings, ok := receivedBody["settings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'settings' to be an object")
	}
	if settings["track_clicks"] != true {
		t.Errorf("expected track_clicks true, got %v", settings["track_clicks"])
	}
	if settings["track_opens"] != true {
		t.Errorf("expected track_opens true, got %v", settings["track_opens"])
	}
	if settings["track_content"] != true {
		t.Errorf("expected track_content true, got %v", settings["track_content"])
	}
}

func TestSendCmd_HTMLFile(t *testing.T) {
	// Create a temporary HTML file
	dir := t.TempDir()
	htmlPath := filepath.Join(dir, "email.html")
	htmlContent := "<h1>From File</h1>"
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		t.Fatal(err)
	}

	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)
		w.Header().Set("x-message-id", "msg-file-123")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{
		"email", "send",
		"--from", "sender@example.com",
		"--to", "test@example.com",
		"--subject", "File test",
		"--html-file", htmlPath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedBody["html"] != htmlContent {
		t.Errorf("expected html body from file %q, got %q", htmlContent, receivedBody["html"])
	}
}

func TestSendCmd_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-message-id", "msg-json-123")
		w.WriteHeader(http.StatusAccepted)
		// Return empty body (202 with no content is common for email send)
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{
		"email", "send",
		"--from", "sender@example.com",
		"--to", "test@example.com",
		"--subject", "JSON test",
		"--text", "body",
		"--json",
	})

	// The --json flag triggers JSON output to stdout. We just verify the
	// command completes without error. (Stdout is written directly via
	// json.Encoder; capturing it would require redirecting os.Stdout.)
	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}
}

func TestSendCmd_MissingTo(t *testing.T) {
	// When --to is not provided and stdin is not a tty, RequireArg returns
	// an error. Tests run non-interactively, so this should fail.
	t.Setenv("MAILERSEND_API_TOKEN", "test-token")

	root := newRootCmd()
	root.SetArgs([]string{
		"email", "send",
		"--subject", "No recipient",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --to is not provided")
	}
}
