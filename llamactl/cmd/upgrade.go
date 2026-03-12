package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/andermurias/llamactl/internal/service"
	"github.com/andermurias/llamactl/internal/updater"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	var checkOnly bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Pull updates from git and refresh the binary",
		RunE:  func(cmd *cobra.Command, args []string) error { return runUpgrade(checkOnly) },
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only check for updates, do not apply")
	return cmd
}

func runUpgrade(checkOnly bool) error {
	pterm.DefaultSection.WithLevel(2).Println("Checking for updates")
	sp, _ := pterm.DefaultSpinner.WithText("Fetching from origin...").Start()
	status, err := updater.Check(cfg)
	if err != nil {
		sp.Fail(err.Error())
		return err
	}
	if !status.HasUpdate {
		sp.Success(fmt.Sprintf("Up to date  (%s on %s)", status.LocalCommit, status.Branch))
		return nil
	}
	sp.Warning(fmt.Sprintf("%d commit(s) behind  (local: %s → remote: %s on %s)",
		status.CommitsBehind, status.LocalCommit, status.RemoteCommit, status.Branch))

	if checkOnly {
		fmt.Println()
		pterm.Info.Println("Run 'llamactl upgrade' to apply")
		return nil
	}

	pterm.DefaultSection.WithLevel(2).Println("Applying updates")
	pullSp, _ := pterm.DefaultSpinner.WithText("Running git pull...").Start()
	pulled, err := updater.Apply(cfg)
	if err != nil {
		pullSp.Fail(err.Error())
		return err
	}
	pullSp.Success(fmt.Sprintf("Pulled %d commit(s) — binary updated at %s", pulled, updater.InstallPath()))

	pterm.DefaultSection.WithLevel(2).Println("Upgrading llama-swap")
	script := filepath.Join(cfg.AIDir, "scripts", "install-llama-swap.sh")
	var upgradeCmd *exec.Cmd
	if _, serr := os.Stat(script); serr == nil {
		upgradeCmd = exec.Command("bash", script)
	} else {
		upgradeCmd = exec.Command("brew", "upgrade", "llama-swap")
	}
	upgradeCmd.Stdout = os.Stdout
	upgradeCmd.Stderr = os.Stderr
	lsSp, _ := pterm.DefaultSpinner.WithText("Upgrading llama-swap...").Start()
	if err := upgradeCmd.Run(); err != nil {
		lsSp.Warning("llama-swap upgrade skipped: " + err.Error())
	} else {
		lsSp.Success("llama-swap upgraded")
	}

	if s := service.GetStatus(cfg); s.IsRunning {
		pterm.Info.Println("Restarting llamaswap service...")
		_ = runStop()
		_ = runStart()
	}

	fmt.Println()
	pterm.Success.Println("All updates applied successfully")
	return nil
}
