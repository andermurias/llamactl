// Package config provides the 'llamactl config' subcommand group.
//
// Subcommands:
//
//	edit      — open llama-swap.yaml in $EDITOR (default: nano)
//	show      — print the config file to stdout
//	validate  — run llama-swap --config ... --validate (if supported) or check YAML syntax
//	path      — print the config file path (useful for scripts)
//	reload    — signal llama-swap to reload the config (watch-config is active by default)
package config

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/andermurias/llamactl/internal/config"
	"github.com/andermurias/llamactl/internal/service"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// NewCmd creates the "config" parent command with all subcommands attached.
func NewCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the llama-swap configuration file",
		Long: `Manage ` + cfg.ConfigFile + `

  llamactl config edit      Open in $EDITOR (nano by default)
  llamactl config show      Print to stdout
  llamactl config validate  Check YAML syntax
  llamactl config path      Print the config file path
  llamactl config reload    Reload running llama-swap (via SIGHUP)`,
	}
	cmd.AddCommand(
		newEditCmd(cfg),
		newShowCmd(cfg),
		newValidateCmd(cfg),
		newPathCmd(cfg),
		newReloadCmd(cfg),
	)
	return cmd
}

// ── edit ──────────────────────────────────────────────────────────────────────

func newEditCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open llama-swap.yaml in $EDITOR",
		Long: `Opens llama-swap.yaml in your preferred editor ($EDITOR env var).
Falls back to nano if $EDITOR is not set.

After saving, llama-swap will auto-reload if it was started with --watch-config
(the default). You can also run 'llamactl config reload' to force a reload.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "nano"
			}

			pterm.Info.Printf("Opening %s with %s\n", cfg.ConfigFile, editor)

			c := exec.Command(editor, cfg.ConfigFile)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}

			// Validate after edit.
			if err := validateYAML(cfg.ConfigFile); err != nil {
				pterm.Warning.Printf("YAML validation failed: %s\n", err)
				pterm.Info.Println("Fix the config and run: llamactl config validate")
				return nil
			}
			pterm.Success.Println("Config saved and YAML is valid")

			// Offer to reload if llama-swap is running.
			s := service.GetStatus(cfg)
			if s.IsRunning {
				pterm.Info.Println("llama-swap is running — reloading config now…")
				if err := reloadLlamaSwap(cfg); err != nil {
					pterm.Warning.Println("Reload failed — restart manually: llamactl restart")
				} else {
					pterm.Success.Println("Config reloaded")
				}
			}
			return nil
		},
	}
}

// ── show ──────────────────────────────────────────────────────────────────────

func newShowCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the config file to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(cfg.ConfigFile)
			if err != nil {
				return fmt.Errorf("read config: %w", err)
			}
			fmt.Print(string(data))
			return nil
		},
	}
}

// ── validate ──────────────────────────────────────────────────────────────────

func newValidateCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check the config file for YAML syntax errors",
		RunE: func(cmd *cobra.Command, args []string) error {
			pterm.Info.Printf("Validating %s\n", cfg.ConfigFile)
			if err := validateYAML(cfg.ConfigFile); err != nil {
				return err
			}
			pterm.Success.Println("Config is valid YAML")
			return nil
		},
	}
}

// ── path ──────────────────────────────────────────────────────────────────────

func newPathCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(cfg.ConfigFile)
		},
	}
}

// ── reload ────────────────────────────────────────────────────────────────────

func newReloadCmd(cfg *config.Config) *cobra.Command {
	return &cobra.Command{
		Use:   "reload",
		Short: "Reload llama-swap config (sends SIGHUP)",
		Long: `Sends SIGHUP to the running llama-swap process to reload its config.
This works because llama-swap is started with --watch-config by default,
meaning it re-reads the YAML whenever the file changes or on SIGHUP.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			s := service.GetStatus(cfg)
			if !s.IsRunning {
				return fmt.Errorf("llama-swap is not running — start it first: llamactl start")
			}
			spinner, _ := pterm.DefaultSpinner.WithText("Reloading config…").Start()
			if err := reloadLlamaSwap(cfg); err != nil {
				spinner.Fail(err.Error())
				return err
			}
			spinner.Success(fmt.Sprintf("Config reloaded (PID %d)", s.PID))
			return nil
		},
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// validateYAML parses the file as YAML and returns any syntax error.
func validateYAML(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("YAML syntax error: %w", err)
	}
	return nil
}

// reloadLlamaSwap sends SIGHUP to the llama-swap process via launchctl kill.
func reloadLlamaSwap(cfg *config.Config) error {
	s := service.GetStatus(cfg)
	if !s.IsRunning || s.PID == 0 {
		return fmt.Errorf("llama-swap not running")
	}
	out, err := exec.Command("kill", "-HUP", fmt.Sprintf("%d", s.PID)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("kill -HUP %d: %w — %s", s.PID, err, out)
	}
	return nil
}
