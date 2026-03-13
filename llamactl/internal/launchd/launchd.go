package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	"github.com/andermurias/llamactl/internal/config"
)

// Svc holds the minimal info launchd functions need for any LaunchAgent service.
// Build one with LlamaSwapSvc(cfg) or WebSvc(cfg).
type Svc struct {
	Label     string
	PlistPath string
	LogFile   string
	UserID    int
}

// LlamaSwapSvc returns a Svc descriptor for the llama-swap LaunchAgent.
func LlamaSwapSvc(cfg *config.Config) Svc {
	return Svc{cfg.Label, cfg.PlistPath, cfg.LogFile, cfg.UserID}
}

// WebSvc returns a Svc descriptor for the llamactl-web LaunchAgent.
func WebSvc(cfg *config.Config) Svc {
	return Svc{cfg.WebLabel, cfg.WebPlistPath, cfg.WebLogFile, cfg.UserID}
}

// Target returns the "gui/<uid>/<label>" service target string.
func Target(svc Svc) string {
	return fmt.Sprintf("gui/%d/%s", svc.UserID, svc.Label)
}

// ServiceTarget is a backwards-compat alias for the llama-swap service.
func ServiceTarget(cfg *config.Config) string { return Target(LlamaSwapSvc(cfg)) }

// IsLoaded returns true if the service is bootstrapped into launchd.
func IsLoaded(svc Svc) bool {
	return exec.Command("launchctl", "print", Target(svc)).Run() == nil
}

// IsRunning returns true if the service process is actively running.
func IsRunning(svc Svc) bool {
	out, err := exec.Command("launchctl", "print", Target(svc)).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "state = running")
}

// GetPID returns the PID of the running service, or 0 if not running.
func GetPID(svc Svc) int {
	out, err := exec.Command("launchctl", "print", Target(svc)).Output()
	if err != nil {
		return 0
	}
	pattern := regexp.MustCompile(`\s+pid = (\d+)`)
	m := pattern.FindStringSubmatch(string(out))
	if m == nil {
		return 0
	}
	pid, _ := strconv.Atoi(m[1])
	return pid
}

// Bootstrap loads the plist into launchd.
func Bootstrap(svc Svc) error {
	return exec.Command("launchctl", "bootstrap",
		fmt.Sprintf("gui/%d", svc.UserID), svc.PlistPath).Run()
}

// Bootout unloads the service from launchd (best-effort).
func Bootout(svc Svc) error {
	err := exec.Command("launchctl", "bootout",
		fmt.Sprintf("gui/%d", svc.UserID), svc.PlistPath).Run()
	if err != nil {
		_ = exec.Command("launchctl", "bootout", Target(svc)).Run()
	}
	return nil
}

// Kickstart starts the service.
func Kickstart(svc Svc) error {
	return exec.Command("launchctl", "kickstart", Target(svc)).Run()
}

// KillSvc sends a signal to the service.
func KillSvc(svc Svc, signal string) error {
	return exec.Command("launchctl", "kill", signal, Target(svc)).Run()
}

// ReadAutoStart returns the RunAtLoad value from the on-disk plist.
func ReadAutoStart(svc Svc) bool {
	out, err := exec.Command("/usr/libexec/PlistBuddy",
		"-c", "Print :RunAtLoad", svc.PlistPath).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}

var llamaSwapPlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>

    <key>ProgramArguments</key>
    <array>
        <string>{{.LlamaSwapBin}}</string>
        <string>--config</string>
        <string>{{.ConfigFile}}</string>
        <string>--listen</string>
        <string>{{.Listen}}</string>
        <string>--watch-config</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>{{.Home}}</string>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>

    <key>WorkingDirectory</key>
    <string>{{.AIDir}}</string>

    <key>StandardOutPath</key>
    <string>{{.LogFile}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogFile}}</string>

    <key>KeepAlive</key>
    <dict>
        <key>Crashed</key>
        <true/>
    </dict>

    <key>ThrottleInterval</key>
    <integer>10</integer>

    <key>RunAtLoad</key>
    <{{.RunAtLoad}}/>
</dict>
</plist>
`

// WriteLlamaSwapPlist writes the llama-swap LaunchAgent plist.
func WriteLlamaSwapPlist(cfg *config.Config, autoStart bool) error {
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	data := struct {
		*config.Config
		Home      string
		RunAtLoad string
	}{cfg, home, boolStr(autoStart)}
	return writePlist(cfg.PlistPath, llamaSwapPlistTmpl, data)
}

// WriteWebPlist writes the llamactl-web LaunchAgent plist.
// The plist launches "llamactl web serve --port <WebPort>".
func WriteWebPlist(cfg *config.Config, autoStart bool) error {
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return err
	}
	llamactlBin, _ := os.Executable()
	home, _ := os.UserHomeDir()
	data := struct {
		*config.Config
		LlamactlBin string
		Home        string
		RunAtLoad   string
	}{cfg, llamactlBin, home, boolStr(autoStart)}
	return writePlist(cfg.WebPlistPath, webPlistTmpl, data)
}

func writePlist(path, tmplStr string, data any) error {
	tmpl, err := template.New("plist").Parse(tmplStr)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, data)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ReadAutoStartCfg reads RunAtLoad from the llama-swap plist (cfg-based helper).
func ReadAutoStartCfg(cfg *config.Config) bool {
	return ReadAutoStart(LlamaSwapSvc(cfg))
}

var webPlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.WebLabel}}</string>

    <key>ProgramArguments</key>
    <array>
        <string>{{.LlamactlBin}}</string>
        <string>web</string>
        <string>serve</string>
        <string>--port</string>
        <string>{{.WebPort}}</string>
    </array>

    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>{{.Home}}</string>
        <key>PATH</key>
        <string>/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin:/usr/bin:/bin</string>
    </dict>

    <key>WorkingDirectory</key>
    <string>{{.AIDir}}</string>

    <key>StandardOutPath</key>
    <string>{{.WebLogFile}}</string>
    <key>StandardErrorPath</key>
    <string>{{.WebLogFile}}</string>

    <key>KeepAlive</key>
    <dict>
        <key>Crashed</key>
        <true/>
    </dict>

    <key>ThrottleInterval</key>
    <integer>10</integer>

    <key>RunAtLoad</key>
    <{{.RunAtLoad}}/>
</dict>
</plist>
`
