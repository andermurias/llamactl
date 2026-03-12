package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade llama-swap to the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			green := color.New(color.FgGreen)
			cyan := color.New(color.FgCyan)

			script := filepath.Join(cfg.AIDir, "scripts", "install-llama-swap.sh")
			if !fileExists(script) {
				cyan.Println("→  Running: brew upgrade llama-swap")
				out, err := exec.Command("brew", "upgrade", "llama-swap").CombinedOutput()
				fmt.Print(string(out))
				if err != nil {
					return fmt.Errorf("brew upgrade failed: %w", err)
				}
			} else {
				cyan.Printf("→  Running %s\n", script)
				out, err := exec.Command("bash", script).CombinedOutput()
				fmt.Print(string(out))
				if err != nil {
					return fmt.Errorf("upgrade script failed: %w", err)
				}
			}

			if launchd.IsRunning(cfg) {
				cyan.Println("→  Restarting service to apply upgrade…")
				if err := runStop(); err != nil {
					return err
				}
				if err := runStart(); err != nil {
					return err
				}
			}
			green.Println("✓  Upgrade complete")
			return nil
		},
	}
}
