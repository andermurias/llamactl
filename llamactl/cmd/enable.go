package cmd

import (
	"fmt"
	"time"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable auto-start at login and start now",
		RunE: func(cmd *cobra.Command, args []string) error {
			green := color.New(color.FgGreen)
			warn := color.New(color.FgYellow)

			if err := launchd.WritePlist(cfg, true); err != nil {
				return fmt.Errorf("write plist: %w", err)
			}
			if launchd.IsLoaded(cfg) {
				_ = launchd.Bootout(cfg)
				time.Sleep(time.Second)
			}
			if err := launchd.Bootstrap(cfg); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			if !launchd.IsRunning(cfg) {
				_ = launchd.Kickstart(cfg)
				for i := 0; i < 8; i++ {
					time.Sleep(time.Second)
					if launchd.IsRunning(cfg) {
						break
					}
				}
			}
			if launchd.IsRunning(cfg) {
				green.Printf("✓  llama-swap enabled — starts at login  (PID %d)\n", launchd.GetPID(cfg))
			} else {
				warn.Println("⚠  Auto-start enabled but service didn't start — check: llamactl logs")
			}
			return nil
		},
	}
}
