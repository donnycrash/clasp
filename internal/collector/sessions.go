package collector

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadSessions reads all JSON files from the usage-data/session-meta/
// directory under claudeDir and returns the parsed session metadata.
func LoadSessions(claudeDir string) ([]SessionMeta, error) {
	dir := filepath.Join(claudeDir, "usage-data", "session-meta")

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("session-meta directory does not exist", "path", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("reading session-meta directory %s: %w", dir, err)
	}

	var sessions []SessionMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("skipping unreadable session file", "path", path, "error", err)
			continue
		}

		var meta SessionMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			slog.Warn("skipping malformed session file", "path", path, "error", err)
			continue
		}

		sessions = append(sessions, meta)
	}

	slog.Debug("loaded sessions", "count", len(sessions), "directory", dir)
	return sessions, nil
}
