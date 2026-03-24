package redactor

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/donnycrash/clasp/internal/collector"
)

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// applyAction tests
// ---------------------------------------------------------------------------

func TestApplyAction_Keep(t *testing.T) {
	input := "some/project/path"
	got := applyAction(Keep, input)
	if got != input {
		t.Errorf("Keep: got %q, want %q", got, input)
	}
}

func TestApplyAction_Hash(t *testing.T) {
	input := "some/project/path"
	got := applyAction(Hash, input)
	want := sha256Hex(input)
	if got != want {
		t.Errorf("Hash: got %q, want %q", got, want)
	}
	// Verify it looks like a hex-encoded SHA-256 (64 hex chars).
	if len(got) != 64 {
		t.Errorf("Hash: expected 64-char hex string, got length %d", len(got))
	}
}

func TestApplyAction_Omit(t *testing.T) {
	input := "sensitive data"
	got := applyAction(Omit, input)
	if got != "" {
		t.Errorf("Omit: got %q, want empty string", got)
	}
}

func TestApplyAction_EmptyString(t *testing.T) {
	// Hash of empty string should be the well-known SHA-256 of "".
	gotHash := applyAction(Hash, "")
	wantHash := sha256Hex("")
	if gotHash != wantHash {
		t.Errorf("Hash of empty string: got %q, want %q", gotHash, wantHash)
	}

	// Omit of empty string should still be empty.
	gotOmit := applyAction(Omit, "")
	if gotOmit != "" {
		t.Errorf("Omit of empty string: got %q, want empty", gotOmit)
	}
}

// ---------------------------------------------------------------------------
// DefaultRules tests
// ---------------------------------------------------------------------------

