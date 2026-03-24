//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	serviceFileName = "clasp.service"
	timerFileName   = "clasp.timer"
)

// systemdScheduler implements Scheduler for Linux using systemd user units.
type systemdScheduler struct{}

func newScheduler() Scheduler {
	return &systemdScheduler{}
}

func userUnitDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user")
}

func (s *systemdScheduler) Install(binaryPath string, interval time.Duration) error {
	unitDir := userUnitDir()
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("creating systemd user unit directory: %w", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=CLASP — Claude Analytics & Standards Platform

[Service]
Type=oneshot
ExecStart=%s run
`, binaryPath)

	timerContent := fmt.Sprintf(`[Unit]
Description=CLASP periodic timer

[Timer]
OnBootSec=60
OnUnitActiveSec=%ds
Persistent=true

[Install]
WantedBy=timers.target
`, int(interval.Seconds()))

	servicePath := filepath.Join(unitDir, serviceFileName)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0o644); err != nil {
		return fmt.Errorf("writing service unit: %w", err)
	}

	timerPath := filepath.Join(unitDir, timerFileName)
	if err := os.WriteFile(timerPath, []byte(timerContent), 0o644); err != nil {
		return fmt.Errorf("writing timer unit: %w", err)
	}

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", "clasp.timer").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable clasp.timer: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (s *systemdScheduler) Uninstall() error {
	_ = exec.Command("systemctl", "--user", "disable", "--now", "clasp.timer").Run()

	unitDir := userUnitDir()
	os.Remove(filepath.Join(unitDir, timerFileName))
	os.Remove(filepath.Join(unitDir, serviceFileName))

	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	return nil
}

func (s *systemdScheduler) IsInstalled() bool {
	timerPath := filepath.Join(userUnitDir(), timerFileName)
	_, err := os.Stat(timerPath)
	return err == nil
}

func (s *systemdScheduler) Status() string {
	if !s.IsInstalled() {
		return "not installed"
	}

	out, err := exec.Command("systemctl", "--user", "is-active", "clasp.timer").CombinedOutput()
	if err != nil {
		return "installed but not active"
	}

	status := strings.TrimSpace(string(out))
	if status == "active" {
		return "installed and active"
	}
	return fmt.Sprintf("installed (%s)", status)
}
