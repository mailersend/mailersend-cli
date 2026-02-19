package cmdutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
)

// domainListResponse builds a JSON paginated response containing the given domains.
func domainListResponse(domains []map[string]string) []byte {
	data, _ := json.Marshal(domains)
	resp := map[string]json.RawMessage{
		"data": data,
		"meta": json.RawMessage(`{"current_page":1,"last_page":1,"per_page":25,"total":` + jsonInt(len(domains)) + `}`),
	}
	b, _ := json.Marshal(resp)
	return b
}

func jsonInt(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func newTestSDKClient(handler http.HandlerFunc) (*mailersend.Mailersend, *sdkclient.CLITransport) {
	srv := httptest.NewServer(handler)
	transport := &sdkclient.CLITransport{
		Base:    http.DefaultTransport,
		BaseURL: srv.URL,
	}
	ms := mailersend.NewMailersend("test-token")
	ms.SetClient(&http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	})
	return ms, transport
}

func TestResolveDomainSDK_IDReturnsAsIs(t *testing.T) {
	// No dots means it's treated as an ID — no API call needed.
	ms := mailersend.NewMailersend("unused")

	got, err := ResolveDomainSDK(ms, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("expected %q, got %q", "abc123", got)
	}
}

func TestResolveDomainSDK_HostnameMatchReturnsDomainID(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{ //nolint:errcheck
			{"id": "domain-1", "name": "example.com"},
			{"id": "domain-2", "name": "test.org"},
		}))
	}
	ms, _ := newTestSDKClient(handler)

	got, err := ResolveDomainSDK(ms, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "domain-1" {
		t.Fatalf("expected %q, got %q", "domain-1", got)
	}
}

func TestResolveDomainSDK_HostnameNoMatchReturnsError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{ //nolint:errcheck
			{"id": "domain-1", "name": "example.com"},
		}))
	}
	ms, _ := newTestSDKClient(handler)

	_, err := ResolveDomainSDK(ms, "notfound.io")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if want := `domain "notfound.io" not found`; err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}

// ---------- ResolveDomainNameSDK ----------

func TestResolveDomainNameSDK_HostnameReturnsAsIs(t *testing.T) {
	// A value with a dot is treated as a hostname and returned unchanged.
	ms := mailersend.NewMailersend("unused")

	got, err := ResolveDomainNameSDK(ms, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "example.com" {
		t.Fatalf("expected %q, got %q", "example.com", got)
	}
}

func TestResolveDomainNameSDK_IDResolvedToName(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{ //nolint:errcheck
			{"id": "domain-1", "name": "example.com"},
			{"id": "domain-2", "name": "test-sdk.com"},
		}))
	}
	ms, _ := newTestSDKClient(handler)

	got, err := ResolveDomainNameSDK(ms, "domain-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "test-sdk.com" {
		t.Fatalf("expected %q, got %q", "test-sdk.com", got)
	}
}

func TestResolveDomainNameSDK_IDNotFoundReturnsError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{ //nolint:errcheck
			{"id": "domain-1", "name": "example.com"},
		}))
	}
	ms, _ := newTestSDKClient(handler)

	_, err := ResolveDomainNameSDK(ms, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if want := `domain ID "nonexistent" not found`; err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}

func TestResolveDomainSDK_CaseInsensitiveMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{ //nolint:errcheck
			{"id": "domain-upper", "name": "Example.COM"},
		}))
	}
	ms, _ := newTestSDKClient(handler)

	got, err := ResolveDomainSDK(ms, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "domain-upper" {
		t.Fatalf("expected %q, got %q", "domain-upper", got)
	}
}
