package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/donnycrash/clasp/internal/config"
	"github.com/donnycrash/clasp/internal/uploader"
)

// setupTestClaudeDir creates a minimal Claude data directory with stats,
// sessions, and facets for testing the upload pipeline.
func setupTestClaudeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	statsJSON := `{
		"dailyActivity": [
			{"date": "2026-03-20", "messageCount": 35, "sessionCount": 4, "toolCallCount": 95}
		],
		"dailyModelTokens": [
			{"date": "2026-03-20", "tokensByModel": {"claude-sonnet-4-20250514": 130000}}
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
		"totalSessions": 4,
		"totalMessages": 35,
		"hourCounts": {"14": 35}
	}`
	if err := os.WriteFile(filepath.Join(dir, "stats-cache.json"), []byte(statsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	sessionDir := filepath.Join(dir, "usage-data", "session-meta")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionJSON := `{
		"session_id": "sess-dry-001",
		"project_path": "/home/user/myproject",
		"start_time": "2026-03-20T14:00:00Z",
		"duration_minutes": 45,
		"user_message_count": 12,
		"assistant_message_count": 14,
		"tool_counts": {"Read": 5, "Edit": 3},
		"languages": {"go": 8},
		"input_tokens": 90000,
		"output_tokens": 25000,
		"first_prompt": "Help me add dry-run mode",
		"lines_added": 50,
		"lines_removed": 10,
		"files_modified": 3
	}`
	if err := os.WriteFile(filepath.Join(sessionDir, "s1.json"), []byte(sessionJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	facetsDir := filepath.Join(dir, "usage-data", "facets")
	if err := os.MkdirAll(facetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	facetJSON := `{
		"session_id": "sess-dry-001",
		"underlying_goal": "Add transparency feature",
		"outcome": "success",
		"claude_helpfulness": "very_helpful",
		"session_type": "feature_development",
		"primary_success": "resolved",
		"brief_summary": "Added dry-run mode to upload command."
	}`
	if err := os.WriteFile(filepath.Join(facetsDir, "f1.json"), []byte(facetJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestDoUpload_DryRun(t *testing.T) {
	claudeDir := setupTestClaudeDir(t)

	// Point config and watermark at temp dirs so we don't touch real state.
	configDir := t.TempDir()
	t.Setenv("CLASP_CONFIG_DIR", configDir)

	cfg := config.Default()
	cfg.ClaudeDataDir = claudeDir
	cfg.Endpoint = "http://localhost:9999" // unused in dry-run

	// Capture stdout to verify JSON output.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = doUpload(cfg, true)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("doUpload dry-run returned error: %v", err)
	}

	// Read captured output.
	var buf [64 * 1024]byte
	n, _ := r.Read(buf[:])
	output := buf[:n]

	// Parse as payload JSON.
	var payload uploader.Payload
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, string(output))
	}

	// Verify metadata.
	if payload.Metadata.ToolName != "clasp" {
		t.Errorf("ToolName = %q, want clasp", payload.Metadata.ToolName)
	}
	if payload.Metadata.GitHubUsername != "(dry-run)" {
		t.Errorf("GitHubUsername = %q, want (dry-run)", payload.Metadata.GitHubUsername)
	}

	// Verify session data is present and redacted.
	if len(payload.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(payload.Sessions))
	}
	sess := payload.Sessions[0]
	if sess.SessionID != "sess-dry-001" {
		t.Errorf("SessionID = %q, want sess-dry-001", sess.SessionID)
	}
	// FirstPrompt should be omitted by default redaction rules.
	if sess.FirstPrompt != "" {
		t.Errorf("FirstPrompt should be redacted (omitted), got %q", sess.FirstPrompt)
	}
	// ProjectPath should be hashed by default redaction rules.
	if sess.ProjectPath == "/home/user/myproject" {
		t.Error("ProjectPath should be hashed, but got the original value")
	}
	if sess.ProjectPath == "" {
		t.Error("ProjectPath should be hashed, not empty")
	}

	// Verify facets are present but sensitive fields redacted.
	if sess.Facets == nil {
		t.Fatal("Facets should be present")
	}
	if sess.Facets.Outcome != "success" {
		t.Errorf("Facets.Outcome = %q, want success", sess.Facets.Outcome)
	}
	// BriefSummary and UnderlyingGoal should be omitted by default.
	if sess.Facets.BriefSummary != "" {
		t.Errorf("Facets.BriefSummary should be redacted, got %q", sess.Facets.BriefSummary)
	}
	if sess.Facets.UnderlyingGoal != "" {
		t.Errorf("Facets.UnderlyingGoal should be redacted, got %q", sess.Facets.UnderlyingGoal)
	}

	// Verify stats are present.
	if payload.StatsSummary == nil {
		t.Fatal("StatsSummary should be present")
	}
	if len(payload.StatsSummary.DailyActivity) != 1 {
		t.Errorf("expected 1 daily activity entry, got %d", len(payload.StatsSummary.DailyActivity))
	}

	// Verify watermark was NOT updated (dry-run should not persist state).
	wmPath := filepath.Join(configDir, "watermark.json")
	if _, err := os.Stat(wmPath); err == nil {
		// If the file exists, it should be empty/default (no sessions marked).
		wmData, _ := os.ReadFile(wmPath)
		if len(wmData) > 0 {
			var wmMap map[string]interface{}
			json.Unmarshal(wmData, &wmMap)
			if ids, ok := wmMap["session_ids_uploaded"]; ok {
				if m, ok := ids.(map[string]interface{}); ok && len(m) > 0 {
					t.Error("watermark should not have recorded uploaded sessions in dry-run mode")
				}
			}
		}
	}
	// File not existing is also fine — means watermark was never written.
}

func TestDoUpload_DryRun_NothingNew(t *testing.T) {
	// Claude data dir with empty stats and no sessions.
	claudeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(claudeDir, "stats-cache.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	configDir := t.TempDir()
	t.Setenv("CLASP_CONFIG_DIR", configDir)

	cfg := config.Default()
	cfg.ClaudeDataDir = claudeDir
	cfg.Endpoint = "http://localhost:9999"

	// Capture stdout.
	oldStdout := os.Stdout
	_, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	err = doUpload(cfg, true)

	w.Close()
	os.Stdout = oldStdout

	// Should succeed with "nothing new" — not an error.
	if err != nil {
		t.Fatalf("doUpload dry-run with no data returned error: %v", err)
	}
}

func TestCountStatsDays(t *testing.T) {
	if got := countStatsDays(nil); got != 0 {
		t.Errorf("countStatsDays(nil) = %d, want 0", got)
	}

	stats := &uploader.PayloadStats{
		DailyActivity: []uploader.PayloadDailyActivity{
			{Date: "2026-03-20"},
			{Date: "2026-03-21"},
		},
	}
	if got := countStatsDays(stats); got != 2 {
		t.Errorf("countStatsDays = %d, want 2", got)
	}
}
