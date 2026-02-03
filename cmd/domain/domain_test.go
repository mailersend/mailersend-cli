package domain

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"
)

// newRootCmd builds a minimal root command tree with persistent flags.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{Use: "mailersend", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().String("profile", "", "config profile to use")
	root.PersistentFlags().BoolP("verbose", "v", false, "show HTTP request/response details")
	root.PersistentFlags().Bool("json", false, "output as JSON")
	root.AddCommand(Cmd)
	return root
}

// ---------- Subcommand registration ----------

func TestDomainCmd_SubcommandsRegistered(t *testing.T) {
	expected := []string{"list", "get", "add", "delete", "dns", "verify", "update-settings"}

	cmds := make(map[string]bool)
	for _, sub := range Cmd.Commands() {
		cmds[sub.Name()] = true
	}

	for _, name := range expected {
		if !cmds[name] {
			t.Errorf("expected subcommand %q to be registered on domain command", name)
		}
	}
}

// ---------- Flag registration ----------

func TestDomainListCmd_FlagsRegistered(t *testing.T) {
	flags := []string{"limit", "verified"}
	for _, name := range flags {
		if listCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q on domain list command", name)
		}
	}
}

func TestDomainAddCmd_FlagsRegistered(t *testing.T) {
	flags := []string{"name", "return-path-subdomain", "custom-tracking-subdomain"}
	for _, name := range flags {
		if addCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q on domain add command", name)
		}
	}
}

func TestDomainUpdateSettingsCmd_FlagsRegistered(t *testing.T) {
	flags := []string{
		"send-paused", "track-clicks", "track-opens", "track-unsubscribe",
		"track-content", "custom-tracking-enabled", "custom-tracking-subdomain",
		"precedence-bulk", "ignore-duplicated-recipients",
	}
	for _, name := range flags {
		if updateSettingsCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q on domain update-settings command", name)
		}
	}
}

// ---------- Mock-server integration ----------

func TestDomainListCmd_MockServer(t *testing.T) {
	var receivedPath string
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":            "domain-id-1",
					"name":          "example.com",
					"is_verified":   true,
					"is_dns_active": true,
					"created_at":    "2024-01-01T00:00:00Z",
					"updated_at":    "2024-01-02T00:00:00Z",
				},
				{
					"id":            "domain-id-2",
					"name":          "test.com",
					"is_verified":   false,
					"is_dns_active": false,
					"created_at":    "2024-02-01T00:00:00Z",
					"updated_at":    "2024-02-02T00:00:00Z",
				},
			},
			"meta": map[string]interface{}{
				"current_page": 1,
				"last_page":    1,
				"per_page":     25,
				"total":        2,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"domain", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedMethod != "GET" {
		t.Errorf("expected GET, got %s", receivedMethod)
	}
	if receivedPath != "/domains" {
		t.Errorf("expected /domains, got %s", receivedPath)
	}
}

func TestDomainGetCmd_MockServer(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":            "domain-id-1",
				"name":          "example.com",
				"is_verified":   true,
				"is_dns_active": true,
				"dkim":          true,
				"spf":           true,
				"tracking":      false,
				"created_at":    "2024-01-01T00:00:00Z",
				"updated_at":    "2024-01-02T00:00:00Z",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	// Use an ID (no dot) so it skips domain resolution.
	root.SetArgs([]string{"domain", "get", "domain-id-1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedPath != "/domains/domain-id-1" {
		t.Errorf("expected /domains/domain-id-1, got %s", receivedPath)
	}
}

func TestDomainAddCmd_MockServer(t *testing.T) {
	var receivedBody map[string]string
	var receivedMethod string
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path

		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &receivedBody)

		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":   "new-domain-id",
				"name": "newdomain.com",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{
		"domain", "add",
		"--name", "newdomain.com",
		"--return-path-subdomain", "rp",
		"--custom-tracking-subdomain", "track",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedMethod != "POST" {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedPath != "/domains" {
		t.Errorf("expected /domains, got %s", receivedPath)
	}
	if receivedBody["name"] != "newdomain.com" {
		t.Errorf("expected name newdomain.com, got %s", receivedBody["name"])
	}
	if receivedBody["return_path_subdomain"] != "rp" {
		t.Errorf("expected return_path_subdomain rp, got %s", receivedBody["return_path_subdomain"])
	}
	if receivedBody["custom_tracking_subdomain"] != "track" {
		t.Errorf("expected custom_tracking_subdomain track, got %s", receivedBody["custom_tracking_subdomain"])
	}
}

func TestDomainListCmd_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "d1", "name": "example.com"},
			},
			"meta": map[string]interface{}{
				"current_page": 1,
				"last_page":    1,
				"per_page":     25,
				"total":        1,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"domain", "list", "--json"})

	// Verify it completes without error when --json is set.
	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}
}
