package collector

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// LoadFacets reads all JSON files from usage-data/facets/ under claudeDir and
// returns a map keyed by session_id.
func LoadFacets(claudeDir string) (map[string]Facets, error) {
	dir := filepath.Join(claudeDir, "usage-data", "facets")

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("facets directory does not exist", "path", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("reading facets directory %s: %w", dir, err)
	}

	result := make(map[string]Facets)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("skipping unreadable facets file", "path", path, "error", err)
			continue
		}

		var f Facets
		if err := json.Unmarshal(data, &f); err != nil {
			slog.Warn("skipping malformed facets file", "path", path, "error", err)
			continue
		}

		if f.SessionID == "" {
			slog.Warn("skipping facets file with empty session_id", "path", path)
			continue
		}

		result[f.SessionID] = f
	}

	slog.Debug("loaded facets", "count", len(result), "directory", dir)
	return result, nil
}
