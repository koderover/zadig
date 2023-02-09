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
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/types/job"
)

const (
	OutputNameRegexString = "^[a-zA-Z0-9_]{1,64}$"
)

var (
	OutputNameRegex = regexp.MustCompile(OutputNameRegexString)
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
	case config.JobZadigDistributeImage:
		resp = &ImageDistributeJob{job: job, workflow: workflow}
	case config.JobIstioRelease:
		resp = &IstioReleaseJob{job: job, workflow: workflow}
	case config.JobIstioRollback:
		resp = &IstioRollBackJob{job: job, workflow: workflow}
	case config.JobJira:
		resp = &JiraJob{job: job, workflow: workflow}
	case config.JobNacos:
		resp = &NacosJob{job: job, workflow: workflow}
	case config.JobApollo:
		resp = &ApolloJob{job: job, workflow: workflow}
	case config.JobMeegoTransition:
		resp = &MeegoTransitionJob{job: job, workflow: workflow}
	default:
		return resp, fmt.Errorf("job type not found %s", job.JobType)
	}
	return resp, nil
}

func Instantiate(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	ctl, err := InitJobCtl(job, workflow)
	if err != nil {
		return warpJobError(job.Name, err)
	}
	return ctl.Instantiate()
}

func SetPreset(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	jobCtl, err := InitJobCtl(job, workflow)
	if err != nil {
		return warpJobError(job.Name, err)
	}
	JobPresetSkiped(job)
	return jobCtl.SetPreset()
}

func JobPresetSkiped(job *commonmodels.Job) {
	if job.RunPolicy == config.ForceRun {
		job.Skipped = false
		return
	}
	if job.RunPolicy == config.DefaultNotRun {
		job.Skipped = true
		return
	}
	job.Skipped = false
}

func ToJobs(job *commonmodels.Job, workflow *commonmodels.WorkflowV4, taskID int64) ([]*commonmodels.JobTask, error) {
	jobCtl, err := InitJobCtl(job, workflow)
	if err != nil {
		return []*commonmodels.JobTask{}, warpJobError(job.Name, err)
	}
	return jobCtl.ToJobs(taskID)
}

func LintJob(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) error {
	jobCtl, err := InitJobCtl(job, workflow)
	if err != nil {
		return warpJobError(job.Name, err)
	}
	return jobCtl.LintJob()
}

func MergeWebhookRepo(workflow *commonmodels.WorkflowV4, repo *types.Repository) error {
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobZadigBuild {
				jobCtl := &BuildJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return warpJobError(job.Name, err)
				}
			}
			if job.JobType == config.JobFreestyle {
				jobCtl := &FreeStyleJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return warpJobError(job.Name, err)
				}
			}
			if job.JobType == config.JobZadigTesting {
				jobCtl := &TestingJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return warpJobError(job.Name, err)
				}
			}
			if job.JobType == config.JobZadigScanning {
				jobCtl := &ScanningJob{job: job, workflow: workflow}
				if err := jobCtl.MergeWebhookRepo(repo); err != nil {
					return warpJobError(job.Name, err)
				}
			}
		}
	}
	return nil
}

func GetWorkflowOutputs(workflow *commonmodels.WorkflowV4, currentJobName string, log *zap.SugaredLogger) []string {
	resp := []string{}
	jobRankMap := getJobRankMap(workflow.Stages)
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			// we only need to get the outputs from job runs before the current job
			if jobRankMap[job.Name] >= jobRankMap[currentJobName] {
				return resp
			}
			if job.JobType == config.JobZadigBuild {
				jobCtl := &BuildJob{job: job, workflow: workflow}
				resp = append(resp, jobCtl.GetOutPuts(log)...)
			}
			if job.JobType == config.JobFreestyle {
				jobCtl := &FreeStyleJob{job: job, workflow: workflow}
				resp = append(resp, jobCtl.GetOutPuts(log)...)
			}
			if job.JobType == config.JobZadigTesting {
				jobCtl := &TestingJob{job: job, workflow: workflow}
				resp = append(resp, jobCtl.GetOutPuts(log)...)
			}
			if job.JobType == config.JobZadigScanning {
				jobCtl := &ScanningJob{job: job, workflow: workflow}
				resp = append(resp, jobCtl.GetOutPuts(log)...)
			}
			if job.JobType == config.JobZadigDistributeImage {
				jobCtl := &ImageDistributeJob{job: job, workflow: workflow}
				resp = append(resp, jobCtl.GetOutPuts(log)...)
			}
			if job.JobType == config.JobPlugin {
				jobCtl := &PluginJob{job: job, workflow: workflow}
				resp = append(resp, jobCtl.GetOutPuts(log)...)
			}
		}
	}
	return resp
}

