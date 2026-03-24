package collector

// CollectedData holds all data gathered from a collection run, ready for
// redaction and upload.
type CollectedData struct {
	Stats    *FilteredStats  `json:"stats,omitempty"`
	Sessions []JoinedSession `json:"sessions"`
}

// FilteredStats contains aggregate usage statistics for the collection period.
type FilteredStats struct {
	PeriodStart      string                `json:"period_start"`
	PeriodEnd        string                `json:"period_end"`
	DailyActivity    []DailyActivity       `json:"daily_activity"`
	DailyModelTokens []DailyModelTokens    `json:"daily_model_tokens"`
	ModelUsage       map[string]ModelUsage `json:"model_usage"`
}

// DailyActivity records daily aggregate activity counts.
type DailyActivity struct {
	Date          string `json:"date"`
	MessageCount  int    `json:"messageCount"`
	SessionCount  int    `json:"sessionCount"`
	ToolCallCount int    `json:"toolCallCount"`
}

// DailyModelTokens records per-model token usage for a single day.
type DailyModelTokens struct {
	Date          string           `json:"date"`
	TokensByModel map[string]int64 `json:"tokensByModel"`
}

// ModelUsage records aggregate token usage and cost for a single model.
type ModelUsage struct {
	InputTokens              int64   `json:"inputTokens"`
	OutputTokens             int64   `json:"outputTokens"`
	CacheReadInputTokens     int64   `json:"cacheReadInputTokens"`
	CacheCreationInputTokens int64   `json:"cacheCreationInputTokens"`
	CostUSD                  float64 `json:"costUsd"`
}

// JoinedSession combines session metadata with optional AI-generated facets.
type JoinedSession struct {
	SessionMeta
	Facets *Facets `json:"facets,omitempty"`
}

// SessionMeta holds metadata extracted from a single Claude Code session.
type SessionMeta struct {
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
	FirstPrompt           string         `json:"first_prompt"`
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
}

// Facets holds AI-generated qualitative analysis of a session.
type Facets struct {
	SessionID              string         `json:"session_id"`
	UnderlyingGoal         string         `json:"underlying_goal,omitempty"`
	GoalCategories         map[string]int `json:"goal_categories,omitempty"`
	Outcome                string         `json:"outcome"`
	UserSatisfactionCounts map[string]int `json:"user_satisfaction_counts,omitempty"`
	ClaudeHelpfulness      string         `json:"claude_helpfulness"`
	SessionType            string         `json:"session_type"`
	FrictionCounts         map[string]int `json:"friction_counts,omitempty"`
	FrictionDetail         string         `json:"friction_detail,omitempty"`
	PrimarySuccess         string         `json:"primary_success"`
	BriefSummary           string         `json:"brief_summary,omitempty"`
}
