package collector

import (
	"os"
	"path/filepath"
	"testing"
)

func makeFacetsDir(t *testing.T, dir string) string {
	t.Helper()
	facetsDir := filepath.Join(dir, "usage-data", "facets")
	if err := os.MkdirAll(facetsDir, 0o755); err != nil {
		t.Fatalf("creating facets dir: %v", err)
	}
	return facetsDir
}

func TestLoadFacets_MultipleFacets(t *testing.T) {
	dir := t.TempDir()
	facetsDir := makeFacetsDir(t, dir)

	files := map[string]string{
		"facets-abc123.json": `{
			"session_id": "abc123",
			"underlying_goal": "Fix failing CI pipeline tests",
			"goal_categories": {"debugging": 1, "testing": 1},
			"outcome": "success",
			"user_satisfaction_counts": {"satisfied": 1},
			"claude_helpfulness": "very_helpful",
			"session_type": "debugging",
			"friction_counts": {"slow_response": 1},
			"friction_detail": "Model took long on initial analysis",
			"primary_success": "resolved",
			"brief_summary": "Fixed 3 failing tests related to timezone handling in the CI pipeline."
		}`,
		"facets-def456.json": `{
			"session_id": "def456",
			"underlying_goal": "Refactor authentication module",
			"goal_categories": {"refactoring": 1},
			"outcome": "partial_success",
			"user_satisfaction_counts": {"neutral": 1},
			"claude_helpfulness": "somewhat_helpful",
			"session_type": "refactoring",
			"friction_counts": {},
			"primary_success": "partially_resolved",
			"brief_summary": "Refactored half the auth module; remaining work tracked in issue #42."
		}`,
		"facets-ghi789.json": `{
			"session_id": "ghi789",
			"underlying_goal": "Understand error message",
			"goal_categories": {"learning": 1},
			"outcome": "success",
			"user_satisfaction_counts": {"satisfied": 1},
			"claude_helpfulness": "very_helpful",
			"session_type": "exploration",
			"friction_counts": {},
			"primary_success": "resolved",
			"brief_summary": "Explained the root cause of the segfault in the FFI binding."
		}`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(facetsDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	facets, err := LoadFacets(dir)
	if err != nil {
		t.Fatalf("LoadFacets returned error: %v", err)
	}

	if len(facets) != 3 {
		t.Fatalf("expected 3 facets, got %d", len(facets))
	}

	// Verify keyed by session_id.
	f1, ok := facets["abc123"]
	if !ok {
		t.Fatal("missing facets for session abc123")
	}
	if f1.UnderlyingGoal != "Fix failing CI pipeline tests" {
		t.Errorf("UnderlyingGoal: got %q", f1.UnderlyingGoal)
	}
	if f1.Outcome != "success" {
		t.Errorf("Outcome: got %q, want %q", f1.Outcome, "success")
	}
	if f1.ClaudeHelpfulness != "very_helpful" {
		t.Errorf("ClaudeHelpfulness: got %q", f1.ClaudeHelpfulness)
	}
	if f1.SessionType != "debugging" {
		t.Errorf("SessionType: got %q", f1.SessionType)
	}
	if f1.PrimarySuccess != "resolved" {
		t.Errorf("PrimarySuccess: got %q", f1.PrimarySuccess)
	}
	if f1.FrictionDetail != "Model took long on initial analysis" {
		t.Errorf("FrictionDetail: got %q", f1.FrictionDetail)
	}
	if f1.GoalCategories["debugging"] != 1 {
		t.Errorf("GoalCategories[debugging]: got %d, want 1", f1.GoalCategories["debugging"])
	}
	if f1.UserSatisfactionCounts["satisfied"] != 1 {
		t.Errorf("UserSatisfactionCounts[satisfied]: got %d, want 1", f1.UserSatisfactionCounts["satisfied"])
	}

	f2, ok := facets["def456"]
	if !ok {
		t.Fatal("missing facets for session def456")
	}
	if f2.Outcome != "partial_success" {
		t.Errorf("Outcome for def456: got %q", f2.Outcome)
	}

	if _, ok := facets["ghi789"]; !ok {
		t.Fatal("missing facets for session ghi789")
	}
}

func TestLoadFacets_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_ = makeFacetsDir(t, dir)

	facets, err := LoadFacets(dir)
	if err != nil {
		t.Fatalf("LoadFacets returned error: %v", err)
	}
	if len(facets) != 0 {
		t.Fatalf("expected 0 facets, got %d", len(facets))
	}
}

func TestLoadFacets_DirNotExist(t *testing.T) {
	dir := t.TempDir()
	// Do NOT create the facets directory.
	facets, err := LoadFacets(dir)
	if err != nil {
		t.Fatalf("LoadFacets returned error for missing dir: %v", err)
	}
	if facets != nil {
		t.Fatalf("expected nil facets for missing dir, got %v", facets)
	}
}

func TestLoadFacets_MissingSessionID(t *testing.T) {
	dir := t.TempDir()
	facetsDir := makeFacetsDir(t, dir)

	// One facet with a session_id and one without.
	withID := `{
		"session_id": "has-id",
		"outcome": "success",
		"claude_helpfulness": "helpful",
		"session_type": "coding",
		"primary_success": "resolved"
	}`
	withoutID := `{
		"outcome": "success",
		"claude_helpfulness": "helpful",
		"session_type": "coding",
		"primary_success": "resolved"
	}`

	if err := os.WriteFile(filepath.Join(facetsDir, "with-id.json"), []byte(withID), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(facetsDir, "without-id.json"), []byte(withoutID), 0o644); err != nil {
		t.Fatal(err)
	}

	facets, err := LoadFacets(dir)
	if err != nil {
		t.Fatalf("LoadFacets returned error: %v", err)
	}

	// Only the facet with a session_id should be present.
	if len(facets) != 1 {
		t.Fatalf("expected 1 facet (empty session_id skipped), got %d", len(facets))
	}
	if _, ok := facets["has-id"]; !ok {
		t.Fatal("missing facets for session has-id")
	}
}

func TestLoadFacets_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	facetsDir := makeFacetsDir(t, dir)

	validJSON := `{"session_id": "valid-facet", "outcome": "success", "claude_helpfulness": "helpful", "session_type": "coding", "primary_success": "resolved"}`
	invalidJSON := `{broken json!!!`

	if err := os.WriteFile(filepath.Join(facetsDir, "valid.json"), []byte(validJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(facetsDir, "invalid.json"), []byte(invalidJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	facets, err := LoadFacets(dir)
	if err != nil {
		t.Fatalf("LoadFacets returned error: %v", err)
	}
	if len(facets) != 1 {
		t.Fatalf("expected 1 facet (invalid skipped), got %d", len(facets))
	}
	if _, ok := facets["valid-facet"]; !ok {
		t.Fatal("missing facets for session valid-facet")
	}
}

func TestLoadFacets_SkipsNonJSONFiles(t *testing.T) {
	dir := t.TempDir()
	facetsDir := makeFacetsDir(t, dir)

	validJSON := `{"session_id": "only-facet", "outcome": "success", "claude_helpfulness": "helpful", "session_type": "coding", "primary_success": "resolved"}`
	if err := os.WriteFile(filepath.Join(facetsDir, "facet.json"), []byte(validJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(facetsDir, "readme.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}

	facets, err := LoadFacets(dir)
	if err != nil {
		t.Fatalf("LoadFacets returned error: %v", err)
	}
	if len(facets) != 1 {
		t.Fatalf("expected 1 facet, got %d", len(facets))
	}
}
