package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func makeSessionDir(t *testing.T, dir string) string {
	t.Helper()
	sessionDir := filepath.Join(dir, "usage-data", "session-meta")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("creating session-meta dir: %v", err)
	}
	return sessionDir
}

func TestLoadSessions_MultipleSessions(t *testing.T) {
	dir := t.TempDir()
	sessionDir := makeSessionDir(t, dir)

	files := map[string]string{
		"session-abc123.json": `{
			"session_id": "abc123",
			"project_path": "/home/user/project-alpha",
			"start_time": "2026-03-20T09:00:00Z",
			"duration_minutes": 45,
			"user_message_count": 12,
			"assistant_message_count": 14,
			"tool_counts": {"Read": 5, "Edit": 3, "Bash": 2},
			"languages": {"go": 8, "python": 2},
			"git_commits": 2,
			"git_pushes": 1,
			"input_tokens": 85000,
			"output_tokens": 22000,
			"first_prompt": "Fix the failing unit tests",
			"tool_errors": 1,
			"tool_error_categories": {"permission_denied": 1},
			"uses_task_agent": false,
			"uses_mcp": true,
			"uses_web_search": false,
			"uses_web_fetch": false,
			"lines_added": 120,
			"lines_removed": 30,
			"files_modified": 4,
			"message_hours": [9, 9, 9, 10, 10],
			"user_interruptions": 0
		}`,
		"session-def456.json": `{
			"session_id": "def456",
			"project_path": "/home/user/project-beta",
			"start_time": "2026-03-21T14:30:00Z",
			"duration_minutes": 120,
			"user_message_count": 30,
			"assistant_message_count": 35,
			"tool_counts": {"Read": 15, "Edit": 10, "Bash": 8, "Grep": 5},
			"languages": {"typescript": 20, "css": 5},
			"git_commits": 5,
			"git_pushes": 2,
			"input_tokens": 250000,
			"output_tokens": 60000,
			"first_prompt": "Refactor the auth module",
			"tool_errors": 3,
			"uses_task_agent": true,
			"uses_mcp": false,
			"uses_web_search": true,
			"uses_web_fetch": true,
			"lines_added": 450,
			"lines_removed": 200,
			"files_modified": 12,
			"user_interruptions": 2
		}`,
		"session-ghi789.json": `{
			"session_id": "ghi789",
			"project_path": "/home/user/project-gamma",
			"start_time": "2026-03-22T08:15:00Z",
			"duration_minutes": 10,
			"user_message_count": 3,
			"assistant_message_count": 4,
			"tool_counts": {"Read": 1},
			"languages": {"rust": 1},
			"git_commits": 0,
			"git_pushes": 0,
			"input_tokens": 15000,
			"output_tokens": 4000,
			"first_prompt": "Explain this error message",
			"tool_errors": 0,
			"uses_task_agent": false,
			"uses_mcp": false,
			"uses_web_search": false,
			"uses_web_fetch": false,
			"lines_added": 0,
			"lines_removed": 0,
			"files_modified": 0,
			"user_interruptions": 0
		}`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(sessionDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	sessions, err := LoadSessions(dir)
	if err != nil {
		t.Fatalf("LoadSessions returned error: %v", err)
	}

	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// Build a map for easier assertion (order is not guaranteed by os.ReadDir).
	byID := make(map[string]SessionMeta)
	for _, s := range sessions {
		byID[s.SessionID] = s
	}

	if _, ok := byID["abc123"]; !ok {
		t.Fatal("missing session abc123")
	}
	if _, ok := byID["def456"]; !ok {
		t.Fatal("missing session def456")
	}
	if _, ok := byID["ghi789"]; !ok {
		t.Fatal("missing session ghi789")
	}
}

func TestLoadSessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_ = makeSessionDir(t, dir)

	sessions, err := LoadSessions(dir)
	if err != nil {
		t.Fatalf("LoadSessions returned error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestLoadSessions_DirNotExist(t *testing.T) {
	dir := t.TempDir()
	// Do NOT create the session-meta directory.
	sessions, err := LoadSessions(dir)
	if err != nil {
		t.Fatalf("LoadSessions returned error for missing dir: %v", err)
	}
	if sessions != nil {
		t.Fatalf("expected nil sessions for missing dir, got %v", sessions)
	}
}

func TestLoadSessions_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	sessionDir := makeSessionDir(t, dir)

	// Write one valid and one invalid file.
	validJSON := `{"session_id": "valid1", "project_path": "/tmp", "start_time": "2026-03-20T09:00:00Z", "duration_minutes": 5, "user_message_count": 1, "assistant_message_count": 1, "input_tokens": 1000, "output_tokens": 500, "first_prompt": "hello"}`
	invalidJSON := `{this is not valid json`

	if err := os.WriteFile(filepath.Join(sessionDir, "valid.json"), []byte(validJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "invalid.json"), []byte(invalidJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions, err := LoadSessions(dir)
	if err != nil {
		t.Fatalf("LoadSessions returned error: %v", err)
	}
	// The invalid file should be skipped; only the valid one should load.
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (invalid skipped), got %d", len(sessions))
	}
	if sessions[0].SessionID != "valid1" {
		t.Errorf("expected session_id valid1, got %s", sessions[0].SessionID)
	}
}

func TestLoadSessions_FieldMapping(t *testing.T) {
	dir := t.TempDir()
	sessionDir := makeSessionDir(t, dir)

	sessionJSON := `{
		"session_id": "fieldmap-001",
		"project_path": "/opt/myrepo",
		"start_time": "2026-03-23T16:45:00Z",
		"duration_minutes": 90,
		"user_message_count": 25,
		"assistant_message_count": 28,
		"tool_counts": {"Read": 10, "Edit": 7, "Bash": 4, "Grep": 3},
		"languages": {"go": 12, "sql": 3, "yaml": 1},
		"git_commits": 4,
		"git_pushes": 1,
		"input_tokens": 180000,
		"output_tokens": 45000,
		"first_prompt": "Add database migration support",
		"tool_errors": 2,
		"tool_error_categories": {"file_not_found": 1, "timeout": 1},
		"uses_task_agent": true,
		"uses_mcp": true,
		"uses_web_search": true,
		"uses_web_fetch": false,
		"lines_added": 350,
		"lines_removed": 80,
		"files_modified": 9,
		"message_hours": [16, 16, 17, 17, 17, 18],
		"user_interruptions": 3
	}`

	if err := os.WriteFile(filepath.Join(sessionDir, "session-fieldmap.json"), []byte(sessionJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions, err := LoadSessions(dir)
	if err != nil {
		t.Fatalf("LoadSessions returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	s := sessions[0]

	if s.SessionID != "fieldmap-001" {
		t.Errorf("SessionID: got %q, want %q", s.SessionID, "fieldmap-001")
	}
	if s.ProjectPath != "/opt/myrepo" {
		t.Errorf("ProjectPath: got %q, want %q", s.ProjectPath, "/opt/myrepo")
	}
	if s.StartTime != "2026-03-23T16:45:00Z" {
		t.Errorf("StartTime: got %q, want %q", s.StartTime, "2026-03-23T16:45:00Z")
	}
	if s.DurationMinutes != 90 {
		t.Errorf("DurationMinutes: got %d, want 90", s.DurationMinutes)
	}
	if s.UserMessageCount != 25 {
		t.Errorf("UserMessageCount: got %d, want 25", s.UserMessageCount)
	}
	if s.AssistantMessageCount != 28 {
		t.Errorf("AssistantMessageCount: got %d, want 28", s.AssistantMessageCount)
	}
	if len(s.ToolCounts) != 4 {
		t.Errorf("ToolCounts length: got %d, want 4", len(s.ToolCounts))
	}
	if s.ToolCounts["Read"] != 10 {
		t.Errorf("ToolCounts[Read]: got %d, want 10", s.ToolCounts["Read"])
	}
	if len(s.Languages) != 3 {
		t.Errorf("Languages length: got %d, want 3", len(s.Languages))
	}
	if s.Languages["go"] != 12 {
		t.Errorf("Languages[go]: got %d, want 12", s.Languages["go"])
	}
	if s.GitCommits != 4 {
		t.Errorf("GitCommits: got %d, want 4", s.GitCommits)
	}
	if s.GitPushes != 1 {
		t.Errorf("GitPushes: got %d, want 1", s.GitPushes)
	}
	if s.InputTokens != 180000 {
		t.Errorf("InputTokens: got %d, want 180000", s.InputTokens)
	}
	if s.OutputTokens != 45000 {
		t.Errorf("OutputTokens: got %d, want 45000", s.OutputTokens)
	}
	if s.FirstPrompt != "Add database migration support" {
		t.Errorf("FirstPrompt: got %q", s.FirstPrompt)
	}
	if s.ToolErrors != 2 {
		t.Errorf("ToolErrors: got %d, want 2", s.ToolErrors)
	}
	if len(s.ToolErrorCategories) != 2 {
		t.Errorf("ToolErrorCategories length: got %d, want 2", len(s.ToolErrorCategories))
	}
	if s.ToolErrorCategories["timeout"] != 1 {
		t.Errorf("ToolErrorCategories[timeout]: got %d, want 1", s.ToolErrorCategories["timeout"])
	}
	if !s.UsesTaskAgent {
		t.Error("UsesTaskAgent: got false, want true")
	}
	if !s.UsesMCP {
		t.Error("UsesMCP: got false, want true")
	}
	if !s.UsesWebSearch {
		t.Error("UsesWebSearch: got false, want true")
	}
	if s.UsesWebFetch {
		t.Error("UsesWebFetch: got true, want false")
	}
	if s.LinesAdded != 350 {
		t.Errorf("LinesAdded: got %d, want 350", s.LinesAdded)
	}
	if s.LinesRemoved != 80 {
		t.Errorf("LinesRemoved: got %d, want 80", s.LinesRemoved)
	}
	if s.FilesModified != 9 {
		t.Errorf("FilesModified: got %d, want 9", s.FilesModified)
	}
	if len(s.MessageHours) != 6 {
		t.Errorf("MessageHours length: got %d, want 6", len(s.MessageHours))
	}
	if s.UserInterruptions != 3 {
		t.Errorf("UserInterruptions: got %d, want 3", s.UserInterruptions)
	}
}

func TestLoadSessions_SkipsNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	sessionDir := makeSessionDir(t, dir)

	validJSON := `{"session_id": "only-one", "project_path": "/tmp", "start_time": "2026-03-20T09:00:00Z"}`
	if err := os.WriteFile(filepath.Join(sessionDir, "session.json"), []byte(validJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a non-JSON file that should be ignored.
	if err := os.WriteFile(filepath.Join(sessionDir, "notes.txt"), []byte("not a session"), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions, err := LoadSessions(dir)
	if err != nil {
		t.Fatalf("LoadSessions returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (txt skipped), got %d", len(sessions))
	}
}
