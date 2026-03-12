package cmd

import (
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
return &cobra.Command{
Use:   "restart",
Short: "Restart llama-swap",
RunE: func(cmd *cobra.Command, args []string) error {
return runRestart()
},
}
}

func runRestart() error {
pterm.Info.Println("Restarting llama-swap…")
if err := runStop(); err != nil {
return err
}
return runStart()
}
