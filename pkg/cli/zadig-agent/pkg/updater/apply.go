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
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
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
	if err := s.validateMetadata(); err != nil {
		return err
	}
	if s.ServiceBackend != "" {
		saved, err := daemon.Load()
		if err != nil {
			return err
		}
		if saved.Backend != s.ServiceBackend || saved.Executable != s.Executable {
			return fmt.Errorf("upgrade service configuration does not match the running agent")
		}
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
	if s.ServiceBackend != "" {
		if err := daemon.PrepareUpgrade(); err != nil {
			return err
		}
		defer func() {
			if err := daemon.FinishUpgrade(); err != nil {
				fmt.Fprintf(os.Stderr, "restore service recovery policy: %v\n", err)
			}
		}()
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
		err := daemon.Stop(stopCtx)
		cancel()
		if err != nil {
			return markFailure(s.ID, fmt.Errorf("stop old agent service: %w", err))
		}
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
	current, err := readState()
	if err != nil {
		return err
	}
	if current == nil {
		return fmt.Errorf("upgrade state disappeared before replacement")
	}
	s = current
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

	if err := replaceExecutable(s); err != nil {
		return rollback(s, &lease, err)
	}
	if err := updateState(s.ID, func(s *state) { s.Status = agentupgrade.Verifying; s.Healthy = false }); err != nil {
		return rollback(s, &lease, err)
	}
	lease.Close()
	lease = nil
	env := append(cleanEnvironment(s.Environment), "ZADIG_AGENT_UPGRADE_ID="+s.ID)
	child, err := startAgent(s, env)
	if err != nil {
		return rollback(s, &lease, fmt.Errorf("start new agent: %w", err))
	}
	defer child.close()
	done := child.done
	if child.command != nil {
		if err := updateState(s.ID, func(s *state) { s.ChildPID = child.command.Process.Pid }); err != nil {
			if stopErr := child.stop(); stopErr != nil && !errors.Is(stopErr, os.ErrProcessDone) {
				err = errors.Join(err, fmt.Errorf("stop new agent: %w", stopErr))
			}
			return rollback(s, &lease, err)
		}
	}

	timer := time.NewTimer(90 * time.Second)
	defer timer.Stop()
	for {
		current, err := readState()
		if err == nil && current != nil && current.ID == s.ID && current.Status == agentupgrade.Verifying && current.Healthy {
			if s.ServiceBackend != "" {
				if err := daemon.FinishUpgrade(); err != nil {
					return markFailure(s.ID, fmt.Errorf("restore service recovery policy: %w", err))
				}
			}
			err := updateState(s.ID, func(s *state) { s.Status = agentupgrade.Succeeded; s.Error = "" })
			if err == nil {
				cleanup(s)
			}
			return err
		}
		select {
		case childErr := <-done:
			cause := fmt.Errorf("new agent exited before health confirmation: %v", childErr)
			if child.managed {
				if err := child.stop(); err != nil && !errors.Is(err, os.ErrProcessDone) {
					cause = errors.Join(cause, fmt.Errorf("stop failed service before rollback: %w", err))
				}
			}
			return rollback(s, &lease, cause)
		case <-timer.C:
			cause := fmt.Errorf("new agent health confirmation timed out")
			if err := child.stop(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				cause = errors.Join(cause, fmt.Errorf("stop unhealthy new agent: %w", err))
			}
			// rollback waits for the instance lock even when stopping reports an
			// error. Never replace files while the child may still be running.
			return rollback(s, &lease, cause)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func replaceExecutable(s *state) error {
	info, err := os.Stat(s.Executable)
	if err != nil {
		return err
	}
	// Staging files are private; retain the installed command's permissions.
	if err := os.Chmod(s.candidate(), info.Mode().Perm()); err != nil {
		return fmt.Errorf("preserve agent executable permissions: %w", err)
	}
	if err := replaceFile(s.Executable, s.backup()); err != nil {
		return err
	}
	return replaceFile(s.candidate(), s.Executable)
}

func markFailure(id string, cause error) error {
	if err := updateState(id, func(s *state) { s.Status = agentupgrade.Failed; s.Error = cause.Error() }); err != nil {
		return fmt.Errorf("%v; persist failure: %w", cause, err)
	}
	return cause
}

func rollback(s *state, lease **os.File, cause error) error {
	if err := updateState(s.ID, func(s *state) { s.Restoring = true; s.Error = cause.Error() }); err != nil {
		return err
	}
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
	child, err := startAgent(s, cleanEnvironment(s.Environment))
	if err != nil {
		return markFailure(s.ID, fmt.Errorf("%v; restart old agent: %w", cause, err))
	}
	defer child.close()
	if child.command != nil {
		if err := updateState(s.ID, func(s *state) { s.ChildPID = child.command.Process.Pid }); err != nil {
			return err
		}
	}
	done := child.done

	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		current, err := readState()
		if err == nil && current != nil && current.ID == s.ID && current.Healthy {
			if s.ServiceBackend != "" {
				if err := daemon.FinishUpgrade(); err != nil {
					return markFailure(s.ID, fmt.Errorf("restore service recovery policy after rollback: %w", err))
				}
			}
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
