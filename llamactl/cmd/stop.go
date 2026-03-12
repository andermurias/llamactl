package cmd

import (
	"fmt"
	"time"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop llama-swap",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop()
		},
	}
}

func runStop() error {
	green := color.New(color.FgGreen)
	warn := color.New(color.FgYellow)

	if !launchd.IsLoaded(cfg) {
		warn.Println("⚠  Service not loaded — nothing to stop")
		return nil
	}
	if !launchd.IsRunning(cfg) {
		warn.Println("⚠  llama-swap is not running")
		return nil
	}

	if verbose {
		fmt.Printf("  Sending SIGTERM to PID %d\n", launchd.GetPID(cfg))
	}
	_ = launchd.Kill(cfg, "SIGTERM")

	for i := 0; i < 15; i++ {
		time.Sleep(time.Second)
		if !launchd.IsRunning(cfg) {
			break
		}
	}
	if launchd.IsRunning(cfg) {
		warn.Println("⚠  Process did not stop gracefully — sending SIGKILL")
		_ = launchd.Kill(cfg, "SIGKILL")
		time.Sleep(2 * time.Second)
	}

	green.Println("✓  llama-swap stopped")
	return nil
}
