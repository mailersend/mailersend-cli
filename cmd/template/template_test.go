package template

import (
	"encoding/json"
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

func TestTemplateCmd_SubcommandsRegistered(t *testing.T) {
	expected := []string{"list", "get", "delete"}

	cmds := make(map[string]bool)
	for _, sub := range Cmd.Commands() {
		cmds[sub.Name()] = true
	}

	for _, name := range expected {
		if !cmds[name] {
			t.Errorf("expected subcommand %q to be registered on template command", name)
		}
	}
}

// ---------- Flag registration ----------

func TestTemplateListCmd_FlagsRegistered(t *testing.T) {
	flags := []string{"limit", "domain"}
	for _, name := range flags {
		if listCmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q on template list command", name)
		}
	}
}

func TestTemplateGetCmd_RequiresArgs(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"template", "get"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no template ID argument is provided")
	}
}

func TestTemplateDeleteCmd_RequiresArgs(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"template", "delete"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no template ID argument is provided")
	}
}

// ---------- Mock-server integration ----------

func TestTemplateListCmd_MockServer(t *testing.T) {
	var receivedPath string
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method

		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":         "tmpl-1",
					"name":       "Welcome Email",
					"type":       "html",
					"created_at": "2024-01-15T10:00:00Z",
				},
				{
					"id":         "tmpl-2",
					"name":       "Password Reset",
					"type":       "html",
					"created_at": "2024-02-20T12:00:00Z",
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
	root.SetArgs([]string{"template", "list"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedMethod != "GET" {
		t.Errorf("expected GET, got %s", receivedMethod)
	}
	if receivedPath != "/templates" {
		t.Errorf("expected /templates, got %s", receivedPath)
	}
}

func TestTemplateListCmd_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "tmpl-1", "name": "Test"},
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
	root.SetArgs([]string{"template", "list", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}
}

func TestTemplateGetCmd_MockServer(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path

		resp := map[string]interface{}{
			"data": map[string]interface{}{
				"id":         "tmpl-abc",
				"name":       "My Template",
				"type":       "html",
				"image_path": "https://example.com/img.png",
				"created_at": "2024-01-15T10:00:00Z",
				"category":   nil,
				"domain":     nil,
				"template_stats": map[string]interface{}{
					"total":              100,
					"queued":             5,
					"sent":               90,
					"rejected":           2,
					"delivered":          88,
					"last_email_sent_at": "2024-03-01T12:00:00Z",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{"template", "get", "tmpl-abc"})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedPath != "/templates/tmpl-abc" {
		t.Errorf("expected /templates/tmpl-abc, got %s", receivedPath)
	}
}
