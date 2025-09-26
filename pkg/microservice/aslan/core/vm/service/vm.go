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
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	commonconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	vmmodel "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/vm"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	vmmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	systemservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	krkubeclient "github.com/koderover/zadig/v2/pkg/tool/kube/client"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	commonjob "github.com/koderover/zadig/v2/pkg/types/job"
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

	vm.Status = setting.VMOffline
	vm.UpdateBy = user
	vm.UpdateTime = time.Now().Unix()
	err = commonrepo.NewPrivateKeyColl().Update(idString, vm)
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

	vm.Status = setting.VMAbnormal
	vm.UpdateBy = user
	vm.UpdateTime = time.Now().Unix()
	err = commonrepo.NewPrivateKeyColl().Update(idString, vm)
	if err != nil {
		logger.Errorf("failed to recovery vm %s, error: %s", idString, err)
		return nil, e.ErrRecoveryZadigVM.AddErr(fmt.Errorf("failed to recovery vm %s, error: %s", idString, err))
	}

	return generateAgentRecoveryCmd(vm)
}

func generateAgentRecoveryCmd(vm *commonmodels.PrivateKey) (*RecoveryAgentCmd, error) {
	cmd := new(RecoveryAgentCmd)

	baseURL, err := getRepoURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent repo url, err: %w", err)
	}
	version, err := getZadigAgentVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get zadig-agent version, error: %s", err)
	}

	if vm.Agent == nil {
		return nil, fmt.Errorf("vm %s not install zadig-agent", vm.Name)
	}

	var token string
	if vm.Agent != nil {
		token = vm.Agent.Token
	}
	serverURL := commonconfig.SystemAddress()

	linuxAMD64Name, linuxARM64Name := fmt.Sprintf("zadig-agent-linux-amd64-v%s", version), fmt.Sprintf("zadig-agent-linux-arm64-v%s", version)
	macOSAMD64Name, macOSARM64Name := fmt.Sprintf("zadig-agent-darwin-amd64-v%s", version), fmt.Sprintf("zadig-agent-darwin-arm64-v%s", version)
	winAMD64Name := fmt.Sprintf("zadig-agent-windows-amd64-v%s.exe", version)
	winAMD64TarballName := strings.ReplaceAll(winAMD64Name, ".exe", ".tar.gz")

	if vm.VMInfo != nil {
		switch fmt.Sprintf("%s_%s", vm.VMInfo.Platform, vm.VMInfo.Architecture) {
		case setting.LinuxAmd64:
			downloadLinuxAMD64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, linuxAMD64Name)
			cmd.RecoveryCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"sudo PATH=$PATH nohup zadig-agent start --server-url %s --token %s &",
				downloadLinuxAMD64URL, linuxAMD64Name, serverURL, token)}
		case setting.LinuxArm64:
			downloadLinuxARM64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, linuxARM64Name)
			cmd.RecoveryCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"sudo PATH=$PATH nohup zadig-agent start --server-url %s --token %s &",
				downloadLinuxARM64URL, linuxARM64Name, serverURL, token)}
		case setting.MacOSAmd64:
			downloadMacOSAMD64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, macOSAMD64Name)
			cmd.RecoveryCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"nohup zadig-agent start --server-url %s --token %s &",
				downloadMacOSAMD64URL, macOSAMD64Name, serverURL, token)}
		case setting.MacOSArm64:
			downloadMacOSARM64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, macOSARM64Name)
			cmd.RecoveryCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"nohup zadig-agent start --server-url %s --token %s &",
				downloadMacOSARM64URL, macOSARM64Name, serverURL, token)}
		case setting.WinAmd64:
			downloadWinAMD64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, strings.TrimSuffix(winAMD64Name, ".exe"))
			cmd.RecoveryCmd = map[string]string{
				"cmd": fmt.Sprintf(
					"curl -L %s | tar xzf - -C C:\\Users\\Administrator \n "+
						"move /Y C:\\Users\\Administrator\\%s C:\\Users\\Administrator\\zadig-agent.exe \n "+
						"start C:\\Users\\Administrator\\zadig-agent.exe start --server-url %s --token %s",
					downloadWinAMD64URL, winAMD64Name, serverURL, token),
				"powershell": fmt.Sprintf(
					"curl %s -o %s \n"+
						"tar xzf %s -C C:\\Users\\Administrator \n "+
						"Remove-Item -Path \"C:\\Users\\Administrator\\%s\" -ErrorAction SilentlyContinue \n"+
						"move -Force C:\\Users\\Administrator\\%s C:\\Users\\Administrator\\zadig-agent.exe \n "+
						"Start-Process -FilePath \"C:\\Users\\Administrator\\zadig-agent.exe\" -ArgumentList \"start\", \"--server-url\", \"%s\", \"--token\", \"%s\"",
					downloadWinAMD64URL, winAMD64TarballName, winAMD64TarballName, winAMD64TarballName, winAMD64Name, serverURL, token)}
		default:
			return nil, fmt.Errorf("unsupported platform %s", vm.VMInfo.Platform)
		}
	}
	return cmd, nil
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
	cmd := new(UpgradeAgentCmd)

	baseURL, err := getRepoURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent repo url, err: %w", err)
	}
	version, err := getZadigAgentVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get zadig-agent version, error: %s", err)
	}

	if vm.Agent == nil {
		return nil, fmt.Errorf("vm %s not install zadig-agent", vm.Name)
	}

	linuxAMD64Name, linuxARM64Name := fmt.Sprintf("zadig-agent-linux-amd64-v%s", version), fmt.Sprintf("zadig-agent-linux-arm64-v%s", version)
	macOSAMD64Name, macOSARM64Name := fmt.Sprintf("zadig-agent-darwin-amd64-v%s", version), fmt.Sprintf("zadig-agent-darwin-arm64-v%s", version)
	winAMD64Name := fmt.Sprintf("zadig-agent-windows-amd64-v%s.exe", version)
	winAMD64TarballName := strings.ReplaceAll(winAMD64Name, ".exe", ".tar.gz")

	if vm.VMInfo != nil {
		switch fmt.Sprintf("%s_%s", vm.VMInfo.Platform, vm.VMInfo.Architecture) {
		case setting.LinuxAmd64:
			downloadLinuxAMD64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, linuxAMD64Name)
			cmd.UpgradeCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo zadig-agent stop \n "+
					"sudo rm -rf /usr/local/bin/zadig-agent \n "+
					"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"sudo PATH=$PATH nohup zadig-agent start &",
				downloadLinuxAMD64URL, linuxAMD64Name)}
		case setting.LinuxArm64:
			downloadLinuxARM64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, linuxARM64Name)
			cmd.UpgradeCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo zadig-agent stop \n "+
					"sudo rm -rf /usr/local/bin/zadig-agent \n "+
					"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"sudo PATH=$PATH nohup zadig-agent start &",
				downloadLinuxARM64URL, linuxARM64Name)}
		case setting.MacOSAmd64:
			downloadMacOSAMD64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, macOSAMD64Name)
			cmd.UpgradeCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo zadig-agent stop \n "+
					"sudo rm -rf /usr/local/bin/zadig-agent \n "+
					"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"nohup zadig-agent start &",
				downloadMacOSAMD64URL, macOSAMD64Name)}
		case setting.MacOSArm64:
			downloadMacOSARM64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, macOSARM64Name)
			cmd.UpgradeCmd = map[string]string{"shell": fmt.Sprintf(
				"sudo zadig-agent stop \n "+
					"sudo rm -rf /usr/local/bin/zadig-agent \n "+
					"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"nohup zadig-agent start &",
				downloadMacOSARM64URL, macOSARM64Name)}
		case setting.WinAmd64:
			downloadWinAMD64URL := fmt.Sprintf("%s/%s.tar.gz", baseURL, strings.TrimSuffix(winAMD64Name, ".exe"))
			cmd.UpgradeCmd = map[string]string{
				"cmd": fmt.Sprintf(
					"C:\\Users\\Administrator\\zadig-agent.exe stop\n "+
						"del C:\\Users\\Administrator\\zadig-agent.exe \n "+
						"curl -L %s | tar xzf - -C C:\\Users\\Administrator \n "+
						"move /Y C:\\Users\\Administrator\\%s C:\\Users\\Administrator\\zadig-agent.exe \n "+
						"start C:\\Users\\Administrator\\zadig-agent.exe start",
					downloadWinAMD64URL, winAMD64Name),
				"powershell": fmt.Sprintf(
					"C:\\Users\\Administrator\\zadig-agent.exe stop\n "+
						"Remove-Item -Path C:\\Users\\Administrator\\zadig-agent.exe -ErrorAction SilentlyContinue \n "+
						"curl %s -o %s \n"+
						"tar xzf %s -C C:\\Users\\Administrator \n "+
						"Remove-Item -Path \"C:\\Users\\Administrator\\%s\" -ErrorAction SilentlyContinue \n"+
						"move -Force C:\\Users\\Administrator\\%s C:\\Users\\Administrator\\zadig-agent.exe \n "+
						"Start-Process -FilePath \"C:\\Users\\Administrator\\zadig-agent.exe\" -ArgumentList \"start\"",
					downloadWinAMD64URL, winAMD64TarballName, winAMD64TarballName, winAMD64TarballName, winAMD64Name)}
		default:
			return nil, fmt.Errorf("unsupported platform %s", vm.VMInfo.Platform)
		}
	}

	if vm.Agent != nil {
		vm.Agent.AgentVersion = version
	}
	err = commonrepo.NewPrivateKeyColl().Update(vm.ID.Hex(), vm)
	if err != nil {
		logger.Errorf("failed to upgrade vm agent %s, error: %s", vm.ID.Hex(), err)
		return nil, e.ErrUpgradeZadigVMAgent.AddErr(fmt.Errorf("failed to upgrade vm agent %s, error: %s", vm.ID.Hex(), err))
	}

	return cmd, nil
}

