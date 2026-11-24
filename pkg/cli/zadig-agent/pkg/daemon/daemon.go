package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
)

var ErrDisconnected = errors.New("agent was deliberately disconnected; explicitly start/connect it to resume")

const (
	Name          = "zadig-agent"
	Label         = "com.koderover.zadig-agent"
	Systemd       = "systemd"
	Cron          = "cron"
	Launchd       = "launchd"
	Windows       = "windows"
	HomeEnv       = "ZADIG_AGENT_HOME"
	BackgroundEnv = "ZADIG_AGENT_BACKGROUND"
	BackendEnv    = "ZADIG_AGENT_SERVICE_BACKEND"
)

type Settings struct {
	Backend     string   `json:"backend"`
	Executable  string   `json:"executable"`
	Home        string   `json:"home"`
	Directory   string   `json:"directory"`
	User        string   `json:"user"`
	AutoStart   bool     `json:"auto_start"`
	Environment []string `json:"environment"`
}

type Options struct {
	User      string
	Password  string
	AutoStart bool
}

func directory() (string, error) {
	p, err := config.GetAgentConfigFilePath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(p), nil
}

func settingsPath() (string, error) {
	dir, err := directory()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "service.json"), nil
}

func Load() (*Settings, error) {
	p, err := settingsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Settings) validate() error {
	if s.Backend != Systemd && s.Backend != Cron && s.Backend != Launchd && s.Backend != Windows {
		return fmt.Errorf("invalid agent service backend")
	}
	for _, value := range []string{s.Executable, s.Home, s.Directory, s.User} {
		if value == "" || strings.ContainsAny(value, "\x00\r\n") {
			return fmt.Errorf("invalid agent service configuration")
		}
	}
	if !filepath.IsAbs(s.Executable) || !filepath.IsAbs(s.Home) || !filepath.IsAbs(s.Directory) {
		return fmt.Errorf("agent service paths must be absolute")
	}
	return nil
}

func save(s *Settings) error {
	if err := s.validate(); err != nil {
		return err
	}
	p, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(p, data, 0600); err != nil {
		return err
	}
	return protectSettings(s, p)
}

