package cmdutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mailersend/mailersend-cli/internal/api"
	"github.com/mailersend/mailersend-cli/internal/config"
	"github.com/spf13/cobra"
)

// ProfileFlag returns the --profile persistent flag value.
func ProfileFlag(cmd *cobra.Command) string {
	v, _ := cmd.Root().PersistentFlags().GetString("profile")
	return v
}

// VerboseFlag returns the --verbose persistent flag value.
func VerboseFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("verbose")
	return v
}

// JSONFlag returns the --json persistent flag value.
func JSONFlag(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("json")
	return v
}

// NewClient creates an API client using the active profile's token.
func NewClient(cmd *cobra.Command) (*api.Client, error) {
	token, err := config.GetToken(ProfileFlag(cmd))
	if err != nil {
		return nil, err
	}
	client := api.New(token)
	client.Verbose = VerboseFlag(cmd)
	// Allow overriding the base URL for testing (e.g., pointing at httptest.NewServer).
	if base := os.Getenv("MAILERSEND_API_BASE_URL"); base != "" {
		client.BaseURL = base
	}
	return client, nil
}

// ResolveDomain takes a value that is either a domain ID or a domain name
// (hostname). If it contains a dot, it's treated as a hostname and resolved
// to a domain ID by listing domains from the API. Otherwise it's returned as-is.
func ResolveDomain(client *api.Client, idOrName string) (string, error) {
	if !strings.Contains(idOrName, ".") {
		return idOrName, nil
	}

	// It looks like a hostname — search for it
	items, err := client.GetPaginated("/v1/domains", nil, 0)
	if err != nil {
		return "", fmt.Errorf("failed to list domains for resolution: %w", err)
	}

	for _, raw := range items {
		var d struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &d); err != nil {
			continue
		}
		if strings.EqualFold(d.Name, idOrName) {
			return d.ID, nil
		}
	}

	return "", fmt.Errorf("domain %q not found", idOrName)
}

// ParseDate accepts a date string in YYYY-MM-DD format or a raw unix
// timestamp and returns the corresponding unix timestamp as int64.
func ParseDate(value string) (int64, error) {
	t, err := time.Parse("2006-01-02", value)
	if err == nil {
		return t.Unix(), nil
	}
	ts, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid date %q: use YYYY-MM-DD or a unix timestamp", value)
	}
	return ts, nil
}

// DefaultDateRange returns parsed dateFrom/dateTo timestamps. If either value
// is empty, it defaults to the last 7 days (dateTo = now, dateFrom = now - 7d).
func DefaultDateRange(dateFromStr, dateToStr string, now time.Time) (int64, int64, error) {
	var dateFrom, dateTo int64
	var err error

	if dateFromStr == "" && dateToStr == "" {
		dateTo = now.Unix()
		dateFrom = now.AddDate(0, 0, -7).Unix()
		return dateFrom, dateTo, nil
	}

	if dateFromStr != "" {
		dateFrom, err = ParseDate(dateFromStr)
		if err != nil {
			return 0, 0, err
		}
	} else {
		dateFrom = now.AddDate(0, 0, -7).Unix()
	}

	if dateToStr != "" {
		dateTo, err = ParseDate(dateToStr)
		if err != nil {
			return 0, 0, err
		}
	} else {
		dateTo = now.Unix()
	}

	return dateFrom, dateTo, nil
}
