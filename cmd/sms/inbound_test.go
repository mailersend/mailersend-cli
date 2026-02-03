package sms

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSmsInboundCreate_EnabledFieldSentByDefault(t *testing.T) {
	// MSD-14006: The "enabled" field must be included in the request body
	// even when the user doesn't explicitly pass --enabled. The default
	// value (true) should always be sent.
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/sms-inbounds" {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedBody)

			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"id":          "inb-1",
					"name":        "QA Inbound",
					"forward_url": "https://example.com/hook",
					"enabled":     true,
					"created_at":  "2024-01-01T00:00:00Z",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{
		"sms", "inbound", "create",
		"--sms-number-id", "num-123",
		"--name", "QA Inbound",
		"--forward-url", "https://example.com/hook",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedBody == nil {
		t.Fatal("expected request body to be captured")
	}

	enabled, ok := receivedBody["enabled"]
	if !ok {
		t.Fatal("expected 'enabled' field in request body, but it was missing")
	}
	if enabled != true {
		t.Errorf("expected enabled=true, got %v", enabled)
	}
}

func TestSmsInboundCreate_EnabledExplicitlyFalse(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &receivedBody)

			resp := map[string]interface{}{
				"data": map[string]interface{}{
					"id":          "inb-2",
					"name":        "Disabled Inbound",
					"forward_url": "https://example.com/hook",
					"enabled":     false,
					"created_at":  "2024-01-01T00:00:00Z",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(resp) //nolint:errcheck
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Setenv("MAILERSEND_API_TOKEN", "test-token")
	t.Setenv("MAILERSEND_API_BASE_URL", server.URL)

	root := newRootCmd()
	root.SetArgs([]string{
		"sms", "inbound", "create",
		"--sms-number-id", "num-123",
		"--name", "Disabled Inbound",
		"--forward-url", "https://example.com/hook",
		"--enabled=false",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("command returned error: %v", err)
	}

	if receivedBody == nil {
		t.Fatal("expected request body to be captured")
	}

	enabled, ok := receivedBody["enabled"]
	if !ok {
		t.Fatal("expected 'enabled' field in request body, but it was missing")
	}
	if enabled != false {
		t.Errorf("expected enabled=false, got %v", enabled)
	}
}
