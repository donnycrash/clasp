package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeDataDir_Default(t *testing.T) {
	cfg := &Config{ClaudeDataDir: "~/.claude"}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unable to get home dir: %v", err)
	}

	want := filepath.Join(home, ".claude")
	got := ClaudeDataDir(cfg)
	if got != want {
		t.Errorf("ClaudeDataDir(): got %q, want %q", got, want)
	}
}

func TestClaudeDataDir_Custom(t *testing.T) {
	cfg := &Config{ClaudeDataDir: "/custom/path/claude"}

	got := ClaudeDataDir(cfg)
	if got != "/custom/path/claude" {
		t.Errorf("ClaudeDataDir(): got %q, want %q", got, "/custom/path/claude")
	}
}

func TestClaudeDataDir_Empty(t *testing.T) {
	// When ClaudeDataDir is empty, it should fall back to ~/.claude.
	cfg := &Config{ClaudeDataDir: ""}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unable to get home dir: %v", err)
	}

	want := filepath.Join(home, ".claude")
	got := ClaudeDataDir(cfg)
	if got != want {
		t.Errorf("ClaudeDataDir(): got %q, want %q", got, want)
	}
}

func TestClaudeDataDir_TildeExpansion(t *testing.T) {
	cfg := &Config{ClaudeDataDir: "~/my-claude-data"}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unable to get home dir: %v", err)
	}

	want := filepath.Join(home, "my-claude-data")
	got := ClaudeDataDir(cfg)
	if got != want {
		t.Errorf("ClaudeDataDir(): got %q, want %q", got, want)
	}
}

func TestWatermarkPath(t *testing.T) {
	t.Setenv("CLASP_CONFIG_DIR", "/test/clasp")

	want := filepath.Join("/test/clasp", "watermark.json")
	got := WatermarkPath()
	if got != want {
		t.Errorf("WatermarkPath(): got %q, want %q", got, want)
	}
}

func TestWatermarkPath_Default(t *testing.T) {
	t.Setenv("CLASP_CONFIG_DIR", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unable to get home dir: %v", err)
	}

	want := filepath.Join(home, ".config", "clasp", "watermark.json")
	got := WatermarkPath()
	if got != want {
		t.Errorf("WatermarkPath(): got %q, want %q", got, want)
	}
}

func TestLogPath(t *testing.T) {
	t.Setenv("CLASP_CONFIG_DIR", "/test/clasp")

	want := filepath.Join("/test/clasp", "upload.log")
	got := LogPath()
	if got != want {
		t.Errorf("LogPath(): got %q, want %q", got, want)
	}
}

func TestLogPath_Default(t *testing.T) {
	t.Setenv("CLASP_CONFIG_DIR", "")

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unable to get home dir: %v", err)
	}

	want := filepath.Join(home, ".config", "clasp", "upload.log")
	got := LogPath()
	if got != want {
		t.Errorf("LogPath(): got %q, want %q", got, want)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unable to get home dir: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tilde only",
			input: "~",
			want:  home,
		},
		{
			name:  "tilde with subpath",
			input: "~/Documents/data",
			want:  filepath.Join(home, "Documents/data"),
		},
		{
			name:  "absolute path unchanged",
			input: "/usr/local/bin",
			want:  "/usr/local/bin",
		},
		{
			name:  "relative path unchanged",
			input: "relative/path",
			want:  "relative/path",
		},
		{
			name:  "empty string unchanged",
			input: "",
			want:  "",
		},
		{
			name:  "tilde in middle not expanded",
			input: "/some/~/path",
			want:  "/some/~/path",
		},
		{
			name:  "tilde with slash only",
			input: "~/",
			want:  home,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandHome(tt.input)
			if got != tt.want {
				t.Errorf("expandHome(%q): got %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
