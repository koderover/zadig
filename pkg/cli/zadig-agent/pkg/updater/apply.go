package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func cleanEnvironment(env []string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		if !strings.HasPrefix(entry, "ZADIG_AGENT_UPGRADE_ID=") {
			result = append(result, entry)
		}
	}
	return result
}

func launch(path string, args []string, dir string, env []string) (*exec.Cmd, error) {
	p, err := statePath()
	if err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(filepath.Dir(p), "helper.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer logFile.Close()
	cmd := exec.Command(path, args...)
	cmd.Dir, cmd.Env, cmd.Stdout, cmd.Stderr = dir, env, logFile, logFile
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func copyFile(from, to string, mode os.FileMode) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		os.Remove(to)
		return err
	}
	if closeErr != nil {
		os.Remove(to)
		return closeErr
	}
	return syncDirectory(filepath.Dir(to))
}

func validateState(s *state) error {
	if _, err := primitive.ObjectIDFromHex(s.ID); err != nil {
		return fmt.Errorf("invalid local upgrade ID")
	}
	if !filepath.IsAbs(s.Executable) || !filepath.IsAbs(s.Directory) || s.Directory != filepath.Join(filepath.Dir(s.Executable), ".zadig-upgrade-"+s.ID) {
		return fmt.Errorf("invalid local upgrade paths")
	}
	if len(s.Arguments) != 1 || s.Arguments[0] != "start" {
		return fmt.Errorf("invalid agent restart arguments")
	}
	return nil
}

