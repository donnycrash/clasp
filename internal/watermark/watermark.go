package watermark

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Watermark tracks which data has already been uploaded so that the collector
// can skip it on subsequent runs.
type Watermark struct {
	LastUploadTime            string            `json:"last_upload_time"`
	StatsCacheUploadedThrough string            `json:"stats_cache_uploaded_through_date"`
	SessionIDsUploaded        map[string]string `json:"session_ids_uploaded"`
}

// Load reads the watermark from the given file path. If the file does not
// exist, an empty Watermark is returned (not an error).
func Load(path string) (*Watermark, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("watermark file not found, starting fresh", "path", path)
			return &Watermark{
				SessionIDsUploaded: make(map[string]string),
			}, nil
		}
		return nil, fmt.Errorf("reading watermark file %s: %w", path, err)
	}

	var wm Watermark
	if err := json.Unmarshal(data, &wm); err != nil {
		return nil, fmt.Errorf("parsing watermark file %s: %w", path, err)
	}

	if wm.SessionIDsUploaded == nil {
		wm.SessionIDsUploaded = make(map[string]string)
	}

	slog.Debug("loaded watermark",
		"path", path,
		"last_upload", wm.LastUploadTime,
		"uploaded_sessions", len(wm.SessionIDsUploaded),
	)
	return &wm, nil
}

// Save writes the watermark to the given file path atomically by first
// writing to a temporary file in the same directory and then renaming.
func (wm *Watermark) Save(path string) error {
	data, err := json.MarshalIndent(wm, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling watermark: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating watermark directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "watermark-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file for watermark: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp watermark file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp watermark file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp watermark to %s: %w", path, err)
	}

	slog.Debug("saved watermark", "path", path, "uploaded_sessions", len(wm.SessionIDsUploaded))
	return nil
}

// IsSessionUploaded returns true if the given session ID has already been
// recorded as uploaded.
func (wm *Watermark) IsSessionUploaded(sessionID string) bool {
	_, ok := wm.SessionIDsUploaded[sessionID]
	return ok
}

// MarkSessionsUploaded records the given session IDs as uploaded with the
// current timestamp.
func (wm *Watermark) MarkSessionsUploaded(sessionIDs []string) {
	now := time.Now().UTC().Format(time.RFC3339)
	for _, id := range sessionIDs {
		wm.SessionIDsUploaded[id] = now
	}
	wm.LastUploadTime = now
}

// UpdateStatsDate sets the stats-cache watermark date to the given date
// string (typically in YYYY-MM-DD format).
func (wm *Watermark) UpdateStatsDate(date string) {
	wm.StatsCacheUploadedThrough = date
}

// Compact removes session entries that were uploaded more than maxAge ago.
func (wm *Watermark) Compact(maxAge time.Duration) {
	cutoff := time.Now().UTC().Add(-maxAge)
	removed := 0

	for id, ts := range wm.SessionIDsUploaded {
		t, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			slog.Warn("removing watermark entry with unparseable timestamp",
				"session_id", id, "timestamp", ts, "error", err)
			delete(wm.SessionIDsUploaded, id)
			removed++
			continue
		}
		if t.Before(cutoff) {
			delete(wm.SessionIDsUploaded, id)
			removed++
		}
	}

	if removed > 0 {
		slog.Info("compacted watermark", "removed", removed, "remaining", len(wm.SessionIDsUploaded))
	}
}
