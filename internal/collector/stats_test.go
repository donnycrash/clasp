package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadStats_Valid(t *testing.T) {
	dir := t.TempDir()
	statsJSON := `{
		"dailyActivity": [
			{
				"date": "2026-03-20",
				"messageCount": 42,
				"sessionCount": 5,
				"toolCallCount": 120
			},
			{
				"date": "2026-03-21",
				"messageCount": 18,
				"sessionCount": 3,
				"toolCallCount": 55
			}
		],
		"dailyModelTokens": [
			{
				"date": "2026-03-20",
				"tokensByModel": {
					"claude-sonnet-4-20250514": 150000,
					"claude-haiku-4-20250414": 30000
				}
			},
			{
				"date": "2026-03-21",
				"tokensByModel": {
					"claude-sonnet-4-20250514": 80000
				}
			}
		],
		"modelUsage": {
			"claude-sonnet-4-20250514": {
				"inputTokens": 200000,
				"outputTokens": 50000,
				"cacheReadInputTokens": 100000,
				"cacheCreationInputTokens": 25000,
				"costUsd": 1.85
			},
			"claude-haiku-4-20250414": {
				"inputTokens": 30000,
				"outputTokens": 5000,
				"cacheReadInputTokens": 10000,
				"cacheCreationInputTokens": 2000,
				"costUsd": 0.12
			}
		},
		"totalSessions": 8,
		"totalMessages": 60,
		"hourCounts": {
			"9": 15,
			"10": 22,
			"14": 18,
			"15": 5
		},
		"longestSession": {
			"sessionId": "sess-longest-001",
			"duration": 346584386,
			"messageCount": 653,
			"timestamp": "2026-03-19T14:46:53.659Z"
		},
		"totalSpeculationTimeSavedMs": 12345
	}`
	if err := os.WriteFile(filepath.Join(dir, "stats-cache.json"), []byte(statsJSON), 0o644); err != nil {
		t.Fatalf("writing stats file: %v", err)
	}

	stats, err := LoadStats(dir)
	if err != nil {
		t.Fatalf("LoadStats returned error: %v", err)
	}

	// DailyActivity
	if len(stats.DailyActivity) != 2 {
		t.Fatalf("expected 2 daily activity entries, got %d", len(stats.DailyActivity))
	}
	da := stats.DailyActivity[0]
	if da.Date != "2026-03-20" {
		t.Errorf("expected date 2026-03-20, got %s", da.Date)
	}
	if da.MessageCount != 42 {
		t.Errorf("expected messageCount 42, got %d", da.MessageCount)
	}
	if da.SessionCount != 5 {
		t.Errorf("expected sessionCount 5, got %d", da.SessionCount)
	}
	if da.ToolCallCount != 120 {
		t.Errorf("expected toolCallCount 120, got %d", da.ToolCallCount)
	}

	// DailyModelTokens
	if len(stats.DailyModelTokens) != 2 {
		t.Fatalf("expected 2 daily model token entries, got %d", len(stats.DailyModelTokens))
	}
	dt := stats.DailyModelTokens[0]
	if dt.Date != "2026-03-20" {
		t.Errorf("expected date 2026-03-20, got %s", dt.Date)
	}
	if dt.TokensByModel["claude-sonnet-4-20250514"] != 150000 {
		t.Errorf("expected 150000 tokens for sonnet, got %d", dt.TokensByModel["claude-sonnet-4-20250514"])
	}
	if dt.TokensByModel["claude-haiku-4-20250414"] != 30000 {
		t.Errorf("expected 30000 tokens for haiku, got %d", dt.TokensByModel["claude-haiku-4-20250414"])
	}

	// ModelUsage
	if len(stats.ModelUsage) != 2 {
		t.Fatalf("expected 2 model usage entries, got %d", len(stats.ModelUsage))
	}
	sonnet, ok := stats.ModelUsage["claude-sonnet-4-20250514"]
	if !ok {
		t.Fatal("missing model usage for claude-sonnet-4-20250514")
	}
	if sonnet.InputTokens != 200000 {
		t.Errorf("expected inputTokens 200000, got %d", sonnet.InputTokens)
	}
	if sonnet.OutputTokens != 50000 {
		t.Errorf("expected outputTokens 50000, got %d", sonnet.OutputTokens)
	}
	if sonnet.CacheReadInputTokens != 100000 {
		t.Errorf("expected cacheReadInputTokens 100000, got %d", sonnet.CacheReadInputTokens)
	}
	if sonnet.CacheCreationInputTokens != 25000 {
		t.Errorf("expected cacheCreationInputTokens 25000, got %d", sonnet.CacheCreationInputTokens)
	}
	if sonnet.CostUSD != 1.85 {
		t.Errorf("expected costUsd 1.85, got %f", sonnet.CostUSD)
	}

	// Top-level aggregates
	if stats.TotalSessions != 8 {
		t.Errorf("expected totalSessions 8, got %d", stats.TotalSessions)
	}
	if stats.TotalMessages != 60 {
		t.Errorf("expected totalMessages 60, got %d", stats.TotalMessages)
	}
	if stats.HourCounts["10"] != 22 {
		t.Errorf("expected hourCounts[10]=22, got %d", stats.HourCounts["10"])
	}

	// LongestSession
	if stats.LongestSession == nil {
		t.Fatal("expected longestSession to be non-nil")
	}
	if stats.LongestSession.SessionID != "sess-longest-001" {
		t.Errorf("expected longestSession.sessionId sess-longest-001, got %s", stats.LongestSession.SessionID)
	}
	if stats.LongestSession.Duration != 346584386 {
		t.Errorf("expected longestSession.duration 346584386, got %d", stats.LongestSession.Duration)
	}
	if stats.LongestSession.MessageCount != 653 {
		t.Errorf("expected longestSession.messageCount 653, got %d", stats.LongestSession.MessageCount)
	}
	if stats.LongestSession.Timestamp != "2026-03-19T14:46:53.659Z" {
		t.Errorf("expected longestSession.timestamp 2026-03-19T14:46:53.659Z, got %s", stats.LongestSession.Timestamp)
	}

	// TotalSpeculationTimeSavedMs
	if stats.TotalSpeculationTimeSavedMs != 12345 {
		t.Errorf("expected totalSpeculationTimeSavedMs 12345, got %d", stats.TotalSpeculationTimeSavedMs)
	}
}

func TestLoadStats_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadStats(dir)
	if err == nil {
		t.Fatal("expected error for missing stats file, got nil")
	}
}

func TestLoadStats_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stats-cache.json"), []byte(`{not valid json`), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	_, err := LoadStats(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadStats_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "stats-cache.json"), []byte(``), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}
	_, err := LoadStats(dir)
	// An empty file is not valid JSON, so we expect an error.
	if err == nil {
		t.Fatal("expected error for empty file, got nil")
	}
}
