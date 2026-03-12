package service

import (
	"fmt"
	"time"

	"github.com/andermurias/llamactl/internal/comfyui"
	"github.com/andermurias/llamactl/internal/config"
)

// ComfyUIStatus holds the runtime state of the ComfyUI process.
type ComfyUIStatus struct {
	IsRunning bool   // process is alive
	PID       int    // 0 when not running
	Uptime    string // human-readable uptime
	URL       string // e.g. "http://192.168.1.5:8188"
	LogFile   string // path to log file
}

// GetComfyUIStatus returns the current state of ComfyUI. Never returns nil.
func GetComfyUIStatus(cfg *config.Config) *ComfyUIStatus {
	s := &ComfyUIStatus{LogFile: cfg.ComfyUILog}
	s.IsRunning = comfyui.IsRunning(cfg)
	if s.IsRunning {
		s.PID = comfyui.GetPID(cfg)
		s.Uptime = processUptime(s.PID)
		s.URL = fmt.Sprintf("http://%s:%s", comfyui.LocalIP(), cfg.ComfyUIPort)
	}
	return s
}

// StartComfyUI launches ComfyUI and waits up to 60 s for it to be ready.
// Returns the PID on success. Idempotent: returns existing PID if already running.
func StartComfyUI(cfg *config.Config) (int, error) {
	if comfyui.IsRunning(cfg) {
		return comfyui.GetPID(cfg), nil
	}

	pid, err := comfyui.Start(cfg)
	if err != nil {
		return 0, err
	}

	if !comfyui.WaitReady(cfg, 60*time.Second) {
		return pid, fmt.Errorf(
			"ComfyUI started (PID %d) but HTTP not ready after 60 s — check: llamactl comfyui logs", pid,
		)
	}

	return pid, nil
}

// StopComfyUI terminates the ComfyUI process gracefully.
func StopComfyUI(cfg *config.Config) error {
	return comfyui.Stop(cfg)
}
