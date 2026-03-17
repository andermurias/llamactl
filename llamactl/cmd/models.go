package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/andermurias/llamactl/internal/llamaswap"
	"github.com/andermurias/llamactl/internal/modelmanager"
	"github.com/andermurias/llamactl/internal/service"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

func newModelsCmd() *cobra.Command {
	// Root: `llamactl models` — lists models (backward-compatible default)
	root := &cobra.Command{
		Use:   "models",
		Short: "Manage models: list, search, install, enable, disable, remove",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModels()
		},
	}

	root.AddCommand(newModelsSearchCmd())
	root.AddCommand(newModelsInstallCmd())
	root.AddCommand(newModelsEnableCmd())
	root.AddCommand(newModelsDisableCmd())
	root.AddCommand(newModelsRemoveCmd())

	return root
}

// ── llamactl models (list) ────────────────────────────────────────────────────

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

	// Show disabled models
	mgr := modelmanager.NewManager(cfg.ConfigFile, cfg.DisabledFile, cfg.ModelsDir)
	disabled, _ := mgr.ListDisabled()
	if len(disabled) > 0 {
		fmt.Println()
		pterm.DefaultSection.WithLevel(2).Printf("Disabled models  (%d)\n", len(disabled))
		tableData := pterm.TableData{{"  Model ID", "Type", "HF ID"}}
		for id, entry := range disabled {
			tableData = append(tableData, []string{"  " + id, string(entry.Type), entry.HFID})
		}
		_ = pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()
	}

	fmt.Println()
	return nil
}

// ── llamactl models search <query> ────────────────────────────────────────────

func newModelsSearchCmd() *cobra.Command {
	var modelType string
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search HuggingFace for models",
		Example: `  llamactl models search gemma
  llamactl models search "code llama" --type text-generation
  llamactl models search phi --limit 10`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsSearch(strings.Join(args, " "), modelType, limit)
		},
	}
	cmd.Flags().StringVar(&modelType, "type", "", "filter type: text-generation, mlx, gguf")
	cmd.Flags().IntVar(&limit, "limit", 15, "max results")
	return cmd
}

func runModelsSearch(query, modelType string, limit int) error {
	spinner, _ := pterm.DefaultSpinner.WithText("Searching HuggingFace…").Start()
	results, err := modelmanager.SearchModels(query, modelType, limit)
	spinner.Stop()

	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}
	if len(results) == 0 {
		pterm.FgGray.Println("No models found.")
		return nil
	}

	fmt.Println()
	pterm.DefaultSection.WithLevel(2).Printf("HuggingFace results for %q  (%d)\n", query, len(results))
	tableData := pterm.TableData{{"  Model ID", "Downloads", "Likes", "Pipeline"}}
	for _, m := range results {
		tableData = append(tableData, []string{
			"  " + m.ID,
			fmt.Sprintf("%d", m.Downloads),
			fmt.Sprintf("%d", m.Likes),
			m.PipelineTag,
		})
	}
	_ = pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()
	fmt.Printf("\nInstall with: llamactl models install <model-id>\n\n")
	return nil
}

// ── llamactl models install <hf-id> ──────────────────────────────────────────

func newModelsInstallCmd() *cobra.Command {
	var forceType string
	var quant string
	var modelID string
	var group string
	var ttl int

	cmd := &cobra.Command{
		Use:   "install <hf-id|url>",
		Short: "Install a model from HuggingFace (auto-detects MLX vs GGUF)",
		Example: `  llamactl models install google/gemma-3-12b-it
  llamactl models install mlx-community/phi-4-4bit
  llamactl models install https://huggingface.co/meta-llama/Llama-3.2-3B-Instruct
  llamactl models install bartowski/Llama-3.2-3B-Instruct-GGUF --type gguf --quant Q4_K_M`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsInstall(args[0], forceType, quant, modelID, group, ttl)
		},
	}
	cmd.Flags().StringVar(&forceType, "type", "", "force backend: mlx | gguf (auto-detect if empty)")
	cmd.Flags().StringVar(&quant, "quant", "Q4_K_M", "GGUF quantization to download")
	cmd.Flags().StringVar(&modelID, "id", "", "model ID in llama-swap.yaml (auto-derived if empty)")
	cmd.Flags().StringVar(&group, "group", "llm", "swap group to add the model to")
	cmd.Flags().IntVar(&ttl, "ttl", 300, "idle timeout in seconds before unloading")
	return cmd
}

