package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "clasp",
	Short: "CLASP — Claude Analytics & Standards Platform",
	Long:  "Collects Claude Code usage data and syncs org-wide configurations.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
