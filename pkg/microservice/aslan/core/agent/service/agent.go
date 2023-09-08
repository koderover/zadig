package service

import (
	"time"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmongodb "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
)

func RegisterAgent(args *RegisterAgentRequest, logger *zap.SugaredLogger) (*RegisterAgentResponse, error) {
	agent, err := commonmongodb.NewZadigAgentColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find agent by token %s, error: %s", args.Token, err)
		return nil, err
	}

	agent.Status = setting.AgentRegistered
	if args.Parameters != nil {
		agent.HostInfo = &commonmodels.AgentHostInfo{
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
	err = commonmongodb.NewZadigAgentColl().Update(agent.ID.Hex(), agent)
	if err != nil {
		logger.Errorf("failed to update agent %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &RegisterAgentResponse{
		Token:        agent.Token,
		Description:  agent.Description,
		ExecutorNum:  agent.TaskConcurrency,
		AgentVersion: agent.AgentVersion,
		ZadigVersion: agent.ZadigVersion,
	}

	return resp, nil
}

func VerifyAgent(args *VerifyAgentRequest, logger *zap.SugaredLogger) (*VerifyAgentResponse, error) {
	agent, err := commonmongodb.NewZadigAgentColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find agent by token %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &VerifyAgentResponse{
		Verified: false,
	}
	if agent.HostInfo != nil && agent.HostInfo.Platform == args.Parameters.OS && agent.HostInfo.Architecture == args.Parameters.Arch {
		resp.Verified = true
	}

	return resp, nil
}

func Heartbeat(args *HeartbeatRequest, logger *zap.SugaredLogger) (*HeartbeatResponse, error) {
	agent, err := commonmongodb.NewZadigAgentColl().FindByToken(args.Token)
	if err != nil {
		logger.Errorf("failed to find agent by token %s, error: %s", args.Token, err)
		return nil, err
	}

	agent.Status = setting.AgentNomal
	if args.Parameters != nil && agent.HostInfo != nil {
		agent.HostInfo.MemeryTotal = args.Parameters.MemTotal
		agent.HostInfo.UsedMemery = args.Parameters.UsedMem
		agent.HostInfo.DiskSpace = args.Parameters.DiskSpace
		agent.HostInfo.FreeDiskSpace = args.Parameters.FreeDiskSpace
		agent.HostInfo.Hostname = args.Parameters.Hostname
	}
	agent.UpdateBy = setting.SystemUser
	agent.UpdateTime = time.Now().Unix()

	err = commonmongodb.NewZadigAgentColl().Update(agent.ID.Hex(), agent)
	if err != nil {
		logger.Errorf("failed to update agent %s, error: %s", args.Token, err)
		return nil, err
	}

	resp := &HeartbeatResponse{
		NeedUpdateAgentVersion: false,
		AgentVersion:           agent.AgentVersion,
		NeedUninstallAgent:     false,
	}
	if agent.NeedUpdate {
		resp.NeedUpdateAgentVersion = true
		resp.AgentVersion = agent.AgentVersion
	}
	if agent.IsDeleted {
		resp.NeedUninstallAgent = true
	}

	// notify heartbeat success to agent heartbeat monitor
	return resp, nil
}
