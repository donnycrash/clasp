package collector

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donnycrash/clasp/internal/watermark"
)

// setupClaudeDir creates a full temp Claude directory structure with stats,
// sessions, and facets, returning the base dir path.
func setupClaudeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// --- stats-cache.json ---
	statsJSON := `{
		"dailyActivity": [
			{"date": "2026-03-18", "messageCount": 10, "sessionCount": 2, "toolCallCount": 30},
			{"date": "2026-03-19", "messageCount": 20, "sessionCount": 3, "toolCallCount": 60},
			{"date": "2026-03-20", "messageCount": 35, "sessionCount": 4, "toolCallCount": 95},
			{"date": "2026-03-21", "messageCount": 15, "sessionCount": 2, "toolCallCount": 40}
		],
		"dailyModelTokens": [
			{"date": "2026-03-18", "tokensByModel": {"claude-sonnet-4-20250514": 50000}},
			{"date": "2026-03-19", "tokensByModel": {"claude-sonnet-4-20250514": 90000}},
			{"date": "2026-03-20", "tokensByModel": {"claude-sonnet-4-20250514": 130000}},
			{"date": "2026-03-21", "tokensByModel": {"claude-sonnet-4-20250514": 60000}}
		],
		"modelUsage": {
			"claude-sonnet-4-20250514": {
				"inputTokens": 300000,
				"outputTokens": 80000,
				"cacheReadInputTokens": 120000,
				"cacheCreationInputTokens": 30000,
				"costUsd": 2.50
			}
		},
		"totalSessions": 11,
		"totalMessages": 80,
		"hourCounts": {"9": 20, "14": 30, "16": 30},
		"longestSession": {
			"sessionId": "sess-new-002",
			"duration": 3600000,
			"messageCount": 33,
			"timestamp": "2026-03-20T14:00:00Z"
		},
		"totalSpeculationTimeSavedMs": 5000
	}`
	if err := os.WriteFile(filepath.Join(dir, "stats-cache.json"), []byte(statsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// --- session-meta files ---
	sessionDir := filepath.Join(dir, "usage-data", "session-meta")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sessions := map[string]string{
		"s1.json": `{
			"session_id": "sess-old-001",
			"project_path": "/home/user/proj",
			"start_time": "2026-03-18T10:00:00Z",
			"duration_minutes": 30,
			"user_message_count": 5,
			"assistant_message_count": 6,
			"input_tokens": 25000,
			"output_tokens": 8000,
			"first_prompt": "Old session prompt"
		}`,
		"s2.json": `{
			"session_id": "sess-new-002",
			"project_path": "/home/user/proj",
			"start_time": "2026-03-20T14:00:00Z",
			"duration_minutes": 60,
			"user_message_count": 15,
			"assistant_message_count": 18,
			"tool_counts": {"Read": 5, "Edit": 3},
			"languages": {"go": 10},
			"input_tokens": 90000,
			"output_tokens": 25000,
			"first_prompt": "New session two"
		}`,
		"s3.json": `{
			"session_id": "sess-new-003",
			"project_path": "/home/user/other",
			"start_time": "2026-03-21T09:00:00Z",
			"duration_minutes": 20,
			"user_message_count": 8,
			"assistant_message_count": 9,
			"input_tokens": 40000,
			"output_tokens": 12000,
			"first_prompt": "New session three"
		}`,
	}
	for name, content := range sessions {
		if err := os.WriteFile(filepath.Join(sessionDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// --- facets files ---
	facetsDir := filepath.Join(dir, "usage-data", "facets")
	if err := os.MkdirAll(facetsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	facets := map[string]string{
		"f-old.json": `{
			"session_id": "sess-old-001",
			"outcome": "success",
			"claude_helpfulness": "helpful",
			"session_type": "coding",
			"primary_success": "resolved",
			"brief_summary": "Old session summary"
		}`,
		"f-new2.json": `{
			"session_id": "sess-new-002",
			"underlying_goal": "Implement feature X",
			"outcome": "success",
			"claude_helpfulness": "very_helpful",
			"session_type": "feature_development",
			"primary_success": "resolved",
			"brief_summary": "Implemented feature X with full test coverage."
		}`,
	}
	for name, content := range facets {
		if err := os.WriteFile(filepath.Join(facetsDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func TestCollect_FullPipeline(t *testing.T) {
	dir := setupClaudeDir(t)

	// Watermark: sess-old-001 already uploaded, stats through 2026-03-19.
	wm := &watermark.Watermark{
		StatsCacheUploadedThrough: "2026-03-19",
		SessionIDsUploaded: map[string]string{
			"sess-old-001": "2026-03-19T12:00:00Z",
		},
	}

	data, err := Collect(dir, wm)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	// Stats: only dates after 2026-03-19 should be included (2026-03-20, 2026-03-21).
	if data.Stats == nil {
		t.Fatal("Stats is nil")
	}
	if len(data.Stats.DailyActivity) != 2 {
		t.Fatalf("expected 2 filtered daily activity entries, got %d", len(data.Stats.DailyActivity))
	}
	if data.Stats.DailyActivity[0].Date != "2026-03-20" {
		t.Errorf("first filtered date: got %q, want 2026-03-20", data.Stats.DailyActivity[0].Date)
	}
	if data.Stats.DailyActivity[1].Date != "2026-03-21" {
		t.Errorf("second filtered date: got %q, want 2026-03-21", data.Stats.DailyActivity[1].Date)
	}
	if data.Stats.PeriodStart != "2026-03-20" {
		t.Errorf("PeriodStart: got %q, want 2026-03-20", data.Stats.PeriodStart)
	}
	if data.Stats.PeriodEnd != "2026-03-21" {
		t.Errorf("PeriodEnd: got %q, want 2026-03-21", data.Stats.PeriodEnd)
	}

	// DailyModelTokens should also be filtered.
	if len(data.Stats.DailyModelTokens) != 2 {
		t.Fatalf("expected 2 filtered daily model token entries, got %d", len(data.Stats.DailyModelTokens))
	}

	// ModelUsage is always included in full.
	if len(data.Stats.ModelUsage) != 1 {
		t.Fatalf("expected 1 model usage entry, got %d", len(data.Stats.ModelUsage))
	}

	// HourCounts should be passed through.
	if len(data.Stats.HourCounts) != 3 {
		t.Fatalf("expected 3 hour count entries, got %d", len(data.Stats.HourCounts))
	}
	if data.Stats.HourCounts["14"] != 30 {
		t.Errorf("expected hourCounts[14]=30, got %d", data.Stats.HourCounts["14"])
	}

	// LongestSession should be passed through.
	if data.Stats.LongestSession == nil {
		t.Fatal("expected LongestSession to be non-nil")
	}
	if data.Stats.LongestSession.SessionID != "sess-new-002" {
		t.Errorf("expected LongestSession.SessionID sess-new-002, got %s", data.Stats.LongestSession.SessionID)
	}

	// TotalSpeculationTimeSavedMs should be passed through.
	if data.Stats.TotalSpeculationTimeSavedMs != 5000 {
		t.Errorf("expected TotalSpeculationTimeSavedMs 5000, got %d", data.Stats.TotalSpeculationTimeSavedMs)
	}

	// Sessions: sess-old-001 should be filtered out, leaving sess-new-002 and sess-new-003.
	if len(data.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(data.Sessions))
	}

	byID := make(map[string]JoinedSession)
	for _, js := range data.Sessions {
		byID[js.SessionID] = js
	}

	if _, ok := byID["sess-old-001"]; ok {
		t.Error("sess-old-001 should have been filtered by watermark")
	}

	js2, ok := byID["sess-new-002"]
	if !ok {
		t.Fatal("missing sess-new-002")
	}
	// sess-new-002 has a matching facet.
	if js2.Facets == nil {
		t.Fatal("sess-new-002 should have facets joined")
	}
	if js2.Facets.BriefSummary != "Implemented feature X with full test coverage." {
		t.Errorf("Facets.BriefSummary: got %q", js2.Facets.BriefSummary)
	}

	js3, ok := byID["sess-new-003"]
	if !ok {
		t.Fatal("missing sess-new-003")
	}
	// sess-new-003 has no matching facet.
	if js3.Facets != nil {
		t.Error("sess-new-003 should have nil Facets (no matching facet file)")
	}
}

func TestCollect_NoWatermark(t *testing.T) {
	dir := setupClaudeDir(t)

	// Empty watermark: nothing uploaded yet.
	wm := &watermark.Watermark{
		SessionIDsUploaded: make(map[string]string),
	}

	data, err := Collect(dir, wm)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	// All 4 daily activity entries should be present.
	if len(data.Stats.DailyActivity) != 4 {
		t.Fatalf("expected 4 daily activity entries, got %d", len(data.Stats.DailyActivity))
	}

	// All 3 sessions should be present.
	if len(data.Sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(data.Sessions))
	}
}

func TestCollect_AllAlreadyUploaded(t *testing.T) {
	dir := setupClaudeDir(t)

	// All sessions already uploaded, stats through the latest date.
	wm := &watermark.Watermark{
		StatsCacheUploadedThrough: "2026-03-21",
		SessionIDsUploaded: map[string]string{
			"sess-old-001": "2026-03-22T00:00:00Z",
			"sess-new-002": "2026-03-22T00:00:00Z",
			"sess-new-003": "2026-03-22T00:00:00Z",
		},
	}

	data, err := Collect(dir, wm)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	// No daily entries should pass the filter.
	if len(data.Stats.DailyActivity) != 0 {
		t.Fatalf("expected 0 daily activity entries, got %d", len(data.Stats.DailyActivity))
	}
	if len(data.Stats.DailyModelTokens) != 0 {
		t.Fatalf("expected 0 daily model token entries, got %d", len(data.Stats.DailyModelTokens))
	}

	// Period should be empty when no activity passes the filter.
	if data.Stats.PeriodStart != "" {
		t.Errorf("PeriodStart should be empty, got %q", data.Stats.PeriodStart)
	}
	if data.Stats.PeriodEnd != "" {
		t.Errorf("PeriodEnd should be empty, got %q", data.Stats.PeriodEnd)
	}

	// No sessions should be returned.
	if len(data.Sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(data.Sessions))
	}
}

func TestCollect_StatsFiltering(t *testing.T) {
	dir := setupClaudeDir(t)

	// Watermark date exactly on 2026-03-20: only 2026-03-21 should pass (strictly after).
	wm := &watermark.Watermark{
		StatsCacheUploadedThrough: "2026-03-20",
		SessionIDsUploaded:        make(map[string]string),
	}

	data, err := Collect(dir, wm)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	if len(data.Stats.DailyActivity) != 1 {
		t.Fatalf("expected 1 daily activity entry after 2026-03-20, got %d", len(data.Stats.DailyActivity))
	}
	if data.Stats.DailyActivity[0].Date != "2026-03-21" {
		t.Errorf("expected date 2026-03-21, got %q", data.Stats.DailyActivity[0].Date)
	}
	if data.Stats.DailyActivity[0].MessageCount != 15 {
		t.Errorf("expected messageCount 15, got %d", data.Stats.DailyActivity[0].MessageCount)
	}

	if len(data.Stats.DailyModelTokens) != 1 {
		t.Fatalf("expected 1 daily model token entry after 2026-03-20, got %d", len(data.Stats.DailyModelTokens))
	}
	if data.Stats.DailyModelTokens[0].Date != "2026-03-21" {
		t.Errorf("expected token date 2026-03-21, got %q", data.Stats.DailyModelTokens[0].Date)
	}

	// PeriodStart and PeriodEnd should both be 2026-03-21.
	if data.Stats.PeriodStart != "2026-03-21" {
		t.Errorf("PeriodStart: got %q, want 2026-03-21", data.Stats.PeriodStart)
	}
	if data.Stats.PeriodEnd != "2026-03-21" {
		t.Errorf("PeriodEnd: got %q, want 2026-03-21", data.Stats.PeriodEnd)
	}

	// ModelUsage is always returned in full regardless of watermark.
	if _, ok := data.Stats.ModelUsage["claude-sonnet-4-20250514"]; !ok {
		t.Error("expected model usage to always be present")
	}
}

func TestCollect_SessionFacetJoin(t *testing.T) {
	dir := setupClaudeDir(t)

	// No watermark filtering: all sessions returned.
	wm := &watermark.Watermark{
		SessionIDsUploaded: make(map[string]string),
	}

	data, err := Collect(dir, wm)
	if err != nil {
		t.Fatalf("Collect returned error: %v", err)
	}

	byID := make(map[string]JoinedSession)
	for _, js := range data.Sessions {
		byID[js.SessionID] = js
	}

	// sess-old-001 has a facet file.
	js1, ok := byID["sess-old-001"]
	if !ok {
		t.Fatal("missing sess-old-001")
	}
	if js1.Facets == nil {
		t.Error("sess-old-001 should have facets (facet file exists)")
	} else if js1.Facets.SessionID != "sess-old-001" {
		t.Errorf("Facets.SessionID: got %q, want sess-old-001", js1.Facets.SessionID)
	}

	// sess-new-002 has a facet file.
	js2, ok := byID["sess-new-002"]
	if !ok {
		t.Fatal("missing sess-new-002")
	}
	if js2.Facets == nil {
		t.Error("sess-new-002 should have facets (facet file exists)")
	} else {
		if js2.Facets.UnderlyingGoal != "Implement feature X" {
			t.Errorf("Facets.UnderlyingGoal: got %q", js2.Facets.UnderlyingGoal)
		}
		if js2.Facets.SessionType != "feature_development" {
			t.Errorf("Facets.SessionType: got %q", js2.Facets.SessionType)
		}
	}

	// sess-new-003 does NOT have a facet file.
	js3, ok := byID["sess-new-003"]
	if !ok {
		t.Fatal("missing sess-new-003")
	}
	if js3.Facets != nil {
		t.Error("sess-new-003 should have nil Facets (no matching facet file)")
	}

	// Verify session metadata is preserved through the join.
	if js2.ProjectPath != "/home/user/proj" {
		t.Errorf("SessionMeta.ProjectPath not preserved: got %q", js2.ProjectPath)
	}
	if js2.DurationMinutes != 60 {
		t.Errorf("SessionMeta.DurationMinutes not preserved: got %d", js2.DurationMinutes)
	}
}