func runModelsInstall(hfID, forceType, quant, modelID, group string, ttl int) error {
	var forceModelType modelmanager.ModelType
	switch strings.ToLower(forceType) {
	case "mlx":
		forceModelType = modelmanager.ModelTypeMLX
	case "gguf":
		forceModelType = modelmanager.ModelTypeGGUF
	}

	req := modelmanager.InstallRequest{
		HFID:         hfID,
		ModelID:      modelID,
		ForceType:    forceModelType,
		Quantization: quant,
		Group:        group,
		TTL:          ttl,
	}

	ins := modelmanager.NewInstaller(cfg.AIDir, cfg.ConfigFile, cfg.LogDir)

	fmt.Println()
	pterm.DefaultSection.WithLevel(2).Printf("Installing %s\n", hfID)

	result, err := ins.Install(req, func(msg string) {
		if strings.HasPrefix(msg, "✓") {
			pterm.Success.Println(msg)
		} else if strings.HasPrefix(msg, "ERROR") {
			pterm.Error.Println(msg)
		} else {
			pterm.FgGray.Println("  " + msg)
		}
	})
	if err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	fmt.Println()
	pterm.Success.Printf("Model %q installed as %s\n", result.ModelID, string(result.Type))
	pterm.FgGray.Println(result.Message)
	pterm.Info.Println("Run 'llamactl restart' to apply the new config.")
	fmt.Println()
	return nil
}

// ── llamactl models enable <id> ───────────────────────────────────────────────

func newModelsEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable <model-id>",
		Short: "Re-enable a previously disabled model",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := modelmanager.NewManager(cfg.ConfigFile, cfg.DisabledFile, cfg.ModelsDir)
			if err := mgr.Enable(args[0]); err != nil {
				return err
			}
			pterm.Success.Printf("Model %q enabled — run 'llamactl restart' to apply\n", args[0])
			return nil
		},
	}
}

// ── llamactl models disable <id> ──────────────────────────────────────────────

func newModelsDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable <model-id>",
		Short: "Disable a model (removes from llama-swap, config preserved for re-enabling)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := modelmanager.NewManager(cfg.ConfigFile, cfg.DisabledFile, cfg.ModelsDir)
			if err := mgr.Disable(args[0]); err != nil {
				return err
			}
			pterm.Success.Printf("Model %q disabled — run 'llamactl restart' to apply\n", args[0])
			return nil
		},
	}
}

// ── llamactl models remove <id> ───────────────────────────────────────────────

func newModelsRemoveCmd() *cobra.Command {
	var deleteFiles bool

	cmd := &cobra.Command{
		Use:   "remove <model-id>",
		Short: "Remove a model from config (and optionally delete its files)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			modelID := args[0]

			if deleteFiles {
				pterm.Warning.Printf("This will delete the GGUF file for %q — are you sure? (y/N) ", modelID)
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
					pterm.FgGray.Println("Aborted.")
					return nil
				}
			}

			mgr := modelmanager.NewManager(cfg.ConfigFile, cfg.DisabledFile, cfg.ModelsDir)
			if err := mgr.Remove(modelID, deleteFiles); err != nil {
				return err
			}

			if deleteFiles {
				pterm.Success.Printf("Model %q removed and files deleted\n", modelID)
			} else {
				pterm.Success.Printf("Model %q removed from config — run 'llamactl restart' to apply\n", modelID)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&deleteFiles, "delete-files", false, "also delete GGUF file and HF cache")
	return cmd
}

