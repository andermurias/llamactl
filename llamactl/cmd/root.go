// Package cmd contains all cobra command definitions for llamactl.
//
// Architecture:
//   - cmd/      presentation layer (pterm UI, cobra wiring)
//   - internal/service/ business logic (no UI, returns structs)
//   - internal/*  low-level wrappers (launchd, llamaswap, comfyui, updater)
//
// To add a new command: create cmd/mycommand.go with newMyCmd(), register in init().
// To expose commands over HTTP: call service.* functions from an HTTP handler.
package cmd

import (
	"os"

	cmdcomfyui "github.com/andermurias/llamactl/cmd/comfyui"
	cmdweb "github.com/andermurias/llamactl/cmd/web"
	"github.com/andermurias/llamactl/internal/config"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	cfg     *config.Config // loaded once at startup from config.Load()
	verbose bool           // --verbose / -v flag, available on every command
)

var rootCmd = &cobra.Command{
	Use:   "llamactl",
	Short: "llamactl — local AI stack manager",
	Long: `llamactl manages the local AI inference stack on macOS Apple Silicon.

  Services:
    llama-swap   OpenAI-compatible API proxy (launchd service)
    ComfyUI      Image generation server (on-demand)

  Quick start:
    llamactl start          Start llama-swap
    llamactl status         Check everything
    llamactl models         List available models
    llamactl comfyui start  Start image generation
    llamactl upgrade        Pull updates and refresh binary

  Run 'llamactl help <command>' for detailed usage.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the entry point called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		pterm.Error.Println(err)
		os.Exit(1)
	}
}

func init() {
	cfg = config.Load()
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show extra detail")
	rootCmd.AddCommand(
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newStatusCmd(),
		newEnableCmd(),
		newDisableCmd(),
		newInstallCmd(),
		newUninstallCmd(),
		newLogsCmd(),
		newModelsCmd(),
		newUpgradeCmd(),
		newVersionCmd(),
		cmdcomfyui.NewCmd(cfg),
		cmdweb.NewCmd(cfg),
	)
}
