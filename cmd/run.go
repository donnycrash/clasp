package cmd

import (
	"fmt"

	"github.com/donnycrash/clasp/internal/config"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run upload and sync in sequence",
	Long:  "Runs the upload step followed by sync (if auto_sync is enabled). This is the command the OS scheduler calls.",
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Step 1: Upload.
	fmt.Println("Running upload...")
	if err := doUpload(cfg); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	fmt.Println("Upload complete.")

	// Step 2: Sync (if auto_sync is enabled).
	if cfg.Sync.AutoSync {
		fmt.Println("Running sync...")
		if err := doSync(cfg, false); err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}
	} else {
		fmt.Println("Auto-sync is disabled, skipping sync step.")
	}

	return nil
}
