package cmd

import (
"fmt"
"os"
"os/exec"

"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
var follow bool
var lines int
var svc string

cmd := &cobra.Command{
Use:   "logs",
Short: "Show llama-swap logs",
RunE: func(cmd *cobra.Command, args []string) error {
return runLogs(follow, lines, svc)
},
}
cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
cmd.Flags().IntVarP(&lines, "lines", "n", 100, "Number of lines to show")
cmd.Flags().StringVarP(&svc, "service", "s", "llamaswap", "Service log: llamaswap or comfyui")
return cmd
}

func runLogs(follow bool, lines int, svc string) error {
logFile := cfg.LogFile
if svc == "comfyui" {
logFile = cfg.ComfyUILog
}

if _, err := os.Stat(logFile); err != nil {
pterm.Warning.Printf("Log file not found: %s\n", logFile)
return nil
}

pterm.Info.Printf("Log: %s\n", logFile)
fmt.Println()

args := []string{"-n", fmt.Sprintf("%d", lines)}
if follow {
args = append(args, "-f")
}
args = append(args, logFile)

c := exec.Command("tail", args...)
c.Stdout = os.Stdout
c.Stderr = os.Stderr
return c.Run()
}
