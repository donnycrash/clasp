package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifest_Valid(t *testing.T) {
	dir := t.TempDir()
	content := `version: 2
claude_md:
  - source: fragments/coding-standards.md
    scope: global
    tags: [backend, frontend]
  - source: fragments/security.md
    scope: project
skills:
  - source: skills/review
    tags: [backend]
hooks:
  - source: hooks/pre-push.sh
    event: pre-push
    tags: [ci]
settings:
  - source: settings/base.json
    tags: [frontend]
`
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	if m.Version != 2 {
		t.Errorf("Version = %d, want 2", m.Version)
	}

	if len(m.ClaudeMD) != 2 {
		t.Fatalf("ClaudeMD length = %d, want 2", len(m.ClaudeMD))
	}
	if m.ClaudeMD[0].Source != "fragments/coding-standards.md" {
		t.Errorf("ClaudeMD[0].Source = %q", m.ClaudeMD[0].Source)
	}
	if m.ClaudeMD[0].Scope != "global" {
		t.Errorf("ClaudeMD[0].Scope = %q", m.ClaudeMD[0].Scope)
	}
	if len(m.ClaudeMD[0].Tags) != 2 || m.ClaudeMD[0].Tags[0] != "backend" || m.ClaudeMD[0].Tags[1] != "frontend" {
		t.Errorf("ClaudeMD[0].Tags = %v", m.ClaudeMD[0].Tags)
	}
	if m.ClaudeMD[1].Source != "fragments/security.md" {
		t.Errorf("ClaudeMD[1].Source = %q", m.ClaudeMD[1].Source)
	}
	if len(m.ClaudeMD[1].Tags) != 0 {
		t.Errorf("ClaudeMD[1].Tags = %v, expected empty", m.ClaudeMD[1].Tags)
	}

	if len(m.Skills) != 1 {
		t.Fatalf("Skills length = %d, want 1", len(m.Skills))
	}
	if m.Skills[0].Source != "skills/review" {
		t.Errorf("Skills[0].Source = %q", m.Skills[0].Source)
	}

	if len(m.Hooks) != 1 {
		t.Fatalf("Hooks length = %d, want 1", len(m.Hooks))
	}
	if m.Hooks[0].Source != "hooks/pre-push.sh" || m.Hooks[0].Event != "pre-push" {
		t.Errorf("Hooks[0] = %+v", m.Hooks[0])
	}

	if len(m.Settings) != 1 {
		t.Fatalf("Settings length = %d, want 1", len(m.Settings))
	}
	if m.Settings[0].Source != "settings/base.json" {
		t.Errorf("Settings[0].Source = %q", m.Settings[0].Source)
	}
}

func TestLoadManifest_FileNotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("LoadManifest should return error when manifest.yaml does not exist")
	}
}

func TestLoadManifest_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	content := `{{{not valid yaml::::`
	if err := os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	_, err := LoadManifest(dir)
	if err == nil {
		t.Fatal("LoadManifest should return error for invalid YAML")
	}
}

func TestFilterByTags_NoTags(t *testing.T) {
	m := &Manifest{
		Version: 1,
		ClaudeMD: []ClaudeMDEntry{
			{Source: "a.md"},       // no tags
			{Source: "b.md", Tags: []string{}}, // empty tags
		},
	}

	// Filter with a specific tag. Entries with no tags should pass.
	filtered := FilterByTags(m, []string{"backend"})
	if len(filtered.ClaudeMD) != 2 {
		t.Errorf("ClaudeMD length = %d, want 2 (entries with no tags always pass)", len(filtered.ClaudeMD))
	}
}

func TestFilterByTags_MatchingTag(t *testing.T) {
	m := &Manifest{
		Version: 1,
		Skills: []SkillEntry{
			{Source: "skills/a", Tags: []string{"backend"}},
			{Source: "skills/b", Tags: []string{"frontend"}},
		},
	}

	filtered := FilterByTags(m, []string{"backend"})
	if len(filtered.Skills) != 1 {
		t.Fatalf("Skills length = %d, want 1", len(filtered.Skills))
	}
	if filtered.Skills[0].Source != "skills/a" {
		t.Errorf("filtered Skills[0].Source = %q, want %q", filtered.Skills[0].Source, "skills/a")
	}
}

