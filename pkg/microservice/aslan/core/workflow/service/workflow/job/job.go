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
	"fmt"
	"regexp"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/util"
	"github.com/pkg/errors"
)

const (
	VMOutputNameRegexString = "^[a-zA-Z0-9_]{1,64}$"
	OutputNameRegexString   = "^[a-zA-Z0-9_]{1,64}$"
	JobNameKey              = "job_name"
)

var (
	OutputNameRegex = regexp.MustCompile(OutputNameRegexString)
)

type JobCtl interface {
	Instantiate() error
	// SetPreset sets all the default values configured by user
	SetPreset() error
	// SetOptions sets all the possible options for the workflow
	SetOptions(ticket *commonmodels.ApprovalTicket) error
	// ClearOptions clears the option field to save space for db and memory
	ClearOptions() error
	ClearSelectionField() error
	ToJobs(taskID int64) ([]*commonmodels.JobTask, error)
	// MergeArgs merge the current workflow with the user input: args
	MergeArgs(args *commonmodels.Job) error
	// UpdateWithLatestSetting update the current workflow arguments with the latest workflow settings.
	// it will also calculate if the user's args is still valid, returning error if it is invalid.
	UpdateWithLatestSetting() error
	LintJob() error
}

func InitJobCtl(job *commonmodels.Job, workflow *commonmodels.WorkflowV4) (JobCtl, error) {
	var resp JobCtl
	switch job.JobType {
	case config.JobZadigBuild:
		resp = &BuildJob{job: job, workflow: workflow}
	case config.JobZadigDeploy:
		resp = &DeployJob{job: job, workflow: workflow}
	case config.JobZadigHelmChartDeploy:
		resp = &HelmChartDeployJob{job: job, workflow: workflow}
	case config.JobZadigVMDeploy:
		resp = &VMDeployJob{job: job, workflow: workflow}
	case config.JobPlugin:
		resp = &PluginJob{job: job, workflow: workflow}
	case config.JobFreestyle:
		resp = &FreeStyleJob{job: job, workflow: workflow}
	case config.JobCustomDeploy:
		resp = &CustomDeployJob{job: job, workflow: workflow}
	case config.JobK8sBlueGreenDeploy:
		resp = &BlueGreenDeployV2Job{job: job, workflow: workflow}
	case config.JobK8sBlueGreenRelease:
		resp = &BlueGreenReleaseV2Job{job: job, workflow: workflow}
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
	case config.JobWorkflowTrigger:
		resp = &WorkflowTriggerJob{job: job, workflow: workflow}
	case config.JobOfflineService:
		resp = &OfflineServiceJob{job: job, workflow: workflow}
	case config.JobMseGrayRelease:
		resp = &MseGrayReleaseJob{job: job, workflow: workflow}
	case config.JobMseGrayOffline:
		resp = &MseGrayOfflineJob{job: job, workflow: workflow}
	case config.JobGuanceyunCheck:
		resp = &GuanceyunCheckJob{job: job, workflow: workflow}
	case config.JobGrafana:
		resp = &GrafanaJob{job: job, workflow: workflow}
	case config.JobJenkins:
		resp = &JenkinsJob{job: job, workflow: workflow}
	case config.JobSQL:
		resp = &SQLJob{job: job, workflow: workflow}
	case config.JobUpdateEnvIstioConfig:
		resp = &UpdateEnvIstioConfigJob{job: job, workflow: workflow}
	case config.JobBlueKing:
		resp = &BlueKingJob{job: job, workflow: workflow}
	case config.JobApproval:
		resp = &ApprovalJob{job: job, workflow: workflow}
	case config.JobNotification:
		resp = &NotificationJob{job: job, workflow: workflow}
	case config.JobSAEDeploy:
		resp = &SAEDeployJob{job: job, workflow: workflow}
	default:
		return resp, fmt.Errorf("job type not found %s", job.JobType)
	}
	return resp, nil
}

func renderString(value, template string, inputs []*commonmodels.Param) string {
	for _, input := range inputs {
		if input.ParamsType == string(commonmodels.MultiSelectType) {
			input.Value = strings.Join(input.ChoiceValue, ",")
		}
		value = strings.ReplaceAll(value, fmt.Sprintf(template, input.Name), input.Value)
	}
	return value
}

