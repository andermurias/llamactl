package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/andermurias/llamactl/internal/llamaswap"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "models",
		Short: "Show available models and download status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModels()
		},
	}
}

func runModels() error {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	cyan := color.New(color.FgCyan)
	dim := color.New(color.FgHiBlack)

	models, err := llamaswap.GetModels(cfg)
	if err != nil {
		color.New(color.FgRed).Println("✗  llama-swap not responding — start with: llamactl start")
		return nil
	}

	running, _ := llamaswap.GetRunning(cfg)
	runningSet := make(map[string]bool)
	for _, r := range running {
		runningSet[r] = true
	}

	fmt.Println()
	bold.Printf("  Available models (%d):\n", len(models))
	for _, m := range models {
		if runningSet[m.ID] {
			green.Printf("    ● %-35s [loaded]\n", m.ID)
		} else {
			dim.Printf("    ○ %s\n", m.ID)
		}
	}

	fmt.Println()
	bold.Println("  GGUF models (local ~/AI/models/):")
	ggufs, _ := llamaswap.GGUFFiles(cfg)
	if len(ggufs) == 0 {
		dim.Println("    (none)")
	}
	for _, f := range ggufs {
		name := filepath.Base(f.Path)
		green.Printf("    ✓ %-45s (%s)\n", name, llamaswap.FormatBytes(f.Size))
	}

	fmt.Println()
	bold.Println("  MLX/HF model cache (~/.cache/huggingface/hub/):")
	cachedNames, total, err := llamaswap.HFCachedModels()
	if err != nil {
		dim.Println("    (cache not found)")
	} else {
		cyan.Printf("    Total: %s\n", llamaswap.FormatBytes(total))
		fmt.Printf("    %s\n", strings.Join(cachedNames, ", "))
	}

	fmt.Println()
	return nil
}
