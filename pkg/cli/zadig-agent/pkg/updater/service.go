package updater

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
)

func launchUpgradeHelper(ctx context.Context, s *state) (*exec.Cmd, error) {
	if s.ServiceBackend != "" {
		if err := daemon.LaunchUpgradeHelper(ctx, s.helper(), s.ID); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return launch(s.helper(), []string{"upgrade-apply"}, s.WorkingDirectory, s.Environment)
}

type runningAgent struct {
	command *exec.Cmd
	done    chan error
	cancel  context.CancelFunc
	managed bool
}

func startAgent(s *state, environment []string) (*runningAgent, error) {
	p := &runningAgent{done: make(chan error, 1), managed: s.ServiceBackend != ""}
	if !p.managed {
		cmd, err := launch(s.Executable, s.Arguments, s.WorkingDirectory, environment)
		if err != nil {
			return nil, err
		}
		p.command = cmd
		go func() { p.done <- cmd.Wait() }()
		return p, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	err := daemon.Start(ctx)
	cancel()
	if err != nil {
		return nil, err
	}
	monitorCtx, stop := context.WithCancel(context.Background())
	p.cancel = stop
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		misses := 0
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				status, err := daemon.Status(monitorCtx)
				if err == nil && status["running"] == true {
					misses = 0
					continue
				}
				misses++
				if misses >= 5 {
					p.done <- fmt.Errorf("managed agent exited or service status is unavailable: %v", err)
					return
				}
			}
		}
	}()
	return p, nil
}

func (p *runningAgent) close() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *runningAgent) stop() error {
	if p.managed {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		return daemon.Stop(ctx)
	}
	return p.command.Process.Kill()
}
