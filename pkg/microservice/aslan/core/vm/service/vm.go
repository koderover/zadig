package service

import (
	"fmt"
	"sync"
	"time"

	agentmodel "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/vm"
	agentmongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/setting"
)

func CreateHost(args *CreateHostRequest, user string, logger *zap.SugaredLogger) error {
	_, err := agentmongodb.NewZadigVMColl().FindByName(args.Name)
	if err == nil {
		return e.ErrCreateZadigHost.AddErr(fmt.Errorf("vm %s already exists", args.Name))
	}

	host := &agentmodel.ZadigVM{
		Name:             args.Name,
		Description:      args.Description,
		Provider:         args.Provider,
		Tags:             args.Tags,
		IP:               args.HostIP,
		User:             args.HostUser,
		Port:             args.HostPort,
		SSHPrivateKey:    args.SSHPrivateKey,
		ScheduleWorkflow: args.ScheduleWorkflow,
		Workspace:        args.Workspace,
		TaskConcurrency:  args.TaskConcurrency,
		CreatedBy:        user,
		CreateTime:       time.Now().Unix(),
		UpdateTime:       time.Now().Unix(),
		UpdateBy:         user,
		Status:           setting.VMCreated,
	}

	// generate token
	host.Token = GenerateAgentToken()

	err = agentmongodb.NewZadigVMColl().Create(host)
	if err != nil {
		logger.Errorf("failed to create vm %s, error: %s", args.Name, err)
		return e.ErrCreateZadigHost.AddErr(fmt.Errorf("failed to create vm %s, error: %s", args.Name, err))
	}
	return nil
}

func GetAgentAccessCmd(platform, agentID string, logger *zap.SugaredLogger) ([]*AgentAccessCmd, error) {
	resp := make([]*AgentAccessCmd, 0)
	return resp, nil
}

func UpdateHost(args *CreateHostRequest, agentID, user string, logger *zap.SugaredLogger) error {
	host, err := agentmongodb.NewZadigVMColl().FindByName(args.Name)
	if err != nil {
		return e.ErrUpdateZadigHost.AddErr(fmt.Errorf("vm %s not exists", args.Name))
	}

	host.Name = args.Name
	host.Description = args.Description
	host.Provider = args.Provider
	host.Tags = args.Tags
	host.IP = args.HostIP
	host.Port = args.HostPort
	host.User = args.HostUser
	host.SSHPrivateKey = args.SSHPrivateKey
	host.ScheduleWorkflow = args.ScheduleWorkflow
	host.Workspace = args.Workspace
	host.TaskConcurrency = args.TaskConcurrency
	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()

	err = agentmongodb.NewZadigVMColl().Update(agentID, host)
	if err != nil {
		logger.Errorf("failed to update vm %s, error: %s", args.Name, err)
		return e.ErrUpdateZadigHost.AddErr(fmt.Errorf("failed to update vm %s, error: %s", args.Name, err))
	}
	return nil
}

