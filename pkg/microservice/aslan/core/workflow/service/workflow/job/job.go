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
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
)

type JobCtl interface {
	Instantiate() error
	SetPreset() error
	ToJobs(taskID int64) ([]*commonmodels.JobTask, error)
	MergeArgs(args *commonmodels.Job) error
	LintJob() error
}

func InitJobCtl(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (JobCtl, error) {
	var resp JobCtl
	switch job.JobType {
	case config.JobZadigBuild:
		resp = &BuildJob{job: job, workflow: workflow}
	case config.JobZadigDeploy:
		resp = &DeployJob{job: job, workflow: workflow}
	case config.JobPlugin:
		resp = &PluginJob{job: job, workflow: workflow}
	case config.JobFreestyle:
		resp = &FreeStyleJob{job: job, workflow: workflow}
	case config.JobCustomDeploy:
		resp = &CustomDeployJob{job: job, workflow: workflow}
	case config.JobK8sBlueGreenDeploy:
		resp = &BlueGreenDeployJob{job: job, workflow: workflow}
	case config.JobK8sBlueGreenRelease:
		resp = &BlueGreenReleaseJob{job: job, workflow: workflow}
	case config.JobK8sCanaryDeploy:
		resp = &CanaryDeployJob{job: job, workflow: workflow}
	case config.JobK8sCanaryRelease:
		resp = &CanaryReleaseJob{job: job, workflow: workflow}
	case config.JobZadigTesting:
		resp = &TestingJob{job: job, workflow: workflow}
	case config.JobK8sGrayRelease:
		resp = &GrayReleaseJob{job: job, workflow: workflow}
	case config.JobK8sGrayRollback:
		resp = &GrayRollbackJob{job: job, workflow: workflow}
	case config.JobK8sPatch:
		resp = &K8sPacthJob{job: job, workflow: workflow}
	case config.JobZadigScanning:
		resp = &ScanningJob{job: job, workflow: workflow}
	default:
		return resp, fmt.Errorf("job type not found %s", job.JobType)
	}
	return resp, nil
}

func Instantiate(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	ctl, err := InitJobCtl(job, workflow)
	if err != nil {
		return err
	}
	return ctl.Instantiate()
}

func SetPreset(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	jobCtl, err := InitJobCtl(job, workflow)
	if err != nil {
		return err
	}
	return jobCtl.SetPreset()
}

func ToJobs(job *commonmodels.Job, workflow *commonmodels.WorkflowV4, taskID int64) ([]*commonmodels.JobTask, error) {
	jobCtl, err := InitJobCtl(job, workflow)
	if err != nil {
		return []*commonmodels.JobTask{}, err
	}
	return jobCtl.ToJobs(taskID)
}

func LintJob(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	jobCtl, err := InitJobCtl(job, workflow)
	if err != nil {
		return err
	}
	return jobCtl.LintJob()
}

func MergeWebhookRepo(workflow *commonmodels.WorkflowV4, repo *types.Repository) error {
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobZadigBuild {
				jobCtl := &BuildJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return err
				}
			}
			if job.JobType == config.JobFreestyle {
				jobCtl := &FreeStyleJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return err
				}
			}
			if job.JobType == config.JobZadigTesting {
				jobCtl := &TestingJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return err
				}
			}
			if job.JobType == config.JobZadigScanning {
				jobCtl := &ScanningJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func GetRepos(workflow *commonmodels.WorkflowV4) ([]*types.Repository, error) {
	resp := []*types.Repository{}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobZadigBuild {
				jobCtl := &BuildJob{job: job, workflow: workflow}
				buildRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, err
				}
				resp = append(resp, buildRepos...)
			}
			if job.JobType == config.JobFreestyle {
				jobCtl := &FreeStyleJob{job: job, workflow: workflow}
				freeStyleRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, err
				}
				resp = append(resp, freeStyleRepos...)
			}
			if job.JobType == config.JobZadigTesting {
				jobCtl := &TestingJob{job: job, workflow: workflow}
				testingRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, err
				}
				resp = append(resp, testingRepos...)
			}
			if job.JobType == config.JobZadigScanning {
				jobCtl := &ScanningJob{job: job, workflow: workflow}
				scanningRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, err
				}
				resp = append(resp, scanningRepos...)
			}
		}
	}
	return resp, nil
}

