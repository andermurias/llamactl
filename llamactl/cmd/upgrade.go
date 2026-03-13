package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	var selfUpgrade bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade llama-swap (and optionally llamactl itself)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(selfUpgrade)
		},
	}
	cmd.Flags().BoolVar(&selfUpgrade, "self", false, "Also upgrade llamactl binary from GitHub releases")
	return cmd
}

func runUpgrade(selfUpgrade bool) error {
	pterm.DefaultSection.WithLevel(2).Println("Upgrading llama-swap")
	script := filepath.Join(cfg.AIDir, "scripts", "install-llama-swap.sh")
	var upgradeCmd *exec.Cmd
	if fileExists(script) {
		upgradeCmd = exec.Command("bash", script)
	} else {
		upgradeCmd = exec.Command("brew", "upgrade", "llama-swap")
	}
	upgradeCmd.Stdout = os.Stdout
	upgradeCmd.Stderr = os.Stderr
	if err := upgradeCmd.Run(); err != nil {
		return fmt.Errorf("llama-swap upgrade failed: %w", err)
	}

	if launchd.IsRunning(launchd.LlamaSwapSvc(cfg)) {
		pterm.Info.Println("Restarting service to apply upgrade\u2026")
		_ = runStop()
		_ = runStart()
	}
	pterm.Success.Println("llama-swap upgraded")

	if selfUpgrade {
		pterm.DefaultSection.WithLevel(2).Println("Upgrading llamactl")
		if err := selfUpgradeBinary(); err != nil {
			pterm.Error.Println(err)
			return err
		}
	}
	return nil
}

// selfUpgradeBinary pulls the latest commit from the git repo and installs
// the pre-built binary from llamactl/bin/llamactl.
//
// Repo layout (git root = ~/AI):
//
//	llamactl/bin/llamactl   — pre-built binary committed to git
//	scripts/llamactl        — deployed binary (in $PATH via symlink)
func selfUpgradeBinary() error {
	repoDir := cfg.AIDir // ~/AI is the git repository root

	// ── 1. Fetch to check for updates ────────────────────────────────────
	spinner, _ := pterm.DefaultSpinner.WithText("Checking for updates\u2026").Start()
	if out, err := exec.Command("git", "-C", repoDir, "fetch", "--quiet").CombinedOutput(); err != nil {
		spinner.Fail(fmt.Sprintf("git fetch failed: %s\n%s", err, out))
		return err
	}

	countOut, err := exec.Command("git", "-C", repoDir, "rev-list", "HEAD..origin/main", "--count").Output()
	if err != nil {
		spinner.Fail("git rev-list failed: " + err.Error())
		return err
	}
	pending := strings.TrimSpace(string(countOut))
	if pending == "0" {
		spinner.Success("Already up to date (" + Version + ")")
		return nil
	}
	spinner.UpdateText(fmt.Sprintf("Pulling %s new commit(s)\u2026", pending))

	// ── 2. Pull ───────────────────────────────────────────────────────────
	if out, err := exec.Command("git", "-C", repoDir, "pull", "--ff-only", "origin", "main").CombinedOutput(); err != nil {
		spinner.Fail(fmt.Sprintf("git pull failed: %s\n%s", err, out))
		return err
	}

	// ── 3. Install the updated binary ────────────────────────────────────
	newBin := filepath.Join(repoDir, "llamactl", "bin", "llamactl")
	if _, err := os.Stat(newBin); err != nil {
		spinner.Fail("pre-built binary not found at " + newBin)
		return fmt.Errorf("binary not found: %w", err)
	}

	deployBin := filepath.Join(repoDir, "scripts", "llamactl")
	if out, err := exec.Command("cp", newBin, deployBin).CombinedOutput(); err != nil {
		spinner.Fail(fmt.Sprintf("copy binary failed: %s\n%s", err, out))
		return err
	}
	if err := os.Chmod(deployBin, 0o755); err != nil {
		spinner.Fail("chmod: " + err.Error())
		return err
	}
	// Ad-hoc code sign required on macOS after replacing a binary (--force replaces existing).
	_ = exec.Command("codesign", "--force", "-s", "-", deployBin).Run()

	spinner.Success(fmt.Sprintf("llamactl updated (%s new commit(s) pulled)", pending))
	pterm.Info.Println("Restart any running services to apply updates if needed.")
	return nil
}
