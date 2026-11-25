//go:build linux || darwin

package daemon

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
)

const brokerLabel = Label + ".upgrade-helper"

func brokerDirectory() string {
	if platform == "darwin" {
		return filepath.Join(launchConfigurationDirectory, "upgrade-helper")
	}
	return "/var/lib/zadig-agent/upgrade-helper"
}
func brokerSocket() string { return filepath.Join(brokerDirectory(), "helper.sock") }

type BrokerRequest struct {
	ID        string `json:"id,omitempty"`
	Version   string `json:"version,omitempty"`
	Manifest  []byte `json:"manifest,omitempty"`
	Signature []byte `json:"signature,omitempty"`
}
type BrokerStatus struct {
	ID         string `json:"id,omitempty"`
	Status     string `json:"status,omitempty"`
	Executable string `json:"executable"`
	Home       string `json:"home"`
	User       string `json:"user"`
}
type brokerConfig struct {
	Settings  Settings `json:"settings"`
	PublicKey string   `json:"public_key"`
	UID       int      `json:"uid"`
	GID       int      `json:"gid"`
}
type brokerTransaction struct {
	ID             string             `json:"id"`
	Status         string             `json:"status"`
	Asset          agentupgrade.Asset `json:"asset"`
	Deadline       time.Time          `json:"deadline"`
	RestartPending bool               `json:"restart_pending,omitempty"`
}

// The client checks the kernel peer identity too: a user-owned socket must not
// be able to impersonate the privileged helper.
func brokerClient() *http.Client {
	return &http.Client{Timeout: time.Minute, Transport: &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, "unix", brokerSocket())
		if err != nil {
			return nil, err
		}
		uid, err := peerUID(c.(*net.UnixConn))
		if err != nil || uid != 0 {
			c.Close()
			return nil, fmt.Errorf("upgrade helper is not owned by root")
		}
		return c, nil
	}}}
}
func brokerCall(ctx context.Context, operation string, request BrokerRequest, body io.Reader) (*BrokerStatus, error) {
	if body == nil {
		data, err := json.Marshal(request)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://helper/"+operation, body)
	if err != nil {
		return nil, err
	}
	if operation == "candidate" {
		req.Header.Set("X-Upgrade-ID", request.ID)
	}
	resp, err := brokerClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upgrade helper: %s", strings.TrimSpace(string(data)))
	}
	var result BrokerStatus
	err = json.Unmarshal(data, &result)
	return &result, err
}
func BrokerOperation(ctx context.Context, operation, id string) (*BrokerStatus, error) {
	return brokerCall(ctx, operation, BrokerRequest{ID: id}, nil)
}
func UsesUpgradeBroker() bool {
	if effectiveUID() == 0 {
		return false
	}
	s, err := Load()
	return err == nil && s.UpgradeBroker
}
func brokerAvailable(s *Settings) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := BrokerOperation(ctx, "status", "")
	if err != nil {
		return false
	}
	return result.Executable == s.Executable && result.Home == s.Home && result.User == s.User
}
func PrepareBrokerUpgrade(ctx context.Context, id, version, manifestURL, candidate string) error {
	client, err := agentupgrade.HTTPClient()
	if err != nil {
		return err
	}
	data, err := agentupgrade.Get(ctx, client, manifestURL, 1<<20)
	if err != nil {
		return err
	}
	signature, err := agentupgrade.Get(ctx, client, manifestURL+".sig", ed25519.SignatureSize)
	if err != nil {
		return err
	}
	if _, err = brokerCall(ctx, "prepare", BrokerRequest{ID: id, Version: version, Manifest: data, Signature: signature}, nil); err != nil {
		return err
	}
	f, err := os.Open(candidate)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = brokerCall(ctx, "candidate", BrokerRequest{ID: id}, f)
	return err
}

