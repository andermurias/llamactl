package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show llama-swap logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(follow)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs in real time")
	return cmd
}

func runLogs(follow bool) error {
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(cfg.LogFile); os.IsNotExist(err) {
		color.New(color.FgYellow).Printf("⚠  No log file yet: %s\n  Start the service first: llamactl start\n", cfg.LogFile)
		return nil
	}

	if follow {
		color.New(color.FgCyan).Println("→  Following logs — Ctrl+C to stop")
		proc, err := os.StartProcess("/usr/bin/tail",
			[]string{"tail", "-n", "50", "-f", cfg.LogFile},
			&os.ProcAttr{Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
		if err != nil {
			return err
		}
		_, err = proc.Wait()
		return err
	}
	return tailN(cfg.LogFile, 100)
}

func tailN(path string, n int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	for _, l := range lines {
		fmt.Println(l)
	}
	return scanner.Err()
}
