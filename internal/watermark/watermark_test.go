package watermark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Load tests
// ---------------------------------------------------------------------------

func TestLoad_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.json")

	wm, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wm == nil {
		t.Fatal("expected non-nil watermark")
	}
	if wm.LastUploadTime != "" {
		t.Errorf("LastUploadTime: got %q, want empty", wm.LastUploadTime)
	}
	if wm.StatsCacheUploadedThrough != "" {
		t.Errorf("StatsCacheUploadedThrough: got %q, want empty", wm.StatsCacheUploadedThrough)
	}
	if wm.SessionIDsUploaded == nil {
		t.Error("SessionIDsUploaded should be initialized, got nil")
	}
	if len(wm.SessionIDsUploaded) != 0 {
		t.Errorf("SessionIDsUploaded: got %d entries, want 0", len(wm.SessionIDsUploaded))
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watermark.json")

	raw := Watermark{
		LastUploadTime:            "2025-06-01T12:00:00Z",
		StatsCacheUploadedThrough: "2025-06-01",
		SessionIDsUploaded: map[string]string{
			"sess-001": "2025-06-01T12:00:00Z",
			"sess-002": "2025-06-01T12:05:00Z",
		},
	}
	data, _ := json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	wm, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if wm.LastUploadTime != "2025-06-01T12:00:00Z" {
		t.Errorf("LastUploadTime: got %q", wm.LastUploadTime)
	}
	if wm.StatsCacheUploadedThrough != "2025-06-01" {
		t.Errorf("StatsCacheUploadedThrough: got %q", wm.StatsCacheUploadedThrough)
	}
	if len(wm.SessionIDsUploaded) != 2 {
		t.Errorf("SessionIDsUploaded: got %d entries, want 2", len(wm.SessionIDsUploaded))
	}
	if wm.SessionIDsUploaded["sess-001"] != "2025-06-01T12:00:00Z" {
		t.Errorf("sess-001 timestamp: got %q", wm.SessionIDsUploaded["sess-001"])
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	if err := os.WriteFile(path, []byte("{not valid json!}"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// Save tests
// ---------------------------------------------------------------------------

func TestSave_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "watermark.json")

	wm := &Watermark{
		LastUploadTime:            "2025-07-01T00:00:00Z",
		StatsCacheUploadedThrough: "2025-07-01",
		SessionIDsUploaded: map[string]string{
			"abc": "2025-07-01T00:00:00Z",
		},
	}

	if err := wm.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read back and verify.
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if loaded.LastUploadTime != wm.LastUploadTime {
		t.Errorf("LastUploadTime: got %q, want %q", loaded.LastUploadTime, wm.LastUploadTime)
	}
	if loaded.SessionIDsUploaded["abc"] != "2025-07-01T00:00:00Z" {
		t.Errorf("session abc: got %q", loaded.SessionIDsUploaded["abc"])
	}
}

func TestSave_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "watermark.json")

	wm := &Watermark{
		LastUploadTime:     "2025-08-01T00:00:00Z",
		SessionIDsUploaded: map[string]string{"x": "2025-08-01T00:00:00Z"},
	}

	if err := wm.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// The final file should exist.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file does not exist after Save: %v", err)
	}
	if info.Size() == 0 {
		t.Error("saved file is empty")
	}

	// No leftover temp files should remain.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "watermark.json" {
			t.Errorf("unexpected file after atomic save: %s", e.Name())
		}
	}
}

// ---------------------------------------------------------------------------
// IsSessionUploaded tests
// ---------------------------------------------------------------------------

func TestIsSessionUploaded_True(t *testing.T) {
	wm := &Watermark{
		SessionIDsUploaded: map[string]string{
			"sess-1": "2025-01-01T00:00:00Z",
		},
	}
	if !wm.IsSessionUploaded("sess-1") {
		t.Error("expected true for existing session")
	}
}

func TestIsSessionUploaded_False(t *testing.T) {
	wm := &Watermark{
		SessionIDsUploaded: map[string]string{
			"sess-1": "2025-01-01T00:00:00Z",
		},
	}
	if wm.IsSessionUploaded("sess-999") {
		t.Error("expected false for missing session")
	}
}

func TestIsSessionUploaded_EmptyMap(t *testing.T) {
	wm := &Watermark{
		SessionIDsUploaded: make(map[string]string),
	}
	if wm.IsSessionUploaded("anything") {
		t.Error("expected false for empty map")
	}
}

// ---------------------------------------------------------------------------
// MarkSessionsUploaded tests
// ---------------------------------------------------------------------------

