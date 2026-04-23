/*
Copyright 2026 The KodeRover Authors.

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

package workflownotify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhooknotify"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
	jobspec "github.com/koderover/zadig/v2/pkg/types/job"
	"github.com/koderover/zadig/v2/pkg/types/step"
)

type RenderJobTemplate func(tpl string, job *models.JobTask) (string, error)
type TestResultGetter func(jobName string) (string, error)
type SonarMetricsGetter func(jobSpec *models.JobTaskFreestyleSpec) (string, string, error)

type BuildJobContentsArgs struct {
	Task            *models.WorkflowTask
	Stages          []*models.StageTask
	WebHookType     setting.NotifyWebHookType
	RenderTemplate  RenderJobTemplate
	GetTestResult   TestResultGetter
	GetSonarMetrics SonarMetricsGetter
}

func BuildWorkflowJobContents(args *BuildJobContentsArgs) ([]string, []*webhooknotify.WorkflowNotifyStage, error) {
	if args == nil || args.Task == nil || len(args.Stages) == 0 {
		return nil, nil, nil
	}
	if args.RenderTemplate == nil {
		return nil, nil, fmt.Errorf("render template callback is required")
	}

	jobContents := make([]string, 0)
	workflowNotifyStages := make([]*webhooknotify.WorkflowNotifyStage, 0, len(args.Stages))

	for _, stage := range args.Stages {
		if stage == nil {
			continue
		}

		workflowNotifyStage := &webhooknotify.WorkflowNotifyStage{
			Name:      stage.Name,
			Status:    stage.Status,
			StartTime: stage.StartTime,
			EndTime:   stage.EndTime,
			Error:     stage.Error,
		}

		for _, job := range stage.Jobs {
			if job == nil {
				continue
			}

			workflowNotifyJob := &webhooknotify.WorkflowNotifyJobTask{
				Name:        job.Name,
				DisplayName: job.DisplayName,
				JobType:     job.JobType,
				Status:      job.Status,
				StartTime:   job.StartTime,
				EndTime:     job.EndTime,
				Error:       job.Error,
			}

			jobTplcontent := "{{if and (ne .WebHookType \"feishu\") (ne .WebHookType \"feishu_app\") (ne .WebHookType \"feishu_person\")}}\n\n{{end}}{{if eq .WebHookType \"dingding\"}}---\n\n##### {{end}}**{{jobType .Job.JobType }}**: {{.Job.DisplayName}}    **{{getText \"notificationTextStatus\"}}**: {{taskStatus .Job.Status }}  \n"
			mailJobTplcontent := "{{jobType .Job.JobType }}：{{.Job.DisplayName}}    {{getText \"notificationTextStatus\"}}：{{taskStatus .Job.Status }} \n"

			switch job.JobType {
			case string(config.JobZadigBuild), string(config.JobFreestyle):
				jobSpec := &models.JobTaskFreestyleSpec{}
				models.IToi(job.Spec, jobSpec)

				workflowNotifyJobTaskSpec := &webhooknotify.WorkflowNotifyJobTaskBuildSpec{}

				repos := []*types.Repository{}
				for _, stepTask := range jobSpec.Steps {
					if stepTask.StepType == config.StepGit {
						stepSpec := &step.StepGitSpec{}
						models.IToi(stepTask.Spec, stepSpec)
						repos = stepSpec.Repos
					}
				}

				branchTag, commitID, gitCommitURL := "", "", ""
				commitMsgs := []string{}
				var prInfoList []string
				var prInfo string
				for idx, buildRepo := range repos {
					workflowNotifyRepository := &webhooknotify.WorkflowNotifyRepository{
						Source:        buildRepo.Source,
						RepoOwner:     buildRepo.RepoOwner,
						RepoNamespace: buildRepo.RepoNamespace,
						RepoName:      buildRepo.RepoName,
						Branch:        buildRepo.Branch,
						Tag:           buildRepo.Tag,
						AuthorName:    buildRepo.AuthorName,
						CommitID:      buildRepo.CommitID,
						CommitMessage: buildRepo.CommitMessage,
					}
					if idx == 0 || buildRepo.IsPrimary {
						branchTag = buildRepo.Branch
						if buildRepo.Tag != "" {
							branchTag = buildRepo.Tag
						}
						if len(buildRepo.CommitID) > 8 {
							commitID = buildRepo.CommitID[0:8]
						}
						var prLinkBuilder func(baseURL, owner, repoName string, prID int) string
						switch buildRepo.Source {
						case types.ProviderGithub:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%s/%s/pull/%d", baseURL, owner, repoName, prID)
							}
						case types.ProviderGitee:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%s/%s/pulls/%d", baseURL, owner, repoName, prID)
							}
						case types.ProviderGitlab:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%s/%s/merge_requests/%d", baseURL, owner, repoName, prID)
							}
						case types.ProviderGerrit:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return fmt.Sprintf("%s/%d", baseURL, prID)
							}
						default:
							prLinkBuilder = func(baseURL, owner, repoName string, prID int) string {
								return ""
							}
						}
						prInfoList = []string{}
						sort.Ints(buildRepo.PRs)
						for _, id := range buildRepo.PRs {
							link := prLinkBuilder(buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, id)
							if link != "" {
								prInfoList = append(prInfoList, fmt.Sprintf("[#%d](%s)", id, link))
							}
						}
						commitMsg := strings.Trim(buildRepo.CommitMessage, "\n")
						commitMsgs = strings.Split(commitMsg, "\n")
						gitCommitURL = fmt.Sprintf("%s/%s/%s/commit/%s", buildRepo.Address, buildRepo.RepoOwner, buildRepo.RepoName, commitID)
						workflowNotifyRepository.CommitURL = gitCommitURL
					}

					workflowNotifyJobTaskSpec.Repositories = append(workflowNotifyJobTaskSpec.Repositories, workflowNotifyRepository)
				}
				if len(prInfoList) != 0 {
					prInfo = strings.Join(prInfoList, " ") + " "
				}

				image := ""
				imageContextKey := strings.Join(strings.Split(jobspec.GetJobOutputKey(job.Key, "IMAGE"), "."), "@?")
				if args.Task.GlobalContext != nil {
					image = args.Task.GlobalContext[imageContextKey]
				}
				if len(commitID) > 0 {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextRepositoryInfo\"}}**：%s %s[%s](%s)  ", branchTag, prInfo, commitID, gitCommitURL)
					jobTplcontent += "{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextCommitMessage\"}}**："
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextRepositoryInfo\"}}：%s %s[%s]( %s )  ", branchTag, prInfo, commitID, gitCommitURL)
					if len(commitMsgs) == 1 {
						jobTplcontent += fmt.Sprintf("%s \n", commitMsgs[0])
					} else {
						jobTplcontent += "\n"
						for _, commitMsg := range commitMsgs {
							jobTplcontent += fmt.Sprintf("%s \n", commitMsg)
						}
					}
				}
				if job.Status == config.StatusPassed && image != "" && !strings.HasPrefix(image, "{{.") && !strings.Contains(image, "}}") {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextImageInfo\"}}**：%s  \n", image)
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextImageInfo\"}}：%s \n", image)
					workflowNotifyJobTaskSpec.Image = image
				}

				workflowNotifyJob.Spec = workflowNotifyJobTaskSpec

			case string(config.JobZadigDeploy):
				jobSpec := &models.JobTaskDeploySpec{}
				models.IToi(job.Spec, jobSpec)
				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextEnvironment\"}}**：%s  \n", jobSpec.Env)
				mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextEnvironment\"}}：%s \n", jobSpec.Env)

				if job.Status == config.StatusPassed && len(jobSpec.ServiceAndImages) > 0 {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextImageInfo\"}}**：  \n")
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextImageInfo\"}}：  \n")
				}

				serviceModules := make([]*webhooknotify.WorkflowNotifyDeployServiceModule, 0, len(jobSpec.ServiceAndImages))
				for _, serviceAndImage := range jobSpec.ServiceAndImages {
					if job.Status == config.StatusPassed && !strings.HasPrefix(serviceAndImage.Image, "{{.") && !strings.Contains(serviceAndImage.Image, "}}") {
						jobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
						mailJobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
					}

					serviceModules = append(serviceModules, &webhooknotify.WorkflowNotifyDeployServiceModule{
						ServiceModule: serviceAndImage.ServiceModule,
						Image:         serviceAndImage.Image,
					})
				}

				workflowNotifyJob.Spec = &webhooknotify.WorkflowNotifyJobTaskDeploySpec{
					Env:            jobSpec.Env,
					ServiceName:    jobSpec.ServiceName,
					ServiceModules: serviceModules,
				}

			case string(config.JobZadigHelmDeploy):
				jobSpec := &models.JobTaskHelmDeploySpec{}
				models.IToi(job.Spec, jobSpec)
				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextEnvironment\"}}**：%s  \n", jobSpec.Env)
				mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextEnvironment\"}}：%s \n", jobSpec.Env)

				if job.Status == config.StatusPassed && len(jobSpec.ImageAndModules) > 0 {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextImageInfo\"}}**：  \n")
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextImageInfo\"}}：  \n")
				}

				serviceModules := make([]*webhooknotify.WorkflowNotifyDeployServiceModule, 0, len(jobSpec.ImageAndModules))
				for _, serviceAndImage := range jobSpec.ImageAndModules {
					if !strings.HasPrefix(serviceAndImage.Image, "{{.") && !strings.Contains(serviceAndImage.Image, "}}") {
						jobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
						mailJobTplcontent += fmt.Sprintf("%s  \n", serviceAndImage.Image)
					}

					serviceModules = append(serviceModules, &webhooknotify.WorkflowNotifyDeployServiceModule{
						ServiceModule: serviceAndImage.ServiceModule,
						Image:         serviceAndImage.Image,
					})
				}

				workflowNotifyJob.Spec = &webhooknotify.WorkflowNotifyJobTaskDeploySpec{
					Env:            jobSpec.Env,
					ServiceName:    jobSpec.ServiceName,
					ServiceModules: serviceModules,
				}

			case string(config.JobZadigTesting):
				if args.GetTestResult == nil {
					return nil, nil, fmt.Errorf("test result callback is required")
				}
				testResult, err := args.GetTestResult(job.Name)
				if err != nil {
					return nil, nil, err
				}
				jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextTestResult\"}}**: %s  \n", testResult)
				mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextTestResult\"}}: %s \n", testResult)

			case string(config.JobZadigScanning):
				if args.GetSonarMetrics == nil {
					return nil, nil, fmt.Errorf("sonar metrics callback is required")
				}
				jobSpec := &models.JobTaskFreestyleSpec{}
				models.IToi(job.Spec, jobSpec)
				sonarMetricsText, mailSonarMetricsText, err := args.GetSonarMetrics(jobSpec)
				if err != nil {
					return nil, nil, err
				}
				if sonarMetricsText != "" {
					jobTplcontent += fmt.Sprintf("{{if eq .WebHookType \"dingding\"}}##### {{end}}**{{getText \"notificationTextSonarMetrics\"}}**: %s  \n", sonarMetricsText)
					mailJobTplcontent += fmt.Sprintf("{{getText \"notificationTextSonarMetrics\"}}: %s \n", mailSonarMetricsText)
				}
			}

			tplToRender := jobTplcontent
			if args.WebHookType == setting.NotifyWebHookTypeMail {
				tplToRender = mailJobTplcontent
			}
			jobContent, err := args.RenderTemplate(tplToRender, job)
			if err != nil {
				return nil, nil, err
			}
			jobContents = append(jobContents, jobContent)
			workflowNotifyStage.Jobs = append(workflowNotifyStage.Jobs, workflowNotifyJob)
		}

		workflowNotifyStages = append(workflowNotifyStages, workflowNotifyStage)
	}

	return jobContents, workflowNotifyStages, nil
}
