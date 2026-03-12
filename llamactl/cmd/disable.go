package cmd

import (
	"fmt"
	"time"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable auto-start at login (service keeps running)",
		RunE: func(cmd *cobra.Command, args []string) error {
			green := color.New(color.FgGreen)
			warn := color.New(color.FgYellow)

			if !fileExists(cfg.PlistPath) {
				warn.Println("⚠  Service not installed")
				return nil
			}
			if err := launchd.WritePlist(cfg, false); err != nil {
				return fmt.Errorf("write plist: %w", err)
			}
			if launchd.IsLoaded(cfg) {
				_ = launchd.Bootout(cfg)
				time.Sleep(time.Second)
				_ = launchd.Bootstrap(cfg)
			}
			green.Println("✓  Auto-start at login disabled")
			warn.Println("⚠  Service is still available — use 'llamactl stop' to stop it now")
			return nil
		},
	}
}
