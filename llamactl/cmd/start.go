package cmd

import (
	"fmt"
	"time"

	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start llama-swap",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart()
		},
	}
}

func runStart() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	blue := color.New(color.FgCyan)
	red := color.New(color.FgRed)
	warn := color.New(color.FgYellow)

	if !launchd.IsLoaded(cfg) {
		if verbose {
			fmt.Println("  Service not loaded — installing…")
		}
		if err := runInstall(); err != nil {
			return err
		}
	}

	if launchd.IsRunning(cfg) {
		pid := launchd.GetPID(cfg)
		warn.Printf("⚠  llama-swap is already running (PID %d)\n", pid)
		return nil
	}

	if err := launchd.Kickstart(cfg); err != nil {
		return fmt.Errorf("kickstart failed: %w", err)
	}

	for i := 0; i < 8; i++ {
		time.Sleep(2 * time.Second)
		if launchd.IsRunning(cfg) {
			break
		}
	}

	if !launchd.IsRunning(cfg) {
		red.Println("✗  llama-swap failed to start — check: llamactl logs")
		return fmt.Errorf("service did not start")
	}

	pid := launchd.GetPID(cfg)
	green.Printf("✓  llama-swap started  (PID %d)\n", pid)
	bold.Printf("   API → http://%s/v1\n", cfg.Listen)
	blue.Printf("   UI  → http://%s\n", cfg.Listen)
	return nil
}
