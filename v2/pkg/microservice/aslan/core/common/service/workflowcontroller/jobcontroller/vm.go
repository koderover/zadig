package jobcontroller

import (
	"context"
	"fmt"
	"time"

	vmmongodb "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/vm"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func waitVMJobStart(ctx context.Context, jobID string, taskTimeout <-chan time.Time, jobTask *commonmodels.JobTask, logger *zap.SugaredLogger) (config.Status, error) {
	logger.Infof("start to wait vm job %s job_id:%s start", jobTask.Name, jobID)
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled, nil
		case <-taskTimeout:
			return config.StatusTimeout, fmt.Errorf("wait job ready timeout")
		default:
			vmJob, err := vmmongodb.NewVMJobColl().FindByID(jobID)
			if err != nil {
				logger.Errorf("failed to find vm job %s, error: %s", jobID, err)
				return config.StatusFailed, fmt.Errorf("failed to find vm job %s, error: %s", jobID, err)
			}
			if vmJob != nil && vmJob.VMID != "" {
				return config.StatusRunning, nil
			}
			time.Sleep(time.Second)
		}
	}
}

func waitVMJobEndByCheckStatus(ctx context.Context, jobID string, taskTimeout <-chan time.Time, jobTask *commonmodels.JobTask, ack func(), logger *zap.SugaredLogger) (status config.Status, errMsg string) {
	logger.Infof("start to wait vm job %s id:%s end", jobTask.Name, jobID)
	for {
		select {
		case <-ctx.Done():
			return config.StatusCancelled, ""
		case <-taskTimeout:
			return config.StatusTimeout, ""
		default:
			vmJob, err := vmmongodb.NewVMJobColl().FindByID(jobID)
			if err != nil {
				logger.Errorf("failed to find vm job %s, error: %s", jobID, err)
				return config.StatusFailed, fmt.Sprintf("failed to find vm job %s, error: %s", jobID, err)
			}
			if vmJob == nil {
				return config.StatusFailed, fmt.Sprintf("failed to find vm job %s", jobID)
			}

			switch vmJob.Status {
			case string(config.StatusCreated):
				jobTask.Status = config.StatusCreated
				continue
			case string(config.StatusPrepare):
				jobTask.Status = config.StatusPrepare
				continue
			case string(config.StatusDistributed):
				jobTask.Status = config.StatusDistributed
				continue
			case string(config.StatusRunning):
				jobTask.Status = config.StatusRunning
				ack()
			case string(config.StatusPassed):
				jobTask.Status = config.StatusPassed
				return config.StatusPassed, ""
			default:
				jobTask.Status = config.StatusFailed
				return config.StatusFailed, vmJob.Error
			}
			time.Sleep(time.Second)
			continue
		}
	}
}
