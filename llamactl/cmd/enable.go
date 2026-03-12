package cmd

import (
"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newEnableCmd() *cobra.Command {
return &cobra.Command{
Use:   "enable",
Short: "Enable auto-start at login",
RunE: func(cmd *cobra.Command, args []string) error {
spinner, _ := pterm.DefaultSpinner.WithText("Enabling auto-start…").Start()
if err := service.Enable(cfg); err != nil {
spinner.Fail(err.Error())
return err
}
spinner.Success("Auto-start enabled  — llama-swap will start at every login")
return nil
},
}
}
