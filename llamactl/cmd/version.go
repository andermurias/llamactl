package cmd

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/andermurias/llamactl/internal/updater"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

// Version is injected at build time via:
//
//	go build -ldflags "-X github.com/andermurias/llamactl/cmd.Version=v1.2.3"
var Version = "dev"

func newVersionCmd() *cobra.Command {
	var checkFlag bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			runVersion(checkFlag)
		},
	}
	cmd.Flags().BoolVar(&checkFlag, "check", false, "Check for a newer release on GitHub")
	return cmd
}

func runVersion(checkForUpdate bool) {
	pterm.DefaultSection.WithLevel(2).Println("llamactl")
	pterm.Println("  Version:   " + pterm.FgCyan.Sprint(Version))
	pterm.Println("  Repo:      https://github.com/" + updater.GitHubRepo)

	out, err := exec.Command(cfg.LlamaSwapBin, "-version").Output()
	if err != nil {
		out, err = exec.Command(cfg.LlamaSwapBin, "--version").Output()
	}
	if err == nil {
		v := strings.TrimSpace(string(out))
		pterm.Println("  llama-swap: " + pterm.FgCyan.Sprint(v))
	}

	if checkForUpdate {
		spinner, _ := pterm.DefaultSpinner.WithText("Checking for updates…").Start()
		rel, hasUpdate, err := updater.CheckLatest(Version)
		if err != nil {
			spinner.Fail("Could not reach GitHub: " + err.Error())
		} else if hasUpdate {
			spinner.Warning(fmt.Sprintf("Update available: %s  (%s)", rel.TagName, rel.HTMLURL))
			pterm.Println()
			pterm.Info.Println("Run:  llamactl upgrade --self")
		} else {
			spinner.Success("Already up to date")
		}
	}
	fmt.Println()
}
