package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.mailersend.com"
	userAgent      = "mailersend-cli/0.1.0"
	maxRetries     = 3
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Verbose    bool
}

func New(token string) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type APIError struct {
	StatusCode int
	Message    string            `json:"message"`
	Errors     map[string][]string `json:"errors,omitempty"`
}

func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		var parts []string
		for field, msgs := range e.Errors {
			for _, msg := range msgs {
				parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
			}
		}
		return fmt.Sprintf("API error %d: %s (%s)", e.StatusCode, e.Message, strings.Join(parts, "; "))
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

type PaginatedResponse struct {
	Data  json.RawMessage `json:"data"`
	Links *PaginationLinks `json:"links,omitempty"`
	Meta  *PaginationMeta  `json:"meta,omitempty"`
}

type PaginationLinks struct {
	First string `json:"first"`
	Last  string `json:"last"`
	Prev  string `json:"prev"`
	Next  string `json:"next"`
}

// FlexInt unmarshals both JSON numbers and JSON strings containing numbers.
type FlexInt int

func (fi *FlexInt) UnmarshalJSON(b []byte) error {
	// Try number first
	var n int
	if err := json.Unmarshal(b, &n); err == nil {
		*fi = FlexInt(n)
		return nil
	}
	// Try string
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("FlexInt: cannot unmarshal %s", string(b))
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("FlexInt: invalid number string %q", s)
	}
	*fi = FlexInt(n)
	return nil
}

type PaginationMeta struct {
	CurrentPage int     `json:"current_page"`
	From        *int    `json:"from"`
	LastPage    int     `json:"last_page"`
	Path        string  `json:"path"`
	PerPage     FlexInt `json:"per_page"`
	To          *int    `json:"to"`
	Total       int     `json:"total"`
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	if req.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Read and reset body for retry
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("failed to reset request body: %w", err)
				}
				req.Body = body
			}
		}

		if c.Verbose {
			fmt.Printf("--> %s %s\n", req.Method, req.URL)
		}

		resp, lastErr = c.HTTPClient.Do(req)
		if lastErr != nil {
			if c.Verbose {
				fmt.Printf("<-- error: %v\n", lastErr)
			}
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			time.Sleep(backoff)
			continue
		}

		if c.Verbose {
			fmt.Printf("<-- %d %s\n", resp.StatusCode, resp.Status)
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if c.Verbose {
				fmt.Printf("    body: %s\n", string(body))
			}

			wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if secs, err := strconv.Atoi(retryAfter); err == nil {
					wait = time.Duration(secs) * time.Second
				}
			}
			if attempt < maxRetries {
				if c.Verbose {
					fmt.Printf("    retrying in %s...\n", wait)
				}
				time.Sleep(wait)
				continue
			}
			return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
		}

		return resp, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, lastErr)
	}
	return resp, nil
}

func (c *Client) Request(method, path string, params map[string]string, body interface{}) ([]byte, http.Header, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, nil, err
	}

	if len(params) > 0 {
		q := u.Query()
		for k, v := range params {
			if v != "" {
				q.Set(k, v)
			}
		}
		u.RawQuery = q.Encode()
	}

	var reqBody io.Reader
	var bodyBytes []byte
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(bodyBytes)
		if c.Verbose {
			fmt.Printf("--> body: %s\n", string(bodyBytes))
		}
	}

	req, err := http.NewRequest(method, u.String(), reqBody)
	if err != nil {
		return nil, nil, err
	}

	if bodyBytes != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response: %w", err)
	}

	if c.Verbose && len(respBody) > 0 {
		fmt.Printf("<-- body: %s\n", string(respBody))
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if len(respBody) > 0 {
			_ = json.Unmarshal(respBody, apiErr)
			if apiErr.Message == "" {
				apiErr.Message = string(respBody)
			}
		}
		return nil, nil, apiErr
	}

	return respBody, resp.Header, nil
}

func (c *Client) Get(path string, params map[string]string) ([]byte, error) {
	body, _, err := c.Request("GET", path, params, nil)
	return body, err
}

func (c *Client) Post(path string, body interface{}) ([]byte, http.Header, error) {
	return c.Request("POST", path, nil, body)
}

func (c *Client) Put(path string, body interface{}) ([]byte, error) {
	respBody, _, err := c.Request("PUT", path, nil, body)
	return respBody, err
}

func (c *Client) Delete(path string) ([]byte, error) {
	body, _, err := c.Request("DELETE", path, nil, nil)
	return body, err
}

func (c *Client) DeleteWithBody(path string, body interface{}) ([]byte, error) {
	respBody, _, err := c.Request("DELETE", path, nil, body)
	return respBody, err
}

// GetPaginated fetches all pages up to limit
func (c *Client) GetPaginated(path string, params map[string]string, limit int) ([]json.RawMessage, error) {
	if params == nil {
		params = make(map[string]string)
	}

	var allItems []json.RawMessage
	page := 1
	perPage := 25
	if limit > 0 && limit < perPage {
		perPage = limit
	}
	// MailerSend API requires limit >= 10
	if perPage < 10 {
		perPage = 10
	}

	for {
		params["page"] = strconv.Itoa(page)
		params["limit"] = strconv.Itoa(perPage)

		body, err := c.Get(path, params)
		if err != nil {
			return nil, err
		}

		var resp PaginatedResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		// Parse data array
		var items []json.RawMessage
		if err := json.Unmarshal(resp.Data, &items); err != nil {
			return nil, fmt.Errorf("failed to parse data array: %w", err)
		}

		allItems = append(allItems, items...)

		if limit > 0 && len(allItems) >= limit {
			allItems = allItems[:limit]
			break
		}

		if resp.Meta == nil || page >= resp.Meta.LastPage {
			break
		}
		page++
	}

	return allItems, nil
}
