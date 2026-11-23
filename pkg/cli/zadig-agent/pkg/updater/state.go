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
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
)

var errLocked = errors.New("agent lock is held by another process")

type state struct {
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

func readState() (*state, error) {
	p, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
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
	s, err := readState()
	if err != nil {
		return err
	}
	s, err = update(s)
	if err != nil {
		return err
	}
	data, err := json.Marshal(s)
	if err != nil {
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
	if err != nil || s == nil {
		return err
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
		if os.Getenv("ZADIG_AGENT_UPGRADE_ID") == s.ID && s.Status == agentupgrade.Verifying {
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
	if _, err := launch(s.helper(), []string{"upgrade-apply"}, s.WorkingDirectory, s.Environment); err != nil {
		return fmt.Errorf("restart upgrade helper: %w", err)
	}
	return fmt.Errorf("recovering interrupted agent upgrade; helper will restart the agent")
}

func (s *state) needsManualRecovery(version string) bool {
	return s.Status == agentupgrade.Failed && (s.Restoring || strings.TrimPrefix(version, "v") != strings.TrimPrefix(s.FromVersion, "v"))
}
