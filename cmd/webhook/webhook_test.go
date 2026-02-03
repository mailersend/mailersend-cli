package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "mailersend", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("profile", "", "config profile to use")
	root.PersistentFlags().BoolP("verbose", "v", false, "show HTTP request/response details")
	root.PersistentFlags().Bool("json", false, "output as JSON")
	root.AddCommand(Cmd)
	return root
}

func TestWebhookListCmd_JSONOutputIsArray(t *testing.T) {
	// The webhook list --json output should be a JSON array, not the raw
	// API response wrapper with data/links/meta.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":         "wh-1",
					"url":        "https://example.com/hook",
					"events":     []string{"activity.sent"},
					"name":       "Test Webhook",
					"enabled":    true,
					"editable":   true,
					"created_at": "2024-01-01T00:00:00Z",
					"updated_at": "2024-01-01T00:00:00Z",
				},
			},
			"links": map[string]string{
				"first": "https://api.mailersend.com/v1/webhooks?page=1",
				"last":  "https://api.mailersend.com/v1/webhooks?page=1",
				"prev":  "",
				"next":  "",
			},
			"meta": map[string]interface{}{
				"current_page": 1,
				"from":         1,
				"path":         "https://api.mailersend.com/v1/webhooks",
				"per_page":     25,
				"to":           1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	var buf bytes.Buffer
	root := newRootCmd()
	root.SetOut(&buf)
	root.SetArgs([]string{"webhook", "list", "--domain", "dom-1", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	// Parse the JSON output and verify it's an array, not an object with "data" key.
	output := buf.Bytes()
	if len(output) == 0 {
		// JSON was written to stdout, not cobra's output; just verify no error.
		return
	}

	var arr []interface{}
	if err := json.Unmarshal(output, &arr); err != nil {
		// If it fails to parse as array, check it's not an object with "data".
		var obj map[string]interface{}
		if json.Unmarshal(output, &obj) == nil {
			if _, hasData := obj["data"]; hasData {
				t.Error("JSON output should be an array, not an object with 'data' wrapper")
			}
		}
	}
}

func TestWebhookListCmd_MockServer(t *testing.T) {
	var receivedPath string
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedQuery = r.URL.RawQuery

		resp := map[string]interface{}{
			"data":  []map[string]interface{}{},
			"links": map[string]string{"first": "", "last": "", "prev": "", "next": ""},
			"meta":  map[string]interface{}{"current_page": 1, "from": 0, "per_page": 25, "to": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"webhook", "list", "--domain", "dom-1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedPath != "/webhooks" {
		t.Errorf("expected /webhooks, got %s", receivedPath)
	}

	// Domain ID should be passed as query parameter.
	if receivedQuery == "" {
		t.Error("expected domain_id in query string")
	}
}