func TestFilterByTags_NoMatch(t *testing.T) {
	m := &Manifest{
		Version: 1,
		Hooks: []HookEntry{
			{Source: "hooks/a.sh", Event: "pre-push", Tags: []string{"ci"}},
		},
	}

	filtered := FilterByTags(m, []string{"backend"})
	if len(filtered.Hooks) != 0 {
		t.Errorf("Hooks length = %d, want 0 (no matching tags)", len(filtered.Hooks))
	}
}

func TestFilterByTags_MultipleTagsOnEntry(t *testing.T) {
	m := &Manifest{
		Version: 1,
		ClaudeMD: []ClaudeMDEntry{
			{Source: "a.md", Tags: []string{"frontend", "mobile"}},
		},
	}

	// Filter with "mobile" - should match.
	filtered := FilterByTags(m, []string{"mobile"})
	if len(filtered.ClaudeMD) != 1 {
		t.Fatalf("ClaudeMD length = %d, want 1 (entry has 'mobile' tag)", len(filtered.ClaudeMD))
	}

	// Filter with "frontend" - should also match.
	filtered = FilterByTags(m, []string{"frontend"})
	if len(filtered.ClaudeMD) != 1 {
		t.Fatalf("ClaudeMD length = %d, want 1 (entry has 'frontend' tag)", len(filtered.ClaudeMD))
	}
}

func TestFilterByTags_EmptyFilterTags(t *testing.T) {
	m := &Manifest{
		Version: 1,
		ClaudeMD: []ClaudeMDEntry{
			{Source: "a.md", Tags: []string{"backend"}},
			{Source: "b.md", Tags: []string{"frontend"}},
			{Source: "c.md"},
		},
		Skills: []SkillEntry{
			{Source: "skills/x", Tags: []string{"ci"}},
		},
	}

	// Empty filter tags means all entries pass.
	filtered := FilterByTags(m, []string{})
	if len(filtered.ClaudeMD) != 3 {
		t.Errorf("ClaudeMD length = %d, want 3 (no filtering with empty tags)", len(filtered.ClaudeMD))
	}
	if len(filtered.Skills) != 1 {
		t.Errorf("Skills length = %d, want 1", len(filtered.Skills))
	}
	// With nil filter tags, the original manifest is returned directly.
	filtered = FilterByTags(m, nil)
	if filtered != m {
		t.Error("FilterByTags with nil tags should return original manifest pointer")
	}
}

func TestFilterByTags_AllTypes(t *testing.T) {
	m := &Manifest{
		Version: 1,
		ClaudeMD: []ClaudeMDEntry{
			{Source: "a.md", Tags: []string{"backend"}},
			{Source: "b.md", Tags: []string{"frontend"}},
		},
		Skills: []SkillEntry{
			{Source: "skills/a", Tags: []string{"backend"}},
			{Source: "skills/b", Tags: []string{"frontend"}},
		},
		Hooks: []HookEntry{
			{Source: "hooks/a.sh", Tags: []string{"backend"}},
			{Source: "hooks/b.sh", Tags: []string{"frontend"}},
		},
		Settings: []SettingEntry{
			{Source: "s1.json", Tags: []string{"backend"}},
			{Source: "s2.json", Tags: []string{"frontend"}},
		},
	}

	filtered := FilterByTags(m, []string{"backend"})
	if len(filtered.ClaudeMD) != 1 || filtered.ClaudeMD[0].Source != "a.md" {
		t.Errorf("ClaudeMD filter failed: %+v", filtered.ClaudeMD)
	}
	if len(filtered.Skills) != 1 || filtered.Skills[0].Source != "skills/a" {
		t.Errorf("Skills filter failed: %+v", filtered.Skills)
	}
	if len(filtered.Hooks) != 1 || filtered.Hooks[0].Source != "hooks/a.sh" {
		t.Errorf("Hooks filter failed: %+v", filtered.Hooks)
	}
	if len(filtered.Settings) != 1 || filtered.Settings[0].Source != "s1.json" {
		t.Errorf("Settings filter failed: %+v", filtered.Settings)
	}
}
