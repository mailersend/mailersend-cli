package cmdutil

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mailersend/mailersend-cli/internal/config"
	"github.com/mailersend/mailersend-cli/internal/sdkclient"
	"github.com/mailersend/mailersend-go"
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

// SetVersion configures the SDK client user-agent with the CLI version.
func SetVersion(v string) {
	sdkclient.SetUserAgent("mailersend-cli/" + v)
}

// NewSDKClient creates a mailersend-go SDK client with CLI-specific behavior
// injected via a custom HTTP transport (retry, verbose, user-agent, base URL).
// Returns both the SDK client and the transport (needed for error body access).
func NewSDKClient(cmd *cobra.Command) (*mailersend.Mailersend, *sdkclient.CLITransport, error) {
	token, err := config.GetToken(ProfileFlag(cmd))
	if err != nil {
		return nil, nil, err
	}

	transport := &sdkclient.CLITransport{
		Base:    http.DefaultTransport,
		Verbose: VerboseFlag(cmd),
	}

	if base := os.Getenv("MAILERSEND_API_BASE_URL"); base != "" {
		transport.BaseURL = base
	}

	ms := mailersend.NewMailersend(token)
	ms.SetClient(&http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	})

	return ms, transport, nil
}

// ResolveDomainSDK takes a value that is either a domain ID or a domain name
// (hostname). If it contains a dot, it's treated as a hostname and resolved
// to a domain ID by listing domains from the API. Otherwise it's returned as-is.
func ResolveDomainSDK(ms *mailersend.Mailersend, transport *sdkclient.CLITransport, idOrName string) (string, error) {
	if !strings.Contains(idOrName, ".") {
		return idOrName, nil
	}

	ctx := context.Background()
	domains, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Domain, bool, error) {
		root, _, err := ms.Domain.List(ctx, &mailersend.ListDomainOptions{Page: page, Limit: perPage})
		if err != nil {
			return nil, false, sdkclient.WrapError(transport, err)
		}
		return root.Data, root.Links.Next != "", nil
	}, 0)
	if err != nil {
		return "", fmt.Errorf("failed to list domains for resolution: %w", err)
	}

	for _, d := range domains {
		if strings.EqualFold(d.Name, idOrName) {
			return d.ID, nil
		}
	}

	return "", fmt.Errorf("domain %q not found", idOrName)
}

// ResolveDomainNameSDK takes a value that is either a domain ID or a domain
// name (hostname) and always returns the domain name. If the input contains a
// dot it is treated as a hostname and returned as-is. Otherwise, the ID is
// resolved to a domain name by listing domains from the API.
func ResolveDomainNameSDK(ms *mailersend.Mailersend, transport *sdkclient.CLITransport, idOrName string) (string, error) {
	if strings.Contains(idOrName, ".") {
		return idOrName, nil
	}

	ctx := context.Background()
	domains, err := sdkclient.FetchAll(ctx, func(ctx context.Context, page, perPage int) ([]mailersend.Domain, bool, error) {
		root, _, err := ms.Domain.List(ctx, &mailersend.ListDomainOptions{Page: page, Limit: perPage})
		if err != nil {
			return nil, false, sdkclient.WrapError(transport, err)
		}
		return root.Data, root.Links.Next != "", nil
	}, 0)
	if err != nil {
		return "", fmt.Errorf("failed to list domains for resolution: %w", err)
	}

	for _, d := range domains {
		if d.ID == idOrName {
			return d.Name, nil
		}
	}

	return "", fmt.Errorf("domain ID %q not found", idOrName)
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
