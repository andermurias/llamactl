package cmd

import (
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Stop then start llama-swap",
		RunE: func(cmd *cobra.Command, args []string) error {
			color.New(color.FgCyan).Println("→  Restarting llama-swap…")
			if err := runStop(); err != nil {
				return err
			}
			time.Sleep(time.Second)
			return runStart()
		},
	}
}
