package cmd

import (
	"os"

	cmdcomfyui "github.com/andermurias/llamactl/cmd/comfyui"
	"github.com/andermurias/llamactl/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfg     *config.Config
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "llamactl",
	Short: "llamactl — llama-swap + ComfyUI service manager",
	Long: `llamactl manages the local AI inference stack:
  • llama-swap  — OpenAI-compatible proxy (launchd service)
  • ComfyUI     — Image generation server (on-demand)`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
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
	)
}
