//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/donnycrash/clasp/internal/config"
)

const (
	plistLabel = "com.lumenalta.clasp"
	plistName  = "com.lumenalta.clasp.plist"
)

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{ .Label }}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{ .BinaryPath }}</string>
        <string>run</string>
    </array>
    <key>StartInterval</key>
    <integer>{{ .IntervalSeconds }}</integer>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>{{ .StdoutLog }}</string>
    <key>StandardErrorPath</key>
    <string>{{ .StderrLog }}</string>
</dict>
</plist>
`))

type plistData struct {
	Label           string
	BinaryPath      string
	IntervalSeconds int
	StdoutLog       string
	StderrLog       string
}

// launchdScheduler implements Scheduler for macOS using launchd.
type launchdScheduler struct{}

func newScheduler() Scheduler {
	return &launchdScheduler{}
}

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistName)
}

func (s *launchdScheduler) Install(binaryPath string, interval time.Duration) error {
	configDir := config.ConfigDir()
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data := plistData{
		Label:           plistLabel,
		BinaryPath:      binaryPath,
		IntervalSeconds: int(interval.Seconds()),
		StdoutLog:       filepath.Join(configDir, "clasp.stdout.log"),
		StderrLog:       filepath.Join(configDir, "clasp.stderr.log"),
	}

	// Ensure LaunchAgents directory exists.
	plist := plistPath()
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		return fmt.Errorf("creating LaunchAgents directory: %w", err)
	}

	f, err := os.Create(plist)
	if err != nil {
		return fmt.Errorf("creating plist file: %w", err)
	}
	defer f.Close()

	if err := plistTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	// Load the agent.
	out, err := exec.Command("launchctl", "load", plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl load: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (s *launchdScheduler) Uninstall() error {
	plist := plistPath()

	if _, err := os.Stat(plist); os.IsNotExist(err) {
		return nil // nothing to uninstall
	}

	out, err := exec.Command("launchctl", "unload", plist).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl unload: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if err := os.Remove(plist); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing plist: %w", err)
	}

	return nil
}

func (s *launchdScheduler) IsInstalled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}

func (s *launchdScheduler) Status() string {
	if !s.IsInstalled() {
		return "not installed"
	}

	out, err := exec.Command("launchctl", "list", plistLabel).CombinedOutput()
	if err != nil {
		return "installed but not loaded"
	}

	if strings.Contains(string(out), plistLabel) {
		return "installed and loaded"
	}

	return "installed (status unknown)"
}
