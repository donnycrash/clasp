//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const taskName = "CLASP"

// taskScheduler implements Scheduler for Windows using schtasks.
type taskScheduler struct{}

func newScheduler() Scheduler {
	return &taskScheduler{}
}

func (s *taskScheduler) Install(binaryPath string, interval time.Duration) error {
	_ = interval // Windows schtasks /sc daily doesn't support arbitrary intervals; use daily at 02:00.

	out, err := exec.Command(
		"schtasks", "/create",
		"/tn", taskName,
		"/tr", fmt.Sprintf(`"%s" run`, binaryPath),
		"/sc", "daily",
		"/st", "02:00",
		"/f",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *taskScheduler) Uninstall() error {
	out, err := exec.Command("schtasks", "/delete", "/tn", taskName, "/f").CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks delete: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (s *taskScheduler) IsInstalled() bool {
	err := exec.Command("schtasks", "/query", "/tn", taskName).Run()
	return err == nil
}

func (s *taskScheduler) Status() string {
	if !s.IsInstalled() {
		return "not installed"
	}

	out, err := exec.Command("schtasks", "/query", "/tn", taskName, "/fo", "LIST").CombinedOutput()
	if err != nil {
		return "installed (unable to query status)"
	}

	output := string(out)
	if strings.Contains(output, "Running") {
		return "installed and running"
	}
	if strings.Contains(output, "Ready") {
		return "installed and ready"
	}
	return "installed"
}
