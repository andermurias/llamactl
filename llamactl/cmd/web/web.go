// Package web provides the 'llamactl web' subcommand group.
//
// Subcommands:
//
//	serve   — internal: run the HTTP server (called by launchd plist)
//	start   — install + kickstart via launchd
//	stop    — send SIGTERM via launchd
//	restart — stop then start
//	status  — pretty-print web UI status
//	enable  — set RunAtLoad=true (auto-start at login)
//	disable — set RunAtLoad=false
//	install — write plist + bootstrap (does not start)
//	uninstall — bootout + remove plist
//	logs    — tail the web UI log file
package web

import (
	"fmt"
	"os"
	"os/exec"

	webserver "github.com/andermurias/llamactl/internal/web"
	"github.com/andermurias/llamactl/internal/config"
	"github.com/andermurias/llamactl/internal/service"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// NewCmd creates the "web" parent command with all subcommands attached.
func NewCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "web",
		Short: "Manage the llamactl Web UI (port " + cfg.WebPort + ")",
		Long: `Manage the embedded web dashboard for the local AI stack.

The web UI runs as a lightweight LaunchAgent on port ` + cfg.WebPort + `.
It provides a browser-based dashboard to start/stop services and view logs.

  llamactl web start       Start the web UI
  llamactl web stop        Stop the web UI
  llamactl web status      Show running status
  llamactl web enable      Auto-start at login
  llamactl web disable     Disable auto-start
  llamactl web logs        View web UI logs`,
	}

	cmd.AddCommand(
		newServeCmd(cfg),
		newStartCmd(cfg),
		newStopCmd(cfg),
		newRestartCmd(cfg),
		newStatusCmd(cfg),
		newEnableCmd(cfg),
		newDisableCmd(cfg),
		newInstallCmd(cfg),
		newUninstallCmd(cfg),
		newLogsCmd(cfg),
	)
	return cmd
}

// ── serve (internal, called by launchd) ──────────────────────────────────────

func newServeCmd(cfg *config.Config) *cobra.Command {
	var port string
	cmd := &cobra.Command{
		Use:    "serve",
		Short:  "Run the HTTP server (called by launchd — not for direct use)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := webserver.New(cfg)
			if err != nil {
				return fmt.Errorf("init web server: %w", err)
			}
			return s.Start(port)
		},
	}
	cmd.Flags().StringVar(&port, "port", cfg.WebPort, "HTTP listen port")
	return cmd
}

// ── start ─────────────────────────────────────────────────────────────────────

func newStartCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := service.GetWebStatus(cfg)
			if ws.IsRunning {
				pterm.Warning.Printf("Web UI is already running (PID %d)\n", ws.PID)
				pterm.Info.Printf("URL: http://localhost:%s\n", cfg.WebPort)
				return nil
			}
			spinner, _ := pterm.DefaultSpinner.WithText("Starting web UI…").Start()
			pid, err := service.StartWeb(cfg)
			if err != nil {
				spinner.Fail(err.Error())
				return err
			}
			spinner.Success(fmt.Sprintf("Web UI started  (PID %d)", pid))
			pterm.Info.Printf("URL: http://localhost:%s\n", cfg.WebPort)
			return nil
		},
	}
}

// ── stop ──────────────────────────────────────────────────────────────────────

func newStopCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			ws := service.GetWebStatus(cfg)
			if !ws.IsRunning {
				pterm.Warning.Println("Web UI is not running")
				return nil
			}
			spinner, _ := pterm.DefaultSpinner.WithText("Stopping web UI…").Start()
			if err := service.StopWeb(cfg); err != nil {
				spinner.Fail(err.Error())
				return err
			}
			spinner.Success("Web UI stopped")
			return nil
		},
	}
}

// ── restart ───────────────────────────────────────────────────────────────────

func newRestartCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the web UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			spinner, _ := pterm.DefaultSpinner.WithText("Restarting web UI…").Start()
			_ = service.StopWeb(cfg)
			pid, err := service.StartWeb(cfg)
			if err != nil {
				spinner.Fail(err.Error())
				return err
			}
			spinner.Success(fmt.Sprintf("Web UI restarted  (PID %d)", pid))
			pterm.Info.Printf("URL: http://localhost:%s\n", cfg.WebPort)
			return nil
		},
	}
}

// ── status ────────────────────────────────────────────────────────────────────

func newStatusCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show web UI status",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println()
			pterm.DefaultSection.WithLevel(2).Println("llamactl Web UI")
			ws := service.GetWebStatus(cfg)

			if !ws.IsInstalled {
				pterm.Warning.Println("Not installed — run: llamactl web install")
				fmt.Println()
				return
			}

			var statusStr string
			switch {
			case ws.IsRunning:
				statusStr = pterm.FgGreen.Sprintf("● running  (PID %d, uptime %s)", ws.PID, ws.Uptime)
			case ws.IsLoaded:
				statusStr = pterm.FgRed.Sprint("○ stopped")
			default:
				statusStr = pterm.FgYellow.Sprint("○ not loaded")
			}

			var autoStr string
			if ws.AutoStart {
				autoStr = pterm.FgGreen.Sprint("enabled  (starts at login)")
			} else {
				autoStr = pterm.FgYellow.Sprint("disabled — run: llamactl web enable")
			}

			_ = pterm.DefaultTable.WithHasHeader(false).WithData(pterm.TableData{
				{"  Status", statusStr},
				{"  Auto-start", autoStr},
				{"  URL", pterm.FgCyan.Sprintf("http://localhost:%s", cfg.WebPort)},
				{"  Log", cfg.WebLogFile},
				{"  Plist", cfg.WebPlistPath},
			}).Render()
			fmt.Println()
		},
	}
}

// ── enable / disable ──────────────────────────────────────────────────────────

func newEnableCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable web UI auto-start at login",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.EnableWeb(cfg); err != nil {
				return err
			}
			pterm.Success.Println("Web UI will auto-start at login")
			return nil
		},
	}
}

func newDisableCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable web UI auto-start at login",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.DisableWeb(cfg); err != nil {
				return err
			}
			pterm.Success.Println("Web UI auto-start disabled")
			return nil
		},
	}
}

// ── install / uninstall ───────────────────────────────────────────────────────

func newInstallCmd(cfg *config.Config) *cobra.Command {
	var autoStart bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install web UI as a launchd service",
		RunE: func(cmd *cobra.Command, args []string) error {
			spinner, _ := pterm.DefaultSpinner.WithText("Installing web UI…").Start()
			if err := service.InstallWeb(cfg, autoStart); err != nil {
				spinner.Fail(err.Error())
				return err
			}
			spinner.Success("Web UI installed")
			pterm.Info.Println("Start it with: llamactl web start")
			return nil
		},
	}
	cmd.Flags().BoolVar(&autoStart, "enable", false, "Enable auto-start at login")
	return cmd
}

func newUninstallCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall web UI from launchd",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := service.UninstallWeb(cfg); err != nil {
				return err
			}
			pterm.Success.Println("Web UI uninstalled")
			return nil
		},
	}
}

// ── logs ──────────────────────────────────────────────────────────────────────

func newLogsCmd(cfg *config.Config) *cobra.Command {
	var follow bool
	var lines int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail the web UI log file",
		RunE: func(cmd *cobra.Command, args []string) error {
			pterm.Info.Printf("Log: %s\n\n", cfg.WebLogFile)
			tailArgs := []string{"-n", fmt.Sprintf("%d", lines)}
			if follow {
				tailArgs = append(tailArgs, "-f")
			}
			tailArgs = append(tailArgs, cfg.WebLogFile)
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
