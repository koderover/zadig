package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var errLocked = errors.New("agent lock is held by another process")
var errRecoveringUpgrade = errors.New("recovering interrupted agent upgrade; helper will restart the agent")

type state struct {
	ServiceBackend   string `json:"service_backend,omitempty"`
	PrivilegedHelper bool   `json:"privileged_helper,omitempty"`
	agentupgrade.Report
	FromVersion      string   `json:"from_version"`
	TargetVersion    string   `json:"target_version"`
	Executable       string   `json:"executable"`
	Directory        string   `json:"directory"`
	WorkingDirectory string   `json:"working_directory"`
	Arguments        []string `json:"arguments"`
	Environment      []string `json:"environment"`
	HelperPID        int      `json:"helper_pid"`
	ChildPID         int      `json:"child_pid"`
	Healthy          bool     `json:"healthy"`
	ConfigSaved      bool     `json:"config_saved"`
	Restoring        bool     `json:"restoring"`
}

func statePath() (string, error) {
	p, err := config.GetAgentConfigFilePath()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(filepath.Dir(p), "upgrade")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "state.json"), nil
}

func instancePath() (string, error) {
	p, err := statePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "instance.lock"), nil
}

func AcquireInstance() (*os.File, error) {
	p, err := instancePath()
	if err != nil {
		return nil, err
	}
	return tryLock(p)
}

func WaitForInstance(ctx context.Context) (*os.File, error) {
	p, err := instancePath()
	if err != nil {
		return nil, err
	}
	return waitLock(ctx, p)
}

