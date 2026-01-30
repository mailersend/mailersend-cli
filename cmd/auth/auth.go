package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/mailersend/mailersend-cli/internal/config"
	"github.com/mailersend/mailersend-cli/internal/output"
	"github.com/mailersend/mailersend-cli/internal/prompt"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "auth",
	Short: "Authenticate with MailerSend",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to MailerSend",
	Long:  "Authenticate via API token or OAuth browser flow.",
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove stored credentials",
	RunE:  runLogout,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runStatus,
}

func init() {
	loginCmd.Flags().String("method", "", "auth method: token or oauth")
	loginCmd.Flags().String("token", "", "API token (for token method)")
	loginCmd.Flags().String("profile", "", "profile name to save credentials to (default: uses active profile or 'default')")
	Cmd.AddCommand(loginCmd, logoutCmd, statusCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	method, _ := cmd.Flags().GetString("method")
	token, _ := cmd.Flags().GetString("token")
	profName, _ := cmd.Flags().GetString("profile")

	if method == "" && prompt.IsInteractive() {
		var err error
		method, err = prompt.Select("Authentication method", []string{"token", "oauth"})
		if err != nil {
			return err
		}
	}
	if method == "" {
		method = "token"
	}

	if profName == "" {
		profName = "default"
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch method {
	case "token":
		if token == "" {
			if !prompt.IsInteractive() {
				return fmt.Errorf("--token is required in non-interactive mode")
			}
			token, err = prompt.Input("API Token", "mlsn_...")
			if err != nil {
				return err
			}
		}
		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}
		cfg.Profiles[profName] = config.Profile{APIToken: token}

	case "oauth":
		oauthToken, err := oauthBrowserFlow()
		if err != nil {
			return fmt.Errorf("OAuth login failed: %w", err)
		}
		cfg.Profiles[profName] = config.Profile{OAuthToken: oauthToken}

	default:
		return fmt.Errorf("unknown auth method: %s (use 'token' or 'oauth')", method)
	}

	cfg.ActiveProfile = profName
	if err := config.Save(cfg); err != nil {
		return err
	}

	output.Success(fmt.Sprintf("Logged in successfully. Profile: %s", profName))
	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	profFlag, _ := cmd.Root().PersistentFlags().GetString("profile")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	name := profFlag
	if name == "" {
		name = cfg.ActiveProfile
	}
	if name == "" {
		name = "default"
	}

	if _, ok := cfg.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	delete(cfg.Profiles, name)
	if cfg.ActiveProfile == name {
		cfg.ActiveProfile = ""
		for n := range cfg.Profiles {
			cfg.ActiveProfile = n
			break
		}
	}

	if err := config.Save(cfg); err != nil {
		return err
	}

	output.Success(fmt.Sprintf("Logged out from profile: %s", name))
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	profFlag, _ := cmd.Root().PersistentFlags().GetString("profile")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	name := profFlag
	if name == "" {
		name, _, err = config.ActiveProfile(cfg)
		if err != nil {
			output.Error("Not logged in. Run 'mailersend auth login' to authenticate.")
			return nil
		}
	}

	prof, ok := cfg.Profiles[name]
	if !ok {
		output.Error(fmt.Sprintf("Profile %q not found.", name))
		return nil
	}

	jsonFlag, _ := cmd.Root().PersistentFlags().GetBool("json")
	if jsonFlag {
		return output.JSON(map[string]interface{}{
			"profile":    name,
			"has_token":  prof.APIToken != "",
			"has_oauth":  prof.OAuthToken != "",
			"expires_at": prof.OAuthExpiresAt,
		})
	}

	method := "API Token"
	if prof.OAuthToken != "" {
		method = "OAuth"
	}
	maskedToken := "none"
	if prof.APIToken != "" {
		t := prof.APIToken
		if len(t) > 10 {
			maskedToken = t[:7] + "..." + t[len(t)-4:]
		} else {
			maskedToken = "***"
		}
	}

	output.Table(
		[]string{"Field", "Value"},
		[][]string{
			{"Profile", name},
			{"Method", method},
			{"Token", maskedToken},
			{"Active", "Yes"},
		},
	)
	return nil
}

func oauthBrowserFlow() (string, error) {
	state, err := randomHex(16)
	if err != nil {
		return "", err
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to start local server: %w", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code != "" {
			tokenCh <- code
			fmt.Fprintf(w, "<html><body><h2>Authentication successful!</h2><p>You can close this window.</p></body></html>")
		} else {
			errMsg := r.URL.Query().Get("error")
			errCh <- fmt.Errorf("OAuth error: %s", errMsg)
			fmt.Fprintf(w, "<html><body><h2>Authentication failed</h2><p>%s</p></body></html>", errMsg)
		}
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer server.Shutdown(context.Background())

	authURL := fmt.Sprintf("https://app.mailersend.com/oauth/authorize?response_type=code&redirect_uri=%s&state=%s",
		url.QueryEscape(callbackURL), state)

	fmt.Printf("Opening browser for authentication...\n")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n\n", authURL)
	openBrowser(authURL)

	select {
	case token := <-tokenCh:
		return token, nil
	case err := <-errCh:
		return "", err
	case <-time.After(5 * time.Minute):
		return "", fmt.Errorf("authentication timed out after 5 minutes")
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
