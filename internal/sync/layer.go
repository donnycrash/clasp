package sync

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	orgMarkerBegin = "<!-- BEGIN ORG CONFIG - managed by clasp, do not edit -->"
	orgMarkerEnd   = "<!-- END ORG CONFIG -->"
)

// LayerInstaller installs org configurations into the Claude Code config directory.
type LayerInstaller struct {
	claudeDir string // e.g. ~/.claude
	repoDir   string // where the org config repo is checked out
}

// NewLayerInstaller creates a LayerInstaller for the given Claude directory and
// org config repo checkout.
func NewLayerInstaller(claudeDir, repoDir string) *LayerInstaller {
	return &LayerInstaller{
		claudeDir: claudeDir,
		repoDir:   repoDir,
	}
}

// Install applies the manifest entries to the Claude config directory and
// returns a summary of what changed.
func (l *LayerInstaller) Install(manifest *Manifest) (*DiffResult, error) {
	diff := &DiffResult{}

	if err := l.installClaudeMD(manifest.ClaudeMD, diff); err != nil {
		return diff, fmt.Errorf("installing CLAUDE.md: %w", err)
	}

	if err := l.installSkills(manifest.Skills, diff); err != nil {
		return diff, fmt.Errorf("installing skills: %w", err)
	}

	if err := l.installSettings(manifest.Settings, diff); err != nil {
		return diff, fmt.Errorf("installing settings: %w", err)
	}

	if err := l.installHooks(manifest.Hooks, diff); err != nil {
		return diff, fmt.Errorf("installing hooks: %w", err)
	}

	return diff, nil
}

// installClaudeMD reads all claude_md entries from the repo, concatenates them,
// and injects them between org markers in ~/.claude/CLAUDE.md.
func (l *LayerInstaller) installClaudeMD(entries []ClaudeMDEntry, diff *DiffResult) error {
	if len(entries) == 0 {
		return nil
	}

	// Build the org content block.
	var orgContent strings.Builder
	for _, e := range entries {
		src := filepath.Join(l.repoDir, e.Source)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("reading claude_md source %s: %w", e.Source, err)
		}
		orgContent.Write(data)
		// Ensure a trailing newline between entries.
		if len(data) > 0 && data[len(data)-1] != '\n' {
			orgContent.WriteByte('\n')
		}
	}

	newBlock := orgMarkerBegin + "\n" + orgContent.String() + orgMarkerEnd

	claudeMDPath := filepath.Join(l.claudeDir, "CLAUDE.md")

	existing, err := os.ReadFile(claudeMDPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading %s: %w", claudeMDPath, err)
	}

	content := string(existing)
	var updated string

	beginIdx := strings.Index(content, orgMarkerBegin)
	endIdx := strings.Index(content, orgMarkerEnd)

	if beginIdx >= 0 && endIdx >= 0 {
		// Replace existing block.
		updated = content[:beginIdx] + newBlock + content[endIdx+len(orgMarkerEnd):]
	} else {
		// Append markers and content.
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		if len(content) > 0 {
			content += "\n"
		}
		updated = content + newBlock + "\n"
	}

	if updated != string(existing) {
		if err := os.MkdirAll(filepath.Dir(claudeMDPath), 0o755); err != nil {
			return fmt.Errorf("creating directory for CLAUDE.md: %w", err)
		}
		if err := os.WriteFile(claudeMDPath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", claudeMDPath, err)
		}
		diff.ClaudeMDUpdated = true
		slog.Info("updated CLAUDE.md org config section")
	}

	return nil
}

// installSkills copies skill directories to ~/.claude/org/skills/.
func (l *LayerInstaller) installSkills(entries []SkillEntry, diff *DiffResult) error {
	if len(entries) == 0 {
		return nil
	}

	skillsDir := filepath.Join(l.claudeDir, "org", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("creating skills directory: %w", err)
	}

	for _, e := range entries {
		srcDir := filepath.Join(l.repoDir, e.Source)
		name := filepath.Base(e.Source)
		destDir := filepath.Join(skillsDir, name)

		existed := dirExists(destDir)

		if err := copyDir(srcDir, destDir); err != nil {
			return fmt.Errorf("copying skill %s: %w", e.Source, err)
		}

		if existed {
			diff.SkillsUpdated = append(diff.SkillsUpdated, name)
		} else {
			diff.SkillsAdded = append(diff.SkillsAdded, name)
		}
		slog.Info("installed skill", "name", name)
	}

	return nil
}

// installSettings reads settings JSON files from the repo and merges them into
// ~/.claude/org/settings.json.
func (l *LayerInstaller) installSettings(entries []SettingEntry, diff *DiffResult) error {
	if len(entries) == 0 {
		return nil
	}

	orgDir := filepath.Join(l.claudeDir, "org")
	if err := os.MkdirAll(orgDir, 0o755); err != nil {
		return fmt.Errorf("creating org directory: %w", err)
	}

	settingsPath := filepath.Join(orgDir, "settings.json")

	// Load existing settings if present.
	merged := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &merged); err != nil {
			slog.Warn("existing settings.json is invalid, overwriting", "error", err)
		}
	}

	// Merge each settings source on top.
	for _, e := range entries {
		src := filepath.Join(l.repoDir, e.Source)
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("reading settings source %s: %w", e.Source, err)
		}
		var incoming map[string]interface{}
		if err := json.Unmarshal(data, &incoming); err != nil {
			return fmt.Errorf("parsing settings source %s: %w", e.Source, err)
		}
		for k, v := range incoming {
			merged[k] = v
		}
	}

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling merged settings: %w", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(settingsPath, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", settingsPath, err)
	}

	diff.SettingsUpdated = true
	slog.Info("updated org settings")
	return nil
}

// installHooks copies hook scripts to ~/.claude/org/hooks/.
func (l *LayerInstaller) installHooks(entries []HookEntry, diff *DiffResult) error {
	if len(entries) == 0 {
		return nil
	}

	hooksDir := filepath.Join(l.claudeDir, "org", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("creating hooks directory: %w", err)
	}

	for _, e := range entries {
		srcPath := filepath.Join(l.repoDir, e.Source)
		name := filepath.Base(e.Source)
		destPath := filepath.Join(hooksDir, name)

		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("copying hook %s: %w", e.Source, err)
		}

		// Make hooks executable.
		if err := os.Chmod(destPath, 0o755); err != nil {
			return fmt.Errorf("chmod hook %s: %w", name, err)
		}

		diff.HooksUpdated = append(diff.HooksUpdated, name)
		slog.Info("installed hook", "name", name, "event", e.Event)
	}

	return nil
}

// dirExists returns true if the path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// copyFile copies a single file from src to dest.
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// copyDir recursively copies a directory tree from src to dest.
func copyDir(src, dest string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dest, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}

	return nil
}