func installUpgradeBroker(s *Settings, publicKey string) error {
	if publicKey == "" {
		return nil
	}
	if err := requireRoot(); err != nil {
		return err
	}
	if _, err := agentupgrade.PublicKey(publicKey); err != nil {
		return err
	}
	if s.Backend != Launchd && s.Backend != Systemd {
		return fmt.Errorf("privileged upgrade helper requires launchd or systemd")
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
	if uid == 0 {
		return fmt.Errorf("privileged upgrade helper requires a non-root agent account")
	}
	// Never elevate an installation whose executable directory can be replaced
	// by the service account. The daemon and all of its ancestors stay root-owned.
	if err := rootPath(filepath.Dir(s.Executable), true); err != nil {
		return err
	}
	if err := rootPath(s.Executable, false); err != nil {
		return err
	}
	dir := brokerDirectory()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := rootPath(dir, true); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0755); err != nil {
		return err
	}
	if platform == "darwin" {
		if err := os.Chmod(launchConfigurationDirectory, 0755); err != nil {
			return err
		}
	}
	configPath := filepath.Join(dir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var old brokerConfig
		if json.Unmarshal(data, &old) != nil || old.Settings.Executable != s.Executable || old.UID != uid || old.Settings.Home != s.Home {
			return fmt.Errorf("uninstall the existing upgrade helper before changing its registration")
		}
	}
	helper := filepath.Join(dir, Name)
	if err := brokerCopy(s.Executable, helper, 0755); err != nil {
		return err
	}
	s.UpgradeBroker = true
	data, err := json.Marshal(brokerConfig{Settings: *s, PublicKey: publicKey, UID: uid, GID: gid})
	if err != nil {
		return err
	}
	if err = writeFile(configPath, data, 0600); err != nil {
		return err
	}
	if s.Backend == Launchd {
		// launchd must not open logs through a user-controlled directory.
		// The account may write the log, but cannot replace its directory entry.
		logFile, err := os.OpenFile(filepath.Join(dir, "agent.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		err = logFile.Chown(uid, gid)
		closeErr := logFile.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		mainPlist := filepath.Join(launchConfigurationDirectory, Label+".plist")
		if err := writeFile(mainPlist, launchPlist(s, Label, s.Executable, []string{"service", "run", "--settings", s.path()}, true), 0644); err != nil {
			return err
		}
		if err := setAutoStartNative(s, s.AutoStart); err != nil {
			return err
		}
		running, err := runningNative(context.Background(), s)
		if err != nil {
			return err
		}
		if running {
			if err := stopNative(context.Background(), s); err != nil {
				return err
			}
			if err := startNative(context.Background(), s); err != nil {
				return err
			}
		}
		rootSettings := &Settings{User: "root", Home: "/var/root", Directory: dir}
		plist := launchPlist(rootSettings, brokerLabel, helper, []string{"upgrade-broker"}, true)
		// Logs must also stay in the root-owned helper directory.
		plist = bytes.ReplaceAll(plist, []byte("/var/root/.zadig-agent/log/service.log"), []byte(filepath.Join(dir, "helper.log")))
		path := filepath.Join(launchDaemonDirectory, brokerLabel+".plist")
		if err := writeFile(path, plist, 0644); err != nil {
			return err
		}
		_, _ = runCommand(context.Background(), "launchctl", "bootout", "system/"+brokerLabel)
		if _, err = runCommand(context.Background(), "launchctl", "enable", "system/"+brokerLabel); err != nil {
			return err
		}
		_, err = runCommand(context.Background(), "launchctl", "bootstrap", "system", path)
		return err
	}
	unit := []byte(fmt.Sprintf("[Unit]\nDescription=Zadig Agent Upgrade Helper\nAfter=local-fs.target\n[Service]\nExecStart=%s upgrade-broker\nWorkingDirectory=%s\nUser=root\nUMask=0077\nRestart=always\nRestartSec=3\n[Install]\nWantedBy=multi-user.target\n", unitQuote(helper), strings.ReplaceAll(dir, "%", "%%")))
	if err = writeFile(filepath.Join(systemdDirectory, Name+"-upgrade-helper.service"), unit, 0644); err != nil {
		return err
	}
	if _, err = runCommand(context.Background(), "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err = runCommand(context.Background(), "systemctl", "enable", Name+"-upgrade-helper.service"); err != nil {
		return err
	}
	_, err = runCommand(context.Background(), "systemctl", "restart", Name+"-upgrade-helper.service")
	return err
}

func rootPath(path string, directory bool) error {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if !rootOwned(info) || info.Mode()&0022 != 0 || info.Mode()&os.ModeSymlink != 0 || (current == path && info.IsDir() != directory) {
			return fmt.Errorf("upgrade helper requires root-owned, non-writable path: %s", current)
		}
		if current == filepath.Dir(current) {
			return nil
		}
	}
}
func brokerCopy(from, to string, mode os.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	return brokerWrite(to, mode, func(out *os.File) error { _, err := io.Copy(out, in); return err })
}
func brokerWrite(to string, mode os.FileMode, write func(*os.File) error) error {
	out, err := os.CreateTemp(filepath.Dir(to), ".upgrade-*")
	if err != nil {
		return err
	}
	defer os.Remove(out.Name())
	if err = out.Chmod(mode); err == nil {
		err = write(out)
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(out.Name(), to); err != nil {
		return err
	}
	d, err := os.Open(filepath.Dir(to))
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

type upgradeBroker struct {
	mu          sync.Mutex
	config      brokerConfig
	transaction *brokerTransaction
}

func (b *upgradeBroker) transactionPath() string {
	return filepath.Join(brokerDirectory(), "transaction.json")
}
func (b *upgradeBroker) save() error {
	data, err := json.Marshal(b.transaction)
	if err != nil {
		return err
	}
	return brokerWrite(b.transactionPath(), 0600, func(f *os.File) error { _, err := f.Write(data); return err })
}
func validBrokerID(id string) bool {
	data, err := hex.DecodeString(id)
	return err == nil && len(data) == 12 && id == strings.ToLower(id)
}
func (b *upgradeBroker) status() *BrokerStatus {
	result := &BrokerStatus{Executable: b.config.Settings.Executable, Home: b.config.Settings.Home, User: b.config.Settings.User}
	if b.transaction != nil {
		result.ID = b.transaction.ID
		result.Status = b.transaction.Status
	}
	return result
}
func (b *upgradeBroker) rollback(ctx context.Context) error {
	t := b.transaction
	if t == nil {
		return fmt.Errorf("no upgrade transaction")
	}
	if t.Status == "committed" {
		return fmt.Errorf("upgrade already committed")
	}
	if t.Status == "rolled_back" {
		return nil
	}
	if t.Status == "applying" || t.Status == "applied" || t.Status == "restoring" {
		t.Status = "restoring"
		if err := b.save(); err != nil {
			return err
		}
		if err := stopNative(ctx, &b.config.Settings); err != nil {
			return err
		}
		if err := brokerCopy(filepath.Join(brokerDirectory(), "previous"), b.config.Settings.Executable, 0755); err != nil {
			return err
		}
	}
	t.Status = "rolled_back"
	t.Deadline = time.Time{}
	return b.save()
}
func (b *upgradeBroker) handle(ctx context.Context, operation string, r BrokerRequest, body io.Reader) error {
	t := b.transaction
	if operation == "status" {
		return nil
	}
	if operation == "prepare" {
		if !validBrokerID(r.ID) {
			return fmt.Errorf("invalid upgrade ID")
		}
		if t != nil && t.Status != "committed" && t.Status != "rolled_back" {
			return fmt.Errorf("another privileged upgrade is active")
		}
		if t != nil && t.ID == r.ID {
			return fmt.Errorf("upgrade ID already used")
		}
		key, err := agentupgrade.PublicKey(b.config.PublicKey)
		if err != nil {
			return err
		}
		if !ed25519.Verify(key, r.Manifest, r.Signature) {
			return fmt.Errorf("invalid agent release signature")
		}
		manifest, err := agentupgrade.ParseManifest(r.Manifest, r.Version)
		if err != nil {
			return err
		}
		asset, err := manifest.Asset(runtime.GOOS, runtime.GOARCH)
		if err != nil {
			return err
		}
		b.transaction = &brokerTransaction{ID: r.ID, Status: "prepared", Asset: *asset, Deadline: time.Now().Add(5 * time.Minute)}
		return b.save()
	}
	if operation == "start" {
		return startNative(ctx, &b.config.Settings)
	}
	if operation == "stop" {
		return stopNative(ctx, &b.config.Settings)
	}
	if !validBrokerID(r.ID) || t == nil || t.ID != r.ID {
		return fmt.Errorf("upgrade transaction does not match")
	}
	switch operation {
	case "candidate":
		if t.Status != "prepared" {
			return fmt.Errorf("upgrade is not prepared")
		}
		err := brokerWrite(filepath.Join(brokerDirectory(), "candidate"), 0755, func(out *os.File) error {
			hash := sha256.New()
			n, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(body, t.Asset.Size+1))
			if err != nil {
				return err
			}
			if n != t.Asset.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), t.Asset.SHA256) {
				return fmt.Errorf("agent asset size or checksum mismatch")
			}
			return nil
		})
		if err != nil {
			return err
		}
		t.Status = "ready"
		return b.save()
	case "launch":
		if t.Status != "ready" && t.Status != "applied" && t.Status != "applying" && t.Status != "restoring" {
			return fmt.Errorf("upgrade is not ready")
		}
		t.Deadline = time.Now().Add(3 * time.Minute)
		if err := b.save(); err != nil {
			return err
		}
		helper := filepath.Join(b.config.Settings.Home, ".zadig-agent", "upgrade", "staging", r.ID, "helper")
		return launchHelperNative(ctx, &b.config.Settings, helper, r.ID)
	case "apply":
		if t.Status != "ready" {
			return fmt.Errorf("upgrade is not ready for replacement")
		}
		if err := rootPath(b.config.Settings.Executable, false); err != nil {
			return err
		}
		if err := brokerCopy(b.config.Settings.Executable, filepath.Join(brokerDirectory(), "previous"), 0755); err != nil {
			return err
		}
		t.Status = "applying"
		if err := b.save(); err != nil {
			return err
		}
		if err := brokerCopy(filepath.Join(brokerDirectory(), "candidate"), b.config.Settings.Executable, 0755); err != nil {
			return err
		}
		t.Status = "applied"
		return b.save()
	case "rollback":
		return b.rollback(ctx)
	case "commit":
		if t.Status != "applied" && t.Status != "committed" {
			return fmt.Errorf("upgrade has not been applied")
		}
		t.Status = "committed"
		t.Deadline = time.Time{}
		if err := b.save(); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(brokerDirectory(), "previous")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		_ = os.Remove(filepath.Join(brokerDirectory(), "candidate"))
		return nil
	default:
		return fmt.Errorf("unsupported upgrade helper operation")
	}
}
func (b *upgradeBroker) recoverExpired(ctx context.Context) error {
	t := b.transaction
	if t == nil || (!t.RestartPending && (t.Deadline.IsZero() || time.Now().Before(t.Deadline))) {
		return nil
	}
	t.RestartPending = true
	if err := b.save(); err != nil {
		return err
	}
	if err := b.rollback(ctx); err != nil {
		return err
	}
	if err := startNative(ctx, &b.config.Settings); err != nil {
		return err
	}
	t.RestartPending = false
	return b.save()
}

