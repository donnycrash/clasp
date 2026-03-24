package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupRepoWithFile creates a file in a temporary repo directory and returns
// the repo dir path.
func setupRepoWithFile(t *testing.T, relPath, content string) string {
	t.Helper()
	dir := t.TempDir()
	fullPath := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return dir
}

func TestInstall_ClaudeMD_NewFile(t *testing.T) {
	repoDir := setupRepoWithFile(t, "fragments/standards.md", "# Coding Standards\nUse gofmt.\n")
	claudeDir := t.TempDir()

	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		ClaudeMD: []ClaudeMDEntry{
			{Source: "fragments/standards.md", Scope: "global"},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !diff.ClaudeMDUpdated {
		t.Error("ClaudeMDUpdated should be true")
	}

	claudeMDPath := filepath.Join(claudeDir, "CLAUDE.md")
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, orgMarkerBegin) {
		t.Error("CLAUDE.md should contain begin marker")
	}
	if !strings.Contains(content, orgMarkerEnd) {
		t.Error("CLAUDE.md should contain end marker")
	}
	if !strings.Contains(content, "# Coding Standards") {
		t.Error("CLAUDE.md should contain the fragment content")
	}
	if !strings.Contains(content, "Use gofmt.") {
		t.Error("CLAUDE.md should contain 'Use gofmt.'")
	}
}

