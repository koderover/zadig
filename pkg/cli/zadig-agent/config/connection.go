package config

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	HomeEnv       = "ZADIG_AGENT_HOME"
	BackgroundEnv = "ZADIG_AGENT_BACKGROUND"
)

var ErrDisconnected = errors.New("agent was deliberately disconnected")

// Keep connection intent outside agent_config.yaml so upgrade rollback cannot
// restore an older connected state over a deliberate disconnect.
func ConnectionAllowed() (bool, error) {
	p, err := GetAgentConfigFilePath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(filepath.Dir(p), ".disconnected"))
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	return false, err
}

func SetConnectionAllowed(allowed bool) error {
	p, err := GetAgentConfigFilePath()
	if err != nil {
		return err
	}
	p = filepath.Join(filepath.Dir(p), ".disconnected")
	if allowed {
		err := os.Remove(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	// Only the existence of the marker matters; creating it is atomic.
	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
