package sync

import (
	"strings"
	"testing"
)

func TestDiffResult_HasChanges_True(t *testing.T) {
	tests := []struct {
		name string
		diff DiffResult
	}{
		{
			name: "ClaudeMDUpdated",
			diff: DiffResult{ClaudeMDUpdated: true},
		},
		{
			name: "SkillsAdded",
			diff: DiffResult{SkillsAdded: []string{"review"}},
		},
		{
			name: "SkillsUpdated",
			diff: DiffResult{SkillsUpdated: []string{"linter"}},
		},
		{
			name: "SettingsUpdated",
			diff: DiffResult{SettingsUpdated: true},
		},
		{
			name: "HooksUpdated",
			diff: DiffResult{HooksUpdated: []string{"pre-push.sh"}},
		},
		{
			name: "Multiple",
			diff: DiffResult{
				ClaudeMDUpdated: true,
				SkillsAdded:     []string{"a"},
				SkillsUpdated:   []string{"b"},
				SettingsUpdated: true,
				HooksUpdated:    []string{"c.sh"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.diff.HasChanges() {
				t.Errorf("HasChanges() = false, want true for %s", tt.name)
			}
		})
	}
}

func TestDiffResult_HasChanges_False(t *testing.T) {
	diff := DiffResult{}
	if diff.HasChanges() {
		t.Error("HasChanges() = true for empty DiffResult, want false")
	}

	// Also test with empty slices (not nil).
	diff2 := DiffResult{
		SkillsAdded:   []string{},
		SkillsUpdated: []string{},
		HooksUpdated:  []string{},
	}
	if diff2.HasChanges() {
		t.Error("HasChanges() = true for DiffResult with empty slices, want false")
	}
}

func TestDiffResult_String_NoChanges(t *testing.T) {
	diff := DiffResult{}
	got := diff.String()
	if got != "No changes detected." {
		t.Errorf("String() = %q, want %q", got, "No changes detected.")
	}
}

func TestDiffResult_String_ClaudeMDUpdated(t *testing.T) {
	diff := DiffResult{ClaudeMDUpdated: true}
	got := diff.String()
	if !strings.Contains(got, "CLAUDE.md") {
		t.Errorf("String() = %q, expected to mention CLAUDE.md", got)
	}
}

func TestDiffResult_String_SkillsAdded(t *testing.T) {
	diff := DiffResult{SkillsAdded: []string{"review", "linter"}}
	got := diff.String()
	if !strings.Contains(got, "Skill added: review") {
		t.Errorf("String() = %q, expected 'Skill added: review'", got)
	}
	if !strings.Contains(got, "Skill added: linter") {
		t.Errorf("String() = %q, expected 'Skill added: linter'", got)
	}
}

func TestDiffResult_String_SkillsUpdated(t *testing.T) {
	diff := DiffResult{SkillsUpdated: []string{"formatter"}}
	got := diff.String()
	if !strings.Contains(got, "Skill updated: formatter") {
		t.Errorf("String() = %q, expected 'Skill updated: formatter'", got)
	}
}

func TestDiffResult_String_Settings(t *testing.T) {
	diff := DiffResult{SettingsUpdated: true}
	got := diff.String()
	if !strings.Contains(got, "Settings") {
		t.Errorf("String() = %q, expected to mention Settings", got)
	}
}

func TestDiffResult_String_Hooks(t *testing.T) {
	diff := DiffResult{HooksUpdated: []string{"pre-push.sh"}}
	got := diff.String()
	if !strings.Contains(got, "Hook updated: pre-push.sh") {
		t.Errorf("String() = %q, expected 'Hook updated: pre-push.sh'", got)
	}
}

func TestDiffResult_String_All(t *testing.T) {
	diff := DiffResult{
		ClaudeMDUpdated: true,
		SkillsAdded:     []string{"a"},
		SkillsUpdated:   []string{"b"},
		SettingsUpdated: true,
		HooksUpdated:    []string{"c.sh"},
	}
	got := diff.String()

	expected := []string{
		"CLAUDE.md",
		"Skill added: a",
		"Skill updated: b",
		"Settings",
		"Hook updated: c.sh",
	}
	for _, exp := range expected {
		if !strings.Contains(got, exp) {
			t.Errorf("String() = %q, expected to contain %q", got, exp)
		}
	}
}
