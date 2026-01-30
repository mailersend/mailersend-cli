package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Client basics
// ---------------------------------------------------------------------------

func TestNew_Defaults(t *testing.T) {
	c := New("test-token")

	if c.BaseURL != DefaultBaseURL {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, DefaultBaseURL)
	}
	if c.Token != "test-token" {
		t.Errorf("Token = %q, want %q", c.Token, "test-token")
	}
	if c.HTTPClient == nil {
		t.Fatal("HTTPClient is nil")
	}
	if c.Verbose {
		t.Error("Verbose should default to false")
	}
}

func TestAuthorizationHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		want := "Bearer my-secret-token"
		if got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("my-secret-token")
	c.BaseURL = srv.URL

	_, _ = c.Get("/test", nil)
}

func TestUserAgentHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("User-Agent")
		if got != userAgent {
			t.Errorf("User-Agent = %q, want %q", got, userAgent)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, _ = c.Get("/test", nil)
}

func TestContentTypeSetForPost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, _, _ = c.Post("/test", map[string]string{"key": "value"})
}

func TestContentTypeSetForPut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want %q", ct, "application/json")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, _ = c.Put("/test", map[string]string{"key": "value"})
}

func TestContentTypeNotSetForGetWithoutBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if ct != "" {
			t.Errorf("Content-Type should be empty for GET, got %q", ct)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, _ = c.Get("/test", nil)
}

// ---------------------------------------------------------------------------
// Request methods
// ---------------------------------------------------------------------------

func TestGet_SendsGETWithParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		if got := r.URL.Query().Get("foo"); got != "bar" {
			t.Errorf("query param foo = %q, want %q", got, "bar")
		}
		if got := r.URL.Query().Get("baz"); got != "qux" {
			t.Errorf("query param baz = %q, want %q", got, "qux")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"ok"}`))
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	body, err := c.Get("/items", map[string]string{"foo": "bar", "baz": "qux"})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if !strings.Contains(string(body), `"result":"ok"`) {
		t.Errorf("body = %s, want to contain result:ok", body)
	}
}

func TestGet_EmptyParamsOmitted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("empty") != "" {
			t.Errorf("empty param should be omitted, got %q", r.URL.Query().Get("empty"))
		}
		if r.URL.Query().Get("present") != "yes" {
			t.Errorf("present param = %q, want yes", r.URL.Query().Get("present"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, _ = c.Get("/test", map[string]string{"empty": "", "present": "yes"})
}

func TestPost_SendsPOSTWithJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}
		if data["name"] != "test" {
			t.Errorf("body name = %q, want test", data["name"])
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"123"}`))
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	resp, headers, err := c.Post("/items", map[string]string{"name": "test"})
	if err != nil {
		t.Fatalf("Post returned error: %v", err)
	}
	if headers == nil {
		t.Error("expected non-nil headers")
	}
	if !strings.Contains(string(resp), `"id":"123"`) {
		t.Errorf("response = %s", resp)
	}
}

