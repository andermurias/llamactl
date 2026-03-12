package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the launchd service without starting",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall()
		},
	}
}

func runInstall() error {
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)

	if _, err := os.Stat(cfg.LlamaSwapBin); err != nil {
		return fmt.Errorf("llama-swap not found at %s\n  Install with: brew install llama-swap", cfg.LlamaSwapBin)
	}
	if _, err := os.Stat(cfg.ConfigFile); err != nil {
		return fmt.Errorf("config not found at %s", cfg.ConfigFile)
	}
	if err := launchd.WritePlist(cfg, false); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if launchd.IsLoaded(cfg) {
		_ = launchd.Bootout(cfg)
		time.Sleep(time.Second)
	}
	if err := launchd.Bootstrap(cfg); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	green.Println("✓  LaunchAgent installed (auto-start at login: disabled)")
	cyan.Println("   Run 'llamactl enable' to auto-start at every login")
	return nil
}
