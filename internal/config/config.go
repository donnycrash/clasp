package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the top-level CLASP application configuration.
type Config struct {
	Endpoint         string          `yaml:"endpoint"`
	ScheduleInterval string          `yaml:"schedule_interval"`
	ClaudeDataDir    string          `yaml:"claude_data_dir"`
	Auth             AuthConfig      `yaml:"auth"`
	Redaction        RedactionConfig `yaml:"redaction"`
	Upload           UploadConfig    `yaml:"upload"`
	Sync             SyncConfig      `yaml:"sync"`
}

// AuthConfig configures the authentication provider and its settings.
type AuthConfig struct {
	Provider string           `yaml:"provider"`
	GitHub   GitHubAuthConfig `yaml:"github"`
	APIKey   APIKeyAuthConfig `yaml:"apikey"`
}

// GitHubAuthConfig holds GitHub OAuth configuration.
type GitHubAuthConfig struct {
	ClientID string `yaml:"client_id"`
}

// APIKeyAuthConfig holds API key authentication configuration.
type APIKeyAuthConfig struct{}

// RedactionConfig controls what fields are redacted and how.
type RedactionConfig struct {
	ProjectPath    string `yaml:"project_path"`
	FirstPrompt    string `yaml:"first_prompt"`
	BriefSummary   string `yaml:"brief_summary"`
	UnderlyingGoal string `yaml:"underlying_goal"`
	FrictionDetail string `yaml:"friction_detail"`
}

// UploadConfig controls batch upload behaviour.
type UploadConfig struct {
	BatchSize    int    `yaml:"batch_size"`
	RetryMax     int    `yaml:"retry_max"`
	RetryBackoff string `yaml:"retry_backoff"`
	Timeout      string `yaml:"timeout"`
}

// SyncConfig controls standards/rules sync from a remote repository.
type SyncConfig struct {
	Repo       string   `yaml:"repo"`
	Branch     string   `yaml:"branch"`
	AutoSync   bool     `yaml:"auto_sync"`
	LocalCache string   `yaml:"local_cache"`
	Tags       []string `yaml:"tags"`
}

// Default returns a Config populated with sensible default values.
func Default() *Config {
	return &Config{
		Endpoint:         "",
		ScheduleInterval: "24h",
		ClaudeDataDir:    "~/.claude",
		Auth: AuthConfig{
			Provider: "github",
			GitHub: GitHubAuthConfig{
				ClientID: "Ov23lixHlMGjU5wU24b1",
			},
		},
		Redaction: RedactionConfig{
			ProjectPath:    "hash",
			FirstPrompt:    "omit",
			BriefSummary:   "omit",
			UnderlyingGoal: "omit",
			FrictionDetail: "omit",
		},
		Upload: UploadConfig{
			BatchSize:    50,
			RetryMax:     3,
			RetryBackoff: "30s",
			Timeout:      "30s",
		},
		Sync: SyncConfig{
			Branch:   "main",
			AutoSync: true,
		},
	}
}

// Load reads a YAML configuration file from path and returns a Config with
// defaults applied for any missing fields. If the file does not exist,
// defaults are returned without error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("config file not found, using defaults", "path", path)
			return Default(), nil
		}
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", path, err)
	}

	applyDefaults(cfg)

	slog.Debug("loaded configuration", "path", path)
	return cfg, nil
}

// applyDefaults fills in zero-value fields with their default counterparts.
func applyDefaults(cfg *Config) {
	d := Default()

	if cfg.ScheduleInterval == "" {
		cfg.ScheduleInterval = d.ScheduleInterval
	}
	if cfg.ClaudeDataDir == "" {
		cfg.ClaudeDataDir = d.ClaudeDataDir
	}

	// Auth defaults
	if cfg.Auth.Provider == "" {
		cfg.Auth.Provider = d.Auth.Provider
	}

	// Redaction defaults
	if cfg.Redaction.ProjectPath == "" {
		cfg.Redaction.ProjectPath = d.Redaction.ProjectPath
	}
	if cfg.Redaction.FirstPrompt == "" {
		cfg.Redaction.FirstPrompt = d.Redaction.FirstPrompt
	}
	if cfg.Redaction.BriefSummary == "" {
		cfg.Redaction.BriefSummary = d.Redaction.BriefSummary
	}
	if cfg.Redaction.UnderlyingGoal == "" {
		cfg.Redaction.UnderlyingGoal = d.Redaction.UnderlyingGoal
	}
	if cfg.Redaction.FrictionDetail == "" {
		cfg.Redaction.FrictionDetail = d.Redaction.FrictionDetail
	}

	// Upload defaults
	if cfg.Upload.BatchSize == 0 {
		cfg.Upload.BatchSize = d.Upload.BatchSize
	}
	if cfg.Upload.RetryMax == 0 {
		cfg.Upload.RetryMax = d.Upload.RetryMax
	}
	if cfg.Upload.RetryBackoff == "" {
		cfg.Upload.RetryBackoff = d.Upload.RetryBackoff
	}
	if cfg.Upload.Timeout == "" {
		cfg.Upload.Timeout = d.Upload.Timeout
	}

	// Sync defaults
	if cfg.Sync.Branch == "" {
		cfg.Sync.Branch = d.Sync.Branch
	}
	// AutoSync defaults to true; since bool zero-value is false we only
	// override when the entire Sync block was left at its zero value and
	// the user didn't explicitly set auto_sync. YAML unmarshalling into a
	// pre-populated struct preserves the default when the key is absent,
	// so this is handled by initialising from Default() above.
}

// ConfigDir returns the CLASP configuration directory. It respects the
// CLASP_CONFIG_DIR environment variable; otherwise it defaults to
// ~/.config/clasp/.
func ConfigDir() string {
	if dir := os.Getenv("CLASP_CONFIG_DIR"); dir != "" {
		return expandHome(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("unable to determine home directory, falling back to current directory", "error", err)
		return filepath.Join(".", ".config", "clasp")
	}
	return filepath.Join(home, ".config", "clasp")
}

// ConfigPath returns the full path to the CLASP config.yaml file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}
