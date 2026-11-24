//go:build windows

package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func replaceFile(from, to string) error {
	a, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return err
	}
	b, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(a, b, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func protectSettings(s *Settings, path string) error { return nil }

func detectBackend() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("administrator privileges are required to register Windows Service: %w", err)
	}
	m.Disconnect()
	return Windows, nil
}

func installNative(s *Settings, password string) error {
	account := s.User
	if strings.EqualFold(account, "LocalSystem") {
		account = ""
	} else if password == "" {
		return fmt.Errorf("Windows Service requires the running account password via --password-stdin; alternatively explicitly select --user LocalSystem")
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	start := uint32(mgr.StartManual)
	if s.AutoStart {
		start = mgr.StartAutomatic
	}
	service, err := m.CreateService(Name, s.Executable, mgr.Config{DisplayName: "Zadig Agent", Description: "Zadig workflow agent", StartType: start, ServiceStartName: account, Password: password}, "service", "run", "--home", s.Home)
	if err != nil {
		return err
	}
	defer service.Close()
	if err := service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 5 * time.Second}}, 86400); err != nil {
		service.Delete()
		return err
	}
	if err := service.SetRecoveryActionsOnNonCrashFailures(true); err != nil {
		service.Delete()
		return err
	}
	return nil
}

func withWindowsService(work func(*mgr.Service) error) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(Name)
	if err != nil {
		return err
	}
	defer s.Close()
	return work(s)
}

func setAutoStartNative(s *Settings, enabled bool) error {
	return withWindowsService(func(service *mgr.Service) error {
		c, err := service.Config()
		if err != nil {
			return err
		}
		c.StartType = mgr.StartManual
		if enabled {
			c.StartType = mgr.StartAutomatic
		}
		return service.UpdateConfig(c)
	})
}

func startNative(ctx context.Context, s *Settings) error {
	return withWindowsService(func(service *mgr.Service) error {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Running || status.State == svc.StartPending {
			return nil
		}
		if err := service.Start(); err != nil {
			return fmt.Errorf("start Windows Service (check the account password and its Log on as a service permission): %w", err)
		}
		return nil
	})
}

func stopNative(ctx context.Context, s *Settings) error {
	return withWindowsService(func(service *mgr.Service) error {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == svc.Stopped {
			return nil
		}
		if status.State != svc.StopPending {
			if _, err := service.Control(svc.Stop); err != nil && !errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
				return err
			}
		}
		timer := time.NewTicker(200 * time.Millisecond)
		defer timer.Stop()
		for {
			status, err := service.Query()
			if err != nil {
				return err
			}
			if status.State == svc.Stopped {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
			}
		}
	})
}

func runningNative(ctx context.Context, s *Settings) (bool, error) {
	running := false
	err := withWindowsService(func(service *mgr.Service) error {
		status, err := service.Query()
		if err == nil {
			running = status.State == svc.Running || status.State == svc.StartPending
		}
		return err
	})
	return running, err
}

func uninstallNative(ctx context.Context, s *Settings) error {
	if err := stopNative(ctx, s); err != nil {
		return err
	}
	return withWindowsService(func(service *mgr.Service) error { return service.Delete() })
}

func canUpgradeNative(s *Settings) bool {
	return withWindowsService(func(service *mgr.Service) error { _, err := service.Query(); return err }) == nil
}

func launchHelperNative(ctx context.Context, s *Settings, executable, id string) error {
	log, err := os.OpenFile(filepath.Join(s.Home, ".zadig-agent", "upgrade", "helper.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	cmd := exec.Command(executable, "upgrade-apply", "--home", s.Home)
	cmd.Dir = s.Directory
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_BREAKAWAY_FROM_JOB}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch independent upgrade helper: %w", err)
	}
	return cmd.Process.Release()
}

type windowsHandler struct {
	work func(context.Context, func()) error
}

func (h *windowsHandler) Execute(args []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	status := svc.Status{State: svc.StartPending, WaitHint: 15000, CheckPoint: 1}
	changes <- status
	ready := make(chan struct{}, 1)
	var once sync.Once
	done := make(chan error, 1)
	go func() { done <- h.work(ctx, func() { once.Do(func() { ready <- struct{}{} }) }) }()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ready:
			if status.State == svc.StartPending {
				status = svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
				changes <- status
			}
		case err := <-done:
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return false, 1
			}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				changes <- status
			case svc.Stop, svc.Shutdown:
				status = svc.Status{State: svc.StopPending, WaitHint: 15000, CheckPoint: 1}
				changes <- status
				cancel()
			}
		case <-ticker.C:
			if status.State == svc.StartPending || status.State == svc.StopPending {
				status.CheckPoint++
				changes <- status
			}
		}
	}
}

func Run(ctx context.Context, work func(context.Context, func()) error) error {
	service, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !service {
		return fmt.Errorf("service run must be launched by Windows Service Control Manager; use start for ordinary execution")
	}
	return svc.Run(Name, &windowsHandler{work: work})
}

func prepareUpgradeNative() error {
	return withWindowsService(func(service *mgr.Service) error { return service.ResetRecoveryActions() })
}
func finishUpgradeNative() error {
	return withWindowsService(func(service *mgr.Service) error {
		if err := service.SetRecoveryActions([]mgr.RecoveryAction{{Type: mgr.ServiceRestart, Delay: 5 * time.Second}}, 86400); err != nil {
			return err
		}
		return service.SetRecoveryActionsOnNonCrashFailures(true)
	})
}

func detachOrdinary(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
}
