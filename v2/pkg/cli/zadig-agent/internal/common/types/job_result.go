package types

import (
	"encoding/json"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/types/job"
)

type JobInfo struct {
	ProjectName  string
	WorkflowName string
	StageName    string
	JobName      string
	JobID        string
	JobType      string
	Status       string
	Error        error
	Log          string
	StartTime    int64
	EndTime      int64
}

func NetJobExecuteResult(jobInfo *JobInfo) *JobExecuteResult {
	return &JobExecuteResult{
		JobInfo: jobInfo,
	}
}

type JobExecuteResult struct {
	JobInfo          *JobInfo
	Error            error
	Log              string
	Status           string
	StartTime        int64
	EndTime          int64
	OutputsJsonBytes []byte
}

func (r *JobExecuteResult) SetOutputs(outputs []*job.JobOutput) error {
	bytes, err := json.Marshal(outputs)
	if err != nil {
		return err
	}

	if len(bytes) > common.MaxContainerTerminationMessageLength {
		return fmt.Errorf("termination message is above max allowed size 4096, caused by large task result")
	}

	r.OutputsJsonBytes = bytes
	return nil
}

func (r *JobExecuteResult) GetError() error {
	if r.Error != nil {
		return r.Error
	}
	return nil
}

func (r *JobExecuteResult) SetError(err error) {
	r.Error = err
}

func (r *JobExecuteResult) SetLog(log string) {
	r.Log = log
}

func (r *JobExecuteResult) GetStatus() string {
	return r.Status
}

func (r *JobExecuteResult) SetStatus(status common.Status) {
	r.Status = status.String()
}

func (r *JobExecuteResult) SetStartTime(startTime int64) {
	r.StartTime = startTime
}

func (r *JobExecuteResult) SetEndTime(endTime int64) {
	r.EndTime = endTime
}
