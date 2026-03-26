package collector

import (
	"fmt"
	"log/slog"

	"github.com/donnycrash/clasp/internal/watermark"
)

// Collect orchestrates data collection from the Claude data directory. It
// loads stats, sessions, and facets, then filters by the watermark to exclude
// already-uploaded data and joins sessions with their facets.
func Collect(claudeDir string, wm *watermark.Watermark) (*CollectedData, error) {
	// Load all raw data sources.
	stats, err := LoadStats(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("collecting stats: %w", err)
	}

	sessions, err := LoadSessions(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("collecting sessions: %w", err)
	}

	facets, err := LoadFacets(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("collecting facets: %w", err)
	}

	// Filter stats daily entries by watermark date.
	filtered := filterStats(stats, wm)

	// Filter sessions by watermark and join with facets.
	joined := filterAndJoinSessions(sessions, facets, wm)

	slog.Info("collection complete",
		"daily_activity_entries", len(filtered.DailyActivity),
		"sessions", len(joined),
	)

	return &CollectedData{
		Stats:    filtered,
		Sessions: joined,
	}, nil
}

// filterStats returns a FilteredStats containing only daily entries after the
// watermark date, plus the full model usage totals.
func filterStats(stats *StatsCache, wm *watermark.Watermark) *FilteredStats {
	watermarkDate := wm.StatsCacheUploadedThrough

	var filteredActivity []DailyActivity
	for _, da := range stats.DailyActivity {
		if watermarkDate == "" || da.Date > watermarkDate {
			filteredActivity = append(filteredActivity, da)
		}
	}

	var filteredTokens []DailyModelTokens
	for _, dt := range stats.DailyModelTokens {
		if watermarkDate == "" || dt.Date > watermarkDate {
			filteredTokens = append(filteredTokens, dt)
		}
	}

	// Determine period boundaries from filtered data.
	periodStart, periodEnd := "", ""
	if len(filteredActivity) > 0 {
		periodStart = filteredActivity[0].Date
		periodEnd = filteredActivity[len(filteredActivity)-1].Date
	}

	return &FilteredStats{
		PeriodStart:                 periodStart,
		PeriodEnd:                   periodEnd,
		DailyActivity:               filteredActivity,
		DailyModelTokens:            filteredTokens,
		ModelUsage:                  stats.ModelUsage,
		HourCounts:                  stats.HourCounts,
		LongestSession:              stats.LongestSession,
		TotalSpeculationTimeSavedMs: stats.TotalSpeculationTimeSavedMs,
	}
}

// filterAndJoinSessions removes already-uploaded sessions and pairs remaining
// sessions with their facets.
func filterAndJoinSessions(sessions []SessionMeta, facets map[string]Facets, wm *watermark.Watermark) []JoinedSession {
	var result []JoinedSession
	for _, s := range sessions {
		if wm.IsSessionUploaded(s.SessionID) {
			slog.Debug("skipping already-uploaded session", "session_id", s.SessionID)
			continue
		}

		js := JoinedSession{
			SessionMeta: s,
		}
		if f, ok := facets[s.SessionID]; ok {
			fc := f // copy to get a stable pointer
			js.Facets = &fc
		}
		result = append(result, js)
	}
	return result
}
