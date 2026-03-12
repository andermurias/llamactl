package cmd

import (
"fmt"

"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
return &cobra.Command{
Use:   "status",
Short: "Show status of llama-swap and ComfyUI",
RunE: func(cmd *cobra.Command, args []string) error {
return runStatus()
},
}
}

func runStatus() error {
s := service.GetStatus(cfg)
cs := service.GetComfyUIStatus(cfg)

fmt.Println()

pterm.DefaultSection.WithLevel(2).Println("llama-swap")

if !s.IsInstalled {
pterm.Warning.Println("Not installed  — run: llamactl install")
fmt.Println()
} else {
var statusStr string
switch {
case s.IsRunning:
statusStr = pterm.FgGreen.Sprintf("● running  (PID %d, uptime %s)", s.PID, s.Uptime)
case s.IsLoaded:
statusStr = pterm.FgRed.Sprint("○ stopped")
default:
statusStr = pterm.FgYellow.Sprint("○ not loaded")
}

var autoStr string
if s.AutoStart {
autoStr = pterm.FgGreen.Sprint("enabled  (starts at login)")
} else {
autoStr = pterm.FgYellow.Sprint("disabled  — run: llamactl enable")
}

_ = pterm.DefaultTable.WithHasHeader(false).WithData(pterm.TableData{
{"  Status", statusStr},
{"  Auto-start", autoStr},
{"  API", pterm.FgCyan.Sprintf("http://%s/v1", s.APIAddr)},
{"  Config", s.ConfigFile},
{"  Log", s.LogFile},
}).Render()
fmt.Println()

if s.IsRunning {
pterm.DefaultSection.WithLevel(3).Println("Loaded models")
if len(s.LoadedModels) == 0 {
pterm.FgGray.Println("  (none — models load on first request)")
} else {
for _, m := range s.LoadedModels {
pterm.FgGreen.Printf("  ● %s\n", m)
}
}
fmt.Println()
}
}

pterm.DefaultSection.WithLevel(2).Println("ComfyUI  (image generation)")

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

return nil
}
