package cmd

import (
"fmt"
"path/filepath"

"github.com/andermurias/llamactl/internal/llamaswap"
"github.com/andermurias/llamactl/internal/service"
"github.com/pterm/pterm"
"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
return &cobra.Command{
Use:   "models",
Short: "Show available models and cache status",
RunE: func(cmd *cobra.Command, args []string) error {
return runModels()
},
}
}

func runModels() error {
spinner, _ := pterm.DefaultSpinner.WithText("Fetching model info…").Start()
info := service.GetModelsInfo(cfg)
spinner.Stop()

fmt.Println()

pterm.DefaultSection.WithLevel(2).Printf("API models  (%d)\n", len(info.APIModels))
if !info.APIReachable {
pterm.Error.Println("llama-swap not responding — start with: llamactl start")
} else if len(info.APIModels) == 0 {
pterm.FgGray.Println("  (no models registered)")
} else {
tableData := pterm.TableData{{"  Model ID", "Status"}}
for _, m := range info.APIModels {
status := pterm.FgGray.Sprint("○ available")
if info.LoadedIDs[m.ID] {
status = pterm.FgGreen.Sprint("● loaded")
}
tableData = append(tableData, []string{"  " + m.ID, status})
}
_ = pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()
}

fmt.Println()

pterm.DefaultSection.WithLevel(2).Println("Local GGUF files  (~/AI/models/)")
if len(info.GGUFFiles) == 0 {
pterm.FgGray.Println("  (none)")
} else {
tableData := pterm.TableData{{"  File", "Size"}}
for _, f := range info.GGUFFiles {
tableData = append(tableData, []string{
"  " + filepath.Base(f.Path),
llamaswap.FormatBytes(f.Size),
})
}
_ = pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()
}

fmt.Println()

pterm.DefaultSection.WithLevel(2).Printf("HuggingFace cache (~/.cache/huggingface/hub/)  total: %s\n",
llamaswap.FormatBytes(info.HFTotalBytes))
if len(info.HFModels) == 0 {
pterm.FgGray.Println("  (empty)")
} else {
for _, name := range info.HFModels {
pterm.FgCyan.Printf("  • %s\n", name)
}
}

fmt.Println()
return nil
}
