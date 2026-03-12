package comfyui

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/andermurias/llamactl/internal/config"
)

// GetPID reads the PID from the pid file. Returns 0 if file missing or invalid.
func GetPID(cfg *config.Config) int {
	data, err := os.ReadFile(cfg.ComfyUIPID)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// IsRunning returns true if the ComfyUI process is alive.
func IsRunning(cfg *config.Config) bool {
	pid := GetPID(cfg)
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// Start launches ComfyUI in the background, writes the pid file.
func Start(cfg *config.Config) (int, error) {
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return 0, err
	}
	logFile, err := os.OpenFile(cfg.ComfyUILog,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open log: %w", err)
	}

	cmd := exec.Command(cfg.ComfyUIPython,
		cfg.ComfyUIDir+"/main.py",
		"--listen", "0.0.0.0",
		"--port", cfg.ComfyUIPort,
	)
	cmd.Dir = cfg.ComfyUIDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return 0, fmt.Errorf("start process: %w", err)
	}
	pid := cmd.Process.Pid

	if err := os.WriteFile(cfg.ComfyUIPID,
		[]byte(strconv.Itoa(pid)), 0o644); err != nil {
		return pid, fmt.Errorf("write pid file: %w", err)
	}

	return pid, nil
}

// WaitReady polls the ComfyUI HTTP server until it responds or timeout.
func WaitReady(cfg *config.Config, timeout time.Duration) bool {
	url := fmt.Sprintf("http://127.0.0.1:%s", cfg.ComfyUIPort)
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			return true
		}
		if !IsRunning(cfg) {
			return false
		}
		time.Sleep(2 * time.Second)
	}
	return false
}

// Stop sends SIGTERM to the ComfyUI process and waits for it to exit.
func Stop(cfg *config.Config) error {
	pid := GetPID(cfg)
	if pid <= 0 {
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		os.Remove(cfg.ComfyUIPID)
		return nil
	}

	_ = proc.Signal(syscall.SIGTERM)

	for i := 0; i < 10; i++ {
		time.Sleep(time.Second)
		if !IsRunning(cfg) {
			break
		}
	}
	if IsRunning(cfg) {
		_ = proc.Signal(syscall.SIGKILL)
		time.Sleep(time.Second)
	}
	os.Remove(cfg.ComfyUIPID)
	return nil
}

// LocalIP returns the LAN IP of en0, or "127.0.0.1" as fallback.
func LocalIP() string {
	out, err := exec.Command("ipconfig", "getifaddr", "en0").Output()
	if err != nil || len(out) == 0 {
		return "127.0.0.1"
	}
	return strings.TrimSpace(string(out))
}
