package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/agentupgrade"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AgentUpgradeResult struct {
	VMID    string             `json:"vm_id"`
	Upgrade *agentupgrade.Task `json:"upgrade,omitempty"`
	Error   string             `json:"error,omitempty"`
}

func BatchRequestAgentUpgrades(ctx context.Context, ids []string, user string, force bool, logger *zap.SugaredLogger) []*AgentUpgradeResult {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	results := make([]*AgentUpgradeResult, 0, len(ids))
	seen := make(map[string]bool)
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		task, err := RequestAgentUpgrade(ctx, id, user, force, logger)
		result := &AgentUpgradeResult{VMID: id, Upgrade: task}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}
	return results
}

func RequestAgentUpgrade(ctx context.Context, id, user string, force bool, logger *zap.SugaredLogger) (*agentupgrade.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return nil, fmt.Errorf("invalid host id")
	}
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{ID: id})
	if err != nil {
		return nil, err
	}
	if vm.Agent == nil || vm.VMInfo == nil || !vm.Agent.UpgradeSupported {
		return nil, fmt.Errorf("agent does not support self upgrade; migrate the agent first")
	}
	if task := vm.Agent.Upgrade; task != nil && !task.Status.Terminal() {
		return task, nil
	}
	if vm.Agent.Disconnected || vm.Status != setting.VMNormal || time.Since(time.Unix(vm.Agent.LastHeartbeatTime, 0)) > time.Duration(setting.AgentDefaultHeartbeatTimeout)*time.Second {
		return nil, fmt.Errorf("agent is not online")
	}
	version, err := config.GetZadigAgentVersion()
	if err != nil {
		return nil, err
	}
	if !agentupgrade.Newer(version, vm.Agent.AgentVersion) {
		return nil, fmt.Errorf("target must be newer than the running agent version")
	}
	base, err := agentResourceURL()
	if err != nil {
		return nil, err
	}
	manifestURL, err := agentupgrade.ManifestURL(base, version)
	if err != nil {
		return nil, err
	}
	key, err := agentupgrade.PublicKey(viper.GetString("ZADIG_AGENT_UPGRADE_PUBLIC_KEY"))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	client, err := agentupgrade.HTTPClient()
	if err != nil {
		return nil, err
	}
	manifest, err := agentupgrade.FetchManifest(ctx, client, manifestURL, version, key)
	if err != nil {
		return nil, fmt.Errorf("validate agent release: %w", err)
	}
	if _, err := manifest.Asset(vm.VMInfo.Platform, vm.VMInfo.Architecture); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	task := &agentupgrade.Task{ID: primitive.NewObjectID().Hex(), Force: force, FromVersion: vm.Agent.AgentVersion, TargetVersion: version, ManifestURL: manifestURL, Status: agentupgrade.Pending, RequestedBy: user, CreateTime: now, UpdateTime: now}
	updated, err := commonrepo.NewPrivateKeyColl().CreateAgentUpgrade(vm.ID, vm.Agent.AgentVersion, task)
	if err == mongo.ErrNoDocuments {
		latest, findErr := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{ID: id})
		if findErr == nil && latest.Agent != nil && latest.Agent.Upgrade != nil && !latest.Agent.Upgrade.Status.Terminal() {
			return latest.Agent.Upgrade, nil
		}
		return nil, fmt.Errorf("agent changed while requesting upgrade; refresh and try again")
	}
	if err != nil {
		return nil, err
	}
	logger.Infof("requested agent upgrade %s for host %s to %s", task.ID, id, version)
	return updated.Agent.Upgrade, nil
}

func heartbeatAgentUpgrade(vm *commonmodels.PrivateKey, params *HeartbeatParameters, resp *HeartbeatResponse) error {
	coll := commonrepo.NewPrivateKeyColl()
	if params != nil && params.UpgradeReport != nil && vm.Agent.Upgrade != nil {
		task, report := vm.Agent.Upgrade, params.UpgradeReport
		if report.ID == task.ID && agentupgrade.CanTransition(task.Status, report.Status) {
			validVersion := true
			if report.Status == agentupgrade.Verifying || report.Status == agentupgrade.Succeeded {
				validVersion = strings.TrimPrefix(params.AgentVersion, "v") == task.TargetVersion
			}
			if report.Status == agentupgrade.RolledBack {
				validVersion = strings.TrimPrefix(params.AgentVersion, "v") == strings.TrimPrefix(task.FromVersion, "v")
			}
			if validVersion && (report.Status != task.Status || report.Error != task.Error) {
				copy := *report
				if len(copy.Error) > 2048 {
					copy.Error = copy.Error[:2048]
				}
				if err := coll.ReportAgentUpgrade(vm.ID, task, &copy); err != nil {
					return err
				}
			}
		}
	}
	latest, err := coll.Find(commonrepo.FindPrivateKeyOption{ID: vm.ID.Hex()})
	if err != nil {
		return err
	}
	if latest.Status == setting.VMOffline || (latest.Agent != nil && latest.Agent.Disconnected) {
		resp.NeedOffline = true
		return nil
	}
	if latest.Agent == nil {
		return nil
	}
	task := latest.Agent.Upgrade
	if task == nil {
		return nil
	}
	resp.UpgradeAccepted = &agentupgrade.Report{ID: task.ID, Status: task.Status, Error: task.Error}
	if !task.Status.Terminal() && params != nil && params.UpgradeSupported {
		resp.Upgrade = &agentupgrade.Instruction{ID: task.ID, Force: task.Force, TargetVersion: task.TargetVersion, ManifestURL: task.ManifestURL}
	}
	return nil
}