func TestMarkSessionsUploaded(t *testing.T) {
	wm := &Watermark{
		SessionIDsUploaded: make(map[string]string),
	}

	before := time.Now().UTC().Add(-time.Second)
	wm.MarkSessionsUploaded([]string{"a", "b", "c"})
	after := time.Now().UTC().Add(time.Second)

	if len(wm.SessionIDsUploaded) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(wm.SessionIDsUploaded))
	}

	for _, id := range []string{"a", "b", "c"} {
		ts, ok := wm.SessionIDsUploaded[id]
		if !ok {
			t.Errorf("session %q not found", id)
			continue
		}
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			t.Errorf("session %q: invalid timestamp %q: %v", id, ts, err)
			continue
		}
		if parsed.Before(before) || parsed.After(after) {
			t.Errorf("session %q: timestamp %v out of expected range [%v, %v]", id, parsed, before, after)
		}
	}

	// LastUploadTime should also be set.
	if wm.LastUploadTime == "" {
		t.Error("LastUploadTime not set")
	}
}

func TestMarkSessionsUploaded_Idempotent(t *testing.T) {
	wm := &Watermark{
		SessionIDsUploaded: make(map[string]string),
	}

	wm.MarkSessionsUploaded([]string{"x"})
	ts1 := wm.SessionIDsUploaded["x"]

	// Small sleep to ensure the timestamp could differ.
	time.Sleep(10 * time.Millisecond)

	wm.MarkSessionsUploaded([]string{"x"})
	ts2 := wm.SessionIDsUploaded["x"]

	// The map should still have exactly one entry.
	if len(wm.SessionIDsUploaded) != 1 {
		t.Errorf("expected 1 entry, got %d", len(wm.SessionIDsUploaded))
	}

	// Timestamp may be updated, but the map shouldn't break.
	if ts2 == "" {
		t.Error("timestamp should not be empty after second mark")
	}
	_ = ts1 // first timestamp consumed
}

// ---------------------------------------------------------------------------
// UpdateStatsDate tests
// ---------------------------------------------------------------------------

func TestUpdateStatsDate(t *testing.T) {
	wm := &Watermark{SessionIDsUploaded: make(map[string]string)}

	wm.UpdateStatsDate("2025-12-31")
	if wm.StatsCacheUploadedThrough != "2025-12-31" {
		t.Errorf("got %q, want %q", wm.StatsCacheUploadedThrough, "2025-12-31")
	}

	wm.UpdateStatsDate("2026-01-15")
	if wm.StatsCacheUploadedThrough != "2026-01-15" {
		t.Errorf("got %q after update, want %q", wm.StatsCacheUploadedThrough, "2026-01-15")
	}
}

// ---------------------------------------------------------------------------
// Compact tests
// ---------------------------------------------------------------------------

func TestCompact_RemovesOld(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour).Format(time.RFC3339)
	recent := now.Add(-1 * time.Hour).Format(time.RFC3339)

	wm := &Watermark{
		SessionIDsUploaded: map[string]string{
			"old-1":    old,
			"old-2":    old,
			"recent-1": recent,
		},
	}

	wm.Compact(24 * time.Hour)

	if len(wm.SessionIDsUploaded) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(wm.SessionIDsUploaded))
	}
	if _, ok := wm.SessionIDsUploaded["recent-1"]; !ok {
		t.Error("recent-1 should have been kept")
	}
	if _, ok := wm.SessionIDsUploaded["old-1"]; ok {
		t.Error("old-1 should have been removed")
	}
	if _, ok := wm.SessionIDsUploaded["old-2"]; ok {
		t.Error("old-2 should have been removed")
	}
}

func TestCompact_EmptyMap(t *testing.T) {
	wm := &Watermark{
		SessionIDsUploaded: make(map[string]string),
	}
	// Must not panic.
	wm.Compact(24 * time.Hour)

	if len(wm.SessionIDsUploaded) != 0 {
		t.Errorf("expected empty map, got %d entries", len(wm.SessionIDsUploaded))
	}
}

// ---------------------------------------------------------------------------
// Roundtrip test
// ---------------------------------------------------------------------------

func TestSave_Load_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.json")

	original := &Watermark{
		LastUploadTime:            "2025-10-10T10:10:10Z",
		StatsCacheUploadedThrough: "2025-10-10",
		SessionIDsUploaded: map[string]string{
			"sess-a": "2025-10-10T10:10:10Z",
			"sess-b": "2025-10-10T10:10:11Z",
			"sess-c": "2025-10-10T10:10:12Z",
		},
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.LastUploadTime != original.LastUploadTime {
		t.Errorf("LastUploadTime: got %q, want %q", loaded.LastUploadTime, original.LastUploadTime)
	}
	if loaded.StatsCacheUploadedThrough != original.StatsCacheUploadedThrough {
		t.Errorf("StatsCacheUploadedThrough: got %q, want %q", loaded.StatsCacheUploadedThrough, original.StatsCacheUploadedThrough)
	}
	if len(loaded.SessionIDsUploaded) != len(original.SessionIDsUploaded) {
		t.Fatalf("SessionIDsUploaded length: got %d, want %d", len(loaded.SessionIDsUploaded), len(original.SessionIDsUploaded))
	}
	for id, ts := range original.SessionIDsUploaded {
		if loaded.SessionIDsUploaded[id] != ts {
			t.Errorf("session %q: got %q, want %q", id, loaded.SessionIDsUploaded[id], ts)
		}
	}
}
