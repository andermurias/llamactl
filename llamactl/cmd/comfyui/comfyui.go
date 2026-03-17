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

// requiredPackages lists pip packages that must be installed for a healthy
// ComfyUI environment on Apple Silicon.
var requiredPackages = []setupPackage{
{"comfyui-manager", "comfyui_manager", "ComfyUI Manager (plugin manager UI)"},
{"opencv-python-headless", "cv2", "OpenCV (image processing, required by NudeNet and other nodes)"},
{"onnxruntime", "onnxruntime", "ONNX Runtime (required by NudeNet and AI upscaling nodes)"},
}

// newSetupCmd returns the "llamactl comfyui setup" command.
// It installs all required Python dependencies into the ComfyUI venv so that
// common custom nodes load correctly.
func newSetupCmd(cfg *config.Config) *cobra.Command {
return &cobra.Command{
Use:   "setup",
Short: "Install required Python dependencies for ComfyUI and common plugins",
Long: `Installs pip packages into the ComfyUI virtual environment that are
required by ComfyUI-Manager and common custom nodes (NudeNet, upscalers, etc.).
Safe to run multiple times — already-installed packages are skipped.`,
RunE: func(cmd *cobra.Command, args []string) error {
python := cfg.ComfyUIPython
if _, err := os.Stat(python); err != nil {
return fmt.Errorf("ComfyUI venv not found at %s — install ComfyUI first", python)
}

pterm.DefaultSection.WithLevel(2).Println("ComfyUI dependency setup")

allOK := true
for _, p := range requiredPackages {
// Check if already installed
checkCmd := exec.Command(python, "-c", fmt.Sprintf(
"import subprocess, sys; subprocess.run([sys.executable, '-c', 'import %s'], check=True)", p.testMod))
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
spinner.Fail(fmt.Sprintf("FAILED: %s\n%s", p.pkg, string(out)))
allOK = false
continue
}
spinner.Success(fmt.Sprintf("Installed %s", p.pkg))
}

// Also install any requirements.txt found in custom nodes
entries, _ := os.ReadDir(cfg.ComfyUIDir + "/custom_nodes")
for _, e := range entries {
if !e.IsDir() {
continue
}
reqFile := cfg.ComfyUIDir + "/custom_nodes/" + e.Name() + "/requirements.txt"
if _, err := os.Stat(reqFile); err != nil {
continue
}
spinner, _ := pterm.DefaultSpinner.WithText(
fmt.Sprintf("Installing deps for %s…", e.Name())).Start()
installCmd := exec.Command(python, "-m", "pip", "install",
"--quiet", "-r", reqFile)
out, err := installCmd.CombinedOutput()
if err != nil {
spinner.Fail(fmt.Sprintf("FAILED for %s: %s", e.Name(), string(out)))
allOK = false
} else {
spinner.Success(fmt.Sprintf("Deps OK: %s", e.Name()))
}
}

fmt.Println()
if allOK {
pterm.Success.Println("All dependencies installed. Restart ComfyUI:")
pterm.Info.Println("  llamactl comfyui restart")
} else {
pterm.Warning.Println("Some packages failed — check output above and retry.")
}
return nil
},
}
}