func ListVMs(logger *zap.SugaredLogger) ([]*AgentBriefListResp, error) {
	vms, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		logger.Errorf("failed to list VMs, error: %s", err)
		return nil, fmt.Errorf("failed to list VMs, error: %s", err)
	}

	resp := make([]*AgentBriefListResp, 0, len(vms))
	for _, vm := range vms {
		if vm.Agent == nil {
			continue
		}

		a := &AgentBriefListResp{
			Name:   vm.Name,
			Label:  vm.Label,
			Status: string(vm.Status),
			Error:  vm.Error,
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
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: args.Token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
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
	}
	err = commonrepo.NewPrivateKeyColl().Update(vm.ID.Hex(), vm)
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
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		Token: args.Token,
	})
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
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

	if vm.Status == setting.VMOffline {
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

	err = commonrepo.NewPrivateKeyColl().Update(vm.ID.Hex(), vm)
	if err != nil {
		logger.Errorf("failed to update vm %s, error: %s", args.Token, err)
		return nil, fmt.Errorf("failed to update vm %s, error: %s", args.Token, err)
	}

	if vm.Agent.NeedUpdate {
		resp.NeedUpdateAgentVersion = true
		resp.AgentVersion = vm.Agent.AgentVersion
	}

	resp.ScheduleWorkflow = vm.ScheduleWorkflow
	if vm.ScheduleWorkflow && vm.Agent.Workspace != "" {
		resp.WorkDir = vm.Agent.Workspace
	}
	if vm.Agent.TaskConcurrency > 0 {
		resp.Concurrency = vm.Agent.TaskConcurrency
	}

	return resp, nil
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
	if vm.Status != setting.VMNormal {
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
	_, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
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
	var token string
	if vm.Agent != nil {
		token = vm.Agent.Token
	}
	baseURL, err := getRepoURL()
	if err != nil {
		return nil, fmt.Errorf("failed to get agent repo url, err: %w", err)
	}
	version, err := getZadigAgentVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get zadig-agent version, error: %s", err)
	}

	serverURL := commonconfig.SystemAddress()

	var downloadLinuxAMD64URL, downloadLinuxARM64URL string
	linuxAMD64Name, linuxARM64Name := fmt.Sprintf("zadig-agent-linux-amd64-v%s", version), fmt.Sprintf("zadig-agent-linux-arm64-v%s", version)
	downloadLinuxAMD64URL = fmt.Sprintf("%s/%s.tar.gz", baseURL, linuxAMD64Name)
	downloadLinuxARM64URL = fmt.Sprintf("%s/%s.tar.gz", baseURL, linuxARM64Name)

	var downloadMacAMD64URL, downloadMacARM64URL string
	macOSAMD64Name, macOSARM64Name := fmt.Sprintf("zadig-agent-darwin-amd64-v%s", version), fmt.Sprintf("zadig-agent-darwin-arm64-v%s", version)
	downloadMacAMD64URL = fmt.Sprintf("%s/%s.tar.gz", baseURL, macOSAMD64Name)
	downloadMacARM64URL = fmt.Sprintf("%s/%s.tar.gz", baseURL, macOSARM64Name)

	var downloadWinAMD64URL string
	winAMD64Name := fmt.Sprintf("zadig-agent-windows-amd64-v%s.exe", version)
	winAMD64TarballName := strings.ReplaceAll(winAMD64Name, ".exe", ".tar.gz")
	downloadWinAMD64URL = fmt.Sprintf("%s/%s.tar.gz", baseURL, strings.TrimSuffix(winAMD64Name, ".exe"))

	resp := &AgentAccessCmds{
		LinuxPlatform: &AgentAccessCmd{
			AMD64: map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"sudo PATH=$PATH nohup zadig-agent start --server-url %s --token %s &",
				downloadLinuxAMD64URL, linuxAMD64Name, serverURL, token)},
			ARM64: map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"sudo PATH=$PATH nohup zadig-agent start --server-url %s --token %s &",
				downloadLinuxARM64URL, linuxARM64Name, serverURL, token)},
		},
		MacOSPlatform: &AgentAccessCmd{
			AMD64: map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"nohup zadig-agent start --server-url %s --token %s &",
				downloadMacAMD64URL, macOSAMD64Name, serverURL, token)},
			ARM64: map[string]string{"shell": fmt.Sprintf(
				"sudo curl -L %s | sudo tar xz -C /usr/local/bin/ \n "+
					"sudo mv /usr/local/bin/%s /usr/local/bin/zadig-agent \n "+
					"sudo chmod +x /usr/local/bin/zadig-agent \n "+
					"nohup zadig-agent start --server-url %s --token %s &",
				downloadMacARM64URL, macOSARM64Name, serverURL, token)},
		},
		WinPlatform: &AgentAccessCmd{
			AMD64: map[string]string{
				"cmd": fmt.Sprintf(
					"curl -L %s | tar xzf - -C C:\\Users\\Administrator \n "+
						"move /Y C:\\Users\\Administrator\\%s C:\\Users\\Administrator\\zadig-agent.exe \n "+
						"start C:\\Users\\Administrator\\zadig-agent.exe start --server-url %s --token %s",
					downloadWinAMD64URL, winAMD64Name, serverURL, token),
				"powershell": fmt.Sprintf(
					"curl %s -o %s \n"+
						"tar xzf %s -C C:\\Users\\Administrator \n "+
						"Remove-Item -Path \"C:\\Users\\Administrator\\%s\" -ErrorAction SilentlyContinue \n"+
						"move -Force C:\\Users\\Administrator\\%s C:\\Users\\Administrator\\zadig-agent.exe \n "+
						"Start-Process -FilePath \"C:\\Users\\Administrator\\zadig-agent.exe\" -ArgumentList \"start\", \"--server-url\", \"%s\", \"--token\", \"%s\"",
					downloadWinAMD64URL, winAMD64TarballName, winAMD64TarballName, winAMD64TarballName, winAMD64Name, serverURL, token),
			},
		},
	}

	return resp, nil
}

func getZadigAgentVersion() (string, error) {
	ns := commonconfig.Namespace()
	kubeClient := krkubeclient.Client()
	configMap, found, err := getter.GetConfigMap(ns, "aslan-config", kubeClient)
	if err != nil || !found {
		return "", fmt.Errorf("failed to get aslan configmap, error: %s", err)
	}
	if found {
		version := configMap.Data["ZADIG_AGENT_VERSION"]
		if version != "" {
			return strings.TrimPrefix(version, "v"), nil
		}
	}
	return "", fmt.Errorf("zadig-agent version not found")
}

func getRepoURL() (string, error) {
	ns := commonconfig.Namespace()
	kubeClient := krkubeclient.Client()
	configMap, found, err := getter.GetConfigMap(ns, "aslan-config", kubeClient)
	if err != nil || !found {
		return "", fmt.Errorf("failed to get aslan configmap, error: %s", err)
	}
	if found {
		version := configMap.Data["ZADIG_AGENT_REPO_URL"]
		if version != "" {
			return version, nil
		}
	}
	return "", fmt.Errorf("zadig-agent repo URL not found")
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