func waitLock(ctx context.Context, path string) (*os.File, error) {
	for {
		f, err := tryLock(path)
		if !errors.Is(err, errLocked) {
			return f, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func readStateFile(path string) (*state, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil, err
	}
	if err := s.validateMetadata(); err != nil {
		return nil, nil, err
	}
	return &s, data, nil
}

func (s *state) validateMetadata() error {
	if _, err := primitive.ObjectIDFromHex(s.ID); err != nil {
		return fmt.Errorf("invalid local upgrade ID")
	}
	expectedDirectory := filepath.Join(filepath.Dir(s.Executable), ".zadig-upgrade-"+s.ID)
	if s.PrivilegedHelper {
		p, err := statePath()
		if err != nil {
			return err
		}
		expectedDirectory = filepath.Join(filepath.Dir(p), "staging", s.ID)
	}
	if !filepath.IsAbs(s.Executable) || s.Directory != expectedDirectory || !filepath.IsAbs(s.WorkingDirectory) {
		return fmt.Errorf("invalid local upgrade paths")
	}
	if s.FromVersion == "" || s.TargetVersion == "" || len(s.Arguments) != 1 || s.Arguments[0] != "start" {
		return fmt.Errorf("incomplete local upgrade metadata")
	}
	switch s.Status {
	case agentupgrade.Pending, agentupgrade.Downloading, agentupgrade.Draining, agentupgrade.Restarting, agentupgrade.Verifying, agentupgrade.Succeeded, agentupgrade.Failed, agentupgrade.RolledBack:
		return nil
	default:
		return fmt.Errorf("unknown local upgrade status %q", s.Status)
	}
}

func readState() (*state, error) {
	p, err := statePath()
	if err != nil {
		return nil, err
	}
	s, _, err := readStateFile(p)
	if err == nil {
		return s, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		if _, backupErr := os.Lstat(p + ".backup"); errors.Is(backupErr, os.ErrNotExist) {
			return nil, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lock, err := waitLock(ctx, p+".lock")
	if err != nil {
		return nil, err
	}
	defer lock.Close()
	return readStateLocked(p)
}

// The caller holds the state write lock, including while restoring a backup.
func readStateLocked(p string) (*state, error) {
	s, _, primaryErr := readStateFile(p)
	if primaryErr == nil {
		return s, nil
	}
	backup, data, backupErr := readStateFile(p + ".backup")
	if errors.Is(primaryErr, os.ErrNotExist) && errors.Is(backupErr, os.ErrNotExist) {
		return nil, nil
	}
	if backupErr != nil {
		return nil, fmt.Errorf("read upgrade state: %v; read backup: %w", primaryErr, backupErr)
	}
	if !errors.Is(primaryErr, os.ErrNotExist) {
		archived := fmt.Sprintf("%s.corrupt-%d", p, time.Now().UnixNano())
		if err := os.Rename(p, archived); err != nil {
			return nil, fmt.Errorf("preserve unreadable upgrade state: %w", err)
		}
	}
	if err := atomicWrite(p, data, 0600); err != nil {
		return nil, fmt.Errorf("restore upgrade state backup: %w", err)
	}
	log.Warnf("restored agent upgrade state from backup after: %v", primaryErr)
	return backup, nil
}

func changeState(update func(*state) (*state, error)) error {
	p, err := statePath()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lock, err := waitLock(ctx, p+".lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	s, err := readStateLocked(p)
	if err != nil {
		return err
	}
	s, err = update(s)
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("upgrade state update returned no metadata")
	}
	if err := s.validateMetadata(); err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// Write the same new snapshot to the backup first. A stale pre-replacement
	// snapshot must never be the only recovery record after files are replaced.
	if err := atomicWrite(p+".backup", data, 0600); err != nil {
		return err
	}
	return atomicWrite(p, data, 0600)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".upgrade-write-*")
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

func updateState(id string, update func(*state)) error {
	return changeState(func(s *state) (*state, error) {
		if s == nil || s.ID != id {
			return nil, fmt.Errorf("upgrade state does not match request")
		}
		update(s)
		return s, nil
	})
}

func (s *state) candidate() string { return filepath.Join(s.Directory, "candidate"+extension()) }
func (s *state) backup() string    { return filepath.Join(s.Directory, "previous"+extension()) }
func (s *state) helper() string    { return filepath.Join(s.Directory, "helper"+extension()) }

// CheckStartup is called with the instance lock held, before loading or writing
// agent configuration. Interrupted replacements are recovered by the helper.
func CheckStartup() error {
	s, err := readState()
	if err != nil {
		log.Errorf("upgrade state unavailable; task scheduling will remain paused: %v", err)
		return nil
	}
	if s != nil && s.PrivilegedHelper {
		if err := reconcileBroker(s); err != nil {
			return err
		}
		s, err = readState()
		if err != nil {
			return err
		}
	}
	return checkStartupState(s)
}

func checkStartupState(s *state) error {
	if s == nil {
		return nil
	}
	if s.Status.Terminal() {
		if s.needsManualRecovery(config.BuildAgentVersion) {
			return fmt.Errorf("previous agent upgrade requires manual recovery; inspect the upgrade state and helper log")
		}
		return nil
	}
	if s.Status == agentupgrade.Downloading || s.Status == agentupgrade.Draining {
		return updateState(s.ID, func(s *state) { s.Status = agentupgrade.Failed; s.Error = "agent restarted before upgrade handoff" })
	}
	p, err := statePath()
	if err != nil {
		return err
	}
	lock, err := tryLock(p + ".helper.lock")
	if errors.Is(err, errLocked) {
		if (os.Getenv("ZADIG_AGENT_UPGRADE_ID") == s.ID || (s.ServiceBackend != "" && daemon.RunningBackend() == s.ServiceBackend)) && s.Status == agentupgrade.Verifying {
			return nil
		}
		return fmt.Errorf("agent upgrade is being applied")
	}
	if err != nil {
		return err
	}
	lock.Close()
	if err := validateState(s); err != nil {
		return err
	}
	if helper, err := launchUpgradeHelper(context.Background(), s); err != nil {
		return fmt.Errorf("restart upgrade helper: %w", err)
	} else if helper != nil {
		helper.Process.Release()
	}
	return errRecoveringUpgrade
}

func (s *state) needsManualRecovery(version string) bool {
	return s.Status == agentupgrade.Failed && (s.Restoring || strings.TrimPrefix(version, "v") != strings.TrimPrefix(s.FromVersion, "v"))
}
