package cmd

import (
"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
return &cobra.Command{
Use:   "uninstall",
Short: "Remove the launchd service",
RunE: func(cmd *cobra.Command, args []string) error {
return runUninstall()
},
}
}

func runUninstall() error {
spinner, _ := pterm.DefaultSpinner.WithText("Uninstalling launchd service…").Start()
if err := service.Uninstall(cfg); err != nil {
spinner.Fail(err.Error())
return err
}
spinner.Success("LaunchAgent removed")
return nil
}
