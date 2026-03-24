package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeDataDir resolves the Claude data directory from the given Config.
// It defaults to ~/.claude and supports ~ expansion.
func ClaudeDataDir(cfg *Config) string {
	dir := cfg.ClaudeDataDir
	if dir == "" {
		dir = "~/.claude"
	}
	resolved := expandHome(dir)
	slog.Debug("resolved claude data directory", "raw", dir, "resolved", resolved)
	return resolved
}

// WatermarkPath returns the path to the watermark.json file inside the CLASP
// configuration directory.
func WatermarkPath() string {
	return filepath.Join(ConfigDir(), "watermark.json")
}

// LogPath returns the path to the upload.log file inside the CLASP
// configuration directory.
func LogPath() string {
	return filepath.Join(ConfigDir(), "upload.log")
}

// expandHome replaces a leading ~ with the current user's home directory.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Warn("unable to expand ~ in path", "path", path, "error", err)
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
