// Package updater provides git-based update checking and applying for llamactl.
package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andermurias/llamactl/internal/config"
)

// UpdateStatus contains the result of a git-based update check.
type UpdateStatus struct {
	HasUpdate     bool
	CommitsBehind int
	LocalCommit   string
	RemoteCommit  string
	Branch        string
}

// Check performs a git fetch and compares local vs remote HEAD.
func Check(cfg *config.Config) (*UpdateStatus, error) {
	repo := cfg.AIDir
	if out, err := gitCmd(repo, "fetch", "--quiet", "origin"); err != nil {
		return nil, fmt.Errorf("git fetch: %w\n%s", err, out)
	}
	branch, _ := gitOutput(repo, "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)
	behind, _ := commitCount(repo, "HEAD", "origin/"+branch)
	local, _ := gitOutput(repo, "rev-parse", "--short", "HEAD")
	remote, _ := gitOutput(repo, "rev-parse", "--short", "origin/"+branch)
	return &UpdateStatus{
		HasUpdate:     behind > 0,
		CommitsBehind: behind,
		LocalCommit:   strings.TrimSpace(local),
		RemoteCommit:  strings.TrimSpace(remote),
		Branch:        branch,
	}, nil
}

// Apply pulls the latest commits and installs the binary from bin/llamactl.
func Apply(cfg *config.Config) (int, error) {
	repo := cfg.AIDir
	branch, _ := gitOutput(repo, "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)
	behind, _ := commitCount(repo, "HEAD", "origin/"+branch)
	if out, err := gitCmd(repo, "pull", "--ff-only", "origin", branch); err != nil {
		return 0, fmt.Errorf("git pull: %w\n%s", err, out)
	}
	if err := installBinary(cfg); err != nil {
		return behind, err
	}
	return behind, nil
}

// InstallBinary copies bin/llamactl from the repo to the system install path.
func InstallBinary(cfg *config.Config) error { return installBinary(cfg) }

// BinaryPath returns the path of the committed binary inside the repo.
func BinaryPath(cfg *config.Config) string {
	return filepath.Join(cfg.AIDir, "llamactl", "bin", "llamactl")
}

// InstallPath returns the system install path for the llamactl binary.
func InstallPath() string {
	if _, err := os.Stat("/opt/homebrew/bin"); err == nil {
		return "/opt/homebrew/bin/llamactl"
	}
	return "/usr/local/bin/llamactl"
}

func installBinary(cfg *config.Config) error {
	src := BinaryPath(cfg)
	dst := InstallPath()
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("binary not found at %s — build with: cd ~/AI/llamactl && make dist", src)
	}
	if out, err := exec.Command("cp", src, dst).CombinedOutput(); err != nil {
		return fmt.Errorf("cp: %w\n%s", err, out)
	}
	return os.Chmod(dst, 0o755)
}

func commitCount(repo, from, to string) (int, error) {
	out, err := gitOutput(repo, "rev-list", from+".."+to, "--count")
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	return n, err
}

func gitOutput(repo string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = repo
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}

func gitCmd(repo string, args ...string) (string, error) {
	c := exec.Command("git", args...)
	c.Dir = repo
	out, err := c.CombinedOutput()
	return string(out), err
}
