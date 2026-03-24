package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/donnycrash/clasp/internal/config"
	"github.com/donnycrash/clasp/internal/sync"
	"github.com/spf13/cobra"
)

var syncDiff bool

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull org config repo and install configurations",
	Long:  "Syncs the org config repository and installs CLAUDE.md, skills, settings, and hooks into the Claude Code config directory.",
	RunE:  runSync,
}

func init() {
	syncCmd.Flags().BoolVar(&syncDiff, "diff", false, "Show what would change without applying (dry run)")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	return doSync(cfg, syncDiff)
}

// doSync performs the sync operation and is shared by both the sync and run commands.
func doSync(cfg *config.Config, dryRun bool) error {
	if cfg.Sync.Repo == "" {
		fmt.Fprintln(os.Stderr, "No sync repo configured. Set sync.repo in config.yaml.")
		return nil
	}

	// Determine local cache path.
	localCache := cfg.Sync.LocalCache
	if localCache == "" {
		localCache = filepath.Join(config.ConfigDir(), "org-config-repo")
	}

	// Pull the org config repo.
	repo := sync.NewRepoManager(cfg.Sync.Repo, cfg.Sync.Branch, localCache)
	if !dryRun {
		fmt.Println("Syncing org config repo...")
		if err := repo.Sync(); err != nil {
			return fmt.Errorf("syncing repo: %w", err)
		}
	}

	// Load and filter the manifest.
	manifest, err := sync.LoadManifest(repo.LocalPath())
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	manifest = sync.FilterByTags(manifest, cfg.Sync.Tags)

	// Resolve the Claude data directory.
	claudeDir := config.ClaudeDataDir(cfg)

	installer := sync.NewLayerInstaller(claudeDir, repo.LocalPath())

	if dryRun {
		fmt.Println("Dry run — showing what would be installed:")
		fmt.Printf("  CLAUDE.md entries: %d\n", len(manifest.ClaudeMD))
		fmt.Printf("  Skills: %d\n", len(manifest.Skills))
		fmt.Printf("  Settings entries: %d\n", len(manifest.Settings))
		fmt.Printf("  Hooks: %d\n", len(manifest.Hooks))
		return nil
	}

	result, err := installer.Install(manifest)
	if err != nil {
		return fmt.Errorf("installing org config: %w", err)
	}

	if result.HasChanges() {
		fmt.Println("Sync complete. Changes:")
		fmt.Print(result.String())
	} else {
		fmt.Println("Sync complete. No changes detected.")
	}

	return nil
}
