package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	APIToken          string `yaml:"api_token,omitempty"`
	OAuthToken        string `yaml:"oauth_token,omitempty"`
	OAuthRefreshToken string `yaml:"oauth_refresh_token,omitempty"`
	OAuthExpiresAt    string `yaml:"oauth_expires_at,omitempty"`
}

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

func Dir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "mailersend"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "mailersend"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{Profiles: make(map[string]Profile)}, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if cfg.Profiles == nil {
		cfg.Profiles = make(map[string]Profile)
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	p, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(p, data, 0600)
}

func ActiveProfile(cfg *Config) (string, Profile, error) {
	name := cfg.ActiveProfile
	if name == "" {
		if len(cfg.Profiles) == 0 {
			return "", Profile{}, fmt.Errorf("no profiles configured — run 'mailersend auth login' or 'mailersend profile add <name>'")
		}
		for n := range cfg.Profiles {
			name = n
			break
		}
	}

	p, ok := cfg.Profiles[name]
	if !ok {
		return "", Profile{}, fmt.Errorf("profile %q not found", name)
	}
	return name, p, nil
}

func GetToken(profileOverride string) (string, error) {
	// Environment variable takes highest precedence
	if token := os.Getenv("MAILERSEND_API_TOKEN"); token != "" {
		return token, nil
	}

	cfg, err := Load()
	if err != nil {
		return "", err
	}

	var prof Profile
	if profileOverride != "" {
		p, ok := cfg.Profiles[profileOverride]
		if !ok {
			return "", fmt.Errorf("profile %q not found", profileOverride)
		}
		prof = p
	} else {
		_, p, err := ActiveProfile(cfg)
		if err != nil {
			return "", err
		}
		prof = p
	}

	if prof.APIToken != "" {
		return prof.APIToken, nil
	}
	if prof.OAuthToken != "" {
		return prof.OAuthToken, nil
	}
	return "", fmt.Errorf("no token found — run 'mailersend auth login' or set MAILERSEND_API_TOKEN")
}
