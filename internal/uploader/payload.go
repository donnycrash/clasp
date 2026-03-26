package uploader

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/donnycrash/clasp/internal/collector"
)

// Identity represents the authenticated user uploading data.
type Identity struct {
	Username string
}

// Payload is the top-level structure sent to the analytics endpoint.
type Payload struct {
	Metadata     PayloadMetadata  `json:"metadata"`
	StatsSummary *PayloadStats    `json:"stats_summary,omitempty"`
	Sessions     []PayloadSession `json:"sessions"`
}

// PayloadMetadata contains information about the upload environment.
type PayloadMetadata struct {
	ToolName        string `json:"tool_name"`
	ToolVersion     string `json:"tool_version"`
	OS              string `json:"os"`
	Arch            string `json:"arch"`
	HostnameHash    string `json:"hostname_hash"`
	GitHubUsername  string `json:"github_username"`
	UploadTimestamp string `json:"upload_timestamp"`
	BatchID         string `json:"batch_id"`
}

// PayloadStats summarises aggregate usage statistics for the collection period.
type PayloadStats struct {
	PeriodStart                 string                       `json:"period_start"`
	PeriodEnd                   string                       `json:"period_end"`
	DailyActivity               []PayloadDailyActivity       `json:"daily_activity"`
	DailyModelTokens            []PayloadDailyModelTokens    `json:"daily_model_tokens"`
	ModelUsage                  map[string]PayloadModelUsage `json:"model_usage"`
	HourCounts                  map[string]int               `json:"hour_counts,omitempty"`
	LongestSession              *PayloadLongestSession       `json:"longest_session,omitempty"`
	TotalSpeculationTimeSavedMs int64                        `json:"total_speculation_time_saved_ms"`
}

// PayloadLongestSession records metadata about the longest session observed.
type PayloadLongestSession struct {
	SessionID    string `json:"session_id"`
	Duration     int64  `json:"duration"`
	MessageCount int    `json:"message_count"`
	Timestamp    string `json:"timestamp"`
}

// PayloadDailyActivity records daily aggregate counts.
type PayloadDailyActivity struct {
	Date          string `json:"date"`
	MessageCount  int    `json:"message_count"`
	SessionCount  int    `json:"session_count"`
	ToolCallCount int    `json:"tool_call_count"`
}

// PayloadDailyModelTokens records per-model token usage for a single day.
type PayloadDailyModelTokens struct {
	Date          string           `json:"date"`
	TokensByModel map[string]int64 `json:"tokens_by_model"`
}

