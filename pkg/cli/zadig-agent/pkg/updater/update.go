package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/network"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Probe struct {
	Version  string `json:"version"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Protocol int    `json:"protocol"`
}

func CurrentProbe() Probe {
	return Probe{config.BuildAgentVersion, runtime.GOOS, runtime.GOARCH, 1}
}

func Supported() bool {
	if runtime.GOOS == "windows" && runtime.GOARCH != "amd64" {
		return false
	}
	if managedService() {
		return false
	}
	_, err := agentupgrade.SecureURL(config.GetServerURL())
	return err == nil
}

type Manager struct {
	controller *agent.AgentController
	stop       chan struct{}
	mu         sync.Mutex
	active     bool
}

func NewManager(controller *agent.AgentController, stop chan struct{}) (*Manager, error) {
	s, err := readState()
	if err != nil {
		return nil, err
	}
	if s != nil && (!s.Status.Terminal() || (s.Status == agentupgrade.RolledBack && !s.Healthy) || s.needsManualRecovery(config.BuildAgentVersion)) {
		controller.SetPaused(true)
	}
	return &Manager{controller: controller, stop: stop}, nil
}

func (m *Manager) Report() (*agentupgrade.Report, error) {
	s, err := readState()
	if err != nil || s == nil {
		return nil, err
	}
	r := s.Report
	return &r, nil
}

func (m *Manager) Handle(ctx context.Context, response *network.HeartbeatServerResponse) error {
	s, err := readState()
	if err != nil {
		return err
	}
	if s != nil && s.Status == agentupgrade.Verifying {
		accepted := response.UpgradeAccepted
		if accepted != nil && accepted.ID == s.ID && accepted.Status == agentupgrade.Verifying && strings.TrimPrefix(config.BuildAgentVersion, "v") == s.TargetVersion {
			if err := updateState(s.ID, func(s *state) { s.Healthy = true }); err != nil {
				return err
			}
		}
		p, err := statePath()
		if err != nil {
			return err
		}
		lease, err := tryLock(p + ".helper.lock")
		if errors.Is(err, errLocked) {
			return nil
		}
		if err != nil {
			return err
		}
		// The helper may have committed and exited immediately after the health
		// acknowledgement. Read the durable state again before requesting recovery.
		current, readErr := readState()
		lease.Close()
		if readErr != nil {
			return readErr
		}
		if current == nil || current.ID != s.ID {
			return fmt.Errorf("upgrade state changed during health confirmation")
		}
		if current.Status == agentupgrade.Verifying {
			if helper, err := launch(s.helper(), []string{"upgrade-apply"}, s.WorkingDirectory, s.Environment); err != nil {
				return err
			} else {
				helper.Process.Release()
			}
			select {
			case m.stop <- struct{}{}:
			default:
			}
		} else if current.Status.Terminal() && !current.needsManualRecovery(config.BuildAgentVersion) {
			m.controller.SetPaused(false)
		}

		return nil
	}
	if s != nil && s.Status == agentupgrade.RolledBack && !s.Healthy {
		if strings.TrimPrefix(config.BuildAgentVersion, "v") != strings.TrimPrefix(s.FromVersion, "v") {
			return fmt.Errorf("rollback agent version mismatch")
		}
		if err := updateState(s.ID, func(s *state) { s.Healthy = true }); err != nil {
			return err
		}
	}
	if s != nil && s.Status.Terminal() {
		if !s.needsManualRecovery(config.BuildAgentVersion) {
			m.controller.SetPaused(false)
		}
	}
	instruction := response.Upgrade
	if instruction == nil || !Supported() || (s != nil && s.ID == instruction.ID) {
		return nil
	}
	if s != nil && s.needsManualRecovery(config.BuildAgentVersion) {
		return fmt.Errorf("previous agent upgrade needs manual recovery")
	}
	p, err := statePath()
	if err != nil {
		return err
	}
	lease, err := tryLock(p + ".helper.lock")
	if errors.Is(err, errLocked) {
		return nil
	}
	if err != nil {
		return err
	}
	lease.Close()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active || (s != nil && !s.Status.Terminal()) {
		return nil
	}
	if _, err := primitive.ObjectIDFromHex(instruction.ID); err != nil {
		return fmt.Errorf("invalid upgrade request ID")
	}

	if s != nil {
		if err := validateState(s); err != nil {
			return err
		}
		if err := os.RemoveAll(s.Directory); err != nil {
			log.Warnf("remove previous upgrade staging directory: %v", err)
		}
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	st := &state{Report: agentupgrade.Report{ID: instruction.ID, Status: agentupgrade.Downloading}, FromVersion: config.BuildAgentVersion, TargetVersion: instruction.TargetVersion, Executable: executable, WorkingDirectory: workingDir, Arguments: []string{"start"}, Environment: append(cleanEnvironment(os.Environ()), config.BackgroundEnv+"=1"), Directory: filepath.Join(filepath.Dir(executable), ".zadig-upgrade-"+instruction.ID)}
	if err := changeState(func(old *state) (*state, error) {
		if old != nil && !old.Status.Terminal() {
			return nil, fmt.Errorf("another upgrade is active")
		}
		return st, nil
	}); err != nil {
		return err
	}
	m.active = true
	go func() {
		defer func() { m.mu.Lock(); m.active = false; m.mu.Unlock() }()
		if err := m.prepare(ctx, instruction, st); err != nil {
			log.Errorf("agent upgrade %s failed: %v", st.ID, err)
			if stateErr := updateState(st.ID, func(s *state) { s.Status = agentupgrade.Failed; s.Error = err.Error() }); stateErr != nil {
				log.Errorf("save upgrade failure: %v", stateErr)
			}
			m.controller.SetPaused(false)
		}
	}()
	return nil
}

func (m *Manager) prepare(ctx context.Context, instruction *agentupgrade.Instruction, s *state) error {
	if !agentupgrade.Newer(instruction.TargetVersion, config.BuildAgentVersion) {
		return fmt.Errorf("upgrade target is not newer than running version")
	}
	if _, err := agentupgrade.SecureURL(instruction.ManifestURL); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 70*time.Minute)
	defer cancel()
	if err := os.Mkdir(s.Directory, 0700); err != nil {
		return fmt.Errorf("create upgrade staging directory: %w", err)
	}
	if err := syncDirectory(filepath.Dir(s.Directory)); err != nil {
		return err
	}
	client, err := agentupgrade.HTTPClient()
	if err != nil {
		return err
	}
	manifest, err := agentupgrade.FetchManifest(ctx, client, instruction.ManifestURL, instruction.TargetVersion)
	if err != nil {
		return err
	}
	asset, err := manifest.Asset(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	free, err := diskFree(s.Directory)
	if err != nil {
		return err
	}
	info, err := os.Stat(s.Executable)
	if err != nil {
		return err
	}
	if free < uint64(asset.Size+2*agentupgrade.MaxAssetSize+info.Size()) {
		return fmt.Errorf("insufficient disk space for upgrade and rollback")
	}
	url, err := agentupgrade.AssetURL(instruction.ManifestURL, asset.Filename)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		err = download(ctx, client, url, s.candidate(), asset)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	if err != nil {
		return err
	}
	if err := os.Chmod(s.candidate(), 0700); err != nil {
		return err
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, 15*time.Second)
	defer probeCancel()
	output, err := exec.CommandContext(probeCtx, s.candidate(), "upgrade-probe").Output()
	if err != nil {
		return fmt.Errorf("probe new agent: %w", err)
	}
	var probe Probe
	if err := json.Unmarshal(output, &probe); err != nil {
		return err
	}
	if strings.TrimPrefix(probe.Version, "v") != s.TargetVersion || probe.OS != runtime.GOOS || probe.Arch != runtime.GOARCH || probe.Protocol != 1 {
		return fmt.Errorf("new agent version, platform or upgrade protocol mismatch")
	}
	if err := copyFile(s.Executable, s.helper(), 0700); err != nil {
		return err
	}
	if err := updateState(s.ID, func(s *state) { s.Status = agentupgrade.Draining }); err != nil {
		return err
	}
	drainCtx, drainCancel := context.WithTimeout(ctx, time.Hour)
	defer drainCancel()
	if err := m.controller.PauseAndDrain(drainCtx, instruction.Force); err != nil {
		return fmt.Errorf("drain agent jobs: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := updateState(s.ID, func(s *state) { s.Status = agentupgrade.Restarting }); err != nil {
		return err
	}
	// Persist the handoff on the server before the old agent exits, so a
	// rollback can be acknowledged even if no intermediate heartbeat was sent.
	if err := reportHandoff(s.ID); err != nil {
		return err
	}
	helper, err := launch(s.helper(), []string{"upgrade-apply"}, s.WorkingDirectory, s.Environment)
	if err != nil {
		return err
	}
	defer helper.Process.Release()
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for {
		current, err := readState()
		if err != nil {
			return err
		}
		if current != nil && current.ID == s.ID && current.HelperPID > 0 && current.HelperPID == helper.Process.Pid {
			select {
			case m.stop <- struct{}{}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		select {
		case <-deadline.C:
			helper.Process.Kill()
			return fmt.Errorf("upgrade helper did not acknowledge handoff")
		case <-ctx.Done():
			helper.Process.Kill()
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func reportHandoff(id string) error {
	platform, err := osutil.GetPlatformParameters()
	if err != nil {
		return err
	}
	params := &network.HeartbeatParameters{}
	if err := osutil.IToi(platform, params); err != nil {
		return err
	}
	params.AgentVersion, params.UpgradeSupported = config.BuildAgentVersion, Supported()
	params.UpgradeReport = &agentupgrade.Report{ID: id, Status: agentupgrade.Restarting}
	resp, err := network.Heartbeat(&network.AgentConfig{Token: config.GetAgentToken(), URL: config.GetServerURL()}, params)
	if err != nil {
		return err
	}
	if resp.NeedOffline || resp.UpgradeAccepted == nil || resp.UpgradeAccepted.ID != id || resp.UpgradeAccepted.Status != agentupgrade.Restarting {
		return fmt.Errorf("server did not accept upgrade handoff")
	}
	return nil
}

func extension() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

func download(ctx context.Context, client *http.Client, url, path string, asset *agentupgrade.Asset) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent download returned HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, asset.Size+1))
	if err != nil {
		return err
	}
	if n != asset.Size || !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), asset.SHA256) {
		return fmt.Errorf("agent asset size or checksum mismatch")
	}
	return f.Sync()
}
