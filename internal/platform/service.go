package platform

import "time"

// Scheduler is the common interface for OS-level scheduling of the CLASP
// background process.
type Scheduler interface {
	// Install registers the CLASP binary as a recurring background task.
	Install(binaryPath string, interval time.Duration) error

	// Uninstall removes the registered background task.
	Uninstall() error

	// IsInstalled reports whether a background task is currently registered.
	IsInstalled() bool

	// Status returns a human-readable description of the scheduler state.
	Status() string
}

// NewScheduler returns the platform-appropriate Scheduler implementation.
// The concrete type is selected at compile time via build tags.
func NewScheduler() Scheduler {
	return newScheduler()
}
