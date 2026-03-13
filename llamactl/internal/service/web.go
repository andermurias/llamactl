// Package service — web.go manages the llamactl-web LaunchAgent service.
// It mirrors the llamaswap.go pattern: pure business logic, no UI output.
package service

import (
	"fmt"
	"time"

	"github.com/andermurias/llamactl/internal/config"
	"github.com/andermurias/llamactl/internal/launchd"
)

// WebStatus holds the runtime state of the llamactl-web service.
type WebStatus struct {
	IsInstalled bool   // plist exists on disk
	IsLoaded    bool   // bootstrapped into launchd
	IsRunning   bool   // OS process alive
	PID         int    // 0 when not running
	Uptime      string // human-readable uptime
	AutoStart   bool   // RunAtLoad in plist
	Port        string // HTTP listen port
	URL         string // http://<host>:<port>
}

// GetWebStatus queries launchd and returns a snapshot. Never returns nil.
func GetWebStatus(cfg *config.Config) *WebStatus {
	s := &WebStatus{Port: cfg.WebPort, URL: "http://0.0.0.0:" + cfg.WebPort}
	svc := launchd.WebSvc(cfg)

	s.IsInstalled = fileExists(cfg.WebPlistPath)
	if !s.IsInstalled {
		return s
	}

	s.IsLoaded = launchd.IsLoaded(svc)
	if !s.IsLoaded {
		return s
	}

	s.PID = launchd.GetPID(svc)
	s.IsRunning = s.PID > 0
	if s.IsRunning {
		s.Uptime = processUptime(s.PID)
	}
	s.AutoStart = launchd.ReadAutoStart(svc)
	return s
}

// StartWeb installs the web service (if needed) and kicks it off.
func StartWeb(cfg *config.Config) (int, error) {
	if !fileExists(cfg.WebPlistPath) {
		if err := InstallWeb(cfg, false); err != nil {
			return 0, fmt.Errorf("auto-install web: %w", err)
		}
	}

	svc := launchd.WebSvc(cfg)
	if launchd.IsRunning(svc) {
		return launchd.GetPID(svc), nil
	}

	if err := launchd.Kickstart(svc); err != nil {
		return 0, fmt.Errorf("kickstart web: %w", err)
	}

	for i := 0; i < 8; i++ {
		time.Sleep(time.Second)
		if launchd.IsRunning(svc) {
			return launchd.GetPID(svc), nil
		}
	}
	return 0, fmt.Errorf("web service did not start — check: llamactl web logs")
}

// StopWeb sends SIGTERM to the web service.
func StopWeb(cfg *config.Config) error {
	svc := launchd.WebSvc(cfg)
	if !launchd.IsLoaded(svc) {
		return fmt.Errorf("web service is not loaded")
	}
	return launchd.KillSvc(svc, "SIGTERM")
}

// InstallWeb writes the plist and bootstraps the web service.
func InstallWeb(cfg *config.Config, autoStart bool) error {
	if err := launchd.WriteWebPlist(cfg, autoStart); err != nil {
		return fmt.Errorf("write web plist: %w", err)
	}
	svc := launchd.WebSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
		time.Sleep(time.Second)
	}
	if err := launchd.Bootstrap(svc); err != nil {
		return fmt.Errorf("bootstrap web: %w", err)
	}
	return nil
}

// UninstallWeb removes the web service from launchd.
func UninstallWeb(cfg *config.Config) error {
	svc := launchd.WebSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
	}
	return nil
}

// EnableWeb sets RunAtLoad=true and reloads the web service.
func EnableWeb(cfg *config.Config) error {
	if err := launchd.WriteWebPlist(cfg, true); err != nil {
		return err
	}
	svc := launchd.WebSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
		time.Sleep(time.Second)
	}
	return launchd.Bootstrap(svc)
}

// DisableWeb sets RunAtLoad=false and reloads the web service.
func DisableWeb(cfg *config.Config) error {
	if err := launchd.WriteWebPlist(cfg, false); err != nil {
		return err
	}
	svc := launchd.WebSvc(cfg)
	if launchd.IsLoaded(svc) {
		_ = launchd.Bootout(svc)
		time.Sleep(time.Second)
	}
	return launchd.Bootstrap(svc)
}
