package cmdutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mailersend/mailersend-cli/internal/api"
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

func newTestClient(handler http.HandlerFunc) *api.Client {
	srv := httptest.NewServer(handler)
	c := api.New("test-token")
	c.BaseURL = srv.URL
	return c
}

func TestResolveDomain_IDReturnsAsIs(t *testing.T) {
	// No dots means it's treated as an ID — no API call needed.
	c := api.New("unused")
	got, err := ResolveDomain(c, "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "abc123" {
		t.Fatalf("expected %q, got %q", "abc123", got)
	}
}

func TestResolveDomain_HostnameMatchReturnsDomainID(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{
			{"id": "domain-1", "name": "example.com"},
			{"id": "domain-2", "name": "test.org"},
		}))
	}
	c := newTestClient(handler)

	got, err := ResolveDomain(c, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "domain-1" {
		t.Fatalf("expected %q, got %q", "domain-1", got)
	}
}

func TestResolveDomain_HostnameNoMatchReturnsError(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{
			{"id": "domain-1", "name": "example.com"},
		}))
	}
	c := newTestClient(handler)

	_, err := ResolveDomain(c, "notfound.io")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if want := `domain "notfound.io" not found`; err.Error() != want {
		t.Fatalf("expected error %q, got %q", want, err.Error())
	}
}

func TestResolveDomain_CaseInsensitiveMatch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(domainListResponse([]map[string]string{
			{"id": "domain-upper", "name": "Example.COM"},
		}))
	}
	c := newTestClient(handler)

	got, err := ResolveDomain(c, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "domain-upper" {
		t.Fatalf("expected %q, got %q", "domain-upper", got)
	}
}
