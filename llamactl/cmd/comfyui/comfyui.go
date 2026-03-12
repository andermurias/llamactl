package comfyui

import (
	"fmt"
	"os"
	"time"

	internalcomfyui "github.com/andermurias/llamactl/internal/comfyui"
	"github.com/andermurias/llamactl/internal/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// NewCmd returns the 'comfyui' cobra command with subcommands.
func NewCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comfyui",
		Short: "Manage ComfyUI image generation server (on-demand)",
		Long: `ComfyUI is managed on-demand (NOT a background service).
It uses ~6 GB for image models — stop it before loading large LLMs.

  llamactl comfyui start    Start on port ` + cfg.ComfyUIPort + `
  llamactl comfyui stop     Stop and free memory
  llamactl comfyui status   Show status and URL
  llamactl comfyui logs     Show logs (-f to follow)`,
	}

	cmd.AddCommand(
		newStartCmd(cfg),
		newStopCmd(cfg),
		newStatusCmd(cfg),
		newLogsCmd(cfg),
	)
	return cmd
}

func newStartCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start ComfyUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			green := color.New(color.FgGreen)
			warn := color.New(color.FgYellow)
			cyan := color.New(color.FgCyan)
			red := color.New(color.FgRed)

			if _, err := os.Stat(cfg.ComfyUIDir); err != nil {
				return fmt.Errorf("ComfyUI not found at %s\n  Run: git clone https://github.com/comfyanonymous/ComfyUI %s",
					cfg.ComfyUIDir, cfg.ComfyUIDir)
			}
			if _, err := os.Stat(cfg.ComfyUIPython); err != nil {
				return fmt.Errorf("ComfyUI venv not found at %s\n  Run the installer: %s/scripts/install.sh",
					cfg.ComfyUIPython, cfg.AIDir)
			}
			if internalcomfyui.IsRunning(cfg) {
				warn.Printf("⚠  ComfyUI is already running (PID %d)\n", internalcomfyui.GetPID(cfg))
				return nil
			}

			cyan.Printf("→  Starting ComfyUI on port %s…\n", cfg.ComfyUIPort)
			pid, err := internalcomfyui.Start(cfg)
			if err != nil {
				return fmt.Errorf("start ComfyUI: %w", err)
			}

			ready := internalcomfyui.WaitReady(cfg, 60*time.Second)
			if !ready || !internalcomfyui.IsRunning(cfg) {
				red.Printf("✗  ComfyUI failed to start — check logs: %s\n", cfg.ComfyUILog)
				os.Remove(cfg.ComfyUIPID)
				return fmt.Errorf("ComfyUI did not become ready")
			}

			ip := internalcomfyui.LocalIP()
			green.Printf("✓  ComfyUI started  (PID %d)\n", pid)
			cyan.Printf("   UI  → http://%s:%s\n", ip, cfg.ComfyUIPort)
			cyan.Printf("   Log → %s\n", cfg.ComfyUILog)
			warn.Println("⚠  Remember to stop ComfyUI before loading large LLMs (shared memory pool).")
			return nil
		},
	}
}

func newStopCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop ComfyUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			green := color.New(color.FgGreen)
			warn := color.New(color.FgYellow)

			if !internalcomfyui.IsRunning(cfg) {
				warn.Println("⚠  ComfyUI is not running")
				return nil
			}
			pid := internalcomfyui.GetPID(cfg)
			color.New(color.FgCyan).Printf("→  Stopping ComfyUI (PID %d)…\n", pid)
			if err := internalcomfyui.Stop(cfg); err != nil {
				return err
			}
			green.Println("✓  ComfyUI stopped")
			return nil
		},
	}
}

func newStatusCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show ComfyUI status",
		RunE: func(cmd *cobra.Command, args []string) error {
			bold := color.New(color.Bold)
			green := color.New(color.FgGreen)
			yellow := color.New(color.FgYellow)
			cyan := color.New(color.FgCyan)

			fmt.Println()
			bold.Println("  ComfyUI")
			if internalcomfyui.IsRunning(cfg) {
				pid := internalcomfyui.GetPID(cfg)
				ip := internalcomfyui.LocalIP()
				green.Printf("  Status:  running  (PID %d)\n", pid)
				cyan.Printf("  UI:      http://%s:%s\n", ip, cfg.ComfyUIPort)
			} else {
				yellow.Println("  Status:  stopped  (manual service — run: llamactl comfyui start)")
			}
			fmt.Printf("  Dir:     %s\n", cfg.ComfyUIDir)
			fmt.Printf("  Log:     %s\n", cfg.ComfyUILog)
			fmt.Println()
			return nil
		},
	}
}

func newLogsCmd(cfg *config.Config) *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show ComfyUI logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(cfg.ComfyUILog); os.IsNotExist(err) {
				color.New(color.FgYellow).Println("⚠  No ComfyUI log yet — start first: llamactl comfyui start")
				return nil
			}
			if follow {
				proc, err := os.StartProcess("/usr/bin/tail",
					[]string{"tail", "-n", "50", "-f", cfg.ComfyUILog},
					&os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
				if err != nil {
					return err
				}
				_, err = proc.Wait()
				return err
			}
			proc, err := os.StartProcess("/usr/bin/tail",
				[]string{"tail", "-n", "100", cfg.ComfyUILog},
				&os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
			if err != nil {
				return err
			}
			_, err = proc.Wait()
			return err
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs in real time")
	return cmd
}
