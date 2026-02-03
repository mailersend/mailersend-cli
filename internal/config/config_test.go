package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setTempConfigDir points XDG_CONFIG_HOME at a temp directory so that
// Dir(), Path(), Load(), Save(), and GetToken() all operate inside it.
func setTempConfigDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	return tmp
}

// writeConfigFile writes raw YAML into the config path used by Load().
func writeConfigFile(t *testing.T, content string) {
	t.Helper()
	p, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Dir()
// ---------------------------------------------------------------------------

func TestDir_WithXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/fakexdg")

	dir, err := Dir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/tmp/fakexdg", "mailersend")
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}

func TestDir_WithoutXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	dir, err := Dir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}
	want := filepath.Join(home, ".config", "mailersend")
	if dir != want {
		t.Errorf("Dir() = %q, want %q", dir, want)
	}
}

// ---------------------------------------------------------------------------
// Path()
// ---------------------------------------------------------------------------

func TestPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/fakexdg")

	p, err := Path()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join("/tmp/fakexdg", "mailersend", "config.yaml")
	if p != want {
		t.Errorf("Path() = %q, want %q", p, want)
	}
}

// ---------------------------------------------------------------------------
// Load()
// ---------------------------------------------------------------------------

func TestLoad_NoConfigFile(t *testing.T) {
	setTempConfigDir(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Profiles == nil {
		t.Fatal("Profiles map is nil, expected initialised map")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(cfg.Profiles))
	}
	if cfg.ActiveProfile != "" {
		t.Errorf("expected empty ActiveProfile, got %q", cfg.ActiveProfile)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	setTempConfigDir(t)
	writeConfigFile(t, `
active_profile: prod
profiles:
  prod:
    api_token: "tok_prod"
  staging:
    oauth_token: "oauth_staging"
    oauth_refresh_token: "refresh_staging"
    oauth_expires_at: "2026-12-31T00:00:00Z"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ActiveProfile != "prod" {
		t.Errorf("ActiveProfile = %q, want %q", cfg.ActiveProfile, "prod")
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles["prod"].APIToken != "tok_prod" {
		t.Errorf("prod APIToken = %q, want %q", cfg.Profiles["prod"].APIToken, "tok_prod")
	}
	if cfg.Profiles["staging"].OAuthToken != "oauth_staging" {
		t.Errorf("staging OAuthToken = %q, want %q", cfg.Profiles["staging"].OAuthToken, "oauth_staging")
	}
	if cfg.Profiles["staging"].OAuthRefreshToken != "refresh_staging" {
		t.Errorf("staging OAuthRefreshToken = %q, want %q", cfg.Profiles["staging"].OAuthRefreshToken, "refresh_staging")
	}
	if cfg.Profiles["staging"].OAuthExpiresAt != "2026-12-31T00:00:00Z" {
		t.Errorf("staging OAuthExpiresAt = %q, want %q", cfg.Profiles["staging"].OAuthExpiresAt, "2026-12-31T00:00:00Z")
	}
}

func TestLoad_ValidYAML_NoProfiles(t *testing.T) {
	setTempConfigDir(t)
	writeConfigFile(t, `active_profile: default
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Profiles == nil {
		t.Fatal("Profiles should be initialised even when missing from YAML")
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(cfg.Profiles))
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	setTempConfigDir(t)
	writeConfigFile(t, `{{{invalid yaml!!!`)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse config") {
		t.Errorf("error = %q, want it to contain 'failed to parse config'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Save() and round-trip
// ---------------------------------------------------------------------------

func TestSave_And_Load_RoundTrip(t *testing.T) {
	setTempConfigDir(t)

	original := &Config{
		ActiveProfile: "myprofile",
		Profiles: map[string]Profile{
			"myprofile": {
				APIToken: "tok_abc123",
			},
			"other": {
				OAuthToken:        "oauth_xyz",
				OAuthRefreshToken: "refresh_xyz",
				OAuthExpiresAt:    "2026-06-15T12:00:00Z",
			},
		},
	}

	if err := Save(original); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.ActiveProfile != original.ActiveProfile {
		t.Errorf("ActiveProfile = %q, want %q", loaded.ActiveProfile, original.ActiveProfile)
	}
	if len(loaded.Profiles) != len(original.Profiles) {
		t.Fatalf("profile count = %d, want %d", len(loaded.Profiles), len(original.Profiles))
	}
	for name, orig := range original.Profiles {
		got, ok := loaded.Profiles[name]
		if !ok {
			t.Errorf("profile %q missing after round-trip", name)
			continue
		}
		if got != orig {
			t.Errorf("profile %q = %+v, want %+v", name, got, orig)
		}
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	setTempConfigDir(t)

	cfg := &Config{
		ActiveProfile: "test",
		Profiles: map[string]Profile{
			"test": {APIToken: "tok"},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	p, _ := Path()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if info.IsDir() {
		t.Error("expected file, got directory")
	}
}

// ---------------------------------------------------------------------------
// ActiveProfile()
// ---------------------------------------------------------------------------

func TestActiveProfile_WithActiveProfileSet(t *testing.T) {
	cfg := &Config{
		ActiveProfile: "prod",
		Profiles: map[string]Profile{
			"prod":    {APIToken: "tok_prod"},
			"staging": {APIToken: "tok_staging"},
		},
	}

	name, prof, err := ActiveProfile(cfg)
	if err != nil {
		t.Fatalf("ActiveProfile() error: %v", err)
	}
	if name != "prod" {
		t.Errorf("name = %q, want %q", name, "prod")
	}
	if prof.APIToken != "tok_prod" {
		t.Errorf("APIToken = %q, want %q", prof.APIToken, "tok_prod")
	}
}

func TestActiveProfile_NoActiveProfile_PicksFirst(t *testing.T) {
	cfg := &Config{
		ActiveProfile: "",
		Profiles: map[string]Profile{
			"only": {APIToken: "tok_only"},
		},
	}

	name, prof, err := ActiveProfile(cfg)
	if err != nil {
		t.Fatalf("ActiveProfile() error: %v", err)
	}
	if name != "only" {
		t.Errorf("name = %q, want %q", name, "only")
	}
	if prof.APIToken != "tok_only" {
		t.Errorf("APIToken = %q, want %q", prof.APIToken, "tok_only")
	}
}

func TestActiveProfile_NoProfiles_Error(t *testing.T) {
	cfg := &Config{
		ActiveProfile: "",
		Profiles:      map[string]Profile{},
	}

	_, _, err := ActiveProfile(cfg)
	if err == nil {
		t.Fatal("expected error for no profiles, got nil")
	}
	if !strings.Contains(err.Error(), "no profiles configured") {
		t.Errorf("error = %q, want it to contain 'no profiles configured'", err.Error())
	}
}

func TestActiveProfile_NamedProfileNotFound(t *testing.T) {
	cfg := &Config{
		ActiveProfile: "nonexistent",
		Profiles: map[string]Profile{
			"other": {APIToken: "tok"},
		},
	}

	_, _, err := ActiveProfile(cfg)
	if err == nil {
		t.Fatal("expected error for missing profile, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// GetToken()
// ---------------------------------------------------------------------------

func TestGetToken_EnvVarTakesPrecedence(t *testing.T) {
	setTempConfigDir(t)
	t.Setenv("MAILERSEND_API_TOKEN", "env_token_123")

	// Even with a config file present, env var should win.
	writeConfigFile(t, `
active_profile: default
profiles:
  default:
    api_token: "config_token"
`)

	token, err := GetToken("")
	if err != nil {
		t.Fatalf("GetToken() error: %v", err)
	}
	if token != "env_token_123" {
		t.Errorf("token = %q, want %q", token, "env_token_123")
	}
}

func TestGetToken_ProfileOverride(t *testing.T) {
	setTempConfigDir(t)
	t.Setenv("MAILERSEND_API_TOKEN", "")

	writeConfigFile(t, `
active_profile: default
profiles:
  default:
    api_token: "default_token"
  override:
    api_token: "override_token"
`)

	token, err := GetToken("override")
	if err != nil {
		t.Fatalf("GetToken() error: %v", err)
	}
	if token != "override_token" {
		t.Errorf("token = %q, want %q", token, "override_token")
	}
}

func TestGetToken_ProfileOverride_NotFound(t *testing.T) {
	setTempConfigDir(t)
	t.Setenv("MAILERSEND_API_TOKEN", "")

	writeConfigFile(t, `
active_profile: default
profiles:
  default:
    api_token: "tok"
`)

	_, err := GetToken("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent profile override, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to contain 'not found'", err.Error())
	}
}

func TestGetToken_APITokenInProfile(t *testing.T) {
	setTempConfigDir(t)
	t.Setenv("MAILERSEND_API_TOKEN", "")

	writeConfigFile(t, `
active_profile: default
profiles:
  default:
    api_token: "api_tok_456"
    oauth_token: "oauth_tok_789"
`)

	token, err := GetToken("")
	if err != nil {
		t.Fatalf("GetToken() error: %v", err)
	}
	// api_token should take precedence over oauth_token
	if token != "api_tok_456" {
		t.Errorf("token = %q, want %q (api_token should take precedence)", token, "api_tok_456")
	}
}

func TestGetToken_OAuthTokenFallback(t *testing.T) {
	setTempConfigDir(t)
	t.Setenv("MAILERSEND_API_TOKEN", "")

	writeConfigFile(t, `
active_profile: default
profiles:
  default:
    oauth_token: "oauth_tok_789"
`)

	token, err := GetToken("")
	if err != nil {
		t.Fatalf("GetToken() error: %v", err)
	}
	if token != "oauth_tok_789" {
		t.Errorf("token = %q, want %q", token, "oauth_tok_789")
	}
}

func TestGetToken_NoTokenAtAll(t *testing.T) {
	setTempConfigDir(t)
	t.Setenv("MAILERSEND_API_TOKEN", "")

	writeConfigFile(t, `
active_profile: default
profiles:
  default: {}
`)

	_, err := GetToken("")
	if err == nil {
		t.Fatal("expected error when no token is available, got nil")
	}
	if !strings.Contains(err.Error(), "no token found") {
		t.Errorf("error = %q, want it to contain 'no token found'", err.Error())
	}
}

func TestGetToken_NoProfilesNoEnv(t *testing.T) {
	setTempConfigDir(t)
	t.Setenv("MAILERSEND_API_TOKEN", "")

	// No config file at all, no env var, no profile override.
	_, err := GetToken("")
	if err == nil {
		t.Fatal("expected error when no profiles exist, got nil")
	}
	if !strings.Contains(err.Error(), "no profiles configured") {
		t.Errorf("error = %q, want it to contain 'no profiles configured'", err.Error())
	}
}
