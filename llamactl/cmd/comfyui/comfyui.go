// Package comfyui provides the 'llamactl comfyui' subcommand group.
package comfyui

import (
"fmt"
"os"
"os/exec"

"github.com/andermurias/llamactl/internal/config"
"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

// NewCmd creates the "comfyui" parent command with all subcommands attached.
func NewCmd(cfg *config.Config) *cobra.Command {
cmd := &cobra.Command{
Use:   "comfyui",
Short: "Manage ComfyUI image generation server",
}
cmd.AddCommand(
newStartCmd(cfg),
newStopCmd(cfg),
newStatusCmd(cfg),
newLogsCmd(cfg),
)
return cmd
}

func newStartCmd(cfg *config.Config) *cobra.Command {
return &cobra.Command{
Use:   "start",
Short: "Start ComfyUI",
RunE: func(cmd *cobra.Command, args []string) error {
cs := service.GetComfyUIStatus(cfg)
if cs.IsRunning {
pterm.Warning.Printf("ComfyUI is already running (PID %d)\n", cs.PID)
pterm.Info.Printf("URL: %s\n", cs.URL)
return nil
}
spinner, _ := pterm.DefaultSpinner.WithText("Starting ComfyUI (waiting up to 60 s)…").Start()
pid, err := service.StartComfyUI(cfg)
if err != nil {
spinner.Fail(err.Error())
return err
}
cs2 := service.GetComfyUIStatus(cfg)
spinner.Success(fmt.Sprintf("ComfyUI started  (PID %d)", pid))
pterm.Info.Printf("URL: %s\n", cs2.URL)
return nil
},
}
}

func newStopCmd(cfg *config.Config) *cobra.Command {
return &cobra.Command{
Use:   "stop",
Short: "Stop ComfyUI",
RunE: func(cmd *cobra.Command, args []string) error {
cs := service.GetComfyUIStatus(cfg)
if !cs.IsRunning {
pterm.Warning.Println("ComfyUI is not running")
return nil
}
spinner, _ := pterm.DefaultSpinner.WithText("Stopping ComfyUI…").Start()
if err := service.StopComfyUI(cfg); err != nil {
spinner.Fail(err.Error())
return err
}
spinner.Success("ComfyUI stopped")
return nil
},
}
}

func newStatusCmd(cfg *config.Config) *cobra.Command {
return &cobra.Command{
Use:   "status",
Short: "Show ComfyUI status",
Run: func(cmd *cobra.Command, args []string) {
fmt.Println()
pterm.DefaultSection.WithLevel(2).Println("ComfyUI")
cs := service.GetComfyUIStatus(cfg)
if cs.IsRunning {
_ = pterm.DefaultTable.WithHasHeader(false).WithData(pterm.TableData{
{"  Status", pterm.FgGreen.Sprintf("● running  (PID %d, uptime %s)", cs.PID, cs.Uptime)},
{"  URL", pterm.FgCyan.Sprint(cs.URL)},
{"  Log", cs.LogFile},
}).Render()
} else {
pterm.Warning.Println("Stopped  — run: llamactl comfyui start")
}
fmt.Println()
},
}
}

func newLogsCmd(cfg *config.Config) *cobra.Command {
var follow bool
var lines int
cmd := &cobra.Command{
Use:   "logs",
Short: "Tail ComfyUI logs",
RunE: func(cmd *cobra.Command, args []string) error {
pterm.Info.Printf("Log: %s\n\n", cfg.ComfyUILog)
tailArgs := []string{"-n", fmt.Sprintf("%d", lines)}
if follow {
tailArgs = append(tailArgs, "-f")
}
tailArgs = append(tailArgs, cfg.ComfyUILog)
c := exec.Command("tail", tailArgs...)
c.Stdout = os.Stdout
c.Stderr = os.Stderr
return c.Run()
},
}
cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
cmd.Flags().IntVarP(&lines, "lines", "n", 100, "Number of lines")
return cmd
}