func TestInstall_ClaudeMD_ExistingWithMarkers(t *testing.T) {
	repoDir := setupRepoWithFile(t, "fragments/new-content.md", "New org content\n")
	claudeDir := t.TempDir()

	// Create an existing CLAUDE.md with markers and surrounding content.
	existingContent := "# My Project\n\nSome personal notes.\n\n" +
		orgMarkerBegin + "\nOld org content\n" + orgMarkerEnd + "\n\nMore personal notes.\n"
	claudeMDPath := filepath.Join(claudeDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte(existingContent), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		ClaudeMD: []ClaudeMDEntry{
			{Source: "fragments/new-content.md"},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !diff.ClaudeMDUpdated {
		t.Error("ClaudeMDUpdated should be true")
	}

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}

	content := string(data)
	// Old content should be replaced.
	if strings.Contains(content, "Old org content") {
		t.Error("CLAUDE.md should not contain 'Old org content'")
	}
	// New content should be present.
	if !strings.Contains(content, "New org content") {
		t.Error("CLAUDE.md should contain 'New org content'")
	}
	// Content outside markers should be preserved.
	if !strings.Contains(content, "# My Project") {
		t.Error("CLAUDE.md should preserve content before markers")
	}
	if !strings.Contains(content, "More personal notes.") {
		t.Error("CLAUDE.md should preserve content after markers")
	}
}

func TestInstall_ClaudeMD_ExistingNoMarkers(t *testing.T) {
	repoDir := setupRepoWithFile(t, "fragments/appended.md", "Appended org content\n")
	claudeDir := t.TempDir()

	// Create an existing CLAUDE.md without markers.
	existingContent := "# Existing Project Notes\n\nDo not remove this.\n"
	claudeMDPath := filepath.Join(claudeDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte(existingContent), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		ClaudeMD: []ClaudeMDEntry{
			{Source: "fragments/appended.md"},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !diff.ClaudeMDUpdated {
		t.Error("ClaudeMDUpdated should be true")
	}

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}

	content := string(data)
	// Existing content preserved.
	if !strings.Contains(content, "# Existing Project Notes") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(content, "Do not remove this.") {
		t.Error("existing content should be preserved")
	}
	// Markers and new content appended.
	if !strings.Contains(content, orgMarkerBegin) {
		t.Error("should contain begin marker")
	}
	if !strings.Contains(content, orgMarkerEnd) {
		t.Error("should contain end marker")
	}
	if !strings.Contains(content, "Appended org content") {
		t.Error("should contain appended content")
	}
}

func TestInstall_Skills(t *testing.T) {
	repoDir := t.TempDir()
	// Create a skill directory with files.
	skillDir := filepath.Join(repoDir, "skills", "code-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompt.md"), []byte("Review this code"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "config.json"), []byte(`{"enabled":true}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	claudeDir := t.TempDir()
	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		Skills: []SkillEntry{
			{Source: "skills/code-review", Tags: []string{"backend"}},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if len(diff.SkillsAdded) != 1 || diff.SkillsAdded[0] != "code-review" {
		t.Errorf("SkillsAdded = %v, want [code-review]", diff.SkillsAdded)
	}

	// Verify skill files were copied.
	destPrompt := filepath.Join(claudeDir, "org", "skills", "code-review", "prompt.md")
	data, err := os.ReadFile(destPrompt)
	if err != nil {
		t.Fatalf("read prompt.md: %v", err)
	}
	if string(data) != "Review this code" {
		t.Errorf("prompt.md content = %q", string(data))
	}

	destConfig := filepath.Join(claudeDir, "org", "skills", "code-review", "config.json")
	data, err = os.ReadFile(destConfig)
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	if string(data) != `{"enabled":true}` {
		t.Errorf("config.json content = %q", string(data))
	}
}

func TestInstall_Skills_Update(t *testing.T) {
	repoDir := t.TempDir()
	skillDir := filepath.Join(repoDir, "skills", "linter")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "run.sh"), []byte("#!/bin/bash\necho lint"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	claudeDir := t.TempDir()
	// Pre-create the skill directory so it counts as an update.
	existingSkill := filepath.Join(claudeDir, "org", "skills", "linter")
	if err := os.MkdirAll(existingSkill, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		Skills: []SkillEntry{
			{Source: "skills/linter"},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if len(diff.SkillsUpdated) != 1 || diff.SkillsUpdated[0] != "linter" {
		t.Errorf("SkillsUpdated = %v, want [linter]", diff.SkillsUpdated)
	}
	if len(diff.SkillsAdded) != 0 {
		t.Errorf("SkillsAdded = %v, want empty", diff.SkillsAdded)
	}
}

func TestInstall_Settings(t *testing.T) {
	repoDir := setupRepoWithFile(t, "settings/base.json", `{"theme":"dark","fontSize":14}`)
	claudeDir := t.TempDir()

	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		Settings: []SettingEntry{
			{Source: "settings/base.json"},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !diff.SettingsUpdated {
		t.Error("SettingsUpdated should be true")
	}

	settingsPath := filepath.Join(claudeDir, "org", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}

	if settings["theme"] != "dark" {
		t.Errorf("theme = %v, want 'dark'", settings["theme"])
	}
	if settings["fontSize"] != float64(14) {
		t.Errorf("fontSize = %v, want 14", settings["fontSize"])
	}
}

func TestInstall_Settings_Merge(t *testing.T) {
	repoDir := t.TempDir()
	// Two settings sources with overlapping and unique keys.
	s1Path := filepath.Join(repoDir, "settings", "base.json")
	s2Path := filepath.Join(repoDir, "settings", "override.json")
	if err := os.MkdirAll(filepath.Join(repoDir, "settings"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(s1Path, []byte(`{"theme":"light","fontSize":12}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(s2Path, []byte(`{"theme":"dark","language":"en"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	claudeDir := t.TempDir()
	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		Settings: []SettingEntry{
			{Source: "settings/base.json"},
			{Source: "settings/override.json"},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !diff.SettingsUpdated {
		t.Error("SettingsUpdated should be true")
	}

	settingsPath := filepath.Join(claudeDir, "org", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// "theme" should be overridden to "dark" by the second source.
	if settings["theme"] != "dark" {
		t.Errorf("theme = %v, want 'dark'", settings["theme"])
	}
	// "fontSize" from first source should be retained.
	if settings["fontSize"] != float64(12) {
		t.Errorf("fontSize = %v, want 12", settings["fontSize"])
	}
	// "language" from second source should be present.
	if settings["language"] != "en" {
		t.Errorf("language = %v, want 'en'", settings["language"])
	}
}

func TestInstall_Hooks(t *testing.T) {
	repoDir := setupRepoWithFile(t, "hooks/pre-push.sh", "#!/bin/bash\necho pre-push\n")
	claudeDir := t.TempDir()

	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		Hooks: []HookEntry{
			{Source: "hooks/pre-push.sh", Event: "pre-push"},
		},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if len(diff.HooksUpdated) != 1 || diff.HooksUpdated[0] != "pre-push.sh" {
		t.Errorf("HooksUpdated = %v, want [pre-push.sh]", diff.HooksUpdated)
	}

	hookPath := filepath.Join(claudeDir, "org", "hooks", "pre-push.sh")
	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read hook: %v", err)
	}
	if !strings.Contains(string(data), "echo pre-push") {
		t.Errorf("hook content = %q", string(data))
	}

	// Verify executable permissions.
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("stat hook: %v", err)
	}
	perm := info.Mode().Perm()
	if perm&0o111 == 0 {
		t.Errorf("hook permissions = %o, expected executable", perm)
	}
}

func TestInstall_DiffResult(t *testing.T) {
	// Set up a repo with all entry types.
	repoDir := t.TempDir()
	// ClaudeMD
	mdDir := filepath.Join(repoDir, "fragments")
	os.MkdirAll(mdDir, 0o755)
	os.WriteFile(filepath.Join(mdDir, "std.md"), []byte("standards\n"), 0o644)
	// Skill
	skillDir := filepath.Join(repoDir, "skills", "myskill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "prompt.md"), []byte("do stuff"), 0o644)
	// Hook
	hookDir := filepath.Join(repoDir, "hooks")
	os.MkdirAll(hookDir, 0o755)
	os.WriteFile(filepath.Join(hookDir, "lint.sh"), []byte("#!/bin/bash\nlint"), 0o644)
	// Settings
	settingsDir := filepath.Join(repoDir, "settings")
	os.MkdirAll(settingsDir, 0o755)
	os.WriteFile(filepath.Join(settingsDir, "base.json"), []byte(`{"k":"v"}`), 0o644)

	claudeDir := t.TempDir()
	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{
		ClaudeMD: []ClaudeMDEntry{{Source: "fragments/std.md"}},
		Skills:   []SkillEntry{{Source: "skills/myskill"}},
		Hooks:    []HookEntry{{Source: "hooks/lint.sh", Event: "pre-commit"}},
		Settings: []SettingEntry{{Source: "settings/base.json"}},
	}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if !diff.HasChanges() {
		t.Error("HasChanges should be true")
	}
	if !diff.ClaudeMDUpdated {
		t.Error("ClaudeMDUpdated should be true")
	}
	if len(diff.SkillsAdded) != 1 {
		t.Errorf("SkillsAdded = %v", diff.SkillsAdded)
	}
	if len(diff.HooksUpdated) != 1 {
		t.Errorf("HooksUpdated = %v", diff.HooksUpdated)
	}
	if !diff.SettingsUpdated {
		t.Error("SettingsUpdated should be true")
	}
}

func TestInstall_EmptyManifest(t *testing.T) {
	claudeDir := t.TempDir()
	repoDir := t.TempDir()

	installer := NewLayerInstaller(claudeDir, repoDir)
	manifest := &Manifest{}

	diff, err := installer.Install(manifest)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	if diff.HasChanges() {
		t.Error("empty manifest should produce no changes")
	}
}
