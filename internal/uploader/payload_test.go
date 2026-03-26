package uploader

import (
	"regexp"
	"runtime"
	"testing"

	"github.com/donnycrash/clasp/internal/collector"
)

func TestBuildPayload_Basic(t *testing.T) {
	data := &collector.CollectedData{
		Stats: &collector.FilteredStats{
			PeriodStart: "2025-01-01",
			PeriodEnd:   "2025-01-31",
			DailyActivity: []collector.DailyActivity{
				{Date: "2025-01-15", MessageCount: 10, SessionCount: 2, ToolCallCount: 5},
			},
			DailyModelTokens: []collector.DailyModelTokens{
				{Date: "2025-01-15", TokensByModel: map[string]int64{"claude-3": 1000}},
			},
			ModelUsage: map[string]collector.ModelUsage{
				"claude-3": {
					InputTokens:              500,
					OutputTokens:             300,
					CacheReadInputTokens:     100,
					CacheCreationInputTokens: 50,
					CostUSD:                  0.42,
				},
			},
			HourCounts: map[string]int{"9": 5, "14": 10},
			LongestSession: &collector.LongestSession{
				SessionID:    "sess-001",
				Duration:     2700000,
				MessageCount: 18,
				Timestamp:    "2025-01-15T10:00:00Z",
			},
			TotalSpeculationTimeSavedMs: 7500,
		},
		Sessions: []collector.JoinedSession{
			{
				SessionMeta: collector.SessionMeta{
					SessionID:             "sess-001",
					ProjectPath:           "/home/user/project",
					StartTime:             "2025-01-15T10:00:00Z",
					DurationMinutes:       45,
					UserMessageCount:      10,
					AssistantMessageCount: 8,
					ToolCounts:            map[string]int{"Read": 3, "Edit": 2},
					Languages:             map[string]int{"go": 5, "python": 2},
					GitCommits:            3,
					GitPushes:             1,
					InputTokens:           500,
					OutputTokens:          300,
					FirstPrompt:           "Fix the bug",
					ToolErrors:            1,
					ToolErrorCategories:   map[string]int{"permission": 1},
					UsesTaskAgent:         true,
					UsesMCP:               false,
					UsesWebSearch:         true,
					UsesWebFetch:          false,
					LinesAdded:            100,
					LinesRemoved:          20,
					FilesModified:         5,
					MessageHours:          []int{10, 11, 12},
					UserInterruptions:     2,
				},
				Facets: &collector.Facets{
					GoalCategories:         map[string]int{"bugfix": 1},
					Outcome:                "success",
					UserSatisfactionCounts: map[string]int{"satisfied": 1},
					ClaudeHelpfulness:      "very_helpful",
					SessionType:            "debugging",
					FrictionCounts:         map[string]int{"slow_response": 1},
					FrictionDetail:         "model was slow",
					PrimarySuccess:         "yes",
					BriefSummary:           "Fixed a permission bug",
					UnderlyingGoal:         "fix auth flow",
				},
			},
		},
	}

	identity := Identity{Username: "testuser"}
	payload := BuildPayload(data, identity, "1.0.0")

	// Metadata
	if payload.Metadata.ToolName != "clasp" {
		t.Errorf("ToolName = %q, want %q", payload.Metadata.ToolName, "clasp")
	}
	if payload.Metadata.ToolVersion != "1.0.0" {
		t.Errorf("ToolVersion = %q, want %q", payload.Metadata.ToolVersion, "1.0.0")
	}
	if payload.Metadata.GitHubUsername != "testuser" {
		t.Errorf("GitHubUsername = %q, want %q", payload.Metadata.GitHubUsername, "testuser")
	}

	// Stats
	if payload.StatsSummary == nil {
		t.Fatal("StatsSummary is nil")
	}
	if payload.StatsSummary.PeriodStart != "2025-01-01" {
		t.Errorf("PeriodStart = %q, want %q", payload.StatsSummary.PeriodStart, "2025-01-01")
	}
	if payload.StatsSummary.PeriodEnd != "2025-01-31" {
		t.Errorf("PeriodEnd = %q, want %q", payload.StatsSummary.PeriodEnd, "2025-01-31")
	}
	if len(payload.StatsSummary.DailyActivity) != 1 {
		t.Fatalf("DailyActivity length = %d, want 1", len(payload.StatsSummary.DailyActivity))
	}
	da := payload.StatsSummary.DailyActivity[0]
	if da.Date != "2025-01-15" || da.MessageCount != 10 || da.SessionCount != 2 || da.ToolCallCount != 5 {
		t.Errorf("DailyActivity = %+v, unexpected values", da)
	}
	if len(payload.StatsSummary.DailyModelTokens) != 1 {
		t.Fatalf("DailyModelTokens length = %d, want 1", len(payload.StatsSummary.DailyModelTokens))
	}
	dmt := payload.StatsSummary.DailyModelTokens[0]
	if dmt.TokensByModel["claude-3"] != 1000 {
		t.Errorf("TokensByModel[claude-3] = %d, want 1000", dmt.TokensByModel["claude-3"])
	}
	mu := payload.StatsSummary.ModelUsage["claude-3"]
	if mu.InputTokens != 500 || mu.OutputTokens != 300 || mu.CacheReadInputTokens != 100 || mu.CacheCreationInputTokens != 50 || mu.CostUSD != 0.42 {
		t.Errorf("ModelUsage[claude-3] = %+v, unexpected values", mu)
	}

	// HourCounts
	if len(payload.StatsSummary.HourCounts) != 2 {
		t.Fatalf("HourCounts length = %d, want 2", len(payload.StatsSummary.HourCounts))
	}
	if payload.StatsSummary.HourCounts["14"] != 10 {
		t.Errorf("HourCounts[14] = %d, want 10", payload.StatsSummary.HourCounts["14"])
	}

	// LongestSession
	ls := payload.StatsSummary.LongestSession
	if ls == nil {
		t.Fatal("LongestSession should not be nil")
	}
	if ls.SessionID != "sess-001" {
		t.Errorf("LongestSession.SessionID = %q, want %q", ls.SessionID, "sess-001")
	}
	if ls.Duration != 2700000 {
		t.Errorf("LongestSession.Duration = %d, want 2700000", ls.Duration)
	}
	if ls.MessageCount != 18 {
		t.Errorf("LongestSession.MessageCount = %d, want 18", ls.MessageCount)
	}

	// TotalSpeculationTimeSavedMs
	if payload.StatsSummary.TotalSpeculationTimeSavedMs != 7500 {
		t.Errorf("TotalSpeculationTimeSavedMs = %d, want 7500", payload.StatsSummary.TotalSpeculationTimeSavedMs)
	}

	// Sessions
	if len(payload.Sessions) != 1 {
		t.Fatalf("Sessions length = %d, want 1", len(payload.Sessions))
	}
	sess := payload.Sessions[0]
	if sess.SessionID != "sess-001" {
		t.Errorf("SessionID = %q, want %q", sess.SessionID, "sess-001")
	}
	if sess.ProjectPath != "/home/user/project" {
		t.Errorf("ProjectPath = %q", sess.ProjectPath)
	}
	if sess.DurationMinutes != 45 {
		t.Errorf("DurationMinutes = %d, want 45", sess.DurationMinutes)
	}
	if sess.UserMessageCount != 10 {
		t.Errorf("UserMessageCount = %d, want 10", sess.UserMessageCount)
	}
	if sess.AssistantMessageCount != 8 {
		t.Errorf("AssistantMessageCount = %d, want 8", sess.AssistantMessageCount)
	}
	if sess.ToolCounts["Read"] != 3 || sess.ToolCounts["Edit"] != 2 {
		t.Errorf("ToolCounts = %v", sess.ToolCounts)
	}
	if sess.Languages["go"] != 5 {
		t.Errorf("Languages[go] = %d, want 5", sess.Languages["go"])
	}
	if sess.GitCommits != 3 || sess.GitPushes != 1 {
		t.Errorf("GitCommits=%d GitPushes=%d", sess.GitCommits, sess.GitPushes)
	}
	if sess.InputTokens != 500 || sess.OutputTokens != 300 {
		t.Errorf("InputTokens=%d OutputTokens=%d", sess.InputTokens, sess.OutputTokens)
	}
	if sess.FirstPrompt != "Fix the bug" {
		t.Errorf("FirstPrompt = %q", sess.FirstPrompt)
	}
	if sess.ToolErrors != 1 {
		t.Errorf("ToolErrors = %d", sess.ToolErrors)
	}
	if !sess.UsesTaskAgent {
		t.Error("UsesTaskAgent should be true")
	}
	if sess.UsesMCP {
		t.Error("UsesMCP should be false")
	}
	if !sess.UsesWebSearch {
		t.Error("UsesWebSearch should be true")
	}
	if sess.UsesWebFetch {
		t.Error("UsesWebFetch should be false")
	}
	if sess.LinesAdded != 100 || sess.LinesRemoved != 20 || sess.FilesModified != 5 {
		t.Errorf("LinesAdded=%d LinesRemoved=%d FilesModified=%d", sess.LinesAdded, sess.LinesRemoved, sess.FilesModified)
	}
	if sess.UserInterruptions != 2 {
		t.Errorf("UserInterruptions = %d", sess.UserInterruptions)
	}
}

