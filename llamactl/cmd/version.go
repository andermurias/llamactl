package cmd

import (
	"fmt"
	"os/exec"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags: -ldflags "-X cmd.Version=v1.2.3"
var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			bold := color.New(color.Bold)
			bold.Printf("llamactl %s\n", Version)
			out, err := exec.Command(cfg.LlamaSwapBin, "-version").Output()
			if err != nil {
				out, err = exec.Command(cfg.LlamaSwapBin, "--version").Output()
			}
			if err == nil {
				fmt.Printf("llama-swap %s", out)
			}
		},
	}
}
