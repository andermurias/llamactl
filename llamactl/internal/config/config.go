package config

import (
	"os"
	"path/filepath"
)

// Config holds all runtime paths and constants for llamactl.
type Config struct {
	// llama-swap
	Label        string // com.llamastack.llama-swap
	PlistPath    string // ~/Library/LaunchAgents/com.llamastack.llama-swap.plist
	AIDir        string // ~/AI
	ConfigFile   string // ~/AI/llama-swap.yaml
	LlamaSwapBin string // /opt/homebrew/bin/llama-swap
	LogDir       string // ~/AI/logs
	LogFile      string // ~/AI/logs/llama-swap.log
	Listen       string // 127.0.0.1:8080
	UserID       int    // current user ID

	// ComfyUI
	ComfyUIDir    string // ~/AI/ComfyUI
	ComfyUIPython string // ~/AI/ComfyUI/.venv/bin/python
	ComfyUIPort   string // 8188
	ComfyUILog    string // ~/AI/logs/comfyui.log
	ComfyUIPID    string // ~/AI/logs/comfyui.pid
}

// Load returns a Config populated from environment / defaults.
func Load() *Config {
	home, _ := os.UserHomeDir()
	aiDir := filepath.Join(home, "AI")
	logDir := filepath.Join(aiDir, "logs")
	label := "com.llamastack.llama-swap"
	comfyDir := filepath.Join(aiDir, "ComfyUI")
	return &Config{
		Label:         label,
		PlistPath:     filepath.Join(home, "Library", "LaunchAgents", label+".plist"),
		AIDir:         aiDir,
		ConfigFile:    filepath.Join(aiDir, "llama-swap.yaml"),
		LlamaSwapBin:  "/opt/homebrew/bin/llama-swap",
		LogDir:        logDir,
		LogFile:       filepath.Join(logDir, "llama-swap.log"),
		Listen:        "0.0.0.0:8080",
		UserID:        os.Getuid(),
		ComfyUIDir:    comfyDir,
		ComfyUIPython: filepath.Join(comfyDir, ".venv", "bin", "python"),
		ComfyUIPort:   "8188",
		ComfyUILog:    filepath.Join(logDir, "comfyui.log"),
		ComfyUIPID:    filepath.Join(logDir, "comfyui.pid"),
	}
}