func TestBuildPayload_EmptySessions(t *testing.T) {
	data := &collector.CollectedData{
		Sessions: []collector.JoinedSession{},
	}
	payload := BuildPayload(data, Identity{Username: "user"}, "1.0.0")

	if payload.Sessions == nil {
		t.Fatal("Sessions should not be nil, expected empty slice")
	}
	if len(payload.Sessions) != 0 {
		t.Errorf("Sessions length = %d, want 0", len(payload.Sessions))
	}
}

func TestBuildPayload_NoStats(t *testing.T) {
	data := &collector.CollectedData{
		Stats:    nil,
		Sessions: []collector.JoinedSession{},
	}
	payload := BuildPayload(data, Identity{Username: "user"}, "1.0.0")

	if payload.StatsSummary != nil {
		t.Errorf("StatsSummary should be nil when Stats is nil, got %+v", payload.StatsSummary)
	}
}

func TestBuildPayload_MetadataFields(t *testing.T) {
	data := &collector.CollectedData{
		Sessions: []collector.JoinedSession{},
	}
	payload := BuildPayload(data, Identity{Username: "alice"}, "2.3.4")

	m := payload.Metadata
	if m.ToolName != "clasp" {
		t.Errorf("ToolName = %q, want %q", m.ToolName, "clasp")
	}
	if m.ToolVersion != "2.3.4" {
		t.Errorf("ToolVersion = %q, want %q", m.ToolVersion, "2.3.4")
	}
	if m.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", m.OS, runtime.GOOS)
	}
	if m.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", m.Arch, runtime.GOARCH)
	}
	if m.GitHubUsername != "alice" {
		t.Errorf("GitHubUsername = %q, want %q", m.GitHubUsername, "alice")
	}

	// HostnameHash should be a 64-char hex string (SHA-256).
	if len(m.HostnameHash) != 64 {
		t.Errorf("HostnameHash length = %d, want 64", len(m.HostnameHash))
	}
	hexPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)
	if !hexPattern.MatchString(m.HostnameHash) {
		t.Errorf("HostnameHash %q is not a valid hex SHA-256", m.HostnameHash)
	}

	// BatchID should be UUID-like: 8-4-4-4-12 hex.
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidPattern.MatchString(m.BatchID) {
		t.Errorf("BatchID %q does not match UUID v4 pattern", m.BatchID)
	}

	// UploadTimestamp should be non-empty and RFC3339.
	if m.UploadTimestamp == "" {
		t.Error("UploadTimestamp is empty")
	}
}

