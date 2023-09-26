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

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/agent/reporter"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/agent/step"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/network"
	"github.com/koderover/zadig/pkg/types/job"
)

func NewJobExecutor(ctx context.Context, job *common.ZadigJobTask, client *network.ZadigClient) *JobExecutor {
	result := &common.JobExecuteResult{
		JobInfo: &common.JobInfo{
			JobID:   job.ID,
			JobName: job.JobName,
		},
		Status: common.StatusPrepare.String(),
		Error:  errors.New(""),
	}

	cancel := new(bool)
	return &JobExecutor{
		Ctx:          ctx,
		Job:          job,
		Client:       client,
		Reporter:     reporter.NewJobReporter(result, client, cancel),
		JobResult:    result,
		Cancel:       cancel,
		FinishedChan: make(chan struct{}, 1),
	}
}

type JobExecutor struct {
	Ctx              context.Context
	Wg               *sync.WaitGroup
	Cmd              *common.Command
	Job              *common.ZadigJobTask
	JobCtx           *common.JobContext
	Client           *network.ZadigClient
	Logger           *log.JobLogger
	Writer           *io.Writer
	Reporter         *reporter.JobReporter
	DockerHost       string
	OutPuts          []*common.JobOutput
	JobResult        *common.JobExecuteResult
	OutputsJsonBytes []byte
	Cancel           *bool
	FinishedChan     chan struct{}
	Workspace        string
	LogPath          string
	OutputDir        string
	CacheDir         string
	ArtifactsDir     string
}

// BeforeExecute init execute context and command
func (e *JobExecutor) BeforeExecute() error {
	err := e.InitWorkDirectory()
	if err != nil {
		log.Info("failed to init work directory", zap.Error(err))
		return err
	}

	return nil
}

func (e *JobExecutor) InitWorkDirectory() error {
	workDir := config.GetActiveWorkDirectory()

	if e.Job != nil && e.Job.ProjectName != "" && e.Job.WorkflowName != "" && e.Job.JobName != "" {
		// init default work directory

		e.Workspace = filepath.Join(workDir, fmt.Sprintf("/%s/%s/%s", e.Job.ProjectName, e.Job.WorkflowName, e.Job.JobName))
	}

	// init work directory
	if _, err := os.Stat(e.Workspace); os.IsNotExist(err) {
		err := os.MkdirAll(e.Workspace, os.ModePerm)
		if err != nil {
			log.Errorf("failed to create work directory, error: %s", err)
			return err
		}
	}

	// init job log executor
	filePath, err := config.GetJobLogFilePath(e.Workspace, e.Job.JobName)
	if err != nil {
		log.Errorf("failed to generate job log file path, error: %s", err)
		return err
	}
	e.Logger = log.NewJobLogger(filePath)
	e.LogPath = filePath
	e.Reporter.Logger = e.Logger

	// init job OutputDir
	outputDir := filepath.Join(e.Workspace, common.JobOutputDir)
	if _, err := os.Stat(outputDir); err == nil {
		if err := os.RemoveAll(outputDir); err != nil {
			log.Errorf("failed to delete job output dir, error: %s", err)
			return fmt.Errorf("failed to delete job output dir, error: %s", err)
		}
	}
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return err
	}
	e.OutputDir = outputDir

	// init job cache directory
	//cacheDir := filepath.Join(e.Workspace, )
	//if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
	//	err = os.MkdirAll(cacheDir, os.ModePerm)
	//	if err != nil {
	//		return err
	//	}
	//}
	//e.CacheDir = cacheDir

	// init job artifacts directory
	//artifactsDir, err := config.GetArtifactsDir(e.Workspace)
	//if err != nil {
	//	log.Errorf("failed to generate job artifacts directory, error: %s", err)
	//	return err
	//}
	//e.ArtifactsDir = artifactsDir

	return nil
}

func (e *JobExecutor) Execute() {
	if e.Job == nil {
		e.JobResult.Error = fmt.Errorf("job is nil")
		return
	}
	var err error

	start := time.Now()
	e.JobResult.StartTime = start.Unix()
	e.JobResult.Status = common.StatusRunning.String()

	defer func() {
		if e.JobResult.Error != nil && e.JobResult.Error.Error() != "" {
			e.JobResult.Status = common.StatusFailed.String()
		} else {
			err := e.collectJobResult(e.Ctx)
			if err != nil {
				e.Logger.Errorf("failed to collect job result, error: %s", err)
				e.JobResult.Error = fmt.Errorf("failed to collect job result, error: %s", err)
				e.JobResult.Status = common.StatusFailed.String()
			} else {
				err = e.Reporter.SetOutputsJsonBytes(e.OutputsJsonBytes)
				if err != nil {
					e.Logger.Errorf("failed to set job outputs, error: %s", err)
					e.JobResult.Error = fmt.Errorf("failed to set job outputs, error: %s", err)
					e.JobResult.Status = common.StatusFailed.String()
				} else {
					// ensure that the passed state and output variables are passed at the same time.
					e.JobResult.Status = common.StatusPassed.String()
					_, err := e.Reporter.ReportWithData(e.JobResult)
					if err != nil {
						e.JobResult.Error = err
					}
				}
			}
		}
		e.JobResult.EndTime = time.Now().Unix()

		e.Logger.Printf("====================== Job Executor End. Duration: %.2f seconds ======================\n", time.Since(start).Seconds())
	}()
	e.Logger.Printf("====================== Job Executor Start ======================\n")
	if e.CheckZadigCancel() {
		err = fmt.Errorf("user cancel job %s", e.Job.JobName)
		return
	}

	err = e.run()
	if err != nil {
		e.Logger.Errorf("failed to execute job, error: %v", err)
		e.JobResult.Error = err
		return
	}
}

