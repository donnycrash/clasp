package sync

import (
	"fmt"
	"strings"
)

// DiffResult tracks what changed during an org config installation.
type DiffResult struct {
	ClaudeMDUpdated bool
	SkillsAdded     []string
	SkillsUpdated   []string
	SettingsUpdated bool
	HooksUpdated    []string
}

// HasChanges returns true if any configuration was modified.
func (d *DiffResult) HasChanges() bool {
	return d.ClaudeMDUpdated ||
		len(d.SkillsAdded) > 0 ||
		len(d.SkillsUpdated) > 0 ||
		d.SettingsUpdated ||
		len(d.HooksUpdated) > 0
}

// String returns a human-readable summary of the changes.
func (d *DiffResult) String() string {
	if !d.HasChanges() {
		return "No changes detected."
	}

	var b strings.Builder

	if d.ClaudeMDUpdated {
		b.WriteString("  CLAUDE.md: updated org config section\n")
	}

	for _, s := range d.SkillsAdded {
		fmt.Fprintf(&b, "  Skill added: %s\n", s)
	}
	for _, s := range d.SkillsUpdated {
		fmt.Fprintf(&b, "  Skill updated: %s\n", s)
	}

	if d.SettingsUpdated {
		b.WriteString("  Settings: updated org settings\n")
	}

	for _, h := range d.HooksUpdated {
		fmt.Fprintf(&b, "  Hook updated: %s\n", h)
	}

	return b.String()
}
