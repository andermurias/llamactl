package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/andermurias/llamactl/internal/updater"
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

	if launchd.IsRunning(cfg) {
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

// selfUpgradeBinary downloads the latest llamactl binary from GitHub releases
// and replaces the current binary.
func selfUpgradeBinary() error {
	spinner, _ := pterm.DefaultSpinner.WithText("Checking latest release\u2026").Start()
	rel, hasUpdate, err := updater.CheckLatest(Version)
	if err != nil {
		spinner.Fail("Could not check GitHub: " + err.Error())
		return err
	}
	if !hasUpdate {
		spinner.Success("Already up to date (" + Version + ")")
		return nil
	}
	spinner.UpdateText(fmt.Sprintf("Downloading %s\u2026", rel.TagName))

	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x86_64"
	}
	assetName := fmt.Sprintf("llamactl_%s_Darwin_%s.tar.gz", rel.TagName, arch)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/%s",
		updater.GitHubRepo, rel.TagName, assetName,
	)

	tmpDir, err := os.MkdirTemp("", "llamactl-upgrade-*")
	if err != nil {
		spinner.Fail("temp dir: " + err.Error())
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, "llamactl.tar.gz")
	if out, err := exec.Command("curl", "-fsSL", "-o", tarPath, downloadURL).CombinedOutput(); err != nil {
		spinner.Fail(fmt.Sprintf("download failed: %s\n%s", err, out))
		return err
	}
	if out, err := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir).CombinedOutput(); err != nil {
		spinner.Fail(fmt.Sprintf("extract failed: %s\n%s", err, out))
		return err
	}

	newBin := filepath.Join(tmpDir, "llamactl")
	currentBin, _ := os.Executable()
	if err := os.Rename(newBin, currentBin); err != nil {
		if out, err2 := exec.Command("cp", newBin, currentBin).CombinedOutput(); err2 != nil {
			spinner.Fail(fmt.Sprintf("replace binary failed: %s\n%s", err2, out))
			return err2
		}
	}
	if err := os.Chmod(currentBin, 0o755); err != nil {
		spinner.Fail("chmod: " + err.Error())
		return err
	}

	spinner.Success(fmt.Sprintf("llamactl upgraded to %s", rel.TagName))
	return nil
}
