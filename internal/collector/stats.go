package collector

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// StatsCache represents the parsed contents of stats-cache.json produced by
// Claude Code.
type StatsCache struct {
	DailyActivity    []DailyActivity      `json:"dailyActivity"`
	DailyModelTokens []DailyModelTokens   `json:"dailyModelTokens"`
	ModelUsage       map[string]ModelUsage `json:"modelUsage"`
	TotalSessions    int                  `json:"totalSessions"`
	TotalMessages    int                  `json:"totalMessages"`
	HourCounts       map[string]int       `json:"hourCounts"`
}

// LoadStats reads and parses the stats-cache.json file from the given Claude
// data directory.
func LoadStats(claudeDir string) (*StatsCache, error) {
	path := filepath.Join(claudeDir, "stats-cache.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading stats cache %s: %w", path, err)
	}

	var stats StatsCache
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("parsing stats cache %s: %w", path, err)
	}

	slog.Debug("loaded stats cache",
		"path", path,
		"daily_activity_entries", len(stats.DailyActivity),
		"models", len(stats.ModelUsage),
	)
	return &stats, nil
}
