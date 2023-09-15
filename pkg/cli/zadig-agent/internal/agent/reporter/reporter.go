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

package reporter

import (
	"context"
	"fmt"
	"time"

	errhelper "github.com/koderover/zadig/pkg/cli/zadig-agent/helper/error"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/network"
)

type JobReporter struct {
	Seq       int
	Ctx       context.Context
	Client    *network.ZadigClient
	Logger    *log.JobLogger
	Offset    uint
	Log       string
	JobCancel *bool
	Result    *common.JobExecuteResult
}

func NewJobReporter(result *common.JobExecuteResult, client *network.ZadigClient, cancel *bool) *JobReporter {
	return &JobReporter{
		Seq:       0,
		Client:    client,
		Result:    result,
		JobCancel: cancel,
	}
}

func (r *JobReporter) Start(ctx context.Context) {
	log.Infof("agent job %s reporter.", r.Result.JobInfo.JobName)
	r.Ctx = ctx
	ticker := time.NewTicker(time.Second)

	for {
		r.Seq++
		select {
		case <-ticker.C:
			// get log from job log file
			err := r.SetLog()
			if err != nil {
				log.Errorf("failed to set job log, error: %s", err)
				continue
			}
			if err := r.Report(); err != nil {
				log.Error(err)
			}
		case <-ctx.Done():
			log.Infof("stop job reporter, received context cancel signal.")
			return
		}
	}
}

func (r *JobReporter) GetJobLog() (string, bool, error) {
	buffer, newOffset, EOFErr, err := r.Logger.ReadByRowNum(r.Offset, common.DefaultJobLogReadNum)
	if err != nil {
		return "", EOFErr, err
	}
	r.Offset = newOffset
	return string(buffer), EOFErr, nil
}

func (r *JobReporter) Report() error {
	if r.Result == nil {
		return fmt.Errorf("reporter result is nil")
	}

	resp, err := r.Client.ReportJob(&common.ReportJobParameters{
		Seq:       r.Seq,
		JobID:     r.Result.JobInfo.JobID,
		JobStatus: r.Result.Status,
		JobError:  errhelper.ErrHandler(r.Result.Error),
		JobLog:    r.Result.Log,
		JobOutput: r.Result.OutputsJsonBytes,
	})

	if err != nil {
		return fmt.Errorf("%s-%s ---------> SEQ: %d failed to report status, error: %s", r.Result.JobInfo.WorkflowName, r.Result.JobInfo.JobName, r.Seq, err)
	}

	if resp.JobID == r.Result.JobInfo.JobID && (resp.JobStatus == common.StatusTimeout.String() || resp.JobStatus == common.StatusCancelled.String()) {
		*r.JobCancel = true
	}
	return nil
}

func (r *JobReporter) ReportWithData(result *common.JobExecuteResult) (*common.ReportAgentJobResp, error) {
	if result == nil {
		return nil, fmt.Errorf("reporter result is nil")
	}
	if result.JobInfo == nil {
		return nil, fmt.Errorf("reporter result job info is nil")
	}

	resp, err := r.Client.ReportJob(&common.ReportJobParameters{
		JobID:     result.JobInfo.JobID,
		JobStatus: result.Status,
		JobError:  errhelper.ErrHandler(result.Error),
		JobLog:    result.Log,
	})
	return resp, err
}

func (r *JobReporter) SetStatus(status string) error {
	if r.Result == nil {
		return fmt.Errorf("reporter job result is nil")
	}

	r.Result.Status = status

	return nil
}

func (r *JobReporter) SetError(err error) error {
	if r.Result == nil {
		return fmt.Errorf("reporter result is nil")
	}

	r.Result.Error = err

	return nil
}

func (r *JobReporter) SetLog() error {
	if r.Result == nil {
		return fmt.Errorf("reporter result is nil")
	}

	logStr, _, err := r.GetJobLog()
	if err != nil {
		return fmt.Errorf("failed to get job log, error: %s", err)
	}

	r.Result.Log = logStr

	return nil
}

func (r *JobReporter) SetStartTime(startTime int64) error {
	if r.Result == nil {
		return fmt.Errorf("reporter result is nil")
	}

	r.Result.StartTime = startTime

	return nil
}

func (r *JobReporter) SetEndTime(endTime int64) error {
	if r.Result == nil {
		return fmt.Errorf("reporter result is nil")
	}

	r.Result.EndTime = endTime

	return nil
}

func (r *JobReporter) SetOutputsJsonBytes(outputsJsonBytes []byte) error {
	if r.Result == nil {
		return fmt.Errorf("reporter result is nil")
	}

	r.Result.OutputsJsonBytes = outputsJsonBytes

	return nil
}
