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
		newRestartCmd(cfg),
		newStatusCmd(cfg),
		newLogsCmd(cfg),
		newModelsCmd(cfg),
		newSetupCmd(cfg),
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

func newRestartCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart ComfyUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			cs := service.GetComfyUIStatus(cfg)
			if cs.IsRunning {
				spinner, _ := pterm.DefaultSpinner.WithText("Stopping ComfyUI…").Start()
				if err := service.StopComfyUI(cfg); err != nil {
					spinner.Fail(err.Error())
					return err
				}
				spinner.Success("ComfyUI stopped")
			}
			spinner, _ := pterm.DefaultSpinner.WithText("Starting ComfyUI (waiting up to 60 s)…").Start()
			pid, err := service.StartComfyUI(cfg)
			if err != nil {
				spinner.Fail(err.Error())
				return err
			}
			cs2 := service.GetComfyUIStatus(cfg)
			spinner.Success(fmt.Sprintf("ComfyUI restarted  (PID %d)", pid))
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

// setupPackage describes a pip package required by ComfyUI or its extensions.
type setupPackage struct {
	pkg     string // pip package name
	testMod string // Python module to import to verify installation
	desc    string // human-readable description
}

// setupExtension describes a custom node to clone into custom_nodes/.
type setupExtension struct {
	name string // directory name in custom_nodes/
	url  string // git clone URL
	desc string // short description
}

// requiredPackages lists pip packages that must be installed for a healthy
// ComfyUI environment on Apple Silicon.
var requiredPackages = []setupPackage{
	{"comfyui-manager", "comfyui_manager", "ComfyUI Manager (plugin manager UI)"},
	{"opencv-python-headless", "cv2", "OpenCV (image processing for custom nodes)"},
	{"onnxruntime", "onnxruntime", "ONNX Runtime (AI upscaling, detection nodes)"},
	{"accelerate", "accelerate", "HuggingFace Accelerate (faster model loading)"},
	{"diffusers", "diffusers", "HuggingFace Diffusers (pipeline support)"},
	{"transformers", "transformers", "HuggingFace Transformers (CLIP, T5, etc.)"},
	{"imageio", "imageio", "imageio (image/video I/O)"},
	{"imageio-ffmpeg", "imageio_ffmpeg", "imageio-ffmpeg (video export)"},
}

// standardExtensions lists the most popular and useful ComfyUI custom nodes.
// These are cloned into custom_nodes/ during setup.
var standardExtensions = []setupExtension{
	{"ComfyUI-Manager", "https://github.com/ltdrdata/ComfyUI-Manager", "Plugin manager — install/update nodes from UI"},
	{"ComfyUI-Easy-Use", "https://github.com/yolain/ComfyUI-Easy-Use", "Simplified all-in-one nodes for quick workflows"},
	{"ComfyUI-Impact-Pack", "https://github.com/ltdrdata/ComfyUI-Impact-Pack", "Detection, segmentation, face detailer"},
	{"ComfyUI-Inspire-Pack", "https://github.com/ltdrdata/ComfyUI-Inspire-Pack", "Prompt helpers, wildcards, style mixing"},
	{"was-node-suite-comfyui", "https://github.com/WASasquatch/was-node-suite-comfyui", "220+ utility nodes (images, text, logic)"},
	{"ComfyUI_IPAdapter_plus", "https://github.com/cubiq/ComfyUI_IPAdapter_plus", "IP-Adapter image conditioning"},
	{"ComfyUI-Advanced-ControlNet", "https://github.com/Kosinkadink/ComfyUI-Advanced-ControlNet", "Advanced ControlNet with batching"},
	{"ComfyUI-VideoHelperSuite", "https://github.com/Kosinkadink/ComfyUI-VideoHelperSuite", "Video load/save/combine nodes"},
	{"ComfyUI-GGUF", "https://github.com/city96/ComfyUI-GGUF", "GGUF model support (Flux, LLMs)"},
	{"rgthree-comfy", "https://github.com/rgthree/rgthree-comfy", "Power user nodes (reroute, bookmarks, etc.)"},
	{"ComfyUI_UltimateSDUpscale", "https://github.com/ssitu/ComfyUI_UltimateSDUpscale", "Ultimate SD Upscale tiled upscaling"},
	{"ComfyUI-Custom-Scripts", "https://github.com/pythongosssss/ComfyUI-Custom-Scripts", "UI enhancements (image feed, wildcards, etc.)"},
	{"efficiency-nodes-comfyui", "https://github.com/jags111/efficiency-nodes-comfyui", "Efficient loaders and workflow helpers"},
	{"ComfyUI-KJNodes", "https://github.com/kijai/ComfyUI-KJNodes", "KJ utility nodes for Flux/video/masks"},
	{"comfyui-portrait-master", "https://github.com/florestefano1975/comfyui-portrait-master", "Portrait lighting, style, and composition controls"},
}