func MergeArgs(workflow, workflowArgs *commonmodels.WorkflowV4) error {
	argsMap := make(map[string]*commonmodels.Job)
	if workflowArgs != nil {
		for _, stage := range workflowArgs.Stages {
			for _, job := range stage.Jobs {
				jobKey := strings.Join([]string{job.Name, string(job.JobType)}, "-")
				argsMap[jobKey] = job
			}
		}
		workflow.Params = renderParams(workflowArgs.Params, workflow.Params)
	}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			jobCtl, err := InitJobCtl(job, workflow)
			if err != nil {
				return err
			}
			if err := jobCtl.SetPreset(); err != nil {
				return err
			}
			jobKey := strings.Join([]string{job.Name, string(job.JobType)}, "-")
			if jobArgs, ok := argsMap[jobKey]; ok {
				job.Skipped = jobArgs.Skipped
				jobCtl, err := InitJobCtl(job, workflow)
				if err != nil {
					return err
				}
				if err := jobCtl.MergeArgs(jobArgs); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// use service name and service module hash to generate job name
func jobNameFormat(jobName string) string {
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}
	jobName = strings.Trim(jobName, "-")
	jobName = strings.ToLower(jobName)
	return jobName
}

func getReposVariables(repos []*types.Repository) []*commonmodels.KeyVal {
	ret := make([]*commonmodels.KeyVal, 0)
	for index, repo := range repos {

		repoNameIndex := fmt.Sprintf("REPONAME_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf(repoNameIndex), Value: repo.RepoName, IsCredential: false})

		repoName := strings.Replace(repo.RepoName, "-", "_", -1)

		repoIndex := fmt.Sprintf("REPO_%d", index)
		ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf(repoIndex), Value: repoName, IsCredential: false})

		if len(repo.Branch) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_BRANCH", repoName), Value: repo.Branch, IsCredential: false})
		}

		if len(repo.Tag) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_TAG", repoName), Value: repo.Tag, IsCredential: false})
		}

		if repo.PR > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_PR", repoName), Value: strconv.Itoa(repo.PR), IsCredential: false})
		}

		if len(repo.PRs) > 0 {
			prStrs := []string{}
			for _, pr := range repo.PRs {
				prStrs = append(prStrs, strconv.Itoa(pr))
			}
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_PR", repoName), Value: strings.Join(prStrs, ","), IsCredential: false})
		}

		if len(repo.CommitID) > 0 {
			ret = append(ret, &commonmodels.KeyVal{Key: fmt.Sprintf("%s_COMMIT_ID", repoName), Value: repo.CommitID, IsCredential: false})
		}
	}
	return ret
}

// before workflowflow task was created, we need to remove the fixed mark from variables.
func RemoveFixedValueMarks(workflow *commonmodels.WorkflowV4) error {
	bf := bytes.NewBuffer([]byte{})
	jsonEncoder := json.NewEncoder(bf)
	jsonEncoder.SetEscapeHTML(false)
	jsonEncoder.Encode(workflow)
	replacedString := strings.ReplaceAll(bf.String(), setting.FixedValueMark, "")
	return json.Unmarshal([]byte(replacedString), &workflow)
}

func RenderGlobalVariables(workflow *commonmodels.WorkflowV4, taskID int64, creator string) error {
	b, err := json.Marshal(workflow)
	if err != nil {
		return fmt.Errorf("marshal workflow error: %v", err)
	}
	replacedString := renderMultiLineString(string(b), setting.RenderValueTemplate, getWorkflowDefaultParams(workflow, taskID, creator))
	return json.Unmarshal([]byte(replacedString), &workflow)
}

func renderString(value, template string, inputs []*commonmodels.Param) string {
	for _, input := range inputs {
		value = strings.ReplaceAll(value, fmt.Sprintf(template, input.Name), input.Value)
	}
	return value
}

func renderMultiLineString(value, template string, inputs []*commonmodels.Param) string {
	for _, input := range inputs {
		inputValue := strings.ReplaceAll(input.Value, "\n", "\\n")
		value = strings.ReplaceAll(value, fmt.Sprintf(template, input.Name), inputValue)
	}
	return value
}

func getWorkflowDefaultParams(workflow *commonmodels.WorkflowV4, taskID int64, creator string) []*commonmodels.Param {
	resp := []*commonmodels.Param{}
	resp = append(resp, &commonmodels.Param{Name: "project", Value: workflow.Project, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.name", Value: workflow.Name, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.id", Value: fmt.Sprintf("%d", taskID), ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.creator", Value: creator, ParamsType: "string", IsCredential: false})
	resp = append(resp, &commonmodels.Param{Name: "workflow.task.timestamp", Value: fmt.Sprintf("%d", time.Now().Unix()), ParamsType: "string", IsCredential: false})
	for _, param := range workflow.Params {
		paramsKey := strings.Join([]string{"workflow", "params", param.Name}, ".")
		resp = append(resp, &commonmodels.Param{Name: paramsKey, Value: param.Value, ParamsType: "string", IsCredential: false})
	}
	return resp
}

func renderParams(input, origin []*commonmodels.Param) []*commonmodels.Param {
	for i, originParam := range origin {
		for _, inputParam := range input {
			if originParam.Name == inputParam.Name {
				// always use origin credential config.
				isCredential := originParam.IsCredential
				origin[i] = inputParam
				origin[i].IsCredential = isCredential
			}
		}
	}
	return origin
}

func getJobRankMap(stages []*commonmodels.WorkflowStage) map[string]int {
	resp := make(map[string]int, 0)
	index := 0
	for _, stage := range stages {
		for _, job := range stage.Jobs {
			if !stage.Parallel {
				index++
			}
			resp[job.Name] = index
		}
		index++
	}
	return resp
}
