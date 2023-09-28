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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/koderover/zadig/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/agent/reporter"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/agent/step"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/common/types"
	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/network"
	"github.com/koderover/zadig/pkg/types/job"
)

func NewJobExecutor(ctx context.Context, job *types.ZadigJobTask, client *network.ZadigClient, reporterCancel context.CancelFunc) *JobExecutor {
	result := types.NetJobExecuteResult(&types.JobInfo{JobID: job.ID, JobName: job.JobName})
	result.SetStatus(common.StatusPrepare)

	cancel := new(bool)
	return &JobExecutor{
		Ctx:            ctx,
		Job:            job,
		Client:         client,
		Reporter:       reporter.NewJobReporter(result, client, cancel),
		JobResult:      result,
		Cancel:         cancel,
		FinishedChan:   make(chan struct{}, 1),
		ReporterCancel: reporterCancel,
	}
}

type JobExecutor struct {
	Ctx              context.Context
	Wg               *sync.WaitGroup
	Cmd              *common.Command
	Job              *types.ZadigJobTask
	JobCtx           *types.JobContext
	Client           *network.ZadigClient
	Logger           *log.JobLogger
	Writer           *io.Writer
	Reporter         *reporter.JobReporter
	DockerHost       string
	JobResult        *types.JobExecuteResult
	OutputsJsonBytes []byte
	Cancel           *bool
	FinishedChan     chan struct{}
	ReporterCancel   context.CancelFunc
	Dirs             *types.AgentWorkDirs
}

// BeforeExecute init execute context and command
func (e *JobExecutor) BeforeExecute() error {
	err := e.InitWorkDirectory()
	if err != nil {
		log.Info("failed to init work directory", err)
		return err
	}

	return nil
}

func (e *JobExecutor) InitWorkDirectory() error {
	workDir := config.GetActiveWorkDirectory()
	e.Dirs = &types.AgentWorkDirs{
		WorkDir: workDir,
	}

	if e.Job != nil && e.Job.ProjectName != "" && e.Job.WorkflowName != "" && e.Job.JobName != "" {
		// init default work directory
		e.Dirs.Workspace = filepath.Join(workDir, fmt.Sprintf("/%s/%s/%s/%s", e.Job.ProjectName, e.Job.WorkflowName, fmt.Sprintf("task-%d", e.Job.TaskID), e.Job.JobName))
	}

	// ------------------------------------------------- init workspace -------------------------------------------------
	if _, err := os.Stat(e.Dirs.Workspace); os.IsNotExist(err) {
		err := os.MkdirAll(e.Dirs.Workspace, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create workspace, error: %v", err)
		}
	}

	// ------------------------------------------- init agent job log tmp dir -------------------------------------------
	// init job log executor
	logDir, err := config.GetJobLogFilePath(workDir, *e.Job)
	if err != nil {
		return fmt.Errorf("failed to generate job log directory, error: %v", err)
	}
	if _, err := os.Stat(logDir); err == nil {
		if err := os.RemoveAll(logDir); err != nil {
			return fmt.Errorf("failed to delete job log dir, error: %v", err)
		}
	}
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create job log tmp directory, error: %v", err)
	}
	filePath := filepath.Join(logDir, fmt.Sprintf("%s.log", e.Job.JobName))
	// remove old log file if exists
	if _, err := os.Stat(filePath); err == nil {
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove log file: %v", err)
		}
	}
	// create new file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Errorf("failed to close log file: %v", err)
		}
	}()
	e.Logger = log.NewJobLogger(filePath)
	e.Dirs.JobLogPath = filePath
	e.Reporter.Logger = e.Logger

	// --------------------------------------------- init job script tmp dir ---------------------------------------------
	jobScriptTmpDir, err := config.GetJobScriptTmpDir(workDir, *e.Job)
	if err != nil {
		return fmt.Errorf("failed to generate job script tmp directory, error: %v", err)
	}
	if _, err := os.Stat(jobScriptTmpDir); err == nil {
		if err := os.RemoveAll(jobScriptTmpDir); err != nil {
			return fmt.Errorf("failed to delete job script dir, error: %v", err)
		}
	}
	if err := os.MkdirAll(jobScriptTmpDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create job script tmp directory, error: %v", err)
	}
	e.Dirs.JobScriptDir = jobScriptTmpDir

	// -------------------------------------------- init job output tmp dir ---------------------------------------------
	outputDir, err := config.GetJobOutputsTmpDir(workDir, *e.Job)
	if err != nil {
		return fmt.Errorf("failed to generate job outputs tmp directory, error: %v", err)
	}
	if _, err := os.Stat(outputDir); err == nil {
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("failed to delete job output dir, error: %v", err)
		}
	}
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create job output tmp directory, error: %v", err)
	}
	e.Dirs.JobOutputsDir = outputDir

	return nil
}

