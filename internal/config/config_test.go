package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Top-level fields
	if cfg.Endpoint != "" {
		t.Errorf("Endpoint: got %q, want %q", cfg.Endpoint, "")
	}
	if cfg.ScheduleInterval != "24h" {
		t.Errorf("ScheduleInterval: got %q, want %q", cfg.ScheduleInterval, "24h")
	}
	if cfg.ClaudeDataDir != "~/.claude" {
		t.Errorf("ClaudeDataDir: got %q, want %q", cfg.ClaudeDataDir, "~/.claude")
	}

	// Auth
	if cfg.Auth.Provider != "github" {
		t.Errorf("Auth.Provider: got %q, want %q", cfg.Auth.Provider, "github")
	}
	if cfg.Auth.GitHub.ClientID != "" {
		t.Errorf("Auth.GitHub.ClientID: got %q, want %q", cfg.Auth.GitHub.ClientID, "")
	}

	// Redaction
	if cfg.Redaction.ProjectPath != "hash" {
		t.Errorf("Redaction.ProjectPath: got %q, want %q", cfg.Redaction.ProjectPath, "hash")
	}
	if cfg.Redaction.FirstPrompt != "omit" {
		t.Errorf("Redaction.FirstPrompt: got %q, want %q", cfg.Redaction.FirstPrompt, "omit")
	}
	if cfg.Redaction.BriefSummary != "omit" {
		t.Errorf("Redaction.BriefSummary: got %q, want %q", cfg.Redaction.BriefSummary, "omit")
	}
	if cfg.Redaction.UnderlyingGoal != "omit" {
		t.Errorf("Redaction.UnderlyingGoal: got %q, want %q", cfg.Redaction.UnderlyingGoal, "omit")
	}
	if cfg.Redaction.FrictionDetail != "omit" {
		t.Errorf("Redaction.FrictionDetail: got %q, want %q", cfg.Redaction.FrictionDetail, "omit")
	}

	// Upload
	if cfg.Upload.BatchSize != 50 {
		t.Errorf("Upload.BatchSize: got %d, want %d", cfg.Upload.BatchSize, 50)
	}
	if cfg.Upload.RetryMax != 3 {
		t.Errorf("Upload.RetryMax: got %d, want %d", cfg.Upload.RetryMax, 3)
	}
	if cfg.Upload.RetryBackoff != "30s" {
		t.Errorf("Upload.RetryBackoff: got %q, want %q", cfg.Upload.RetryBackoff, "30s")
	}
	if cfg.Upload.Timeout != "30s" {
		t.Errorf("Upload.Timeout: got %q, want %q", cfg.Upload.Timeout, "30s")
	}

	// Sync
	if cfg.Sync.Repo != "" {
		t.Errorf("Sync.Repo: got %q, want %q", cfg.Sync.Repo, "")
	}
	if cfg.Sync.Branch != "main" {
		t.Errorf("Sync.Branch: got %q, want %q", cfg.Sync.Branch, "main")
	}
	if !cfg.Sync.AutoSync {
		t.Error("Sync.AutoSync: got false, want true")
	}
	if cfg.Sync.LocalCache != "" {
		t.Errorf("Sync.LocalCache: got %q, want %q", cfg.Sync.LocalCache, "")
	}
	if len(cfg.Sync.Tags) != 0 {
		t.Errorf("Sync.Tags: got %v, want empty slice", cfg.Sync.Tags)
	}
}