func TestBuildPayload_FacetsJoined(t *testing.T) {
	data := &collector.CollectedData{
		Sessions: []collector.JoinedSession{
			{
				SessionMeta: collector.SessionMeta{
					SessionID:   "sess-facet",
					ToolCounts:  map[string]int{},
					Languages:   map[string]int{},
					StartTime:   "2025-01-15T10:00:00Z",
					ProjectPath: "/tmp/test",
				},
				Facets: &collector.Facets{
					GoalCategories:         map[string]int{"feature": 2, "refactor": 1},
					Outcome:                "partial_success",
					UserSatisfactionCounts: map[string]int{"neutral": 1},
					ClaudeHelpfulness:      "helpful",
					SessionType:            "coding",
					FrictionCounts:         map[string]int{"context_limit": 1},
					FrictionDetail:         "ran out of context",
					PrimarySuccess:         "partial",
					BriefSummary:           "Refactored auth module",
					UnderlyingGoal:         "improve code quality",
				},
			},
		},
	}

	payload := BuildPayload(data, Identity{Username: "bob"}, "1.0.0")
	sess := payload.Sessions[0]

	if sess.Facets == nil {
		t.Fatal("Facets should not be nil")
	}
	f := sess.Facets
	if f.Outcome != "partial_success" {
		t.Errorf("Outcome = %q, want %q", f.Outcome, "partial_success")
	}
	if f.ClaudeHelpfulness != "helpful" {
		t.Errorf("ClaudeHelpfulness = %q", f.ClaudeHelpfulness)
	}
	if f.SessionType != "coding" {
		t.Errorf("SessionType = %q", f.SessionType)
	}
	if f.PrimarySuccess != "partial" {
		t.Errorf("PrimarySuccess = %q", f.PrimarySuccess)
	}
	if f.BriefSummary != "Refactored auth module" {
		t.Errorf("BriefSummary = %q", f.BriefSummary)
	}
	if f.UnderlyingGoal != "improve code quality" {
		t.Errorf("UnderlyingGoal = %q", f.UnderlyingGoal)
	}
	if f.FrictionDetail != "ran out of context" {
		t.Errorf("FrictionDetail = %q", f.FrictionDetail)
	}
	if f.GoalCategories["feature"] != 2 || f.GoalCategories["refactor"] != 1 {
		t.Errorf("GoalCategories = %v", f.GoalCategories)
	}
	if f.UserSatisfactionCounts["neutral"] != 1 {
		t.Errorf("UserSatisfactionCounts = %v", f.UserSatisfactionCounts)
	}
	if f.FrictionCounts["context_limit"] != 1 {
		t.Errorf("FrictionCounts = %v", f.FrictionCounts)
	}
}

func TestBuildPayload_NilFacets(t *testing.T) {
	data := &collector.CollectedData{
		Sessions: []collector.JoinedSession{
			{
				SessionMeta: collector.SessionMeta{
					SessionID:   "sess-nofacet",
					ToolCounts:  map[string]int{},
					Languages:   map[string]int{},
					StartTime:   "2025-01-15T10:00:00Z",
					ProjectPath: "/tmp/test",
				},
				Facets: nil,
			},
		},
	}

	payload := BuildPayload(data, Identity{Username: "user"}, "1.0.0")
	sess := payload.Sessions[0]

	if sess.Facets != nil {
		t.Errorf("Facets should be nil when source facets are nil, got %+v", sess.Facets)
	}
}