func TestDefaultRules(t *testing.T) {
	r := DefaultRules()

	checks := []struct {
		name string
		got  Action
		want Action
	}{
		{"ProjectPath", r.ProjectPath, Hash},
		{"FirstPrompt", r.FirstPrompt, Omit},
		{"BriefSummary", r.BriefSummary, Omit},
		{"UnderlyingGoal", r.UnderlyingGoal, Omit},
		{"FrictionDetail", r.FrictionDetail, Omit},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("DefaultRules().%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// RulesFromConfig tests
// ---------------------------------------------------------------------------

func TestRulesFromConfig_AllKeep(t *testing.T) {
	cfg := RedactionConfig{
		ProjectPath:    "keep",
		FirstPrompt:    "keep",
		BriefSummary:   "keep",
		UnderlyingGoal: "keep",
		FrictionDetail: "keep",
	}
	r := RulesFromConfig(cfg)

	for _, pair := range []struct {
		name string
		got  Action
	}{
		{"ProjectPath", r.ProjectPath},
		{"FirstPrompt", r.FirstPrompt},
		{"BriefSummary", r.BriefSummary},
		{"UnderlyingGoal", r.UnderlyingGoal},
		{"FrictionDetail", r.FrictionDetail},
	} {
		if pair.got != Keep {
			t.Errorf("AllKeep: %s = %q, want %q", pair.name, pair.got, Keep)
		}
	}
}

func TestRulesFromConfig_AllHash(t *testing.T) {
	cfg := RedactionConfig{
		ProjectPath:    "hash",
		FirstPrompt:    "hash",
		BriefSummary:   "hash",
		UnderlyingGoal: "hash",
		FrictionDetail: "hash",
	}
	r := RulesFromConfig(cfg)

	for _, pair := range []struct {
		name string
		got  Action
	}{
		{"ProjectPath", r.ProjectPath},
		{"FirstPrompt", r.FirstPrompt},
		{"BriefSummary", r.BriefSummary},
		{"UnderlyingGoal", r.UnderlyingGoal},
		{"FrictionDetail", r.FrictionDetail},
	} {
		if pair.got != Hash {
			t.Errorf("AllHash: %s = %q, want %q", pair.name, pair.got, Hash)
		}
	}
}

func TestRulesFromConfig_AllOmit(t *testing.T) {
	cfg := RedactionConfig{
		ProjectPath:    "omit",
		FirstPrompt:    "omit",
		BriefSummary:   "omit",
		UnderlyingGoal: "omit",
		FrictionDetail: "omit",
	}
	r := RulesFromConfig(cfg)

	for _, pair := range []struct {
		name string
		got  Action
	}{
		{"ProjectPath", r.ProjectPath},
		{"FirstPrompt", r.FirstPrompt},
		{"BriefSummary", r.BriefSummary},
		{"UnderlyingGoal", r.UnderlyingGoal},
		{"FrictionDetail", r.FrictionDetail},
	} {
		if pair.got != Omit {
			t.Errorf("AllOmit: %s = %q, want %q", pair.name, pair.got, Omit)
		}
	}
}

func TestRulesFromConfig_Empty(t *testing.T) {
	// An entirely empty config should produce the same result as DefaultRules.
	r := RulesFromConfig(RedactionConfig{})
	d := DefaultRules()

	if r != d {
		t.Errorf("Empty config: got %+v, want %+v", r, d)
	}
}

func TestRulesFromConfig_InvalidAction(t *testing.T) {
	cfg := RedactionConfig{
		ProjectPath:    "invalid",
		FirstPrompt:    "KEEP",  // wrong case
		BriefSummary:   "delete",
		UnderlyingGoal: "redact",
		FrictionDetail: "purge",
	}
	r := RulesFromConfig(cfg)
	d := DefaultRules()

	// Every invalid value should fall back to its default.
	if r != d {
		t.Errorf("Invalid config: got %+v, want defaults %+v", r, d)
	}
}

// ---------------------------------------------------------------------------
// Apply tests
// ---------------------------------------------------------------------------

func TestApply_FullData(t *testing.T) {
	rules := Rules{
		ProjectPath:    Hash,
		FirstPrompt:    Omit,
		BriefSummary:   Keep,
		UnderlyingGoal: Hash,
		FrictionDetail: Omit,
	}

	data := &collector.CollectedData{
		Sessions: []collector.JoinedSession{
			{
				SessionMeta: collector.SessionMeta{
					SessionID:   "s1",
					ProjectPath: "/home/user/project",
					FirstPrompt: "build me a thing",
				},
				Facets: &collector.Facets{
					BriefSummary:   "a brief summary",
					UnderlyingGoal: "the goal",
					FrictionDetail: "some friction",
				},
			},
		},
	}

	rules.Apply(data)

	s := data.Sessions[0]

	// ProjectPath should be hashed.
	wantPath := sha256Hex("/home/user/project")
	if s.ProjectPath != wantPath {
		t.Errorf("ProjectPath: got %q, want %q", s.ProjectPath, wantPath)
	}

	// FirstPrompt should be omitted.
	if s.FirstPrompt != "" {
		t.Errorf("FirstPrompt: got %q, want empty", s.FirstPrompt)
	}

	// BriefSummary should be kept.
	if s.Facets.BriefSummary != "a brief summary" {
		t.Errorf("BriefSummary: got %q, want %q", s.Facets.BriefSummary, "a brief summary")
	}

	// UnderlyingGoal should be hashed.
	wantGoal := sha256Hex("the goal")
	if s.Facets.UnderlyingGoal != wantGoal {
		t.Errorf("UnderlyingGoal: got %q, want %q", s.Facets.UnderlyingGoal, wantGoal)
	}

	// FrictionDetail should be omitted.
	if s.Facets.FrictionDetail != "" {
		t.Errorf("FrictionDetail: got %q, want empty", s.Facets.FrictionDetail)
	}
}

func TestApply_NilFacets(t *testing.T) {
	rules := DefaultRules()

	data := &collector.CollectedData{
		Sessions: []collector.JoinedSession{
			{
				SessionMeta: collector.SessionMeta{
					SessionID:   "s1",
					ProjectPath: "/tmp/proj",
					FirstPrompt: "hello",
				},
				Facets: nil, // no facets
			},
		},
	}

	// Must not panic.
	rules.Apply(data)

	if data.Sessions[0].Facets != nil {
		t.Error("Facets should still be nil after Apply")
	}
}

func TestApply_NilData(t *testing.T) {
	rules := DefaultRules()
	// Must not panic.
	rules.Apply(nil)
}

func TestApply_MultipleSessionsDifferentPaths(t *testing.T) {
	rules := Rules{
		ProjectPath:    Hash,
		FirstPrompt:    Keep,
		BriefSummary:   Keep,
		UnderlyingGoal: Keep,
		FrictionDetail: Keep,
	}

	data := &collector.CollectedData{
		Sessions: []collector.JoinedSession{
			{SessionMeta: collector.SessionMeta{SessionID: "a", ProjectPath: "/path/alpha"}},
			{SessionMeta: collector.SessionMeta{SessionID: "b", ProjectPath: "/path/beta"}},
			{SessionMeta: collector.SessionMeta{SessionID: "c", ProjectPath: "/path/gamma"}},
		},
	}

	rules.Apply(data)

	hashes := make(map[string]bool)
	for _, s := range data.Sessions {
		if len(s.ProjectPath) != 64 {
			t.Fatalf("Expected 64-char hex hash, got %q", s.ProjectPath)
		}
		if hashes[s.ProjectPath] {
			t.Errorf("Duplicate hash detected: %q — different paths should produce different hashes", s.ProjectPath)
		}
		hashes[s.ProjectPath] = true
	}

	if len(hashes) != 3 {
		t.Errorf("Expected 3 unique hashes, got %d", len(hashes))
	}
}

func TestApply_HashDeterministic(t *testing.T) {
	rules := Rules{
		ProjectPath:    Hash,
		FirstPrompt:    Hash,
		BriefSummary:   Hash,
		UnderlyingGoal: Hash,
		FrictionDetail: Hash,
	}

	makeData := func() *collector.CollectedData {
		return &collector.CollectedData{
			Sessions: []collector.JoinedSession{
				{
					SessionMeta: collector.SessionMeta{
						SessionID:   "s1",
						ProjectPath: "/deterministic/path",
						FirstPrompt: "same prompt",
					},
					Facets: &collector.Facets{
						BriefSummary:   "summary",
						UnderlyingGoal: "goal",
						FrictionDetail: "friction",
					},
				},
			},
		}
	}

	d1 := makeData()
	d2 := makeData()
	rules.Apply(d1)
	rules.Apply(d2)

	s1 := d1.Sessions[0]
	s2 := d2.Sessions[0]

	if s1.ProjectPath != s2.ProjectPath {
		t.Error("ProjectPath hashes differ between runs")
	}
	if s1.FirstPrompt != s2.FirstPrompt {
		t.Error("FirstPrompt hashes differ between runs")
	}
	if s1.Facets.BriefSummary != s2.Facets.BriefSummary {
		t.Error("BriefSummary hashes differ between runs")
	}
	if s1.Facets.UnderlyingGoal != s2.Facets.UnderlyingGoal {
		t.Error("UnderlyingGoal hashes differ between runs")
	}
	if s1.Facets.FrictionDetail != s2.Facets.FrictionDetail {
		t.Error("FrictionDetail hashes differ between runs")
	}
}
