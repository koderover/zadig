package service

import (
	"fmt"
	"sync"
	"time"

	agentmodel "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/host"
	agentmongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/host"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/setting"
)

func CreateHost(args *CreateHostRequest, user string, logger *zap.SugaredLogger) error {
	_, err := agentmongodb.NewZadigHostColl().FindByName(args.Name)
	if err == nil {
		return e.ErrCreateZadigHost.AddErr(fmt.Errorf("vm %s already exists", args.Name))
	}

	host := &agentmodel.ZadigHost{
		Name:             args.Name,
		Description:      args.Description,
		Provider:         args.Provider,
		Tags:             args.Tags,
		HostIP:           args.HostIP,
		HostUser:         args.HostUser,
		HostPort:         args.HostPort,
		SSHPrivateKey:    args.SSHPrivateKey,
		ScheduleWorkflow: args.ScheduleWorkflow,
		Workspace:        args.Workspace,
		TaskConcurrency:  args.TaskConcurrency,
		CreatedBy:        user,
		CreateTime:       time.Now().Unix(),
		UpdateTime:       time.Now().Unix(),
		UpdateBy:         user,
		Status:           setting.HostCreated,
	}

	// generate token
	host.Token = GenerateAgentToken()

	err = agentmongodb.NewZadigHostColl().Create(host)
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
	host, err := agentmongodb.NewZadigHostColl().FindByName(args.Name)
	if err != nil {
		return e.ErrUpdateZadigHost.AddErr(fmt.Errorf("vm %s not exists", args.Name))
	}

	host.Name = args.Name
	host.Description = args.Description
	host.Provider = args.Provider
	host.Tags = args.Tags
	host.HostIP = args.HostIP
	host.HostPort = args.HostPort
	host.HostUser = args.HostUser
	host.SSHPrivateKey = args.SSHPrivateKey
	host.ScheduleWorkflow = args.ScheduleWorkflow
	host.Workspace = args.Workspace
	host.TaskConcurrency = args.TaskConcurrency
	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()

	err = agentmongodb.NewZadigHostColl().Update(agentID, host)
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
	host, err := agentmongodb.NewZadigHostColl().FindByName(idString)
	if err != nil {
		return e.ErrDeleteZadigHost.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	host.IsDeleted = true
	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()
	err = agentmongodb.NewZadigHostColl().Update(idString, host)
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
	host, err := agentmongodb.NewZadigHostColl().FindByName(idString)
	if err != nil {
		return e.ErrOfflineZadigHost.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	host.Status = setting.HostOffline
	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()
	err = agentmongodb.NewZadigHostColl().Update(idString, host)
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
	host, err := agentmongodb.NewZadigHostColl().FindByName(idString)
	if err != nil {
		return e.ErrUpgradeZadigHost.AddErr(fmt.Errorf("vm %s not exists", idString))
	}

	host.NeedUpdate = true
	// TODO: 确定升级方案

	host.UpdateBy = user
	host.UpdateTime = time.Now().Unix()
	err = agentmongodb.NewZadigHostColl().Update(idString, host)
	if err != nil {
		logger.Errorf("failed to upgrade vm %s, error: %s", idString, err)
		return e.ErrUpgradeZadigHost.AddErr(fmt.Errorf("failed to upgrade vm %s, error: %s", idString, err))
	}

	return nil
}

func ListHosts(logger *zap.SugaredLogger) ([]*AgentBriefListResp, error) {
	hosts, _, err := agentmongodb.NewZadigHostColl().ListByOptions(agentmongodb.ZadigHostListOptions{})
	if err != nil {
		logger.Errorf("failed to list hosts, error: %s", err)
		return nil, e.ErrListZadigHost.AddErr(fmt.Errorf("failed to list hosts, error: %s", err))
	}

	resp := make([]*AgentBriefListResp, 0, len(hosts))
	for _, host := range hosts {
		// if vm is not heartbeat for a long time, set vm status to abnormal
		if host.LastHeartbeatTime < time.Now().Unix()-setting.AgentDefaultHeartbeatTimeout {
			host.Status = setting.HostAbnormal
			host.Error = fmt.Errorf("vm heartbeat timeout").Error()
		}

		a := &AgentBriefListResp{
			Name:   host.Name,
			Tags:   host.Tags,
			Status: host.Status,
			Error:  host.Error,
		}
		if host.HostInfo != nil {
			a.IP = host.HostInfo.IP
			a.Platform = host.HostInfo.Platform
			a.Architecture = host.HostInfo.Architecture
		}
		resp = append(resp, a)
	}

	return resp, nil
}

func GetHost(idString string, logger *zap.SugaredLogger) (*AgentDetails, error) {
	host, err := agentmongodb.NewZadigHostColl().FindByName(idString)
	if err != nil {
		logger.Errorf("failed to find vm %s, error: %s", idString, err)
		return nil, fmt.Errorf("failed to find vm %s, error: %s", idString, err)
	}

	// if vm is not heartbeat for a long time, set vm status to abnormal
	if host.LastHeartbeatTime < time.Now().Unix()-setting.AgentDefaultHeartbeatTimeout {
		host.Status = setting.HostAbnormal
		host.Error = fmt.Errorf("vm heartbeat timeout").Error()
	}

	resp := &AgentDetails{
		Name:             host.Name,
		Description:      host.Description,
		Provider:         host.Provider,
		Tags:             host.Tags,
		HostIP:           host.HostIP,
		HostPort:         host.HostPort,
		HostUser:         host.HostUser,
		SSHPrivateKey:    host.SSHPrivateKey,
		ScheduleWorkflow: host.ScheduleWorkflow,
		Workspace:        host.Workspace,
		Status:           host.Status,
	}
	if host.HostInfo != nil {
		resp.Architecture = host.HostInfo.Architecture
		resp.Platform = host.HostInfo.Platform
	}

	return resp, nil
}

func ListVMLabels(logger *zap.SugaredLogger) ([]string, error) {
	return nil, nil
}

func RegisterAgent(args *RegisterAgentRequest, logger *zap.SugaredLogger) (*RegisterAgentResponse, error) {
	host, err := agentmongodb.NewZadigHostColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	host.Status = setting.HostRegistered
	if args.Parameters != nil {
		host.HostInfo = &agentmodel.HostInfo{
			Platform:      args.Parameters.OS,
			Architecture:  args.Parameters.Arch,
			MemeryTotal:   args.Parameters.MemTotal,
			UsedMemery:    args.Parameters.UsedMem,
			CpuNum:        args.Parameters.CpuNum,
			DiskSpace:     args.Parameters.DiskSpace,
			FreeDiskSpace: args.Parameters.FreeDiskSpace,
			Hostname:      args.Parameters.Hostname,
		}
	}
	err = agentmongodb.NewZadigHostColl().Update(host.ID.Hex(), host)
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
	host, err := agentmongodb.NewZadigHostColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &VerifyAgentResponse{
		Verified: false,
	}
	if host.HostInfo != nil && host.HostInfo.Platform == args.Parameters.OS && host.HostInfo.Architecture == args.Parameters.Arch {
		resp.Verified = true
	}

	return resp, nil
}

func Heartbeat(args *HeartbeatRequest, logger *zap.SugaredLogger) (*HeartbeatResponse, error) {
	host, err := agentmongodb.NewZadigHostColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", args.Token, err)
		return nil, err
	}

	host.Status = setting.HostNomal
	if args.Parameters != nil && host.HostInfo != nil {
		host.HostInfo.MemeryTotal = args.Parameters.MemTotal
		host.HostInfo.UsedMemery = args.Parameters.UsedMem
		host.HostInfo.DiskSpace = args.Parameters.DiskSpace
		host.HostInfo.FreeDiskSpace = args.Parameters.FreeDiskSpace
		host.HostInfo.Hostname = args.Parameters.Hostname
	}
	host.UpdateBy = setting.SystemUser
	host.UpdateTime = time.Now().Unix()

	// set vm heartbeat time
	host.LastHeartbeatTime = time.Now().Unix()

	err = agentmongodb.NewZadigHostColl().Update(host.ID.Hex(), host)
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

func PollingAgentJob(token string, logger *zap.SugaredLogger) (job *agentmodel.HostJob, err error) {
	host, err := agentmongodb.NewZadigHostColl().FindByToken(token)
	if err != nil {
		logger.Errorf("failed to find vm by token %s, error: %s", token, err)
		return nil, fmt.Errorf("failed to find vm by token %s, error: %s", token, err)
	}

	// get job
	job, err = agentmongodb.NewHostJobColl().FindOldestByTags(host.Tags)
	if err != nil {
		if err == mongo.ErrNilDocument || err == mongo.ErrNoDocuments {
			return nil, nil
		}
		logger.Errorf("failed to find job by tags %v, error: %s", host.Tags, err)
		return nil, fmt.Errorf("failed to find job by tags %v, error: %s", host.Tags, err)
	}

	if job != nil && jobGetter.AddGetter(job.ID.Hex()) {
		job.Status = setting.HostJobStatusDistributed
		job.HostID = host.ID.Hex()
		err := agentmongodb.NewHostJobColl().Update(job.ID.Hex(), job)
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
	_, err := agentmongodb.NewZadigHostColl().FindByToken(args.Token)
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

func CreateWfJob2DB(args *agentmodel.HostJob, logger *zap.SugaredLogger) error {
	err := agentmongodb.NewHostJobColl().Create(args)
	if err != nil {
		logger.Errorf("failed to create job %s, error: %s", args.ID.Hex(), err)
		return fmt.Errorf("failed to create job %s, error: %s", args.ID.Hex(), err)
	}
	return nil
}
