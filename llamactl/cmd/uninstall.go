package cmd

import (
	"os"
	"time"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the launchd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			green := color.New(color.FgGreen)
			if launchd.IsRunning(cfg) {
				_ = runStop()
			}
			if launchd.IsLoaded(cfg) {
				_ = launchd.Bootout(cfg)
				time.Sleep(time.Second)
			}
			_ = os.Remove(cfg.PlistPath)
			green.Println("✓  Service uninstalled (logs and config preserved)")
			return nil
		},
	}
}