func TestPut_SendsPUTWithJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Method = %q, want PUT", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("failed to unmarshal body: %v", err)
		}
		if data["name"] != "updated" {
			t.Errorf("body name = %q, want updated", data["name"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	resp, err := c.Put("/items/1", map[string]string{"name": "updated"})
	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	if !strings.Contains(string(resp), `"ok":true`) {
		t.Errorf("response = %s", resp)
	}
}

func TestDelete_SendsDELETE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/items/42" {
			t.Errorf("path = %q, want /items/42", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, err := c.Delete("/items/42")
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

func TestAPIError_ParsedFrom4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"Validation failed","errors":{"email":["is required"]}}`))
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, err := c.Get("/test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 422 {
		t.Errorf("StatusCode = %d, want 422", apiErr.StatusCode)
	}
	if apiErr.Message != "Validation failed" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Validation failed")
	}
	if len(apiErr.Errors) == 0 {
		t.Fatal("expected Errors map to be populated")
	}
	if msgs := apiErr.Errors["email"]; len(msgs) == 0 || msgs[0] != "is required" {
		t.Errorf("Errors[email] = %v", msgs)
	}
}

func TestAPIError_ErrorStringMessageOnly(t *testing.T) {
	e := &APIError{StatusCode: 404, Message: "Not found"}
	got := e.Error()
	want := "API error 404: Not found"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_ErrorStringWithFieldErrors(t *testing.T) {
	e := &APIError{
		StatusCode: 422,
		Message:    "Validation failed",
		Errors: map[string][]string{
			"email": {"is required"},
		},
	}
	got := e.Error()
	if !strings.Contains(got, "API error 422") {
		t.Errorf("Error() = %q, missing status code", got)
	}
	if !strings.Contains(got, "Validation failed") {
		t.Errorf("Error() = %q, missing message", got)
	}
	if !strings.Contains(got, "email: is required") {
		t.Errorf("Error() = %q, missing field error", got)
	}
}

func TestAPIError_4xxNonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`Forbidden`))
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, err := c.Get("/test", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
	// Message should fall back to raw body
	if apiErr.Message != "Forbidden" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "Forbidden")
	}
}

// ---------------------------------------------------------------------------
// Retry logic
// ---------------------------------------------------------------------------

func TestRetry_On429(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}

	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&count, 1)
		if n <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	body, err := c.Get("/test", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Errorf("body = %s", body)
	}
	if got := atomic.LoadInt32(&count); got < 3 {
		t.Errorf("expected at least 3 attempts, got %d", got)
	}
}

func TestNoRetry_On4xx(t *testing.T) {
	var count int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&count, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"bad request"}`))
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	_, err := c.Get("/test", nil)
	if err == nil {
		t.Fatal("expected error for 400")
	}

	if got := atomic.LoadInt32(&count); got != 1 {
		t.Errorf("expected exactly 1 attempt (no retry), got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

func makePaginatedHandler(totalItems int, perPage int) http.HandlerFunc {
	// Build all items
	type item struct {
		ID int `json:"id"`
	}
	var allItems []item
	for i := 1; i <= totalItems; i++ {
		allItems = append(allItems, item{ID: i})
	}

	lastPage := (totalItems + perPage - 1) / perPage
	if lastPage == 0 {
		lastPage = 1
	}

	return func(w http.ResponseWriter, r *http.Request) {
		pageStr := r.URL.Query().Get("page")
		page := 1
		if pageStr != "" {
			fmt.Sscanf(pageStr, "%d", &page)
		}

		start := (page - 1) * perPage
		end := start + perPage
		if start > len(allItems) {
			start = len(allItems)
		}
		if end > len(allItems) {
			end = len(allItems)
		}

		pageItems := allItems[start:end]

		from := start + 1
		to := end
		resp := PaginatedResponse{
			Meta: &PaginationMeta{
				CurrentPage: page,
				From:        &from,
				LastPage:    lastPage,
				PerPage:     FlexInt(perPage),
				To:          &to,
				Total:       totalItems,
			},
		}

		data, _ := json.Marshal(pageItems)
		resp.Data = data

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func TestGetPaginated_SinglePage(t *testing.T) {
	srv := httptest.NewServer(makePaginatedHandler(3, 25))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	items, err := c.GetPaginated("/items", nil, 0)
	if err != nil {
		t.Fatalf("GetPaginated error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
}

func TestGetPaginated_MultiplePages(t *testing.T) {
	// 7 items, 3 per page -> 3 pages (3+3+1)
	srv := httptest.NewServer(makePaginatedHandler(7, 3))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	items, err := c.GetPaginated("/items", nil, 0)
	if err != nil {
		t.Fatalf("GetPaginated error: %v", err)
	}
	if len(items) != 7 {
		t.Errorf("got %d items, want 7", len(items))
	}

	// Verify first and last item
	var first, last map[string]int
	json.Unmarshal(items[0], &first)
	json.Unmarshal(items[6], &last)
	if first["id"] != 1 {
		t.Errorf("first item id = %d, want 1", first["id"])
	}
	if last["id"] != 7 {
		t.Errorf("last item id = %d, want 7", last["id"])
	}
}

func TestGetPaginated_RespectsLimit(t *testing.T) {
	// 10 items available, but we only want 4
	srv := httptest.NewServer(makePaginatedHandler(10, 3))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	items, err := c.GetPaginated("/items", nil, 4)
	if err != nil {
		t.Fatalf("GetPaginated error: %v", err)
	}
	if len(items) != 4 {
		t.Errorf("got %d items, want 4", len(items))
	}
}

func TestGetPaginated_StopsAtLastPage(t *testing.T) {
	var requestCount int32
	totalItems := 5
	perPage := 3
	lastPage := 2

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)

		pageStr := r.URL.Query().Get("page")
		page := 1
		if pageStr != "" {
			fmt.Sscanf(pageStr, "%d", &page)
		}

		var items []map[string]int
		if page == 1 {
			items = []map[string]int{{"id": 1}, {"id": 2}, {"id": 3}}
		} else if page == 2 {
			items = []map[string]int{{"id": 4}, {"id": 5}}
		} else {
			// Should never reach page 3
			t.Errorf("unexpected request for page %d", page)
			items = []map[string]int{}
		}

		from := (page-1)*perPage + 1
		to := from + len(items) - 1
		resp := PaginatedResponse{
			Meta: &PaginationMeta{
				CurrentPage: page,
				From:        &from,
				LastPage:    lastPage,
				PerPage:     FlexInt(perPage),
				To:          &to,
				Total:       totalItems,
			},
		}
		data, _ := json.Marshal(items)
		resp.Data = data

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	items, err := c.GetPaginated("/items", nil, 0)
	if err != nil {
		t.Fatalf("GetPaginated error: %v", err)
	}
	if len(items) != 5 {
		t.Errorf("got %d items, want 5", len(items))
	}

	// Should have made exactly 2 requests (pages 1 and 2)
	if got := atomic.LoadInt32(&requestCount); got != 2 {
		t.Errorf("made %d requests, want 2", got)
	}
}

func TestGetPaginated_LimitSmallerThanPerPage(t *testing.T) {
	// When limit < default perPage (25), perPage is clamped to API minimum of 10.
	// The result set is still truncated to the requested limit.
	var capturedLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLimit = r.URL.Query().Get("limit")

		items := []map[string]int{{"id": 1}, {"id": 2}, {"id": 3}, {"id": 4}, {"id": 5}}
		from := 1
		to := 5
		resp := PaginatedResponse{
			Data: mustMarshal(items),
			Meta: &PaginationMeta{
				CurrentPage: 1,
				From:        &from,
				LastPage:    1,
				PerPage:     FlexInt(10),
				To:          &to,
				Total:       5,
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := New("tok")
	c.BaseURL = srv.URL

	items, err := c.GetPaginated("/items", nil, 2)
	if err != nil {
		t.Fatalf("GetPaginated error: %v", err)
	}
	// API gets minimum of 10
	if capturedLimit != "10" {
		t.Errorf("limit query param = %q, want %q", capturedLimit, "10")
	}
	// But results are truncated to the user's requested limit
	if len(items) != 2 {
		t.Errorf("got %d items, want 2", len(items))
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