// Apply runs in a detached copy of the old binary. The instance lock is held
// throughout file replacement; only the new agent may hold it during probing.
func Apply() error {
	p, err := statePath()
	if err != nil {
		return err
	}
	helperLock, err := tryLock(p + ".helper.lock")
	if err != nil {
		return err
	}
	defer helperLock.Close()
	s, err := readState()
	if err != nil {
		return err
	}
	if s == nil || s.Status.Terminal() {
		return nil
	}
	if err := validateState(s); err != nil {
		return err
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return err
	}
	if self != s.helper() {
		return fmt.Errorf("upgrade helper must run from the staging directory")
	}
	if err := updateState(s.ID, func(s *state) { s.HelperPID = os.Getpid() }); err != nil {
		return err
	}
	instance, err := instancePath()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	lease, err := waitLock(ctx, instance)
	if err != nil {
		return err
	}
	defer func() {
		if lease != nil {
			lease.Close()
		}
	}()
	s, err = readState()
	if err != nil {
		return err
	}
	if s.Status.Terminal() {
		return nil
	}
	if _, err := os.Stat(s.backup()); err == nil || s.Status == agentupgrade.Verifying || s.Restoring {
		return rollback(s, &lease, fmt.Errorf("recovering interrupted upgrade"))
	}
	configPath, err := config.GetAgentConfigFilePath()
	if err != nil {
		return err
	}
	if !s.ConfigSaved {
		backup, err := os.ReadFile(configPath)
		if err != nil {
			return rollback(s, &lease, fmt.Errorf("read agent config: %w", err))
		}
		if err := atomicWrite(filepath.Join(s.Directory, "config.backup"), backup, 0600); err != nil {
			return rollback(s, &lease, fmt.Errorf("back up agent config: %w", err))
		}
		if err := updateState(s.ID, func(s *state) { s.ConfigSaved = true }); err != nil {
			return rollback(s, &lease, err)
		}
		s.ConfigSaved = true
	}

	if err := replaceFile(s.Executable, s.backup()); err != nil {
		return rollback(s, &lease, err)
	}
	if err := replaceFile(s.candidate(), s.Executable); err != nil {
		return rollback(s, &lease, err)
	}
	if err := updateState(s.ID, func(s *state) { s.Status = agentupgrade.Verifying; s.Healthy = false }); err != nil {
		return rollback(s, &lease, err)
	}
	lease.Close()
	lease = nil
	env := append(cleanEnvironment(s.Environment), "ZADIG_AGENT_UPGRADE_ID="+s.ID)
	child, err := launch(s.Executable, s.Arguments, s.WorkingDirectory, env)
	if err != nil {
		return rollback(s, &lease, fmt.Errorf("start new agent: %w", err))
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	if err := updateState(s.ID, func(s *state) { s.ChildPID = child.Process.Pid }); err != nil {
		child.Process.Kill()
		<-done
		return rollback(s, &lease, err)
	}
	timer := time.NewTimer(90 * time.Second)
	defer timer.Stop()
	for {
		current, err := readState()
		if err == nil && current.ID == s.ID && current.Status == agentupgrade.Verifying && current.Healthy {
			err := updateState(s.ID, func(s *state) { s.Status = agentupgrade.Succeeded; s.Error = "" })
			if err == nil {
				cleanup(s)
			}
			return err
		}
		select {
		case childErr := <-done:
			return rollback(s, &lease, fmt.Errorf("new agent exited before health confirmation: %v", childErr))
		case <-timer.C:
			if err := child.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return markFailure(s.ID, fmt.Errorf("stop unhealthy new agent: %w", err))
			}
			<-done
			return rollback(s, &lease, fmt.Errorf("new agent health confirmation timed out"))
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func markFailure(id string, cause error) error {
	if err := updateState(id, func(s *state) { s.Status = agentupgrade.Failed; s.Error = cause.Error() }); err != nil {
		return fmt.Errorf("%v; persist failure: %w", cause, err)
	}
	return cause
}

func rollback(s *state, lease **os.File, cause error) error {
	if *lease == nil {
		instance, err := instancePath()
		if err != nil {
			return markFailure(s.ID, err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		lock, err := waitLock(ctx, instance)
		if err != nil {
			return markFailure(s.ID, fmt.Errorf("%v; rollback lock: %w", cause, err))
		}
		*lease = lock
	}
	if err := updateState(s.ID, func(s *state) { s.Restoring = true }); err != nil {
		return err
	}

	if _, err := os.Stat(s.backup()); err == nil {
		if err := replaceFile(s.backup(), s.Executable); err != nil {
			return markFailure(s.ID, fmt.Errorf("%v; restore old executable: %w", cause, err))
		}
	} else {
		probeCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		output, probeErr := exec.CommandContext(probeCtx, s.Executable, "upgrade-probe").Output()
		cancel()
		var probe Probe
		if probeErr != nil || json.Unmarshal(output, &probe) != nil || strings.TrimPrefix(probe.Version, "v") != strings.TrimPrefix(s.FromVersion, "v") {
			return markFailure(s.ID, fmt.Errorf("%v; old executable backup is missing", cause))
		}
	}
	if s.ConfigSaved {
		backup, err := os.ReadFile(filepath.Join(s.Directory, "config.backup"))
		if err != nil {
			return markFailure(s.ID, fmt.Errorf("%v; read config backup: %w", cause, err))
		}
		configPath, err := config.GetAgentConfigFilePath()
		if err != nil {
			return markFailure(s.ID, err)
		}
		if err := atomicWrite(configPath, backup, 0600); err != nil {
			return markFailure(s.ID, fmt.Errorf("%v; restore old config: %w", cause, err))
		}
	}

	if err := updateState(s.ID, func(s *state) {
		s.Status = agentupgrade.RolledBack
		s.Error = cause.Error()
		s.Healthy = false
		s.Restoring = false
	}); err != nil {
		return err
	}
	(*lease).Close()
	*lease = nil
	child, err := launch(s.Executable, s.Arguments, s.WorkingDirectory, cleanEnvironment(s.Environment))
	if err != nil {
		return markFailure(s.ID, fmt.Errorf("%v; restart old agent: %w", cause, err))
	}
	if err := updateState(s.ID, func(s *state) { s.ChildPID = child.Process.Pid }); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		current, err := readState()
		if err == nil && current.ID == s.ID && current.Healthy {
			cleanup(s)
			return nil
		}
		select {
		case err := <-done:
			return markFailure(s.ID, fmt.Errorf("%v; old agent exited: %v", cause, err))
		case <-deadline.C:
			return markFailure(s.ID, fmt.Errorf("%v; old agent did not confirm recovery", cause))
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func cleanup(s *state) {
	// The running helper is removed by the next upgrade (Windows locks its image).
	for _, path := range []string{s.candidate(), s.backup(), filepath.Join(s.Directory, "config.backup")} {
		_ = os.Remove(path)
	}
}