func TestLoad_FileNotExist(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := Default()
	if cfg.ScheduleInterval != defaults.ScheduleInterval {
		t.Errorf("ScheduleInterval: got %q, want %q", cfg.ScheduleInterval, defaults.ScheduleInterval)
	}
	if cfg.Auth.Provider != defaults.Auth.Provider {
		t.Errorf("Auth.Provider: got %q, want %q", cfg.Auth.Provider, defaults.Auth.Provider)
	}
	if cfg.Upload.BatchSize != defaults.Upload.BatchSize {
		t.Errorf("Upload.BatchSize: got %d, want %d", cfg.Upload.BatchSize, defaults.Upload.BatchSize)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	content := `
endpoint: "https://api.example.com"
schedule_interval: "12h"
claude_data_dir: "/custom/claude"
auth:
  provider: "apikey"
  github:
    client_id: "abc123"
redaction:
  project_path: "keep"
  first_prompt: "keep"
  brief_summary: "keep"
  underlying_goal: "keep"
  friction_detail: "keep"
upload:
  batch_size: 100
  retry_max: 5
  retry_backoff: "60s"
  timeout: "45s"
sync:
  repo: "org/repo"
  branch: "develop"
  auto_sync: false
  local_cache: "/tmp/cache"
  tags:
    - "go"
    - "security"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Top-level
	if cfg.Endpoint != "https://api.example.com" {
		t.Errorf("Endpoint: got %q, want %q", cfg.Endpoint, "https://api.example.com")
	}
	if cfg.ScheduleInterval != "12h" {
		t.Errorf("ScheduleInterval: got %q, want %q", cfg.ScheduleInterval, "12h")
	}
	if cfg.ClaudeDataDir != "/custom/claude" {
		t.Errorf("ClaudeDataDir: got %q, want %q", cfg.ClaudeDataDir, "/custom/claude")
	}

	// Auth
	if cfg.Auth.Provider != "apikey" {
		t.Errorf("Auth.Provider: got %q, want %q", cfg.Auth.Provider, "apikey")
	}
	if cfg.Auth.GitHub.ClientID != "abc123" {
		t.Errorf("Auth.GitHub.ClientID: got %q, want %q", cfg.Auth.GitHub.ClientID, "abc123")
	}

	// Redaction
	if cfg.Redaction.ProjectPath != "keep" {
		t.Errorf("Redaction.ProjectPath: got %q, want %q", cfg.Redaction.ProjectPath, "keep")
	}
	if cfg.Redaction.FirstPrompt != "keep" {
		t.Errorf("Redaction.FirstPrompt: got %q, want %q", cfg.Redaction.FirstPrompt, "keep")
	}
	if cfg.Redaction.BriefSummary != "keep" {
		t.Errorf("Redaction.BriefSummary: got %q, want %q", cfg.Redaction.BriefSummary, "keep")
	}
	if cfg.Redaction.UnderlyingGoal != "keep" {
		t.Errorf("Redaction.UnderlyingGoal: got %q, want %q", cfg.Redaction.UnderlyingGoal, "keep")
	}
	if cfg.Redaction.FrictionDetail != "keep" {
		t.Errorf("Redaction.FrictionDetail: got %q, want %q", cfg.Redaction.FrictionDetail, "keep")
	}

	// Upload
	if cfg.Upload.BatchSize != 100 {
		t.Errorf("Upload.BatchSize: got %d, want %d", cfg.Upload.BatchSize, 100)
	}
	if cfg.Upload.RetryMax != 5 {
		t.Errorf("Upload.RetryMax: got %d, want %d", cfg.Upload.RetryMax, 5)
	}
	if cfg.Upload.RetryBackoff != "60s" {
		t.Errorf("Upload.RetryBackoff: got %q, want %q", cfg.Upload.RetryBackoff, "60s")
	}
	if cfg.Upload.Timeout != "45s" {
		t.Errorf("Upload.Timeout: got %q, want %q", cfg.Upload.Timeout, "45s")
	}

	// Sync
	if cfg.Sync.Repo != "org/repo" {
		t.Errorf("Sync.Repo: got %q, want %q", cfg.Sync.Repo, "org/repo")
	}
	if cfg.Sync.Branch != "develop" {
		t.Errorf("Sync.Branch: got %q, want %q", cfg.Sync.Branch, "develop")
	}
	if cfg.Sync.AutoSync {
		t.Error("Sync.AutoSync: got true, want false")
	}
	if cfg.Sync.LocalCache != "/tmp/cache" {
		t.Errorf("Sync.LocalCache: got %q, want %q", cfg.Sync.LocalCache, "/tmp/cache")
	}
	if len(cfg.Sync.Tags) != 2 || cfg.Sync.Tags[0] != "go" || cfg.Sync.Tags[1] != "security" {
		t.Errorf("Sync.Tags: got %v, want [go security]", cfg.Sync.Tags)
	}
}

func TestLoad_PartialYAML(t *testing.T) {
	// Only set endpoint and upload.batch_size; everything else should get defaults.
	content := `
endpoint: "https://partial.example.com"
upload:
  batch_size: 200
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := Default()

	// Explicitly set values
	if cfg.Endpoint != "https://partial.example.com" {
		t.Errorf("Endpoint: got %q, want %q", cfg.Endpoint, "https://partial.example.com")
	}
	if cfg.Upload.BatchSize != 200 {
		t.Errorf("Upload.BatchSize: got %d, want %d", cfg.Upload.BatchSize, 200)
	}

	// Defaulted values
	if cfg.ScheduleInterval != defaults.ScheduleInterval {
		t.Errorf("ScheduleInterval: got %q, want default %q", cfg.ScheduleInterval, defaults.ScheduleInterval)
	}
	if cfg.ClaudeDataDir != defaults.ClaudeDataDir {
		t.Errorf("ClaudeDataDir: got %q, want default %q", cfg.ClaudeDataDir, defaults.ClaudeDataDir)
	}
	if cfg.Auth.Provider != defaults.Auth.Provider {
		t.Errorf("Auth.Provider: got %q, want default %q", cfg.Auth.Provider, defaults.Auth.Provider)
	}
	if cfg.Redaction.ProjectPath != defaults.Redaction.ProjectPath {
		t.Errorf("Redaction.ProjectPath: got %q, want default %q", cfg.Redaction.ProjectPath, defaults.Redaction.ProjectPath)
	}
	if cfg.Redaction.FirstPrompt != defaults.Redaction.FirstPrompt {
		t.Errorf("Redaction.FirstPrompt: got %q, want default %q", cfg.Redaction.FirstPrompt, defaults.Redaction.FirstPrompt)
	}
	if cfg.Upload.RetryMax != defaults.Upload.RetryMax {
		t.Errorf("Upload.RetryMax: got %d, want default %d", cfg.Upload.RetryMax, defaults.Upload.RetryMax)
	}
	if cfg.Upload.RetryBackoff != defaults.Upload.RetryBackoff {
		t.Errorf("Upload.RetryBackoff: got %q, want default %q", cfg.Upload.RetryBackoff, defaults.Upload.RetryBackoff)
	}
	if cfg.Upload.Timeout != defaults.Upload.Timeout {
		t.Errorf("Upload.Timeout: got %q, want default %q", cfg.Upload.Timeout, defaults.Upload.Timeout)
	}
	if cfg.Sync.Branch != defaults.Sync.Branch {
		t.Errorf("Sync.Branch: got %q, want default %q", cfg.Sync.Branch, defaults.Sync.Branch)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	content := `
endpoint: [invalid yaml
  this is not valid:
    - [
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := Default()
	if cfg.ScheduleInterval != defaults.ScheduleInterval {
		t.Errorf("ScheduleInterval: got %q, want default %q", cfg.ScheduleInterval, defaults.ScheduleInterval)
	}
	if cfg.Auth.Provider != defaults.Auth.Provider {
		t.Errorf("Auth.Provider: got %q, want default %q", cfg.Auth.Provider, defaults.Auth.Provider)
	}
	if cfg.Upload.BatchSize != defaults.Upload.BatchSize {
		t.Errorf("Upload.BatchSize: got %d, want default %d", cfg.Upload.BatchSize, defaults.Upload.BatchSize)
	}
	if cfg.Redaction.ProjectPath != defaults.Redaction.ProjectPath {
		t.Errorf("Redaction.ProjectPath: got %q, want default %q", cfg.Redaction.ProjectPath, defaults.Redaction.ProjectPath)
	}
	if cfg.Sync.Branch != defaults.Sync.Branch {
		t.Errorf("Sync.Branch: got %q, want default %q", cfg.Sync.Branch, defaults.Sync.Branch)
	}
}

func TestApplyDefaults(t *testing.T) {
	// Start with a completely zero-value Config and verify applyDefaults fills everything.
	cfg := &Config{}
	applyDefaults(cfg)

	defaults := Default()

	if cfg.ScheduleInterval != defaults.ScheduleInterval {
		t.Errorf("ScheduleInterval: got %q, want %q", cfg.ScheduleInterval, defaults.ScheduleInterval)
	}
	if cfg.ClaudeDataDir != defaults.ClaudeDataDir {
		t.Errorf("ClaudeDataDir: got %q, want %q", cfg.ClaudeDataDir, defaults.ClaudeDataDir)
	}
	if cfg.Auth.Provider != defaults.Auth.Provider {
		t.Errorf("Auth.Provider: got %q, want %q", cfg.Auth.Provider, defaults.Auth.Provider)
	}
	if cfg.Redaction.ProjectPath != defaults.Redaction.ProjectPath {
		t.Errorf("Redaction.ProjectPath: got %q, want %q", cfg.Redaction.ProjectPath, defaults.Redaction.ProjectPath)
	}
	if cfg.Redaction.FirstPrompt != defaults.Redaction.FirstPrompt {
		t.Errorf("Redaction.FirstPrompt: got %q, want %q", cfg.Redaction.FirstPrompt, defaults.Redaction.FirstPrompt)
	}
	if cfg.Redaction.BriefSummary != defaults.Redaction.BriefSummary {
		t.Errorf("Redaction.BriefSummary: got %q, want %q", cfg.Redaction.BriefSummary, defaults.Redaction.BriefSummary)
	}
	if cfg.Redaction.UnderlyingGoal != defaults.Redaction.UnderlyingGoal {
		t.Errorf("Redaction.UnderlyingGoal: got %q, want %q", cfg.Redaction.UnderlyingGoal, defaults.Redaction.UnderlyingGoal)
	}
	if cfg.Redaction.FrictionDetail != defaults.Redaction.FrictionDetail {
		t.Errorf("Redaction.FrictionDetail: got %q, want %q", cfg.Redaction.FrictionDetail, defaults.Redaction.FrictionDetail)
	}
	if cfg.Upload.BatchSize != defaults.Upload.BatchSize {
		t.Errorf("Upload.BatchSize: got %d, want %d", cfg.Upload.BatchSize, defaults.Upload.BatchSize)
	}
	if cfg.Upload.RetryMax != defaults.Upload.RetryMax {
		t.Errorf("Upload.RetryMax: got %d, want %d", cfg.Upload.RetryMax, defaults.Upload.RetryMax)
	}
	if cfg.Upload.RetryBackoff != defaults.Upload.RetryBackoff {
		t.Errorf("Upload.RetryBackoff: got %q, want %q", cfg.Upload.RetryBackoff, defaults.Upload.RetryBackoff)
	}
	if cfg.Upload.Timeout != defaults.Upload.Timeout {
		t.Errorf("Upload.Timeout: got %q, want %q", cfg.Upload.Timeout, defaults.Upload.Timeout)
	}
	if cfg.Sync.Branch != defaults.Sync.Branch {
		t.Errorf("Sync.Branch: got %q, want %q", cfg.Sync.Branch, defaults.Sync.Branch)
	}
}

func TestApplyDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &Config{
		ScheduleInterval: "1h",
		ClaudeDataDir:    "/my/dir",
		Auth:             AuthConfig{Provider: "apikey"},
		Redaction: RedactionConfig{
			ProjectPath:    "keep",
			FirstPrompt:    "keep",
			BriefSummary:   "keep",
			UnderlyingGoal: "keep",
			FrictionDetail: "keep",
		},
		Upload: UploadConfig{
			BatchSize:    999,
			RetryMax:     10,
			RetryBackoff: "5m",
			Timeout:      "2m",
		},
		Sync: SyncConfig{
			Branch: "release",
		},
	}

	applyDefaults(cfg)

	if cfg.ScheduleInterval != "1h" {
		t.Errorf("ScheduleInterval: got %q, want %q", cfg.ScheduleInterval, "1h")
	}
	if cfg.ClaudeDataDir != "/my/dir" {
		t.Errorf("ClaudeDataDir: got %q, want %q", cfg.ClaudeDataDir, "/my/dir")
	}
	if cfg.Auth.Provider != "apikey" {
		t.Errorf("Auth.Provider: got %q, want %q", cfg.Auth.Provider, "apikey")
	}
	if cfg.Redaction.ProjectPath != "keep" {
		t.Errorf("Redaction.ProjectPath: got %q, want %q", cfg.Redaction.ProjectPath, "keep")
	}
	if cfg.Upload.BatchSize != 999 {
		t.Errorf("Upload.BatchSize: got %d, want %d", cfg.Upload.BatchSize, 999)
	}
	if cfg.Upload.RetryMax != 10 {
		t.Errorf("Upload.RetryMax: got %d, want %d", cfg.Upload.RetryMax, 10)
	}
	if cfg.Upload.RetryBackoff != "5m" {
		t.Errorf("Upload.RetryBackoff: got %q, want %q", cfg.Upload.RetryBackoff, "5m")
	}
	if cfg.Upload.Timeout != "2m" {
		t.Errorf("Upload.Timeout: got %q, want %q", cfg.Upload.Timeout, "2m")
	}
	if cfg.Sync.Branch != "release" {
		t.Errorf("Sync.Branch: got %q, want %q", cfg.Sync.Branch, "release")
	}
}

func TestConfigDir(t *testing.T) {
	t.Run("default uses home directory", func(t *testing.T) {
		// Clear the env var to ensure we get the default behaviour.
		t.Setenv("CLASP_CONFIG_DIR", "")

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("unable to get home dir: %v", err)
		}

		want := filepath.Join(home, ".config", "clasp")
		got := ConfigDir()
		if got != want {
			t.Errorf("ConfigDir(): got %q, want %q", got, want)
		}
	})

	t.Run("CLASP_CONFIG_DIR override", func(t *testing.T) {
		customDir := "/custom/config/dir"
		t.Setenv("CLASP_CONFIG_DIR", customDir)

		got := ConfigDir()
		if got != customDir {
			t.Errorf("ConfigDir(): got %q, want %q", got, customDir)
		}
	})

	t.Run("CLASP_CONFIG_DIR with tilde expansion", func(t *testing.T) {
		t.Setenv("CLASP_CONFIG_DIR", "~/my-clasp-config")

		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatalf("unable to get home dir: %v", err)
		}

		want := filepath.Join(home, "my-clasp-config")
		got := ConfigDir()
		if got != want {
			t.Errorf("ConfigDir(): got %q, want %q", got, want)
		}
	})
}

func TestConfigPath(t *testing.T) {
	t.Setenv("CLASP_CONFIG_DIR", "/test/config")

	want := filepath.Join("/test/config", "config.yaml")
	got := ConfigPath()
	if got != want {
		t.Errorf("ConfigPath(): got %q, want %q", got, want)
	}
}

func TestLoad_UnreadableFile(t *testing.T) {
	// Test that a permission error (not ENOENT) returns an error.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("endpoint: test"), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0644) })

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}
}
