/*
Copyright 2023 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	vmmodel "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/vm"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	vmmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	systemservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	commonjob "github.com/koderover/zadig/v2/pkg/types/job"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
)

func GetAgentAccessCmd(vmID string, logger *zap.SugaredLogger) (*AgentAccessCmds, error) {
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: vmID,
	})
	if err != nil {
		logger.Errorf("failed to find vm %s, error: %s", vmID, err)
		return nil, fmt.Errorf("failed to find vm %s, error: %s", vmID, err)
	}

	// generate token
	if vm.Agent == nil {
		return nil, fmt.Errorf("zadig server vm %s agent is nil in db", vmID)
	}
	if vm.Agent.Token == "" {
		vm.Agent.Token = GenerateAgentToken()
	}
	err = commonrepo.NewPrivateKeyColl().Update(vm.ID.Hex(), vm)
	if err != nil {
		logger.Errorf("failed to update vm %s, error: %s", vmID, err)
		return nil, fmt.Errorf("failed to update vm %s, error: %s", vmID, err)
	}

	return GenerateAgentAccessCmds(vm)
}

func OfflineVM(idString, user string, logger *zap.SugaredLogger) error {
	if idString == "" {
		return e.ErrOfflineZadigVM.AddDesc("empty vm id")
	}
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: idString,
	})
	if err != nil {
		return e.ErrOfflineZadigVM.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	if vm.Agent == nil {
		return fmt.Errorf("host is not connected through an agent")
	}

	vm.Status = setting.VMOffline
	vm.UpdateBy = user
	vm.UpdateTime = time.Now().Unix()
	err = commonrepo.NewPrivateKeyColl().SetAgentDisconnected(vm.ID, true, user)
	if err != nil {
		logger.Errorf("failed to offline vm %s, error: %s", idString, err)
		return e.ErrOfflineZadigVM.AddErr(fmt.Errorf("failed to offline vm %s, error: %s", idString, err))
	}

	return nil
}

type RecoveryAgentCmd struct {
	RecoveryCmd map[string]string `json:"recovery_cmd"`
}

func RecoveryVM(idString, user string, logger *zap.SugaredLogger) (*RecoveryAgentCmd, error) {
	if idString == "" {
		return nil, e.ErrRecoveryZadigVM.AddDesc("empty vm id")
	}
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: idString,
	})
	if err != nil {
		return nil, e.ErrRecoveryZadigVM.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	if vm.Agent == nil {
		return nil, fmt.Errorf("host is not connected through an agent")
	}

	vm.Status = setting.VMAbnormal
	vm.UpdateBy = user
	vm.UpdateTime = time.Now().Unix()
	err = commonrepo.NewPrivateKeyColl().SetAgentDisconnected(vm.ID, false, user)
	if err != nil {
		logger.Errorf("failed to recovery vm %s, error: %s", idString, err)
		return nil, e.ErrRecoveryZadigVM.AddErr(fmt.Errorf("failed to recovery vm %s, error: %s", idString, err))
	}

	return generateAgentRecoveryCmd(vm)
}

func generateAgentRecoveryCmd(vm *commonmodels.PrivateKey) (*RecoveryAgentCmd, error) {
	if vm.VMInfo == nil {
		return nil, fmt.Errorf("agent platform has not been reported")
	}
	commands, err := agentInstallCommands(vm, vm.VMInfo.Platform, vm.VMInfo.Architecture, true)
	if err != nil {
		return nil, err
	}
	return &RecoveryAgentCmd{RecoveryCmd: commands}, nil
}

type UpgradeAgentCmd struct {
	UpgradeCmd map[string]string `json:"upgrade_cmd"`
	Upgrade    bool              `json:"upgrade"`
}

func UpgradeAgent(idString, user string, logger *zap.SugaredLogger) (*UpgradeAgentCmd, error) {
	if idString == "" {
		return nil, e.ErrUpgradeZadigVMAgent.AddDesc("empty vm id")
	}
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: idString,
	})
	if err != nil {
		return nil, e.ErrUpgradeZadigVMAgent.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	vm.UpdateBy = user
	vm.UpdateTime = time.Now().Unix()

	return generateAgentUpgradeCmd(vm, logger)
}

func generateAgentUpgradeCmd(vm *commonmodels.PrivateKey, logger *zap.SugaredLogger) (*UpgradeAgentCmd, error) {
	if vm.VMInfo == nil {
		return nil, fmt.Errorf("agent platform has not been reported")
	}
	commands, err := agentInstallCommands(vm, vm.VMInfo.Platform, vm.VMInfo.Architecture, true)
	if err != nil {
		return nil, err
	}
	return &UpgradeAgentCmd{UpgradeCmd: commands}, nil
}

func ListVMs(logger *zap.SugaredLogger) ([]*AgentBriefListResp, error) {
	vms, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		logger.Errorf("failed to list VMs, error: %s", err)
		return nil, fmt.Errorf("failed to list VMs, error: %s", err)
	}

	resp := make([]*AgentBriefListResp, 0, len(vms))
	recommended, _ := config.GetZadigAgentVersion()
	for _, vm := range vms {
		if vm.Agent == nil {
			continue
		}

		a := &AgentBriefListResp{
			ID:                 vm.ID.Hex(),
			AgentVersion:       vm.Agent.AgentVersion,
			RecommendedVersion: recommended,
			UpgradeSupported:   vm.Agent.UpgradeSupported,
			UpgradeError:       vm.Agent.UpgradeError,
			Upgrade:            vm.Agent.Upgrade,
			Disconnected:       vm.Agent.Disconnected,
			Name:               vm.Name,
			Label:              vm.Label,
			Status:             string(vm.Status),
			Error:              vm.Error,
		}
		if vm.VMInfo != nil {
			a.IP = vm.VMInfo.IP
			a.Platform = vm.VMInfo.Platform
			a.Architecture = vm.VMInfo.Architecture
		}
		resp = append(resp, a)
	}

	return resp, nil
}

func ListVMLabels(projectKey string, logger *zap.SugaredLogger) ([]string, error) {
	vms, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		logger.Errorf("failed to list VMs, error: %s", err)
		return nil, fmt.Errorf("failed to list VMs, error: %s", err)
	}

	labelSet := sets.NewString()
	for _, vm := range vms {
		if vm.ProjectName != "" && vm.ProjectName != projectKey {
			continue
		}

		if vm.ScheduleWorkflow && vm.Agent != nil && vm.Type == setting.NewVMType && vm.Status == setting.VMNormal {
			if vm.Label != "" && !labelSet.Has(vm.Label) {
				labelSet.Insert(vm.Label)
			}
		}
	}

	resp := labelSet.List()
	return resp, nil
}

func RegisterAgent(args *RegisterAgentRequest, logger *zap.SugaredLogger) (*RegisterAgentResponse, error) {
	if strings.TrimSpace(args.Token) == "" {
		return nil, fmt.Errorf("token is required")
	}

	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: args.Token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	if vm.Status == setting.VMOffline || (vm.Agent != nil && vm.Agent.Disconnected) {
		return &RegisterAgentResponse{NeedOffline: true}, nil
	}
	vm.Status = setting.VMRegistered
	if args.Parameters != nil {
		vm.VMInfo = &commonmodels.VMInfo{
			Platform:      args.Parameters.OS,
			Architecture:  args.Parameters.Arch,
			MemeryTotal:   args.Parameters.MemTotal,
			UsedMemery:    args.Parameters.UsedMem,
			CpuNum:        args.Parameters.CpuNum,
			DiskSpace:     args.Parameters.DiskSpace,
			FreeDiskSpace: args.Parameters.FreeDiskSpace,
			HostName:      args.Parameters.HostName,
		}

		if vm.IP == "" && args.Parameters.IP != "" {
			vm.IP = args.Parameters.IP
		}

		if vm.Agent == nil {
			logger.Errorf("zadig server vm %s agent is nil in db", vm.Name)
			return nil, fmt.Errorf("zadig server vm %s agent is nil in db", args.Token)
		}
		vm.Agent.AgentVersion = args.Parameters.AgentVersion
		updateAgentVersionStatus(vm.Agent, logger)
	}
	fields := bson.M{"status": vm.Status, "vm_info": vm.VMInfo, "ip": vm.IP}
	if vm.Agent != nil {
		fields["agent.agent_version"] = vm.Agent.AgentVersion
		fields["agent.zadig_version"] = vm.Agent.ZadigVersion
		fields["agent.need_update"] = vm.Agent.NeedUpdate
	}
	err = commonrepo.NewPrivateKeyColl().UpdateAgentRegistration(vm.ID, fields)
	if err == mongo.ErrNoDocuments {
		return &RegisterAgentResponse{NeedOffline: true}, nil
	}
	if err != nil {
		logger.Errorf("failed to update vm %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &RegisterAgentResponse{
		Token:            vm.Agent.Token,
		Description:      vm.Description,
		TaskConcurrency:  vm.Agent.TaskConcurrency,
		AgentVersion:     vm.Agent.AgentVersion,
		ZadigVersion:     vm.Agent.ZadigVersion,
		VmName:           vm.Name,
		WorkDir:          vm.Agent.Workspace,
		ScheduleWorkflow: vm.ScheduleWorkflow,
	}

	return resp, nil
}

func VerifyAgent(args *VerifyAgentRequest, logger *zap.SugaredLogger) (*VerifyAgentResponse, error) {
	if strings.TrimSpace(args.Token) == "" {
		return nil, fmt.Errorf("token is required")
	}

	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: args.Token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	if vm.Status == setting.VMOffline || (vm.Agent != nil && vm.Agent.Disconnected) {
		return &VerifyAgentResponse{NeedOffline: true}, nil
	}
	if args.Parameters == nil {
		return nil, fmt.Errorf("agent platform parameters are required")
	}
	resp := &VerifyAgentResponse{
		Verified: false,
	}
	if vm.VMInfo != nil && vm.VMInfo.Platform == args.Parameters.OS && vm.VMInfo.Architecture == args.Parameters.Arch {
		resp.Verified = true
	}

	return resp, nil
}

func Heartbeat(args *HeartbeatRequest, logger *zap.SugaredLogger) (*HeartbeatResponse, error) {
	if strings.TrimSpace(args.Token) == "" {
		return nil, fmt.Errorf("token is required")
	}

	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: args.Token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &HeartbeatResponse{
		NeedUpdateAgentVersion: false,
		NeedOffline:            false,
	}

	if vm.Status == setting.VMOffline || (vm.Agent != nil && vm.Agent.Disconnected) {
		resp.NeedOffline = true
		return resp, nil
	}

	vm.Status = setting.VMNormal
	if args.Parameters != nil && vm.VMInfo != nil {
		vm.VMInfo.MemeryTotal = args.Parameters.MemTotal
		vm.VMInfo.UsedMemery = args.Parameters.UsedMem
		vm.VMInfo.DiskSpace = args.Parameters.DiskSpace
		vm.VMInfo.FreeDiskSpace = args.Parameters.FreeDiskSpace
	}
	vm.UpdateBy = setting.SystemUser
	vm.UpdateTime = time.Now().Unix()

	// set vm heartbeat time
	if vm.Agent == nil {
		logger.Errorf("zadig server vm %s agent is nil in db", vm.Name)
		return nil, fmt.Errorf("zadig server vm %s agent is nil in db", args.Token)
	}
	vm.Agent.LastHeartbeatTime = time.Now().Unix()
	if args.Parameters != nil && args.Parameters.AgentVersion != "" {
		vm.Agent.AgentVersion = args.Parameters.AgentVersion
	}
	updateAgentVersionStatus(vm.Agent, logger)

	fields := bson.M{
		"agent.last_heartbeat_time": vm.Agent.LastHeartbeatTime,
		"agent.agent_version":       vm.Agent.AgentVersion,
		"agent.zadig_version":       vm.Agent.ZadigVersion,
		"agent.need_update":         vm.Agent.NeedUpdate,
		"update_by":                 vm.UpdateBy, "update_time": vm.UpdateTime,
	}
	if vm.VMInfo != nil {
		fields["vm_info.memery_total"] = vm.VMInfo.MemeryTotal
		fields["vm_info.used_memery"] = vm.VMInfo.UsedMemery
		fields["vm_info.disk_space"] = vm.VMInfo.DiskSpace
		fields["vm_info.free_disk_space"] = vm.VMInfo.FreeDiskSpace
	}
	if args.Parameters != nil {
		fields["agent.upgrade_supported"] = args.Parameters.UpgradeSupported
		upgradeError := args.Parameters.UpgradeError
		if len(upgradeError) > 2048 {
			upgradeError = upgradeError[:2048]
		}
		fields["agent.upgrade_error"] = upgradeError
	}
	if err := commonrepo.NewPrivateKeyColl().UpdateAgentHeartbeat(vm.ID, fields); err != nil {
		return nil, err
	}
	if err := heartbeatAgentUpgrade(vm, args.Parameters, resp); err != nil {
		return nil, err
	}

	if vm.Agent.NeedUpdate {
		resp.NeedUpdateAgentVersion = true
		resp.AgentVersion = vm.Agent.ZadigVersion
	}
	resp.ZadigVersion = vm.Agent.ZadigVersion

	resp.ScheduleWorkflow = vm.ScheduleWorkflow
	if vm.ScheduleWorkflow && vm.Agent.Workspace != "" {
		resp.WorkDir = vm.Agent.Workspace
	}
	if vm.Agent.TaskConcurrency > 0 {
		resp.Concurrency = vm.Agent.TaskConcurrency
	}

	return resp, nil
}

func updateAgentVersionStatus(agent *commonmodels.VMAgent, logger *zap.SugaredLogger) {
	latestVersion, err := config.GetZadigAgentVersion()
	if err != nil {
		logger.Warnf("failed to get zadig-agent version while handling heartbeat: %v", err)
		return
	}
	agent.ZadigVersion = latestVersion

	normalizedLatestVersion := strings.TrimPrefix(latestVersion, "v")
	if normalizedLatestVersion == "" {
		agent.NeedUpdate = false
		return
	}

	normalizedCurrentVersion := strings.TrimPrefix(agent.AgentVersion, "v")
	agent.NeedUpdate = normalizedCurrentVersion != normalizedLatestVersion
}

type VMJobGetterMap struct {
	M         sync.Mutex
	GetterMap map[string]struct{}
}

func (m *VMJobGetterMap) AddGetter(jobID string) bool {
	if _, ok := m.GetterMap[jobID]; ok {
		return false
	}

	m.M.Lock()
	defer m.M.Unlock()
	if _, ok := m.GetterMap[jobID]; ok {
		return false
	}
	m.GetterMap[jobID] = struct{}{}

	return true
}

func (m *VMJobGetterMap) RemoveGetter(jobID string) {
	m.M.Lock()
	defer m.M.Unlock()
	delete(m.GetterMap, jobID)
}

var jobGetter = &VMJobGetterMap{
	GetterMap: make(map[string]struct{}),
	M:         sync.Mutex{},
}

func PollingAgentJob(token string, retry int, logger *zap.SugaredLogger) (*PollingJobResp, error) {
	if retry >= 3 {
		return nil, nil
	}

	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", token, err)
		return nil, fmt.Errorf("failed to find vm by token %s, error: %s", token, err)
	}

	// check vm status
	if vm.Status != setting.VMNormal || (vm.Agent != nil && vm.Agent.Disconnected) {
		return nil, fmt.Errorf("vm %s status is %s", vm.Name, vm.Status)
	}

	var resp *PollingJobResp
	jobs, err := vmmongodb.NewVMJobColl().ListByOpts(&vmmongodb.VMJobOpts{
		Status: string(config.StatusCreated),
	})
	if err != nil {
		if err == mongo.ErrNilDocument || err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("vm %s failed to find job that status is created, error: %v", vm.Name, err)
	}

	var job *vmmodel.VMJob
	for _, j := range jobs {
		if !vm.IsAvailableToProject(j.ProjectName) {
			continue
		}
		labelSet := sets.NewString(j.VMLabels...)
		if j.VMLabels != nil && (j.VMLabels[0] == setting.VMLabelAnyOne || labelSet.Has(vm.Label)) {
			job = j
			break
		}
	}
	if job == nil {
		return nil, nil
	}

	if job != nil && jobGetter.AddGetter(job.ID.Hex()) {
		defer jobGetter.RemoveGetter(job.ID.Hex())
		job.Status = string(config.StatusPrepare)
		job.VMID = vm.ID.Hex()
		err := vmmongodb.NewVMJobColl().Update(job.ID.Hex(), job)
		if err != nil {
			logger.Errorf("failed to update job %s, error: %s", job.ID.Hex(), err)
			return nil, fmt.Errorf("failed to update job %s, error: %s", job.ID.Hex(), err)
		}

		resp = &PollingJobResp{
			ID:            job.ID.Hex(),
			ProjectName:   job.ProjectName,
			WorkflowName:  job.WorkflowName,
			TaskID:        job.TaskID,
			JobOriginName: job.JobOriginName,
			JobName:       job.JobName,
			JobType:       job.JobType,
			Status:        job.Status,
			JobCtx:        job.JobCtx,
		}
	} else {
		retry++
		resp, err = PollingAgentJob(token, retry, logger)
	}

	return resp, err
}

type ReportAgentJobResp struct {
	JobID     string `json:"job_id"`
	JobStatus string `json:"job_status"`
}

func ReportAgentJob(args *ReportJobArgs, logger *zap.SugaredLogger) (*ReportAgentJobResp, error) {
	if strings.TrimSpace(args.Token) == "" {
		return nil, fmt.Errorf("token is required")
	}

	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: args.Token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, fmt.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
	}

	// update job
	job, err := vmmongodb.NewVMJobColl().FindByID(args.JobID)
	if err != nil {
		logger.Errorf("failed to find job %s, error: %s", args.JobID, err)
		return nil, fmt.Errorf("failed to find job %s, error: %s", args.JobID, err)
	}
	if job.VMID != vm.ID.Hex() {
		return nil, fmt.Errorf("job %s does not belong to this agent", args.JobID)
	}

	// if job is cancelled or timeout, stop agent job
	if job.Status == string(config.StatusCancelled) || job.Status == string(config.StatusTimeout) {
		return &ReportAgentJobResp{
			JobID:     job.ID.Hex(),
			JobStatus: job.Status,
		}, nil
	}

	job.Status = args.JobStatus
	job.Error = args.JobError

	if args.JobOutput != nil {
		outputs := make([]*commonjob.JobOutput, 0)
		if err := json.Unmarshal(args.JobOutput, &outputs); err != nil {
			logger.Errorf("failed to unmarshal job output, error: %s", err)
			return nil, fmt.Errorf("failed to unmarshal job output, error: %s", err)
		}
		job.Outputs = outputs
	}

	err = savaVMJobLog(job, args.JobLog, logger)
	if err != nil {
		logger.Errorf("failed to save job %s log, error: %s", args.JobID, err)
		return nil, fmt.Errorf("failed to save job %s log, error: %s", args.JobID, err)
	}

	err = vmmongodb.NewVMJobColl().Update(job.ID.Hex(), job)
	if err != nil {
		logger.Errorf("failed to update job %s, error: %s", args.JobID, err)
		return nil, fmt.Errorf("failed to update job %s, error: %s", args.JobID, err)
	}

	return nil, nil
}

// GenerateAgentToken TODO: consider how to generate vm token
func GenerateAgentToken() string {
	return primitive.NewObjectID().Hex()
}

func GenerateAgentAccessCmds(vm *commonmodels.PrivateKey) (*AgentAccessCmds, error) {
	platforms := make([]*AgentAccessCmd, 0, 3)
	for _, platform := range []string{"linux", "darwin", "windows"} {
		amd64, err := agentInstallCommands(vm, platform, "amd64", false)
		if err != nil {
			return nil, err
		}
		commands := &AgentAccessCmd{AMD64: amd64}
		if platform != "windows" {
			commands.ARM64, err = agentInstallCommands(vm, platform, "arm64", false)
			if err != nil {
				return nil, err
			}
		}
		platforms = append(platforms, commands)
	}
	return &AgentAccessCmds{LinuxPlatform: platforms[0], MacOSPlatform: platforms[1], WinPlatform: platforms[2]}, nil
}

func DownloadTemporaryFile(fileID, token string, c *gin.Context, logger *zap.SugaredLogger) error {
	_, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token for file download, error: %s", err)
		return fmt.Errorf("invalid token for file download: %v", err)
	}

	// Delegate to the system service for actual file download
	return systemservice.DownloadTemporaryFile(fileID, c, logger)
}
