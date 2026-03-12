package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/andermurias/llamactl/internal/comfyui"
	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/andermurias/llamactl/internal/llamaswap"
	"github.com/fatih/color"
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
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	fmt.Println()
	bold.Println("  llama-swap")

	if !fileExists(cfg.PlistPath) {
		yellow.Println("  Status:     not installed")
		fmt.Println()
		fmt.Println("  Run: llamactl install  or  llamactl start")
		fmt.Println()
		return nil
	}

	if !launchd.IsLoaded(cfg) {
		red.Println("  Status:     not loaded")
		fmt.Printf("  Plist:      %s\n", cfg.PlistPath)
		fmt.Println()
		return nil
	}

	pid := launchd.GetPID(cfg)
	if pid > 0 {
		uptime := processUptime(pid)
		green.Printf("  Status:     running  (PID %d, uptime %s)\n", pid, uptime)
	} else {
		exitS := launchd.GetExitStatus(cfg)
		red.Printf("  Status:     stopped  (last exit: %s)\n", exitS)
	}

	autoStart := launchd.ReadAutoStart(cfg)
	if autoStart {
		green.Println("  Auto-start: enabled  (starts at login)")
	} else {
		yellow.Println("  Auto-start: disabled  (run 'llamactl enable' to activate)")
	}

	cyan.Printf("  API:        http://%s/v1\n", cfg.Listen)
	fmt.Printf("  Config:     %s\n", cfg.ConfigFile)
	fmt.Printf("  Log:        %s\n", cfg.LogFile)

	fmt.Println()
	bold.Println("  Loaded models:")
	running, err := llamaswap.GetRunning(cfg)
	if err != nil {
		fmt.Println("    (service not responding)")
	} else if len(running) == 0 {
		fmt.Println("    (none — models load on first request)")
	} else {
		for _, m := range running {
			green.Printf("    ● %s\n", m)
		}
	}

	fmt.Println()
	bold.Println("  ComfyUI  (image generation — manual)")
	if comfyui.IsRunning(cfg) {
		cpid := comfyui.GetPID(cfg)
		cuptime := processUptime(cpid)
		ip := comfyui.LocalIP()
		green.Printf("  Status:     running  (PID %d, uptime %s)\n", cpid, cuptime)
		cyan.Printf("  URL:        http://%s:%s\n", ip, cfg.ComfyUIPort)
	} else {
		yellow.Println("  Status:     stopped  (run: llamactl comfyui start)")
	}

	fmt.Println()
	return nil
}

func processUptime(pid int) string {
	out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "etime=").Output()
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(out))
}
