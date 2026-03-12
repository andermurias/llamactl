// Package service contains the core business logic for llamactl.
//
// This package is intentionally free of UI/presentation concerns — it returns
// plain structs and errors. This makes it easy to:
//   - Reuse from a future HTTP API or web controller
//   - Unit-test without mocking terminal output
//   - Extend by junior contributors without touching the UI layer
package service

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/andermurias/llamactl/internal/config"
	"github.com/andermurias/llamactl/internal/launchd"
	"github.com/andermurias/llamactl/internal/llamaswap"
)

// ── Status ─────────────────────────────────────────────────────────────────

// LlamaSwapStatus holds the full runtime state of the llama-swap service.
// All fields are safe to read even when the service is not running.
type LlamaSwapStatus struct {
	IsInstalled  bool     // plist file exists on disk
	IsLoaded     bool     // service is bootstrapped into launchd
	IsRunning    bool     // OS process is alive
	PID          int      // process ID; 0 when not running
	Uptime       string   // human-readable uptime from ps(1)
	AutoStart    bool     // RunAtLoad value in the plist
	APIAddr      string   // listen address, e.g. "127.0.0.1:8080"
	ConfigFile   string   // path to llama-swap.yaml
	LogFile      string   // path to the log file
	LoadedModels []string // model IDs currently resident in memory
	APIReachable bool     // true if /running endpoint returned 200
}

// GetStatus queries launchd and the llama-swap API, returning a snapshot
// of the service's current state. Never returns nil.
func GetStatus(cfg *config.Config) *LlamaSwapStatus {
	s := &LlamaSwapStatus{
		APIAddr:    cfg.Listen,
		ConfigFile: cfg.ConfigFile,
		LogFile:    cfg.LogFile,
	}

	s.IsInstalled = fileExists(cfg.PlistPath)
	if !s.IsInstalled {
		return s
	}

	s.IsLoaded = launchd.IsLoaded(cfg)
	if !s.IsLoaded {
		return s
	}

	s.PID = launchd.GetPID(cfg)
	s.IsRunning = s.PID > 0
	if s.IsRunning {
		s.Uptime = processUptime(s.PID)
	}

	s.AutoStart = launchd.ReadAutoStart(cfg)
	s.APIReachable = llamaswap.IsReachable(cfg)
	if s.APIReachable {
		s.LoadedModels, _ = llamaswap.GetRunning(cfg)
	}

	return s
}

// ── Lifecycle ──────────────────────────────────────────────────────────────

// Start ensures the service is installed, then kicks it off.
// Returns the PID on success. Idempotent: if already running, returns current PID.
func Start(cfg *config.Config) (int, error) {
	if !fileExists(cfg.PlistPath) {
		if err := Install(cfg, false); err != nil {
			return 0, fmt.Errorf("auto-install: %w", err)
		}
	}

	if launchd.IsRunning(cfg) {
		return launchd.GetPID(cfg), nil
	}

	if err := launchd.Kickstart(cfg); err != nil {
		return 0, fmt.Errorf("kickstart: %w", err)
	}

	// Poll up to 16 seconds for the process to appear.
	for i := 0; i < 8; i++ {
		time.Sleep(2 * time.Second)
		if launchd.IsRunning(cfg) {
			return launchd.GetPID(cfg), nil
		}
	}

	return 0, fmt.Errorf("service did not start — check: llamactl logs")
}

// Stop sends SIGTERM to the running service via launchd.
func Stop(cfg *config.Config) error {
	if !launchd.IsLoaded(cfg) {
		return fmt.Errorf("service is not loaded")
	}
	return launchd.Kill(cfg, "SIGTERM")
}

// Install writes the launchd plist and bootstraps the service (does NOT start it).
// Set autoStart=true to write RunAtLoad=true.
func Install(cfg *config.Config, autoStart bool) error {
	if !fileExists(cfg.LlamaSwapBin) {
		return fmt.Errorf("llama-swap binary not found at %s\n  Install with: brew install llama-swap", cfg.LlamaSwapBin)
	}
	if !fileExists(cfg.ConfigFile) {
		return fmt.Errorf("config not found at %s", cfg.ConfigFile)
	}

	if err := launchd.WritePlist(cfg, autoStart); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	if launchd.IsLoaded(cfg) {
		_ = launchd.Bootout(cfg)
		time.Sleep(time.Second)
	}

	if err := launchd.Bootstrap(cfg); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	return nil
}

// Uninstall removes the service from launchd (does not delete the plist file).
func Uninstall(cfg *config.Config) error {
	if launchd.IsLoaded(cfg) {
		_ = launchd.Bootout(cfg)
	}
	return nil
}

// Enable sets RunAtLoad=true and reloads the service.
func Enable(cfg *config.Config) error {
	if err := launchd.WritePlist(cfg, true); err != nil {
		return err
	}
	if launchd.IsLoaded(cfg) {
		_ = launchd.Bootout(cfg)
		time.Sleep(time.Second)
	}
	return launchd.Bootstrap(cfg)
}

// Disable sets RunAtLoad=false and reloads the service.
func Disable(cfg *config.Config) error {
	if err := launchd.WritePlist(cfg, false); err != nil {
		return err
	}
	if launchd.IsLoaded(cfg) {
		_ = launchd.Bootout(cfg)
		time.Sleep(time.Second)
	}
	return launchd.Bootstrap(cfg)
}

// ── Models ─────────────────────────────────────────────────────────────────

// ModelsInfo aggregates all model-related data for the "models" command.
type ModelsInfo struct {
	APIModels    []llamaswap.Model    // registered models from /v1/models
	LoadedIDs    map[string]bool      // model IDs currently in memory
	GGUFFiles    []llamaswap.FileInfo // local .gguf files under ~/AI/models/
	HFModels     []string             // directory names in ~/.cache/huggingface/hub/
	HFTotalBytes int64                // combined size of the HF cache
	APIReachable bool                 // false when llama-swap is not running
}

// GetModelsInfo collects model data from all sources. Never returns nil.
func GetModelsInfo(cfg *config.Config) *ModelsInfo {
	info := &ModelsInfo{LoadedIDs: make(map[string]bool)}

	models, err := llamaswap.GetModels(cfg)
	if err == nil {
		info.APIModels = models
		info.APIReachable = true
	}

	running, _ := llamaswap.GetRunning(cfg)
	for _, r := range running {
		info.LoadedIDs[r] = true
	}

	info.GGUFFiles, _ = llamaswap.GGUFFiles(cfg)
	info.HFModels, info.HFTotalBytes, _ = llamaswap.HFCachedModels()

	return info
}

// ── Helpers (unexported) ───────────────────────────────────────────────────

// fileExists returns true if path exists on the filesystem.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// processUptime returns a human-readable uptime string for the given PID
// by calling ps(1). Returns "?" if ps fails.
func processUptime(pid int) string {
	out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "etime=").Output()
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(out))
}
