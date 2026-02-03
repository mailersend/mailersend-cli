package sms

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestSmsActivityList_SmsNumberIdQueryParam(t *testing.T) {
	// The SDK serializes SmsNumberId with the url:"sms_number_id" struct tag,
	// so the query parameter is sent as "sms_number_id" (underscores).
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery

		resp := map[string]interface{}{
			"data":  []interface{}{},
			"links": map[string]string{"next": ""},
			"meta":  map[string]interface{}{"current_page": 1, "last_page": 1, "per_page": 25, "total": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"sms", "activity", "list", "--sms-number-id", "test-number-123"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if !strings.Contains(receivedQuery, "sms_number_id=test-number-123") {
		t.Errorf("expected sms_number_id=test-number-123 in query, got: %s", receivedQuery)
	}
}

func TestSmsActivityList_NoSmsNumberId(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		resp := map[string]interface{}{
			"data":  []interface{}{},
			"links": map[string]string{"next": ""},
			"meta":  map[string]interface{}{"current_page": 1, "last_page": 1, "per_page": 25, "total": 0},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"sms", "activity", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedPath != "/sms-activity" {
		t.Errorf("expected /sms-activity, got %s", receivedPath)
	}
}
