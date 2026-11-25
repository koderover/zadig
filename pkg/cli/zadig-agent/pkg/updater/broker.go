package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
)

// Reconcile the root-owned transaction before loading user configuration. A
// watchdog rollback can finish even when the unprivileged coordinator died.
func reconcileBroker(s *state) error {
	result, err := daemon.BrokerOperation(context.Background(), "status", s.ID)
	if err != nil {
		return err
	}
	if result.ID != s.ID {
		if s.Status.Terminal() || s.Status == agentupgrade.Downloading || s.Status == agentupgrade.Draining {
			return nil
		}
		return fmt.Errorf("privileged upgrade transaction is missing")
	}
	if result.Status == "committed" {
		return updateState(s.ID, func(s *state) { s.Status = agentupgrade.Succeeded; s.Error = ""; s.Restoring = false })
	}
	if result.Status != "rolled_back" || s.Status == agentupgrade.RolledBack {
		return nil
	}
	if s.ConfigSaved {
		backup, err := os.ReadFile(filepath.Join(s.Directory, "config.backup"))
		if err != nil {
			return err
		}
		p, err := config.GetAgentConfigFilePath()
		if err != nil {
			return err
		}
		if err = atomicWrite(p, backup, 0600); err != nil {
			return err
		}
	}
	return updateState(s.ID, func(s *state) {
		s.Status = agentupgrade.RolledBack
		s.Error = "privileged helper recovered an interrupted upgrade"
		s.Healthy = false
		s.Restoring = false
	})
}
