package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/donnycrash/clasp/internal/update"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "clasp",
	Short: "CLASP — Claude Analytics & Standards Platform",
	Long:  "Collects Claude Code usage data and syncs org-wide configurations.",
}

func Execute() {
	// Start background update check (non-blocking).
	updateCh := update.CheckUpdateNoticeAsync(Version)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Show update notice if available (wait up to 500ms).
	select {
	case latest := <-updateCh:
		if latest != "" {
			fmt.Fprintf(os.Stderr, "\nA new version of clasp is available: %s → %s\nRun `clasp update` to upgrade.\n", Version, latest)
		}
	case <-time.After(500 * time.Millisecond):
	}
}
