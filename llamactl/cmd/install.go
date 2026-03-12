package cmd

import (
"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
return &cobra.Command{
Use:   "install",
Short: "Install the launchd service (without starting)",
RunE: func(cmd *cobra.Command, args []string) error {
return runInstall()
},
}
}

func runInstall() error {
spinner, _ := pterm.DefaultSpinner.WithText("Installing launchd service…").Start()
if err := service.Install(cfg, false); err != nil {
spinner.Fail(err.Error())
return err
}
spinner.Success("LaunchAgent installed  (auto-start at login: disabled)")
pterm.Info.Println("Run 'llamactl enable' to auto-start at every login")
return nil
}
