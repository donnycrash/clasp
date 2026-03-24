package cmd

import (
	"fmt"

	"github.com/donnycrash/clasp/internal/update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update CLASP to the latest version",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	fmt.Printf("Current version: %s\n", Version)

	fmt.Println("Checking for updates...")
	latest, err := update.CheckLatest()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	if !update.IsNewer(Version, latest) {
		fmt.Printf("Already up to date (%s)\n", latest)
		return nil
	}

	fmt.Printf("Updating to %s...\n", latest)
	if err := update.Upgrade(latest); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	fmt.Printf("Updated to %s\n", latest)
	return nil
}
