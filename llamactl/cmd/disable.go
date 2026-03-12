package cmd

import (
"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newDisableCmd() *cobra.Command {
return &cobra.Command{
Use:   "disable",
Short: "Disable auto-start at login",
RunE: func(cmd *cobra.Command, args []string) error {
spinner, _ := pterm.DefaultSpinner.WithText("Disabling auto-start…").Start()
if err := service.Disable(cfg); err != nil {
spinner.Fail(err.Error())
return err
}
spinner.Success("Auto-start disabled")
return nil
},
}
}