func (b *upgradeBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	operation := strings.TrimPrefix(r.URL.Path, "/")
	var request BrokerRequest
	if operation == "candidate" {
		request.ID = r.Header.Get("X-Upgrade-ID")
	} else {
		decoder := json.NewDecoder(io.LimitReader(r.Body, 2<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "invalid helper request", 400)
			return
		}
	}
	if err := b.handle(r.Context(), operation, request, r.Body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b.status())
}

func RunUpgradeBroker(ctx context.Context) error {
	if err := requireRoot(); err != nil {
		return err
	}
	// A root helper never inherits agent job environment, proxies or user PATH.
	os.Clearenv()
	_ = os.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	dir := brokerDirectory()
	if err := rootPath(dir, true); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return err
	}
	b := &upgradeBroker{}
	if err = json.Unmarshal(data, &b.config); err != nil {
		return err
	}
	if err = b.config.Settings.validate(); err != nil {
		return err
	}
	if b.config.UID <= 0 {
		return fmt.Errorf("invalid helper account")
	}
	if _, err = agentupgrade.PublicKey(b.config.PublicKey); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if self != filepath.Join(dir, Name) {
		return fmt.Errorf("upgrade helper must run from its protected installation")
	}
	lock, err := os.OpenFile(filepath.Join(dir, "helper.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = lockBroker(lock); err != nil {
		return err
	}
	if data, err := os.ReadFile(b.transactionPath()); err == nil {
		if err = json.Unmarshal(data, &b.transaction); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = os.Remove(brokerSocket())
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: brokerSocket(), Net: "unix"})
	if err != nil {
		return err
	}
	defer listener.Close()
	if err = os.Chown(brokerSocket(), b.config.UID, b.config.GID); err != nil {
		return err
	}
	if err = os.Chmod(brokerSocket(), 0600); err != nil {
		return err
	}
	server := &http.Server{Handler: b, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 4096, ReadTimeout: time.Minute, WriteTimeout: time.Minute, IdleTimeout: time.Second}
	authenticated := &brokerListener{UnixListener: listener, uid: b.config.UID}
	go func() { <-ctx.Done(); _ = server.Close() }()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				b.mu.Lock()
				rollbackCtx, cancel := context.WithTimeout(ctx, time.Minute)
				if err := b.recoverExpired(rollbackCtx); err != nil {
					fmt.Fprintln(os.Stderr, "upgrade recovery:", err)
				}
				cancel()
				b.mu.Unlock()
			}
		}
	}()
	err = server.Serve(authenticated)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type brokerListener struct {
	*net.UnixListener
	uid int
}

func (l *brokerListener) Accept() (net.Conn, error) {
	for {
		c, err := l.AcceptUnix()
		if err != nil {
			return nil, err
		}
		uid, err := peerUID(c)
		if err == nil && uid == l.uid {
			return c, nil
		}
		_ = c.Close()
	}
}

func uninstallUpgradeBroker(ctx context.Context, s *Settings) error {
	if !s.UpgradeBroker {
		return nil
	}
	if err := requireRoot(); err != nil {
		return err
	}
	if s.Backend == Launchd {
		_, _ = runCommand(ctx, "launchctl", "bootout", "system/"+brokerLabel)
		if err := os.Remove(filepath.Join(launchDaemonDirectory, brokerLabel+".plist")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else if s.Backend == Systemd {
		if _, err := runCommand(ctx, "systemctl", "disable", "--now", Name+"-upgrade-helper.service"); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(systemdDirectory, Name+"-upgrade-helper.service")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if _, err := runCommand(ctx, "systemctl", "daemon-reload"); err != nil {
			return err
		}
	}
	return os.RemoveAll(brokerDirectory())
}
