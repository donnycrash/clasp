package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/donnycrash/clasp/internal/config"
	"github.com/donnycrash/clasp/internal/platform"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Register CLASP as a background scheduled task",
	RunE: func(cmd *cobra.Command, args []string) error {
		binaryPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determining executable path: %w", err)
		}

		interval, err := scheduleInterval()
		if err != nil {
			return err
		}

		scheduler := platform.NewScheduler()

		if scheduler.IsInstalled() {
			fmt.Println("CLASP is already installed. Reinstalling...")
			if err := scheduler.Uninstall(); err != nil {
				return fmt.Errorf("uninstalling existing schedule: %w", err)
			}
		}

		if err := scheduler.Install(binaryPath, interval); err != nil {
			return fmt.Errorf("installing schedule: %w", err)
		}

		fmt.Printf("CLASP installed successfully (interval: %s).\n", interval)
		fmt.Printf("Status: %s\n", scheduler.Status())
		return nil
	},
}

var purgeFlag bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove CLASP background scheduled task",
	RunE: func(cmd *cobra.Command, args []string) error {
		scheduler := platform.NewScheduler()

		if !scheduler.IsInstalled() {
			fmt.Println("CLASP is not currently installed as a scheduled task.")
		} else {
			if err := scheduler.Uninstall(); err != nil {
				return fmt.Errorf("uninstalling: %w", err)
			}
			fmt.Println("CLASP scheduled task removed.")
		}

		if purgeFlag {
			configDir := config.ConfigDir()
			if err := os.RemoveAll(configDir); err != nil {
				return fmt.Errorf("removing config directory %s: %w", configDir, err)
			}
			fmt.Printf("Purged configuration directory: %s\n", configDir)
		}

		return nil
	},
}

func init() {
	uninstallCmd.Flags().BoolVar(&purgeFlag, "purge", false, "Also remove ~/.config/clasp/ directory")
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}

// scheduleInterval loads the config and parses the schedule_interval field.
func scheduleInterval() (time.Duration, error) {
	cfg, err := config.Load(config.ConfigPath())
	if err != nil {
		// If no config file exists, use the default.
		cfg = config.Default()
	}

	d, err := time.ParseDuration(cfg.ScheduleInterval)
	if err != nil {
		return 0, fmt.Errorf("parsing schedule_interval %q: %w", cfg.ScheduleInterval, err)
	}
	return d, nil
}
