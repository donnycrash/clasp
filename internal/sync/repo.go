package sync

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

// RepoManager handles git operations for the org config repository.
type RepoManager struct {
	repoURL   string
	branch    string
	localPath string
}

// NewRepoManager creates a RepoManager for the given repo URL, branch, and
// local checkout path.
func NewRepoManager(repoURL, branch, localPath string) *RepoManager {
	return &RepoManager{
		repoURL:   repoURL,
		branch:    branch,
		localPath: localPath,
	}
}

// Sync clones the repository if it does not exist locally, or pulls the latest
// changes if it does.
func (r *RepoManager) Sync() error {
	if err := checkGitInstalled(); err != nil {
		return err
	}

	if _, err := os.Stat(r.localPath); os.IsNotExist(err) {
		return r.clone()
	}

	return r.pull()
}

// LocalPath returns the local checkout directory.
func (r *RepoManager) LocalPath() string {
	return r.localPath
}

func (r *RepoManager) clone() error {
	slog.Info("cloning org config repo", "url", r.repoURL, "branch", r.branch, "path", r.localPath)

	cmd := exec.Command("git", "clone",
		"--branch", r.branch,
		"--single-branch",
		r.repoURL,
		r.localPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}
	return nil
}

func (r *RepoManager) pull() error {
	slog.Info("pulling latest org config", "path", r.localPath, "branch", r.branch)

	cmd := exec.Command("git", "-C", r.localPath, "pull", "--ff-only")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}
	return nil
}

func checkGitInstalled() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed or not in PATH: %w", err)
	}
	return nil
}