func (e *JobExecutor) Execute() {
	if e.Job == nil {
		e.JobResult.SetError(fmt.Errorf("job is nil"))
		return
	}
	var err error

	start := time.Now()
	e.JobResult.SetStartTime(start.Unix())
	e.JobResult.SetStatus(common.StatusRunning)

	defer func() {
		e.ReporterCancel()

		if outputs, err := e.getJobOutputVars(); err != nil {
			e.Logger.Errorf("failed to collect job result, error: %s", err)
			e.JobResult.SetError(fmt.Errorf("failed to collect job result, error: %s", err))
		} else {
			err = e.JobResult.SetOutputs(outputs)
			if err != nil {
				e.Logger.Errorf("failed to set job outputs, error: %s", err)
				e.JobResult.SetError(fmt.Errorf("failed to set job outputs, error: %s", err))
			}
		}

		e.Logger.Printf("====================== Job Executor End. Duration: %.2f seconds ======================\n", time.Since(start).Seconds())
	}()
	e.Logger.Printf("====================== Job Executor Start ======================\n")
	if e.CheckZadigCancel() {
		err = fmt.Errorf("user cancel job %s", e.Job.JobName)
		return
	}

	err = e.run()
	if err != nil {
		e.Logger.Errorf(fmt.Sprintf("failed to execute job, error: %v", err))
		e.JobResult.SetError(err)
		return
	}
}

func (e *JobExecutor) run() error {
	hasFailed := false
	var respErr error
	if e.Job.JobCtx == "" {
		return fmt.Errorf("job context is empty")
	}
	e.JobCtx = new(types.JobContext)
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
		if err := step.RunStep(e.Ctx, e.JobCtx, stepInfo, e.Dirs, e.getUserEnvs(), e.JobCtx.SecretEnvs, e.Logger); err != nil {
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
		fmt.Sprintf("WORKSPACE=%s", e.Dirs.Workspace),
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
	log.Infof("start workflow %s job %s AfterExecute stage", e.Job.WorkflowName, e.Job.JobName)
	for logStr, EOFErr, err := e.Reporter.GetJobLog(); err == nil; {
		resp, err := e.Reporter.ReportWithData(
			&types.JobExecuteResult{
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

	// delete all temp file and dir
	err := e.deleteTempFileAndDir()
	if err != nil {
		log.Errorf("failed to delete temp file and dir, error: %s", err)
	}

	return nil
}

func (e *JobExecutor) getJobOutputVars() ([]*job.JobOutput, error) {
	outputs := []*job.JobOutput{}
	for _, outputName := range e.JobCtx.Outputs {
		fileContents, err := ioutil.ReadFile(filepath.Join(e.Dirs.JobOutputsDir, outputName))
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
	// --------------------------------------------- delete job workspace ---------------------------------------------
	if err := os.RemoveAll(e.Dirs.Workspace); err != nil {
		log.Errorf("failed to delete job workspace, error: %s", err)
		return err
	}

	// --------------------------------------------- delete job log file ---------------------------------------------
	if err := os.RemoveAll(filepath.Dir(e.Dirs.JobLogPath)); err != nil {
		log.Errorf("failed to delete job log file, error: %s", err)
		return err
	}

	// --------------------------------------------- delete job script dir ---------------------------------------------
	if err := os.RemoveAll(e.Dirs.JobScriptDir); err != nil {
		log.Errorf("failed to delete user script file, error: %s", err)
		return err
	}

	// --------------------------------------------- delete job output dir ---------------------------------------------
	if err := os.RemoveAll(filepath.Dir(filepath.Dir(filepath.Dir(e.Dirs.JobOutputsDir)))); err != nil {
		log.Errorf("failed to delete job output dir, error: %s", err)
		return err
	}

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
