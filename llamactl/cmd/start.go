package cmd

import (
"fmt"

"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
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
s := service.GetStatus(cfg)
if s.IsRunning {
pterm.Warning.Printf("llama-swap is already running (PID %d)\n", s.PID)
return nil
}

spinner, _ := pterm.DefaultSpinner.WithText("Starting llama-swap…").Start()

pid, err := service.Start(cfg)
if err != nil {
spinner.Fail(err.Error())
return err
}

spinner.Success(fmt.Sprintf("llama-swap started  (PID %d)", pid))
pterm.Println()
pterm.Info.Printf("API → http://%s/v1\n", cfg.Listen)
pterm.Info.Printf("UI  → http://%s\n", cfg.Listen)
return nil
}
