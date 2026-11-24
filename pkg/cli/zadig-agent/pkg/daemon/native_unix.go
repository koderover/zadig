//go:build linux || darwin

package daemon

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var effectiveUID = os.Geteuid
var platform = runtime.GOOS
var systemdRuntimeDirectory = "/run/systemd/system"
var processDirectory = "/proc"

var systemdDirectory = "/etc/systemd/system"
var launchDaemonDirectory = "/Library/LaunchDaemons"
var launchConfigurationDirectory = "/Library/Application Support/Zadig"

func replaceFile(from, to string) error { return os.Rename(from, to) }

func protectSettings(s *Settings, path string) error {
	if effectiveUID() != 0 {
		return nil
	}
	u, err := user.Lookup(s.User)
	if err != nil {
		return err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	if uid != 0 {
		configuration := filepath.Join(s.Home, ".zadig-agent", "agent_config.yaml")
		for _, existing := range []string{filepath.Dir(configuration), configuration} {
			info, err := os.Stat(existing)
			if err != nil {
				return err
			}
			owner, ok := info.Sys().(*syscall.Stat_t)
			if !ok || int(owner.Uid) != uid {
				return fmt.Errorf("service account %s must own its existing agent configuration at %s; register using that account and select its --home before installing the service", s.User, existing)
			}
		}
	}
	return os.Chown(path, uid, gid)
}

func requireRoot() error {
	if effectiveUID() != 0 {
		return fmt.Errorf("administrator/root privileges are required to manage the system agent service")
	}
	return nil
}

func detectBackend() (string, error) {
	if platform == "darwin" {
		if _, err := exec.LookPath("launchctl"); err != nil {
			return "", fmt.Errorf("launchd is unavailable: %w", err)
		}
		if err := requireRoot(); err != nil {
			return "", err
		}
		return Launchd, nil
	}
	if _, err := os.Stat(systemdRuntimeDirectory); err == nil {
		if _, err := exec.LookPath("systemctl"); err == nil {
			if _, err := runCommand(context.Background(), "systemctl", "show", "--property=Version"); err == nil {
				if err := requireRoot(); err != nil {
					return "", err
				}
				return Systemd, nil
			}
		}
	}
	if _, err := exec.LookPath("crontab"); err != nil {
		return "", fmt.Errorf("auto-start unavailable: systemd is not running and crontab is not installed; use ordinary agent start or configure startup manually")
	}
	entries, err := os.ReadDir(processDirectory)
	if err != nil {
		return "", fmt.Errorf("cannot verify cron is running: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, err := os.ReadFile(filepath.Join(processDirectory, entry.Name(), "comm"))
		if err == nil && (strings.TrimSpace(string(name)) == "cron" || strings.TrimSpace(string(name)) == "crond") {
			return Cron, nil
		}
	}
	return "", fmt.Errorf("auto-start unavailable: systemd is not running and no running cron daemon was found; ordinary agent start remains available")
}

func unitQuote(value string) string {
	value = strings.ReplaceAll(value, "%", "%%")
	value = strings.ReplaceAll(value, "$", "$$")
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func systemdUnit(s *Settings) []byte {
	return []byte(fmt.Sprintf("[Unit]\nDescription=Zadig Agent\nWants=network-online.target\nAfter=network-online.target\n\n[Service]\nType=simple\nUser=%s\nWorkingDirectory=%s\nExecStart=%s service run --home %s\nRestart=on-failure\nRestartSec=5\nTimeoutStopSec=30\n\n[Install]\nWantedBy=multi-user.target\n", s.User, strings.ReplaceAll(s.Directory, "%", "%%"), unitQuote(s.Executable), unitQuote(s.Home)))
}

func xmlString(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return "<string>" + b.String() + "</string>"
}

func launchPlist(s *Settings, label, executable string, arguments []string, keepAlive bool) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key>`)
	b.WriteString(xmlString(label) + "<key>ProgramArguments</key><array>" + xmlString(executable))
	for _, argument := range arguments {
		b.WriteString(xmlString(argument))
	}
	b.WriteString("</array><key>UserName</key>" + xmlString(s.User) + "<key>WorkingDirectory</key>" + xmlString(s.Directory) + "<key>RunAtLoad</key><true/>")
	if keepAlive {
		b.WriteString("<key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict><key>ThrottleInterval</key><integer>5</integer>")
	}
	logPath := filepath.Join(s.Home, ".zadig-agent", "log", "service.log")
	b.WriteString("<key>StandardOutPath</key>" + xmlString(logPath) + "<key>StandardErrorPath</key>" + xmlString(logPath) + "</dict></plist>\n")
	return []byte(b.String())
}

const cronBegin = "# BEGIN ZADIG AGENT AUTOSTART"
const cronEnd = "# END ZADIG AGENT AUTOSTART"

func cronQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }

func cronTable(current string, s *Settings, enabled bool) (string, error) {
	var lines []string
	inside := false
	for _, line := range strings.Split(strings.TrimRight(current, "\n"), "\n") {
		if line == cronBegin {
			if inside {
				return "", fmt.Errorf("invalid Zadig crontab block")
			}
			inside = true
			continue
		}
		if line == cronEnd {
			if !inside {
				return "", fmt.Errorf("invalid Zadig crontab block")
			}
			inside = false
			continue
		}
		if !inside {
			lines = append(lines, line)
		}
	}
	if inside {
		return "", fmt.Errorf("incomplete Zadig crontab block; repair it before modifying auto-start")
	}
	text := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	if text != "" {
		text += "\n"
	}
	if enabled {
		// cron treats percent specially even inside shell quotes.
		entry := "@reboot " + cronQuote(s.Executable) + " service run --home " + cronQuote(s.Home) + " >> " + cronQuote(filepath.Join(s.Home, ".zadig-agent", "log", "service.log")) + " 2>&1"
		text += cronBegin + "\n" + strings.ReplaceAll(entry, "%", `\%`) + "\n" + cronEnd + "\n"
	}
	return text, nil
}

func updateCron(s *Settings, enabled bool) error {
	args := []string{}
	currentUser, err := user.Current()
	if err != nil {
		return err
	}
	if s.User != currentUser.Username {
		if err := requireRoot(); err != nil {
			return err
		}
		args = append(args, "-u", s.User)
	}
	out, err := runCommand(context.Background(), "crontab", append(args, "-l")...)
	if err != nil {
		if !strings.Contains(strings.ToLower(string(out)), "no crontab") {
			return err
		}
		out = nil
	}
	text, err := cronTable(string(out), s, enabled)
	if err != nil {
		return err
	}
	dir, err := directory()
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".zadig-crontab-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(text); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if _, err := runCommand(context.Background(), "crontab", append(args, f.Name())...); err != nil {
		return fmt.Errorf("cannot install cron @reboot entry: %w", err)
	}
	return nil
}

func installNative(s *Settings, password string) error {
	if err := os.MkdirAll(filepath.Join(s.Home, ".zadig-agent", "log"), 0700); err != nil {
		return err
	}
	switch s.Backend {
	case Systemd:
		if err := requireRoot(); err != nil {
			return err
		}
		path := filepath.Join(systemdDirectory, Name+".service")
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("service unit already exists; refusing to overwrite %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := writeFile(path, systemdUnit(s), 0644); err != nil {
			return err
		}
		if _, err := runCommand(context.Background(), "systemctl", "daemon-reload"); err != nil {
			os.Remove(path)
			return err
		}
		if err := setAutoStartNative(s, s.AutoStart); err != nil {
			os.Remove(path)
			_, _ = runCommand(context.Background(), "systemctl", "daemon-reload")
			return err
		}
		return nil
	case Launchd:
		if err := requireRoot(); err != nil {
			return err
		}
		path := filepath.Join(launchConfigurationDirectory, Label+".plist")
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("agent launch configuration already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(launchConfigurationDirectory, 0755); err != nil {
			return err
		}
		if err := writeFile(path, launchPlist(s, Label, s.Executable, []string{"service", "run", "--home", s.Home}, true), 0644); err != nil {
			return err
		}
		if err := setAutoStartNative(s, s.AutoStart); err != nil {
			os.Remove(path)
			return err
		}
		return nil
	case Cron:
		return updateCron(s, s.AutoStart)
	}
	return fmt.Errorf("unsupported service backend %s", s.Backend)
}

func setAutoStartNative(s *Settings, enabled bool) error {
	switch s.Backend {
	case Systemd:
		if err := requireRoot(); err != nil {
			return err
		}
		action := "disable"
		if enabled {
			action = "enable"
		}
		_, err := runCommand(context.Background(), "systemctl", action, Name+".service")
		return err
	case Launchd:
		if err := requireRoot(); err != nil {
			return err
		}
		bootPath := filepath.Join(launchDaemonDirectory, Label+".plist")
		if !enabled {
			err := os.Remove(bootPath)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		data, err := os.ReadFile(filepath.Join(launchConfigurationDirectory, Label+".plist"))
		if err != nil {
			return err
		}
		if err := writeFile(bootPath, data, 0644); err != nil {
			return err
		}
		_, err = runCommand(context.Background(), "launchctl", "enable", "system/"+Label)
		if err != nil {
			os.Remove(bootPath)
		}
		return err
	case Cron:
		return updateCron(s, enabled)
	}
	return fmt.Errorf("unsupported service backend")
}

func startNative(ctx context.Context, s *Settings) error {
	switch s.Backend {
	case Systemd:
		_, _ = runCommand(ctx, "systemctl", "reset-failed", Name+".service")
		_, err := runCommand(ctx, "systemctl", "start", Name+".service")
		return err
	case Launchd:
		if _, err := runCommand(ctx, "launchctl", "print", "system/"+Label); err != nil {
			_, err = runCommand(ctx, "launchctl", "bootstrap", "system", filepath.Join(launchConfigurationDirectory, Label+".plist"))
			return err
		}
		_, err := runCommand(ctx, "launchctl", "kickstart", "system/"+Label)
		return err
	case Cron:
		return fmt.Errorf("cron only configures boot startup; run zadig-agent start to connect now")
	}
	return fmt.Errorf("unsupported service backend")
}

func stopNative(ctx context.Context, s *Settings) error {
	switch s.Backend {
	case Systemd:
		_, err := runCommand(ctx, "systemctl", "stop", Name+".service")
		return err
	case Launchd:
		if _, err := runCommand(ctx, "launchctl", "print", "system/"+Label); err != nil {
			return nil
		}
		_, err := runCommand(ctx, "launchctl", "bootout", "system/"+Label)
		return err
	case Cron:
		return nil
	}
	return fmt.Errorf("unsupported service backend")
}

func runningNative(ctx context.Context, s *Settings) (bool, error) {
	switch s.Backend {
	case Systemd:
		out, err := runCommand(ctx, "systemctl", "show", Name+".service", "--property=ActiveState")
		if err != nil {
			return false, err
		}
		status := strings.TrimPrefix(strings.TrimSpace(string(out)), "ActiveState=")
		return status == "active" || status == "activating", nil
	case Launchd:
		out, err := runCommand(ctx, "launchctl", "print", "system/"+Label)
		if err != nil {
			return false, nil
		}
		return strings.Contains(string(out), "state = running"), nil
	case Cron:
		f, err := os.OpenFile(filepath.Join(s.Home, ".zadig-agent", "upgrade", "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		defer f.Close()
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if errors.Is(err, unix.EWOULDBLOCK) {
			return true, nil
		}
		return false, err
	}
	return false, fmt.Errorf("unsupported service backend")
}

func uninstallNative(ctx context.Context, s *Settings) error {
	if err := stopNative(ctx, s); err != nil {
		return err
	}
	if err := setAutoStartNative(s, false); err != nil {
		return err
	}
	switch s.Backend {
	case Systemd:
		if err := os.Remove(filepath.Join(systemdDirectory, Name+".service")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		_, err := runCommand(ctx, "systemctl", "daemon-reload")
		return err
	case Launchd:
		return os.Remove(filepath.Join(launchConfigurationDirectory, Label+".plist"))
	}
	return nil
}

func canUpgradeNative(s *Settings) bool {
	if effectiveUID() != 0 {
		return false
	}
	if s.Backend == Systemd {
		_, err := exec.LookPath("systemd-run")
		return err == nil
	}
	return s.Backend == Launchd
}

func launchHelperNative(ctx context.Context, s *Settings, executable, id string) error {
	if s.Backend == Systemd {
		_, err := runCommand(ctx, "systemd-run", "--unit="+Name+"-upgrade-"+id, "--property=Type=simple", "--property=Restart=no", strings.ReplaceAll(executable, "$", "$$"), "upgrade-apply", "--home", strings.ReplaceAll(s.Home, "$", "$$"))
		return err
	}
	if s.Backend == Launchd {
		label := Label + ".upgrade." + id
		path := filepath.Join(launchConfigurationDirectory, label+".plist")
		_, _ = runCommand(ctx, "launchctl", "bootout", "system/"+label)
		if err := writeFile(path, launchPlist(s, label, executable, []string{"upgrade-apply", "--home", s.Home}, false), 0644); err != nil {
			return err
		}
		defer os.Remove(path)
		_, err := runCommand(ctx, "launchctl", "bootstrap", "system", path)
		return err
	}
	return fmt.Errorf("service backend does not support managed upgrade")
}

func Run(ctx context.Context, work func(context.Context, func()) error) error {
	return work(ctx, func() {})
}

func prepareUpgradeNative() error { return nil }
func finishUpgradeNative() error  { return nil }

func detachOrdinary(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
