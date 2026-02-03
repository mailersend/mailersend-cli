package recipient

import (
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

// recipientsMockHandler returns a handler that serves a /domains endpoint and
// a /recipients endpoint with recipients from multiple domains.
func recipientsMockHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/domains" {
			resp := map[string]interface{}{
				"data": []map[string]interface{}{
					{"id": "dom-1", "name": "example.com"},
					{"id": "dom-2", "name": "test-sdk.com"},
				},
				"links": map[string]string{"next": ""},
				"meta":  map[string]interface{}{"current_page": 1, "last_page": 1, "per_page": 25, "total": 2},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
			return
		}

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "r1", "email": "alice@example.com", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "deleted_at": ""},
				{"id": "r2", "email": "bob@test-sdk.com", "created_at": "2024-01-02T00:00:00Z", "updated_at": "2024-01-02T00:00:00Z", "deleted_at": ""},
				{"id": "r3", "email": "carol@test-sdk.com", "created_at": "2024-01-03T00:00:00Z", "updated_at": "2024-01-03T00:00:00Z", "deleted_at": ""},
				{"id": "r4", "email": "dave@other.org", "created_at": "2024-01-04T00:00:00Z", "updated_at": "2024-01-04T00:00:00Z", "deleted_at": ""},
			},
			"links": map[string]string{"next": ""},
			"meta":  map[string]interface{}{"current_page": 1, "last_page": 1, "per_page": 25, "total": 4},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}

// TestRecipientListCmd_NoDomainFilter must run first because cobra retains
// parsed flag state on the package-level listCmd between tests.
func TestRecipientListCmd_NoDomainFilter(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		resp := map[string]interface{}{
			"data":  []map[string]interface{}{},
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
	root.SetArgs([]string{"recipient", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedPath != "/recipients" {
		t.Errorf("expected /recipients, got %s", receivedPath)
	}
}

func TestRecipientListCmd_DomainFilterByName(t *testing.T) {
	// The API doesn't support domain_id filtering for /recipients,
	// so the CLI must do client-side filtering by email suffix.
	var receivedPaths []string

	server := httptest.NewServer(recipientsMockHandler())
	defer server.Close()

	// Capture paths from the mock.
	origHandler := server.Config.Handler
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPaths = append(receivedPaths, r.URL.Path)
		origHandler.ServeHTTP(w, r)
	})

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"recipient", "list", "--domain", "test-sdk.com", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	var calledRecipients bool
	for _, p := range receivedPaths {
		if p == "/recipients" {
			calledRecipients = true
		}
	}
	if !calledRecipients {
		t.Error("expected /recipients endpoint to be called")
	}
}

func TestRecipientListCmd_DomainFilterByID(t *testing.T) {
	// When domain is passed as an ID (no dot), the CLI resolves to a name
	// and then filters by email suffix.
	server := httptest.NewServer(recipientsMockHandler())
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"recipient", "list", "--domain", "dom-2", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}
}
