package cmd

import (
"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
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
s := service.GetStatus(cfg)
if !s.IsRunning {
pterm.Warning.Println("llama-swap is not running")
return nil
}
spinner, _ := pterm.DefaultSpinner.WithText("Stopping llama-swap…").Start()
if err := service.Stop(cfg); err != nil {
spinner.Fail(err.Error())
return err
}
spinner.Success("llama-swap stopped")
return nil
}
