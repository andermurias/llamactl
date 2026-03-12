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

// ServiceTarget returns "gui/<uid>/<label>"
func ServiceTarget(cfg *config.Config) string {
	return fmt.Sprintf("gui/%d/%s", cfg.UserID, cfg.Label)
}

// IsLoaded returns true if the service is bootstrapped into launchd.
func IsLoaded(cfg *config.Config) bool {
	err := exec.Command("launchctl", "print", ServiceTarget(cfg)).Run()
	return err == nil
}

// IsRunning returns true if the service process is actively running.
func IsRunning(cfg *config.Config) bool {
	out, err := exec.Command("launchctl", "print", ServiceTarget(cfg)).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "state = running")
}

// GetPID returns the PID of the running service, or 0 if not running.
func GetPID(cfg *config.Config) int {
	out, err := exec.Command("launchctl", "print", ServiceTarget(cfg)).Output()
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(`\s+pid = (\d+)`)
	m := re.FindStringSubmatch(string(out))
	if m == nil {
		return 0
	}
	pid, _ := strconv.Atoi(m[1])
	return pid
}

// GetExitStatus returns the last exit status of the service.
func GetExitStatus(cfg *config.Config) string {
	out, err := exec.Command("launchctl", "print", ServiceTarget(cfg)).Output()
	if err != nil {
		return "?"
	}
	re := regexp.MustCompile(`last exit.*?(\d+)`)
	m := re.FindStringSubmatch(string(out))
	if m == nil {
		return "?"
	}
	return m[1]
}

// Bootstrap loads the plist into launchd.
func Bootstrap(cfg *config.Config) error {
	return exec.Command("launchctl", "bootstrap",
		fmt.Sprintf("gui/%d", cfg.UserID), cfg.PlistPath).Run()
}

// Bootout unloads the service from launchd.
func Bootout(cfg *config.Config) error {
	err := exec.Command("launchctl", "bootout",
		fmt.Sprintf("gui/%d", cfg.UserID), cfg.PlistPath).Run()
	if err != nil {
		_ = exec.Command("launchctl", "bootout", ServiceTarget(cfg)).Run()
	}
	return nil
}

// Kickstart starts the service.
func Kickstart(cfg *config.Config) error {
	return exec.Command("launchctl", "kickstart",
		fmt.Sprintf("gui/%d/%s", cfg.UserID, cfg.Label)).Run()
}

// Kill sends a signal to the service.
func Kill(cfg *config.Config, signal string) error {
	return exec.Command("launchctl", "kill", signal, ServiceTarget(cfg)).Run()
}

var plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
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

// WritePlist writes the LaunchAgent plist to cfg.PlistPath.
func WritePlist(cfg *config.Config, autoStart bool) error {
	if err := os.MkdirAll(cfg.LogDir, 0o755); err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	data := struct {
		*config.Config
		Home      string
		RunAtLoad string
	}{cfg, home, boolStr(autoStart)}

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return err
	}
	f, err := os.Create(cfg.PlistPath)
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

// ReadAutoStart returns the current RunAtLoad value from the plist.
func ReadAutoStart(cfg *config.Config) bool {
	out, err := exec.Command("/usr/libexec/PlistBuddy",
		"-c", "Print :RunAtLoad", cfg.PlistPath).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
}
