/*
Copyright 2022 The KodeRover Authors.

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

package job

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/microservice/jobexecutor/config"
	"github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service/meta"
	"github.com/koderover/zadig/pkg/microservice/jobexecutor/core/service/step"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types/job"
	"gopkg.in/yaml.v3"
)

type Job struct {
	Ctx             *meta.JobContext
	StartTime       time.Time
	ActiveWorkspace string
	UserEnvs        map[string]string
}

const (
	// MaxContainerTerminationMessageLength is the upper bound any one container may write to
	// its termination message path. Contents above this length will cause a failure.
	MaxContainerTerminationMessageLength = 1024 * 4
)

func NewJob() (*Job, error) {
	context, err := ioutil.ReadFile(config.JobConfigFile())
	if err != nil {
		return nil, fmt.Errorf("read job config file error: %v", err)
	}

	var ctx *meta.JobContext
	if err := yaml.Unmarshal(context, &ctx); err != nil {
		return nil, fmt.Errorf("cannot unmarshal job data: %v", err)
	}

	if ctx.Paths != "" {
		ctx.Paths = fmt.Sprintf("%s:%s", config.Path(), ctx.Paths)
	} else {
		ctx.Paths = config.Path()
	}

	job := &Job{
		Ctx: ctx,
	}

	err = job.EnsureActiveWorkspace(ctx.Workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure active workspace `%s`: %s", ctx.Workspace, err)
	}

	userEnvs := job.getUserEnvs()
	job.UserEnvs = make(map[string]string, len(userEnvs))
	for _, env := range userEnvs {
		items := strings.Split(env, "=")
		if len(items) != 2 {
			continue
		}

		job.UserEnvs[items[0]] = items[1]
	}

	return job, nil
}

func (j *Job) EnsureActiveWorkspace(workspace string) error {
	if workspace == "" {
		tempWorkspace, err := ioutil.TempDir(os.TempDir(), "jobexecutor")
		if err != nil {
			return fmt.Errorf("create workspace error: %v", err)
		}
		j.ActiveWorkspace = tempWorkspace
		return os.Chdir(j.ActiveWorkspace)
	}

	err := os.MkdirAll(workspace, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}
	j.ActiveWorkspace = workspace

	return os.Chdir(j.ActiveWorkspace)
}

func (j *Job) getUserEnvs() []string {
	envs := []string{
		"CI=true",
		"ZADIG=true",
		fmt.Sprintf("HOME=%s", config.Home()),
		fmt.Sprintf("WORKSPACE=%s", j.ActiveWorkspace),
	}

	j.Ctx.Paths = strings.Replace(j.Ctx.Paths, "$HOME", config.Home(), -1)
	envs = append(envs, fmt.Sprintf("PATH=%s", j.Ctx.Paths))
	envs = append(envs, fmt.Sprintf("DOCKER_HOST=%s", config.DockerHost()))
	envs = append(envs, j.Ctx.Envs...)
	envs = append(envs, j.Ctx.SecretEnvs...)
	// share output var between steps.
	outputs, err := j.getJobOutputVars(context.Background())
	if err != nil {
		log.Errorf("get job output vars error: %v", err)
	}
	for _, output := range outputs {
		envs = append(envs, fmt.Sprintf("%s=%s", output.Name, output.Value))
	}

	return envs
}

func (j *Job) Run(ctx context.Context) error {
	if err := os.MkdirAll(job.JobOutputDir, os.ModePerm); err != nil {
		return err
	}
	hasFailed := false
	var respErr error
	for _, stepInfo := range j.Ctx.Steps {
		if hasFailed && !stepInfo.Onfailure {
			continue
		}
		if err := step.RunStep(ctx, stepInfo, j.ActiveWorkspace, j.Ctx.Paths, j.getUserEnvs(), j.Ctx.SecretEnvs); err != nil {
			hasFailed = true
			respErr = err
		}
	}
	return respErr
}

func (j *Job) AfterRun(ctx context.Context) error {
	return j.collectJobResult(ctx)
}

func (j *Job) collectJobResult(ctx context.Context) error {
	outputs, err := j.getJobOutputVars(ctx)
	if err != nil {
		return fmt.Errorf("get job output vars error: %v", err)
	}
	jsonOutput, err := json.Marshal(outputs)
	if err != nil {
		return err
	}

	if len(jsonOutput) > MaxContainerTerminationMessageLength {
		return fmt.Errorf("termination message is above max allowed size 4096, caused by large task result")
	}

	f, err := os.OpenFile(job.JobTerminationFile, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.Write(jsonOutput); err != nil {
		return err
	}
	return f.Sync()
}

func (j *Job) getJobOutputVars(ctx context.Context) ([]*job.JobOutput, error) {
	outputs := []*job.JobOutput{}
	for _, outputName := range j.Ctx.Outputs {
		fileContents, err := ioutil.ReadFile(filepath.Join(job.JobOutputDir, outputName))
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
