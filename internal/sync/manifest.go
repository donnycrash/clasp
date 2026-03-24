package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Manifest represents the org config repo's manifest.yaml that describes
// which configuration artifacts should be synced.
type Manifest struct {
	Version  int             `yaml:"version"`
	ClaudeMD []ClaudeMDEntry `yaml:"claude_md"`
	Skills   []SkillEntry    `yaml:"skills"`
	Hooks    []HookEntry     `yaml:"hooks"`
	Settings []SettingEntry  `yaml:"settings"`
}

// ClaudeMDEntry describes a CLAUDE.md fragment to inject.
type ClaudeMDEntry struct {
	Source string   `yaml:"source"`
	Scope  string   `yaml:"scope"`
	Tags   []string `yaml:"tags"`
}

// SkillEntry describes a skill directory to install.
type SkillEntry struct {
	Source string   `yaml:"source"`
	Tags   []string `yaml:"tags"`
}

// HookEntry describes a hook script to install.
type HookEntry struct {
	Source string   `yaml:"source"`
	Event  string   `yaml:"event"`
	Tags   []string `yaml:"tags"`
}

// SettingEntry describes a settings file to merge.
type SettingEntry struct {
	Source string   `yaml:"source"`
	Tags   []string `yaml:"tags"`
}

// LoadManifest reads and parses the manifest.yaml from the given repo directory.
func LoadManifest(repoDir string) (*Manifest, error) {
	path := filepath.Join(repoDir, "manifest.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest %s: %w", path, err)
	}

	return &m, nil
}

// FilterByTags returns a copy of the manifest containing only entries whose
// tags overlap with the provided tags. Entries with no tags always pass the
// filter. If tags is empty, all entries are included.
func FilterByTags(m *Manifest, tags []string) *Manifest {
	if len(tags) == 0 {
		return m
	}

	tagSet := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		tagSet[t] = struct{}{}
	}

	filtered := &Manifest{
		Version: m.Version,
	}

	for _, e := range m.ClaudeMD {
		if matchesTags(e.Tags, tagSet) {
			filtered.ClaudeMD = append(filtered.ClaudeMD, e)
		}
	}
	for _, e := range m.Skills {
		if matchesTags(e.Tags, tagSet) {
			filtered.Skills = append(filtered.Skills, e)
		}
	}
	for _, e := range m.Hooks {
		if matchesTags(e.Tags, tagSet) {
			filtered.Hooks = append(filtered.Hooks, e)
		}
	}
	for _, e := range m.Settings {
		if matchesTags(e.Tags, tagSet) {
			filtered.Settings = append(filtered.Settings, e)
		}
	}

	return filtered
}

// matchesTags returns true if the entry has no tags (always passes) or if at
// least one of the entry's tags is in the given set.
func matchesTags(entryTags []string, tagSet map[string]struct{}) bool {
	if len(entryTags) == 0 {
		return true
	}
	for _, t := range entryTags {
		if _, ok := tagSet[t]; ok {
			return true
		}
	}
	return false
}