// DeleteAgent The node detects that the vm has been deleted during heartbeat and will automatically uninstall the vm
func DeleteHost(idString, user string, logger *zap.SugaredLogger) error {
	if idString == "" {
		return e.ErrDeleteZadigHost.AddDesc("empty vm id")
	}
	host, err := agentmongodb.NewZadigVMColl().FindByName(idString)
	if err != nil {
		return e.ErrDeleteZadigHost.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	host.IsDeleted = true
	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()
	err = agentmongodb.NewZadigVMColl().Update(idString, host)
	if err != nil {
		logger.Errorf("failed to delete vm %s, error: %s", idString, err)
		return e.ErrDeleteZadigHost.AddErr(fmt.Errorf("failed to delete vm %s, error: %s", idString, err))
	}

	return nil
}

func OfflineHost(idString, user string, logger *zap.SugaredLogger) error {
	if idString == "" {
		return e.ErrOfflineZadigHost.AddDesc("empty vm id")
	}
	host, err := agentmongodb.NewZadigVMColl().FindByName(idString)
	if err != nil {
		return e.ErrOfflineZadigHost.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	host.Status = setting.VMOffline
	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()
	err = agentmongodb.NewZadigVMColl().Update(idString, host)
	if err != nil {
		logger.Errorf("failed to offline vm %s, error: %s", idString, err)
		return e.ErrOfflineZadigHost.AddErr(fmt.Errorf("failed to offline vm %s, error: %s", idString, err))
	}

	return nil
}

func UpgradeAgent(idString, user string, logger *zap.SugaredLogger) error {
	if idString == "" {
		return e.ErrUpgradeZadigHost.AddDesc("empty vm id")
	}
	host, err := agentmongodb.NewZadigVMColl().FindByName(idString)
	if err != nil {
		return e.ErrUpgradeZadigHost.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	host.NeedUpdate = true
	// TODO: 确定升级方案

	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()
	err = agentmongodb.NewZadigVMColl().Update(idString, host)
	if err != nil {
		logger.Errorf("failed to upgrade vm %s, error: %s", idString, err)
		return e.ErrUpgradeZadigHost.AddErr(fmt.Errorf("failed to upgrade vm %s, error: %s", idString, err))
	}

	return nil
}

func ListHosts(logger *zap.SugaredLogger) ([]*AgentBriefListResp, error) {
	hosts, _, err := agentmongodb.NewZadigVMColl().ListByOptions(agentmongodb.ZadigVMListOptions{})
	if err != nil {
		logger.Errorf("failed to list hosts, error: %s", err)
		return nil, e.ErrListZadigHost.AddErr(fmt.Errorf("failed to list hosts, error: %s", err))
	}

	resp := make([]*AgentBriefListResp, 0, len(hosts))
	for _, host := range hosts {
		// if vm is not heartbeat for a long time, set vm status to abnormal
		if host.LastHeartbeatTime < time.Now().Unix()-setting.AgentDefaultHeartbeatTimeout {
			host.Status = setting.VMAbnormal
			host.Error = fmt.Errorf("vm heartbeat timeout").Error()
		}

		a := &AgentBriefListResp{
			Name:   host.Name,
			Tags:   host.Tags,
			Status: host.Status,
			Error:  host.Error,
		}
		if host.VMInfo != nil {
			a.IP = host.VMInfo.IP
			a.Platform = host.VMInfo.Platform
			a.Architecture = host.VMInfo.Architecture
		}
		resp = append(resp, a)
	}

	return resp, nil
}

func GetHost(idString string, logger *zap.SugaredLogger) (*AgentDetails, error) {
	host, err := agentmongodb.NewZadigVMColl().FindByName(idString)
	if err != nil {
		logger.Errorf("failed to find vm %s, error: %s", idString, err)
		return nil, fmt.Errorf("failed to find vm %s, error: %s", idString, err)
	}

	// if vm is not heartbeat for a long time, set vm status to abnormal
	if host.LastHeartbeatTime < time.Now().Unix()-setting.AgentDefaultHeartbeatTimeout {
		host.Status = setting.VMAbnormal
		host.Error = fmt.Errorf("vm heartbeat timeout").Error()
	}

	resp := &AgentDetails{
		Name:             host.Name,
		Description:      host.Description,
		Provider:         host.Provider,
		Tags:             host.Tags,
		HostIP:           host.IP,
		HostPort:         host.Port,
		HostUser:         host.User,
		SSHPrivateKey:    host.SSHPrivateKey,
		ScheduleWorkflow: host.ScheduleWorkflow,
		Workspace:        host.Workspace,
		Status:           host.Status,
	}
	if host.VMInfo != nil {
		resp.Architecture = host.VMInfo.Architecture
		resp.Platform = host.VMInfo.Platform
	}

	return resp, nil
}

func ListVMLabels(logger *zap.SugaredLogger) ([]string, error) {
	return nil, nil
}

func RegisterAgent(args *RegisterAgentRequest, logger *zap.SugaredLogger) (*RegisterAgentResponse, error) {
	host, err := agentmongodb.NewZadigVMColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	host.Status = setting.VMRegistered
	if args.Parameters != nil {
		host.VMInfo = &agentmodel.VMInfo{
			Platform:      args.Parameters.OS,
			Architecture:  args.Parameters.Arch,
			MemeryTotal:   args.Parameters.MemTotal,
			UsedMemery:    args.Parameters.UsedMem,
			CpuNum:        args.Parameters.CpuNum,
			DiskSpace:     args.Parameters.DiskSpace,
			FreeDiskSpace: args.Parameters.FreeDiskSpace,
			VMname:        args.Parameters.Hostname,
		}
	}
	err = agentmongodb.NewZadigVMColl().Update(host.ID.Hex(), host)
	if err != nil {
		logger.Errorf("failed to update vm %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &RegisterAgentResponse{
		Token:        host.Token,
		Description:  host.Description,
		ExecutorNum:  host.TaskConcurrency,
		AgentVersion: host.AgentVersion,
		ZadigVersion: host.ZadigVersion,
	}

	return resp, nil
}

func VerifyAgent(args *VerifyAgentRequest, logger *zap.SugaredLogger) (*VerifyAgentResponse, error) {
	host, err := agentmongodb.NewZadigVMColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &VerifyAgentResponse{
		Verified: false,
	}
	if host.VMInfo != nil && host.VMInfo.Platform == args.Parameters.OS && host.VMInfo.Architecture == args.Parameters.Arch {
		resp.Verified = true
	}

	return resp, nil
}

func Heartbeat(args *HeartbeatRequest, logger *zap.SugaredLogger) (*HeartbeatResponse, error) {
	host, err := agentmongodb.NewZadigVMColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	host.Status = setting.VMNomal
	if args.Parameters != nil && host.VMInfo != nil {
		host.VMInfo.MemeryTotal = args.Parameters.MemTotal
		host.VMInfo.UsedMemery = args.Parameters.UsedMem
		host.VMInfo.DiskSpace = args.Parameters.DiskSpace
		host.VMInfo.FreeDiskSpace = args.Parameters.FreeDiskSpace
		host.VMInfo.VMname = args.Parameters.Hostname
	}
	host.UpdateBy = setting.SystemUser
	host.UpdateTime = time.Now().Unix()

	// set vm heartbeat time
	host.LastHeartbeatTime = time.Now().Unix()

	err = agentmongodb.NewZadigVMColl().Update(host.ID.Hex(), host)
	if err != nil {
		logger.Errorf("failed to update vm %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &HeartbeatResponse{
		NeedUpdateAgentVersion: false,
		AgentVersion:           host.AgentVersion,
		NeedUninstallAgent:     false,
	}
	if host.NeedUpdate {
		resp.NeedUpdateAgentVersion = true
		resp.AgentVersion = host.AgentVersion
	}
	if host.IsDeleted {
		resp.NeedUninstallAgent = true
	}

	return resp, nil
}

type hostJobGetterMap struct {
	M         sync.Mutex
	GetterMap map[string]struct{}
}

func (m *hostJobGetterMap) AddGetter(jobID string) bool {
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

func (m *hostJobGetterMap) RemoveGetter(jobID string) {
	m.M.Lock()
	defer m.M.Unlock()
	delete(m.GetterMap, jobID)
}

var jobGetter = &hostJobGetterMap{
	GetterMap: make(map[string]struct{}),
	M:         sync.Mutex{},
}

func PollingAgentJob(token string, logger *zap.SugaredLogger) (job *agentmodel.VMJob, err error) {
	host, err := agentmongodb.NewZadigVMColl().FindByToken(token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", token, err)
		return nil, fmt.Errorf("failed to find vm by token %s, error: %s", token, err)
	}

	// get job
	job, err = agentmongodb.NewVMJobColl().FindOldestByTags(host.Tags)
	if err != nil {
		if err == mongo.ErrNilDocument || err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logger.Errorf("failed to find job by tags %v, error: %s", host.Tags, err)
		return nil, fmt.Errorf("failed to find job by tags %v, error: %s", host.Tags, err)
	}

	if job != nil && jobGetter.AddGetter(job.ID.Hex()) {
		job.Status = setting.VMJobStatusDistributed
		job.VMID = host.ID.Hex()
		err := agentmongodb.NewVMJobColl().Update(job.ID.Hex(), job)
		if err != nil {
			logger.Errorf("failed to update job %s, error: %s", job.ID.Hex(), err)
			return nil, fmt.Errorf("failed to update job %s, error: %s", job.ID.Hex(), err)
		}

		jobGetter.RemoveGetter(job.ID.Hex())
	} else {
		job, err = PollingAgentJob(token, logger)
	}

	return job, err
}

func ReportAgentJob(args *ReportJobArgs, logger *zap.SugaredLogger) error {
	_, err := agentmongodb.NewZadigVMColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return fmt.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
	}

	// update job

	return nil
}

// GenerateAgentToken TODO: consider how to generate vm token
func GenerateAgentToken() string {
	return primitive.NewObjectID().Hex()
}

func CreateWfJob2DB(args *agentmodel.VMJob, logger *zap.SugaredLogger) error {
	err := agentmongodb.NewVMJobColl().Create(args)
	if err != nil {
		logger.Errorf("failed to create job %s, error: %s", args.ID.Hex(), err)
		return fmt.Errorf("failed to create job %s, error: %s", args.ID.Hex(), err)
	}
	return nil
}