func (e *JobExecutor) run() error {
	hasFailed := false
	var respErr error
	if e.Job.JobCtx == "" {
		return fmt.Errorf("job context is empty")
	}
	e.JobCtx = new(common.JobContext)
	if err := e.JobCtx.Decode(e.Job.JobCtx); err != nil {
		return fmt.Errorf("decode job context error: %v", err)
	}

	for _, stepInfo := range e.JobCtx.Steps {
		if e.CheckZadigCancel() {
			return fmt.Errorf("user cancel job %s", e.Job.JobName)
		}
		if hasFailed && !stepInfo.Onfailure {
			continue
		}
		if err := step.RunStep(e.Ctx, e.JobCtx, stepInfo, e.Workspace, "", e.getUserEnvs(), e.JobCtx.SecretEnvs, e.Logger); err != nil {
			hasFailed = true
			respErr = err
		}
	}
	return respErr
}

func (e *JobExecutor) getUserEnvs() []string {
	envs := os.Environ()
	envs = append(envs,
		"CI=true",
		"ZADIG=true",
		fmt.Sprintf("HOME=%s", config.Home()),
		fmt.Sprintf("WORKSPACE=%s", e.Workspace),
	)

	//e.JobCtx.Paths = strings.Replace(e.JobCtx.Paths, "$HOME", config.Home(), -1)
	//envs = append(envs, fmt.Sprintf("PATH=%s", e.JobCtx.Paths))
	envs = append(envs, fmt.Sprintf("DOCKER_HOST=%s", e.DockerHost))
	envs = append(envs, e.JobCtx.Envs...)
	envs = append(envs, e.JobCtx.SecretEnvs...)
	// share output var between steps.
	outputs, err := e.getJobOutputVars()
	if err != nil {
		log.Errorf("get job output vars error: %v", err)
	}
	for _, output := range outputs {
		envs = append(envs, fmt.Sprintf("%s=%s", output.Name, output.Value))
	}

	return envs
}

func (e *JobExecutor) AfterExecute() error {
	// report all job log
	log.Infof("start to workflow %s job %s AfterExecute stage", e.Job.WorkflowName, e.Job.JobName)
	for logStr, EOFErr, err := e.Reporter.GetJobLog(); err == nil; {
		resp, err := e.Reporter.ReportWithData(
			&common.JobExecuteResult{
				JobInfo: e.JobResult.JobInfo,
				Status:  e.JobResult.Status,
				Log:     logStr,
				Error:   e.JobResult.Error,
			})
		if err != nil {
			log.Errorf("report workflow %s job %s log error: %v", e.Job.WorkflowName, e.Job.JobName, err)
			return nil
		}
		if resp != nil && (resp.JobStatus == common.StatusCancelled.String() || resp.JobStatus == common.StatusTimeout.String()) {
			*e.Cancel = true
			return nil
		}

		if EOFErr {
			break
		}
	}
	return nil
}

func (j *JobExecutor) collectJobResult(ctx context.Context) error {
	outputs, err := j.getJobOutputVars()
	if err != nil {
		return fmt.Errorf("get job output vars error: %v", err)
	}
	jsonOutput, err := json.Marshal(outputs)
	if err != nil {
		return err
	}

	if len(jsonOutput) > common.MaxContainerTerminationMessageLength {
		return fmt.Errorf("termination message is above max allowed size 4096, caused by large task result")
	}

	j.OutputsJsonBytes = jsonOutput
	return nil
}

func (e *JobExecutor) getJobOutputVars() ([]*job.JobOutput, error) {
	outputs := []*job.JobOutput{}
	for _, outputName := range e.JobCtx.Outputs {
		fileContents, err := ioutil.ReadFile(filepath.Join(e.OutputDir, outputName))
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return outputs, err
		}
		value := strings.Trim(string(fileContents), "\n")
		outputs = append(outputs, &job.JobOutput{Name: outputName, Value: value})
	}
	return outputs, nil
}

func (e *JobExecutor) deleteTempFileAndDir() error {
	if err := os.RemoveAll(e.Workspace); err != nil {
		log.Errorf("failed to delete job workspace, error: %s", err)
		return err
	}
	//// delete job log file
	//if err := os.Remove(e.LogPath); err != nil {
	//	log.Errorf("failed to delete job log file, error: %s", err)
	//	return err
	//}
	//
	//// delete job output dir
	//if err := os.RemoveAll(e.OutputDir); err != nil {
	//	log.Errorf("failed to delete job output dir, error: %s", err)
	//	return err
	//}
	//
	//// delete user script file
	//if err := os.Remove(config.GetUserScriptFilePath(e.Workspace)); err != nil {
	//	log.Errorf("failed to delete user script file, error: %s", err)
	//	return err
	//}
	return nil
}

func (e *JobExecutor) StopJob() error {
	return nil
}

func (e *JobExecutor) CheckZadigCancel() bool {
	if *e.Cancel {
		return true
	}
	return false
}