func getWorkflowStageParams(workflow *commonmodels.WorkflowV4) ([]*commonmodels.Param, error) {
	resp := []*commonmodels.Param{}
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case config.JobZadigBuild:
				build := new(commonmodels.ZadigBuildJobSpec)
				if err := commonmodels.IToi(job.Spec, build); err != nil {
					return nil, errors.Wrap(err, "Itoi")
				}
				var serviceAndModuleName, branchList, gitURLs []string
				for _, serviceAndBuild := range build.ServiceAndBuilds {
					serviceAndModuleName = append(serviceAndModuleName, serviceAndBuild.ServiceModule+"/"+serviceAndBuild.ServiceName)
					branch, commitID, gitURL := "", "", ""
					if len(serviceAndBuild.Repos) > 0 {
						branch = serviceAndBuild.Repos[0].Branch
						commitID = serviceAndBuild.Repos[0].CommitID
						if serviceAndBuild.Repos[0].AuthType == types.SSHAuthType {
							gitURL = fmt.Sprintf("%s:%s/%s", serviceAndBuild.Repos[0].Address, serviceAndBuild.Repos[0].RepoOwner, serviceAndBuild.Repos[0].RepoName)
						} else {
							gitURL = fmt.Sprintf("%s/%s/%s", serviceAndBuild.Repos[0].Address, serviceAndBuild.Repos[0].RepoOwner, serviceAndBuild.Repos[0].RepoName)
						}
					}
					branchList = append(branchList, branch)
					gitURLs = append(gitURLs, gitURL)
					resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.%s.%s.BRANCH",
						job.Name, serviceAndBuild.ServiceName, serviceAndBuild.ServiceModule),
						Value: branch, ParamsType: "string", IsCredential: false})
					resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.%s.%s.COMMITID",
						job.Name, serviceAndBuild.ServiceName, serviceAndBuild.ServiceModule),
						Value: commitID, ParamsType: "string", IsCredential: false})
					resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.%s.%s.GITURL",
						job.Name, serviceAndBuild.ServiceName, serviceAndBuild.ServiceModule),
						Value: gitURL, ParamsType: "string", IsCredential: false})
				}
				resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.SERVICES", job.Name), Value: strings.Join(serviceAndModuleName, ","), ParamsType: "string", IsCredential: false})
				resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.BRANCHES", job.Name), Value: strings.Join(branchList, ","), ParamsType: "string", IsCredential: false})
				resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.GITURLS", job.Name), Value: strings.Join(gitURLs, ","), ParamsType: "string", IsCredential: false})
			case config.JobZadigDeploy:
				deploy := new(commonmodels.ZadigDeployJobSpec)
				if err := commonmodels.IToi(job.Spec, deploy); err != nil {
					return nil, errors.Wrap(err, "Itoi")
				}
				resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.envName", job.Name), Value: deploy.Env, ParamsType: "string", IsCredential: false})

				services := []string{}
				for _, service := range deploy.Services {
					for _, module := range service.Modules {
						services = append(services, module.ServiceModule+"/"+service.ServiceName)
					}
				}
				resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.SERVICES", job.Name), Value: strings.Join(services, ","), ParamsType: "string", IsCredential: false})

				images := []string{}
				for _, service := range deploy.Services {
					for _, module := range service.Modules {
						images = append(images, module.Image)
					}
				}
				resp = append(resp, &commonmodels.Param{Name: fmt.Sprintf("job.%s.IMAGES", job.Name), Value: strings.Join(images, ","), ParamsType: "string", IsCredential: false})
			}
		}
	}
	return resp, nil
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

func checkOutputNames(outputs []*commonmodels.Output) error {
	for _, output := range outputs {
		if match := OutputNameRegex.MatchString(output.Name); !match {
			return fmt.Errorf("output name must match %s", OutputNameRegexString)
		}
	}
	return nil
}

func warpJobError(jobName string, err error) error {
	return fmt.Errorf("[job: %s] %v", jobName, err)
}

func findMatchedRepoFromParams(params []*commonmodels.Param, paramName string) (*types.Repository, error) {
	for _, param := range params {
		if param.Name == paramName {
			if param.ParamsType != "repo" {
				continue
			}
			return param.Repo, nil
		}
	}
	return nil, fmt.Errorf("not found repo from params")
}

func renderServiceVariables(workflow *commonmodels.WorkflowV4, envs []*commonmodels.KeyVal, serviceName string, serviceModule string) ([]*commonmodels.KeyVal, error) {
	duplicatedEnvs := make([]*commonmodels.KeyVal, 0)

	err := util.DeepCopy(&duplicatedEnvs, &envs)
	if err != nil {
		return nil, err
	}

	if serviceName == "" || serviceModule == "" {
		return duplicatedEnvs, nil
	}

	params, err := getWorkflowStageParams(workflow)
	if err != nil {
		err = fmt.Errorf("failed to get workflow stage parameters, error: %s", err)
		return nil, err
	}

	for _, env := range duplicatedEnvs {
		if strings.HasPrefix(env.Value, "{{.") && strings.HasSuffix(env.Value, "}}") {
			env.Value = strings.ReplaceAll(env.Value, "<SERVICE>", serviceName)
			env.Value = strings.ReplaceAll(env.Value, "<MODULE>", serviceModule)
			env.Value = renderString(env.Value, setting.RenderValueTemplate, params)
		}
	}
	return duplicatedEnvs, nil
}