// newSetupCmd returns the "llamactl comfyui setup" command.
// It clones standard extensions and installs all pip dependencies so that
// common custom nodes load correctly. Safe to run multiple times.
func newSetupCmd(cfg *config.Config) *cobra.Command {
	var skipExtensions bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install common ComfyUI extensions and Python dependencies",
		Long: `Clones the most popular ComfyUI custom nodes and installs all
required pip packages into the ComfyUI venv. Safe to run multiple times —
already-installed items are skipped. Also installs requirements.txt from
any custom node already present in custom_nodes/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			python := cfg.ComfyUIPython
			if _, err := os.Stat(python); err != nil {
				return fmt.Errorf("ComfyUI venv not found at %s — install ComfyUI first", python)
			}

			allOK := true
			nodesDir := cfg.ComfyUIDir + "/custom_nodes"

			// ── Clone standard extensions ──────────────────────────────────
			if !skipExtensions {
				pterm.DefaultSection.WithLevel(2).Println("ComfyUI extensions")
				for _, ext := range standardExtensions {
					dest := nodesDir + "/" + ext.name
					if _, err := os.Stat(dest); err == nil {
						pterm.Success.Printf("%-40s already installed\n", ext.name)
						continue
					}
					spinner, _ := pterm.DefaultSpinner.WithText(
						fmt.Sprintf("Cloning %s — %s", ext.name, ext.desc)).Start()
					cloneCmd := exec.Command("git", "clone", "--depth=1", ext.url, dest)
					out, err := cloneCmd.CombinedOutput()
					if err != nil {
						spinner.Fail(fmt.Sprintf("FAILED: %s — %s", ext.name, string(out)))
						allOK = false
					} else {
						spinner.Success(fmt.Sprintf("Installed %s", ext.name))
					}
				}
			}

			// ── Install pip packages ───────────────────────────────────────
			pterm.DefaultSection.WithLevel(2).Println("Python dependencies")
			for _, p := range requiredPackages {
				checkCmd := exec.Command(python, "-c",
					fmt.Sprintf("import subprocess,sys; subprocess.run([sys.executable,'-c','import %s'],check=True)", p.testMod))
				if checkCmd.Run() == nil {
					pterm.Success.Printf("%-35s already installed\n", p.pkg)
					continue
				}
				spinner, _ := pterm.DefaultSpinner.WithText(
					fmt.Sprintf("Installing %s — %s", p.pkg, p.desc)).Start()
				installCmd := exec.Command(python, "-m", "pip", "install", "--quiet", p.pkg)
				installCmd.Dir = cfg.ComfyUIDir
				out, err := installCmd.CombinedOutput()
				if err != nil {
					spinner.Fail(fmt.Sprintf("FAILED: %s — %s", p.pkg, string(out)))
					allOK = false
				} else {
					spinner.Success(fmt.Sprintf("Installed %s", p.pkg))
				}
			}

			// ── Install requirements.txt from every custom node ────────────
			pterm.DefaultSection.WithLevel(2).Println("Custom node dependencies")
			entries, _ := os.ReadDir(nodesDir)
			anyReq := false
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				reqFile := nodesDir + "/" + e.Name() + "/requirements.txt"
				if _, err := os.Stat(reqFile); err != nil {
					continue
				}
				anyReq = true
				spinner, _ := pterm.DefaultSpinner.WithText(
					fmt.Sprintf("Deps for %s…", e.Name())).Start()
				installCmd := exec.Command(python, "-m", "pip", "install", "--quiet", "-r", reqFile)
				out, err := installCmd.CombinedOutput()
				if err != nil {
					spinner.Fail(fmt.Sprintf("FAILED for %s: %s", e.Name(), string(out)))
					allOK = false
				} else {
					spinner.Success(fmt.Sprintf("OK: %s", e.Name()))
				}
			}
			if !anyReq {
				pterm.Info.Println("No additional requirements.txt found in custom nodes.")
			}

			fmt.Println()
			if allOK {
				pterm.Success.Println("Setup complete! Restart ComfyUI to load new extensions:")
				pterm.Info.Println("  llamactl comfyui restart")
			} else {
				pterm.Warning.Println("Some steps failed — check output above and retry.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipExtensions, "skip-extensions", false, "Skip cloning standard extensions (pip deps only)")
	return cmd
}