// PayloadModelUsage records aggregate token and cost data for a single model.
type PayloadModelUsage struct {
	InputTokens              int64   `json:"input_tokens"`
	OutputTokens             int64   `json:"output_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	CostUSD                  float64 `json:"cost_usd"`
}

// PayloadSession represents a single session in the upload payload.
type PayloadSession struct {
	SessionID             string         `json:"session_id"`
	ProjectPath           string         `json:"project_path"`
	StartTime             string         `json:"start_time"`
	DurationMinutes       int            `json:"duration_minutes"`
	UserMessageCount      int            `json:"user_message_count"`
	AssistantMessageCount int            `json:"assistant_message_count"`
	ToolCounts            map[string]int `json:"tool_counts"`
	Languages             map[string]int `json:"languages"`
	GitCommits            int            `json:"git_commits"`
	GitPushes             int            `json:"git_pushes"`
	InputTokens           int64          `json:"input_tokens"`
	OutputTokens          int64          `json:"output_tokens"`
	FirstPrompt           string         `json:"first_prompt,omitempty"`
	ToolErrors            int            `json:"tool_errors"`
	ToolErrorCategories   map[string]int `json:"tool_error_categories,omitempty"`
	UsesTaskAgent         bool           `json:"uses_task_agent"`
	UsesMCP               bool           `json:"uses_mcp"`
	UsesWebSearch         bool           `json:"uses_web_search"`
	UsesWebFetch          bool           `json:"uses_web_fetch"`
	LinesAdded            int            `json:"lines_added"`
	LinesRemoved          int            `json:"lines_removed"`
	FilesModified         int            `json:"files_modified"`
	MessageHours          []int          `json:"message_hours,omitempty"`
	UserInterruptions     int            `json:"user_interruptions"`
	Facets                *PayloadFacets `json:"facets,omitempty"`
}

// PayloadFacets holds AI-generated qualitative analysis for a session.
type PayloadFacets struct {
	GoalCategories         map[string]int `json:"goal_categories,omitempty"`
	Outcome                string         `json:"outcome"`
	UserSatisfactionCounts map[string]int `json:"user_satisfaction_counts,omitempty"`
	ClaudeHelpfulness      string         `json:"claude_helpfulness"`
	SessionType            string         `json:"session_type"`
	FrictionCounts         map[string]int `json:"friction_counts,omitempty"`
	FrictionDetail         string         `json:"friction_detail,omitempty"`
	PrimarySuccess         string         `json:"primary_success"`
	BriefSummary           string         `json:"brief_summary,omitempty"`
	UnderlyingGoal         string         `json:"underlying_goal,omitempty"`
}

// BuildPayload converts collected data into an upload-ready Payload.
func BuildPayload(data *collector.CollectedData, identity Identity, version string) *Payload {
	hostname, _ := os.Hostname()
	hostnameHash := sha256Hex(hostname)
	batchID := generateUUID()

	p := &Payload{
		Metadata: PayloadMetadata{
			ToolName:        "clasp",
			ToolVersion:     version,
			OS:              runtime.GOOS,
			Arch:            runtime.GOARCH,
			HostnameHash:    hostnameHash,
			GitHubUsername:  identity.Username,
			UploadTimestamp: time.Now().UTC().Format(time.RFC3339),
			BatchID:         batchID,
		},
		Sessions: make([]PayloadSession, 0, len(data.Sessions)),
	}

	if data.Stats != nil {
		p.StatsSummary = convertStats(data.Stats)
	}

	for _, s := range data.Sessions {
		p.Sessions = append(p.Sessions, convertSession(s))
	}

	return p
}

// convertStats maps collector FilteredStats to PayloadStats.
func convertStats(fs *collector.FilteredStats) *PayloadStats {
	ps := &PayloadStats{
		PeriodStart:      fs.PeriodStart,
		PeriodEnd:        fs.PeriodEnd,
		DailyActivity:    make([]PayloadDailyActivity, 0, len(fs.DailyActivity)),
		DailyModelTokens: make([]PayloadDailyModelTokens, 0, len(fs.DailyModelTokens)),
		ModelUsage:       make(map[string]PayloadModelUsage, len(fs.ModelUsage)),
	}

	for _, da := range fs.DailyActivity {
		ps.DailyActivity = append(ps.DailyActivity, PayloadDailyActivity{
			Date:          da.Date,
			MessageCount:  da.MessageCount,
			SessionCount:  da.SessionCount,
			ToolCallCount: da.ToolCallCount,
		})
	}

	for _, dt := range fs.DailyModelTokens {
		ps.DailyModelTokens = append(ps.DailyModelTokens, PayloadDailyModelTokens{
			Date:          dt.Date,
			TokensByModel: dt.TokensByModel,
		})
	}

	for model, mu := range fs.ModelUsage {
		ps.ModelUsage[model] = PayloadModelUsage{
			InputTokens:              mu.InputTokens,
			OutputTokens:             mu.OutputTokens,
			CacheReadInputTokens:     mu.CacheReadInputTokens,
			CacheCreationInputTokens: mu.CacheCreationInputTokens,
			CostUSD:                  mu.CostUSD,
		}
	}

	ps.HourCounts = fs.HourCounts
	ps.TotalSpeculationTimeSavedMs = fs.TotalSpeculationTimeSavedMs
	if fs.LongestSession != nil {
		ps.LongestSession = &PayloadLongestSession{
			SessionID:    fs.LongestSession.SessionID,
			Duration:     fs.LongestSession.Duration,
			MessageCount: fs.LongestSession.MessageCount,
			Timestamp:    fs.LongestSession.Timestamp,
		}
	}

	return ps
}

// convertSession maps a collector JoinedSession to a PayloadSession.
func convertSession(s collector.JoinedSession) PayloadSession {
	ps := PayloadSession{
		SessionID:             s.SessionID,
		ProjectPath:           s.ProjectPath,
		StartTime:             s.StartTime,
		DurationMinutes:       s.DurationMinutes,
		UserMessageCount:      s.UserMessageCount,
		AssistantMessageCount: s.AssistantMessageCount,
		ToolCounts:            s.ToolCounts,
		Languages:             s.Languages,
		GitCommits:            s.GitCommits,
		GitPushes:             s.GitPushes,
		InputTokens:           s.InputTokens,
		OutputTokens:          s.OutputTokens,
		FirstPrompt:           s.FirstPrompt,
		ToolErrors:            s.ToolErrors,
		ToolErrorCategories:   s.ToolErrorCategories,
		UsesTaskAgent:         s.UsesTaskAgent,
		UsesMCP:               s.UsesMCP,
		UsesWebSearch:         s.UsesWebSearch,
		UsesWebFetch:          s.UsesWebFetch,
		LinesAdded:            s.LinesAdded,
		LinesRemoved:          s.LinesRemoved,
		FilesModified:         s.FilesModified,
		MessageHours:          s.MessageHours,
		UserInterruptions:     s.UserInterruptions,
	}

	if s.Facets != nil {
		ps.Facets = &PayloadFacets{
			GoalCategories:         s.Facets.GoalCategories,
			Outcome:                s.Facets.Outcome,
			UserSatisfactionCounts: s.Facets.UserSatisfactionCounts,
			ClaudeHelpfulness:      s.Facets.ClaudeHelpfulness,
			SessionType:            s.Facets.SessionType,
			FrictionCounts:         s.Facets.FrictionCounts,
			FrictionDetail:         s.Facets.FrictionDetail,
			PrimarySuccess:         s.Facets.PrimarySuccess,
			BriefSummary:           s.Facets.BriefSummary,
			UnderlyingGoal:         s.Facets.UnderlyingGoal,
		}
	}

	return ps
}

// sha256Hex returns the lowercase hex-encoded SHA-256 digest of s.
func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// generateUUID produces a random UUID v4 string.
func generateUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