func GetRepos(workflow *commonmodels.WorkflowV4) ([]*types.Repository, error) {
	resp := []*types.Repository{}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == config.JobZadigBuild {
				jobCtl := &BuildJob{job: job, workflow: workflow}
				buildRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, warpJobError(job.Name, err)
				}
				resp = append(resp, buildRepos...)
			}
			if job.JobType == config.JobFreestyle {
				jobCtl := &FreeStyleJob{job: job, workflow: workflow}
				freeStyleRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, warpJobError(job.Name, err)
				}
				resp = append(resp, freeStyleRepos...)
			}
			if job.JobType == config.JobZadigTesting {
				jobCtl := &TestingJob{job: job, workflow: workflow}
				testingRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, warpJobError(job.Name, err)
				}
				resp = append(resp, testingRepos...)
			}
			if job.JobType == config.JobZadigScanning {
				jobCtl := &ScanningJob{job: job, workflow: workflow}
				scanningRepos, err := jobCtl.GetRepos()
				if err != nil {
					return resp, warpJobError(job.Name, err)
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
			if err := SetPreset(job, workflow); err != nil {
				return warpJobError(job.Name, err)
			}
			jobKey := strings.Join([]string{job.Name, string(job.JobType)}, "-")
			if jobArgs, ok := argsMap[jobKey]; ok {
				job.Skipped = JobSkiped(jobArgs)
				jobCtl, err := InitJobCtl(job, workflow)
				if err != nil {
					return warpJobError(job.Name, err)
				}
				if err := jobCtl.MergeArgs(jobArgs); err != nil {
					return warpJobError(job.Name, err)
				}
			}
		}
	}
	return nil
}

func JobSkiped(job *commonmodels.Job) bool {
	if job.RunPolicy == config.ForceRun {
		return false
	}
	return job.Skipped
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
		repoName = strings.Replace(repo.RepoName, ".", "_", -1)

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
		ret = append(ret, getEnvFromCommitMsg(repo.CommitMessage)...)
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

func getOutputKey(jobKey string, outputs []*commonmodels.Output) []string {
	resp := []string{}
	for _, output := range outputs {
		resp = append(resp, job.GetJobOutputKey(jobKey, output.Name))
	}
	return resp
}

// generate script to save outputs variable to file
func outputScript(outputs []*commonmodels.Output) []string {
	resp := []string{"set +ex"}
	for _, output := range outputs {
		resp = append(resp, fmt.Sprintf("echo $%s > %s", output.Name, path.Join(job.JobOutputDir, output.Name)))
	}
	return resp
}

func checkOutputNames(outputs []*commonmodels.Output) error {
	for _, output := range outputs {
		if match := OutputNameRegex.MatchString(output.Name); !match {
			return fmt.Errorf("output name must match %s", OutputNameRegexString)
		}
	}
	return nil
}

func getShareStorageDetail(shareStorages []*commonmodels.ShareStorage, shareStorageInfo *commonmodels.ShareStorageInfo, workflowName string, taskID int64) []*commonmodels.StorageDetail {
	resp := []*commonmodels.StorageDetail{}
	if shareStorageInfo == nil {
		return resp
	}
	if !shareStorageInfo.Enabled {
		return resp
	}
	if len(shareStorages) == 0 || len(shareStorageInfo.ShareStorages) == 0 {
		return resp
	}
	storageMap := make(map[string]*commonmodels.ShareStorage, len(shareStorages))
	for _, shareStorage := range shareStorages {
		storageMap[shareStorage.Name] = shareStorage
	}
	for _, storageInfo := range shareStorageInfo.ShareStorages {
		storage, ok := storageMap[storageInfo.Name]
		if !ok {
			continue
		}
		storageDetail := &commonmodels.StorageDetail{
			Name:      storageInfo.Name,
			Type:      types.NFSMedium,
			SubPath:   types.GetShareStorageSubPath(workflowName, storageInfo.Name, taskID),
			MountPath: storage.Path,
		}
		resp = append(resp, storageDetail)
	}
	return resp
}

func getEnvFromCommitMsg(commitMsg string) []*commonmodels.KeyVal {
	resp := []*commonmodels.KeyVal{}
	if commitMsg == "" {
		return resp
	}
	compileRegex := regexp.MustCompile(`(?U)#(\w+=.+)#`)
	kvArrs := compileRegex.FindAllStringSubmatch(commitMsg, -1)
	for _, kvArr := range kvArrs {
		if len(kvArr) == 0 {
			continue
		}
		keyValStr := kvArr[len(kvArr)-1]
		keyValArr := strings.Split(keyValStr, "=")
		if len(keyValArr) == 2 {
			resp = append(resp, &commonmodels.KeyVal{Key: keyValArr[0], Value: keyValArr[1], Type: commonmodels.StringType})
		}
	}
	return resp
}

func warpJobError(jobName string, err error) error {
	return fmt.Errorf("[job: %s] %v", jobName, err)
}