func Install(options Options) (*Settings, error) {
	if _, err := config.GetAgentConfigFilePathWithCheck(); err != nil {
		return nil, fmt.Errorf("register the agent before installing auto-start: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return nil, err
	}
	if old, err := Load(); err == nil {
		if old.Executable != executable {
			return nil, fmt.Errorf("existing service uses %s; upgrade that installation instead", old.Executable)
		}
		if options.User != "" && options.User != old.User && !(runtime.GOOS == "windows" && strings.EqualFold(options.User, old.User)) {
			return nil, fmt.Errorf("uninstall the existing service before changing its account")
		}
		if err := setAutoStartNative(old, options.AutoStart); err != nil {
			return nil, err
		}
		old.AutoStart = options.AutoStart
		return old, save(old)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	homePath, err := config.GetAgentConfigFilePath()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	account, err := user.Current()
	if err != nil {
		return nil, err
	}
	installerUser := account.Username
	if options.User != "" {
		if runtime.GOOS == "windows" && strings.EqualFold(options.User, "LocalSystem") {
			account = &user.User{Username: "LocalSystem"}
		} else {
			account, err = user.Lookup(options.User)
			if err != nil {
				return nil, err
			}
		}
	}
	backend, err := detectBackend()
	if err != nil {
		return nil, err
	}
	s := &Settings{Backend: backend, Executable: executable, Home: filepath.Dir(filepath.Dir(homePath)), Directory: cwd, User: account.Username, AutoStart: options.AutoStart}
	for _, name := range []string{"PATH", "HOME", "USERPROFILE", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "SSL_CERT_FILE", "SSL_CERT_DIR", "ZADIG_AGENT_UPGRADE_CA_FILE", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"} {
		if value, ok := os.LookupEnv(name); ok {
			if name == "HOME" && runtime.GOOS != "windows" && account.Username != installerUser {
				value = account.HomeDir
			}
			s.Environment = append(s.Environment, name+"="+value)
		}
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	// The runtime reads these private settings, so unit files never contain
	// proxy credentials or agent tokens.
	if err := save(s); err != nil {
		p, _ := settingsPath()
		_ = os.Remove(p)
		return nil, err
	}
	if err := installNative(s, options.Password); err != nil {
		p, _ := settingsPath()
		_ = os.Remove(p)
		return nil, err
	}
	return s, nil
}

func SetAutoStart(enabled bool) error {
	s, err := Load()
	if err != nil {
		return err
	}
	if err := setAutoStartNative(s, enabled); err != nil {
		return err
	}
	s.AutoStart = enabled
	return save(s)
}

func Start(ctx context.Context) error {
	s, err := Load()
	if err != nil {
		return err
	}
	return startNative(ctx, s)
}

func Stop(ctx context.Context) error {
	s, err := Load()
	if err != nil {
		return err
	}
	return stopNative(ctx, s)
}

func Status(ctx context.Context) (map[string]interface{}, error) {
	s, err := Load()
	if err != nil {
		return nil, err
	}
	running, err := runningNative(ctx, s)
	if err != nil {
		return nil, err
	}
	allowed, err := ConnectionAllowed()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"backend": s.Backend, "auto_start": s.AutoStart, "running": running, "connection_enabled": allowed, "user": s.User, "home": s.Home}, nil
}

func Uninstall(ctx context.Context) error {
	s, err := Load()
	if err != nil {
		return err
	}
	if err := uninstallNative(ctx, s); err != nil {
		return err
	}
	p, err := settingsPath()
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// Connection intent is separate from agent_config.yaml: upgrade rollback must
// never restore an older "connected" value over a deliberate disconnect.
func ConnectionAllowed() (bool, error) {
	dir, err := directory()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(dir, ".disconnected"))
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return false, err
}

func SetConnectionAllowed(allowed bool) error {
	dir, err := directory()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	p := filepath.Join(dir, ".disconnected")
	if allowed {
		err := os.Remove(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return writeFile(p, []byte("disconnected\n"), 0600)
}

func PrepareRuntime() (*Settings, error) {
	s, err := Load()
	if err != nil {
		return nil, err
	}
	for _, entry := range s.Environment {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !runtimeEnvironment(key) {
			return nil, fmt.Errorf("invalid service environment")
		}
		if err := os.Setenv(key, value); err != nil {
			return nil, err
		}
	}
	if err := os.Setenv(HomeEnv, s.Home); err != nil {
		return nil, err
	}
	if err := os.Setenv(BackgroundEnv, "1"); err != nil {
		return nil, err
	}
	if err := os.Setenv(BackendEnv, s.Backend); err != nil {
		return nil, err
	}
	if err := os.Chdir(s.Directory); err != nil {
		return nil, err
	}
	return s, nil
}

func runtimeEnvironment(key string) bool {
	switch key {
	case "PATH", "HOME", "USERPROFILE", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "SSL_CERT_FILE", "SSL_CERT_DIR", "ZADIG_AGENT_UPGRADE_CA_FILE", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY":
		return true
	}
	return false
}

func RunningBackend() string {
	backend := os.Getenv(BackendEnv)
	if backend == Systemd || backend == Launchd || backend == Windows {
		return backend
	}
	return ""
}

func command(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s failed: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

var runCommand = command

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".zadig-write-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(mode); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return replaceFile(f.Name(), path)
}

func CanUpgrade() bool {
	backend := RunningBackend()
	if backend == "" {
		return false
	}
	s, err := Load()
	if err != nil || s.Backend != backend {
		return false
	}
	return canUpgradeNative(s)
}

func LaunchUpgradeHelper(ctx context.Context, executable, id string) error {
	s, err := Load()
	if err != nil {
		return err
	}
	if !canUpgradeNative(s) {
		return fmt.Errorf("service account cannot safely control self upgrade")
	}
	return launchHelperNative(ctx, s, executable, id)
}

func StartOrdinary() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	dir, err := directory()
	if err != nil {
		return err
	}
	home := filepath.Dir(dir)
	if err := os.MkdirAll(filepath.Join(dir, "log"), 0700); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(dir, "log", "service.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(executable, "start", "--auto-start=false", "--home", home)
	cmd.Env = append(os.Environ(), BackgroundEnv+"=1")
	cmd.Stdout, cmd.Stderr = log, log
	detachOrdinary(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func PrepareUpgrade() error { return prepareUpgradeNative() }
func FinishUpgrade() error  { return finishUpgradeNative() }
