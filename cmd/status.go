package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/donnycrash/clasp/internal/config"
	"github.com/donnycrash/clasp/internal/watermark"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show CLASP status: watermark, pending sessions, and auth",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Load watermark.
	wmPath := config.WatermarkPath()
	wm, err := watermark.Load(wmPath)
	if err != nil {
		return fmt.Errorf("loading watermark: %w", err)
	}

	// Print watermark info.
	fmt.Println("=== Watermark ===")
	if wm.LastUploadTime != "" {
		fmt.Printf("Last upload:       %s\n", wm.LastUploadTime)
	} else {
		fmt.Println("Last upload:       never")
	}
	fmt.Printf("Uploaded sessions: %d\n", len(wm.SessionIDsUploaded))
	if wm.StatsCacheUploadedThrough != "" {
		fmt.Printf("Stats cursor:      %s\n", wm.StatsCacheUploadedThrough)
	} else {
		fmt.Println("Stats cursor:      none")
	}

	// Count pending sessions.
	claudeDir := config.ClaudeDataDir(cfg)
	sessionDir := filepath.Join(claudeDir, "usage-data", "session-meta")
	totalSessions := 0
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading session directory: %w", err)
		}
		// Directory does not exist — no sessions.
	} else {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				totalSessions++
			}
		}
	}
	pending := totalSessions - len(wm.SessionIDsUploaded)
	if pending < 0 {
		pending = 0
	}

	fmt.Println()
	fmt.Println("=== Sessions ===")
	fmt.Printf("Total on disk:     %d\n", totalSessions)
	fmt.Printf("Pending upload:    %d\n", pending)

	// Show auth status.
	fmt.Println()
	fmt.Println("=== Auth ===")
	provider, err := createProvider(cfg)
	if err != nil {
		fmt.Printf("Provider: %s (error: %v)\n", cfg.Auth.Provider, err)
		return nil
	}

	fmt.Printf("Provider:          %s\n", provider.Name())
	if provider.IsAuthenticated() {
		identity, err := provider.GetIdentity()
		if err != nil {
			fmt.Println("Status:            authenticated (could not get identity)")
		} else {
			fmt.Printf("Status:            authenticated as %s\n", identity.Username)
		}
	} else {
		fmt.Println("Status:            not authenticated")
	}

	return nil
}
