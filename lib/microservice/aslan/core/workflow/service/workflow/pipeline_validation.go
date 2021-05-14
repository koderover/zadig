/*
Copyright 2021 The KodeRover Authors.

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

package workflow

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v35/github"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/qiniu/x/log.v7"

	"github.com/koderover/zadig/lib/internal/kube/wrapper"
	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models/task"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	commonservice "github.com/koderover/zadig/lib/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	git "github.com/koderover/zadig/lib/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/s3"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/kube/getter"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/types"
	"github.com/koderover/zadig/lib/util"
)

const (
	NameSpaceRegexString   = "[^a-z0-9.-]"
	defaultNameRegexString = "^[a-zA-Z0-9-_]{1,50}$"

	// Zadig 系统提供渲染键值
	namespaceRegexString = `\$Namespace\$`
	envRegexString       = `\$EnvName\$`
	productRegexString   = `\$Product\$`
	serviceRegexString   = `\$Service\$`

	GithubAPIServer = "https://api.github.com/"
)

var (
	NameSpaceRegex   = regexp.MustCompile(NameSpaceRegexString)
	defaultNameRegex = regexp.MustCompile(defaultNameRegexString)

	// Zadig 系统提供渲染键值
	namespaceRegex = regexp.MustCompile(namespaceRegexString)
	productRegex   = regexp.MustCompile(productRegexString)
	envNameRegex   = regexp.MustCompile(envRegexString)
	serviceRegex   = regexp.MustCompile(serviceRegexString)
)

func ensurePipeline(args *commonmodels.Pipeline, log *xlog.Logger) error {
	if !defaultNameRegex.MatchString(args.Name) {
		log.Errorf("pipeline name must match %s", defaultNameRegexString)
		return fmt.Errorf("%s %s", e.InvalidFormatErrMsg, defaultNameRegexString)
	}

	for _, schedule := range args.Schedules.Items {
		if err := schedule.Validate(); err != nil {
			return err
		}
	}

	if err := validateSubTaskSetting(args.Name, args.SubTasks); err != nil {
		log.Errorf("validateSubTaskSetting: %+v", err)
		return err
	}
	return nil
}

// validateSubTaskSetting Validating subtasks
func validateSubTaskSetting(pipeName string, subtasks []map[string]interface{}) error {
	for i, subTask := range subtasks {
		pre, err := commonservice.ToPreview(subTask)
		if err != nil {
			return errors.New(e.InterfaceToTaskErrMsg)
		}

		if !pre.Enabled {
			continue
		}

		switch pre.TaskType {

		case config.TaskBuild:
			t, err := commonservice.ToBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			// 设置默认 build OS 为 Ubuntu 16.04
			if t.BuildOS == "" {
				t.BuildOS = setting.UbuntuXenial
				t.ImageFrom = commonmodels.ImageFromKoderover
			}

			ensureTaskSecretEnvs(pipeName, config.TaskBuild, t.JobCtx.EnvVars)

			subtasks[i], err = t.ToSubTask()
			if err != nil {
				return err
			}

		case config.TaskTestingV2:
			t, err := commonservice.ToTestingTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if !defaultNameRegex.MatchString(t.TestName) {
				log.Errorf("test name must match %s", defaultNameRegexString)
				return fmt.Errorf("%s %s", e.InvalidFormatErrMsg, defaultNameRegexString)
			}
			// 设置模板 build os
			if t.BuildOS == "" {
				t.BuildOS = setting.UbuntuXenial
				t.ImageFrom = commonmodels.ImageFromKoderover
			}

			ensureTaskSecretEnvs(pipeName, config.TaskTestingV2, t.JobCtx.EnvVars)

			// 设置 build 安装脚本
			t.InstallCtx, err = buildInstallCtx(t.InstallItems)
			if err != nil {
				log.Errorf("buildInstallCtx error: %v", err)
				return err
			}

			subtasks[i], err = t.ToSubTask()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureTaskSecretEnvs(pipelineName string, taskType config.TaskType, kvs []*commonmodels.KeyVal) {
	envsMap := getTaskEnvs(pipelineName)
	for _, kv := range kvs {
		// 如果用户的value已经给mask了，认为不需要修改
		if kv.Value == setting.MaskValue {
			kv.Value = envsMap[fmt.Sprintf("%s/%s", taskType, kv.Key)]
		}
	}
}

// getTaskEnvs 获取subtask中的环境变量信息
// 格式为: {Task Type}/{Key name}
func getTaskEnvs(pipelineName string) map[string]string {
	resp := make(map[string]string, 0)
	pipe, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: pipelineName})
	if err != nil {
		log.Error(fmt.Sprintf("find pipeline %s ERR: ", pipelineName), err)
		return resp
	}

	for _, subTask := range pipe.SubTasks {

		pre, err := commonservice.ToPreview(subTask)
		if err != nil {
			log.Error(err)
			return resp
		}

		switch pre.TaskType {

		case config.TaskBuild:
			t, err := commonservice.ToBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return resp
			}

			for _, v := range t.JobCtx.EnvVars {
				key := fmt.Sprintf("%s/%s", config.TaskBuild, v.Key)
				resp[key] = v.Value
			}

		case config.TaskTestingV2:
			t, err := commonservice.ToTestingTask(subTask)
			if err != nil {
				log.Error(err)
				return resp
			}
			for _, v := range t.JobCtx.EnvVars {
				key := fmt.Sprintf("%s/%s", config.TaskTestingV2, v.Key)
				resp[key] = v.Value
			}
		}
	}
	return resp
}

//根据用户的配置和BuildStep中步骤的依赖，从系统配置的InstallItems中获取配置项，构建Install Context
func buildInstallCtx(installItems []*commonmodels.Item) ([]*commonmodels.Install, error) {
	resp := make([]*commonmodels.Install, 0)

	for _, item := range installItems {
		if item.Name == "" || item.Version == "" {
			// 防止前端传入空的 install item
			continue
		}

		install, err := commonrepo.NewInstallColl().Find(item.Name, item.Version)
		if err != nil {
			return resp, fmt.Errorf("%s:%s not found", item.Name, item.Version)
		}
		if !install.Enabled {
			return resp, fmt.Errorf("%s:%s disabled", item.Name, item.Version)
		}

		resp = append(resp, install)
	}

	return resp, nil
}

func fmtBuildsTask(build *task.Build, log *xlog.Logger) {
	//detail,err := s.Codehost.Detail.GetCodehostDetail(build.JobCtx.Builds)
	FmtBuilds(build.JobCtx.Builds, log)
}

//replace gitInfo with codehostID
func FmtBuilds(builds []*types.Repository, log *xlog.Logger) {
	//detail,err := s.Codehost.Detail.GetCodehostDetail(build.JobCtx.Builds)
	for _, repo := range builds {
		cId := repo.CodehostID
		if cId == 0 {
			log.Error("codehostID can't be empty")
			return
		}
		opt := &codehost.CodeHostOption{
			CodeHostID: cId,
		}
		detail, err := codehost.GetCodeHostInfo(opt)
		if err != nil {
			log.Error(err)
			return
		}
		repo.Source = detail.Type
		repo.OauthToken = detail.AccessToken
		repo.Address = detail.Address
	}
}

// 外部触发任务设置build参数
func setTriggerBuilds(builds []*types.Repository, buildArgs []*types.Repository) error {
	for _, buildArg := range buildArgs {
		// 同名repo修改主repo(没有主repo第一个同名为主)
		applyBuildArgFlag := false
		buildIndex := -1
		for index, build := range builds {
			if buildArg.RepoOwner == build.RepoOwner && buildArg.RepoName == build.RepoName && (!applyBuildArgFlag || build.IsPrimary) {
				applyBuildArgFlag = true
				buildIndex = index
			}
		}
		if buildIndex >= 0 {
			setBuildFromArg(builds[buildIndex], buildArg)
		}
	}

	// 并发获取commit信息
	var wg sync.WaitGroup
	for _, build := range builds {
		wg.Add(1)
		go func(build *types.Repository) {
			defer func() {
				wg.Done()
			}()

			setBuildInfo(build)
		}(build)
	}
	wg.Wait()
	return nil
}

func setBuildInfo(build *types.Repository) {
	opt := &codehost.CodeHostOption{
		CodeHostID: build.CodehostID,
	}
	gitInfo, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Errorf("failed to get codehost detail %d %v", build.CodehostID, err)
		return
	}

	//gitInfo, err := s.Codehost.Detail.GetCodehostDetail(build.CodehostID)
	//if err != nil {
	//	log.Errorf("failed to get codehost detail %d %v", build.CodehostID, err)
	//	return
	//}

	if gitInfo.Type == codehost.GitLabProvider || gitInfo.Type == codehost.GerritProvider {
		if build.CommitID == "" {
			var commit *RepoCommit
			var pr *PRCommit
			var err error
			if build.Tag != "" {
				commit, err = QueryByTag(build.CodehostID, build.RepoOwner, build.RepoName, build.Tag)
			} else if build.Branch != "" && build.PR == 0 {
				commit, err = QueryByBranch(build.CodehostID, build.RepoOwner, build.RepoName, build.Branch)
			} else if build.Branch != "" && build.PR > 0 {
				pr, err = GetLatestPrCommit(build.CodehostID, build.PR, build.RepoOwner, build.RepoName)
				if err == nil && pr != nil {
					commit = &RepoCommit{
						ID:         pr.ID,
						Message:    pr.Title,
						AuthorName: pr.AuthorName,
					}
					build.CheckoutRef = pr.CheckoutRef
				} else {
					log.Warnf("pr setBuildInfo failed, use build:%v err:%v", build, err)
					return
				}
			} else {
				return
			}

			if err != nil {
				log.Warnf("setBuildInfo failed, use build %+v %s", build, err)
				return
			}

			build.CommitID = commit.ID
			build.CommitMessage = commit.Message
			build.AuthorName = commit.AuthorName
		}
	} else {
		gitCli := git.NewGithubAppClient(gitInfo.AccessToken, GithubAPIServer, config.ProxyHTTPSAddr())
		//// 需要后端自动获取Branch当前Commit，并填写到build中
		if build.CommitID == "" {
			// 如果是仅填写Branch编译
			if build.Branch != "" && build.PR == 0 {
				branch, _, err := gitCli.Repositories.GetBranch(context.Background(), build.RepoOwner, build.RepoName, build.Branch)
				if err == nil {
					build.CommitID = *branch.Commit.SHA
					build.CommitMessage = *branch.Commit.Commit.Message
					build.AuthorName = *branch.Commit.Commit.Author.Name
				}
			} else if build.Tag != "" && build.PR == 0 {
				opt := &github.ListOptions{Page: 1, PerPage: 100}
				tags, _, err := gitCli.Repositories.ListTags(context.Background(), build.RepoOwner, build.RepoName, opt)
				if err == nil {
					for _, tag := range tags {
						if *tag.Name == build.Tag {
							build.CommitID = *tag.Commit.SHA
							build.CommitMessage = *tag.Commit.Message
							build.AuthorName = *tag.Commit.Author.Name
							return
						}
					}
				}
			} else if build.Branch != "" && build.PR > 0 {
				opt := &github.ListOptions{Page: 1, PerPage: 100}
				prCommits, _, err := gitCli.PullRequests.ListCommits(context.Background(), build.RepoOwner, build.RepoName, build.PR, opt)
				sort.SliceStable(prCommits, func(i, j int) bool {
					return prCommits[i].Commit.Committer.Date.Unix() > prCommits[j].Commit.Committer.Date.Unix()
				})
				if err == nil && len(prCommits) > 0 {
					for _, commit := range prCommits {
						build.CommitID = *commit.Commit.Tree.SHA
						build.CommitMessage = *commit.Commit.Message
						build.AuthorName = *commit.Commit.Author.Name
						return
					}
				}
			} else {
				log.Warn("github setBuildInfo failed, use build ", build)
				return
			}
			//if build.Branch != "" && build.PR > 0 {
			//	prCommits := make([]*github.RepositoryCommit, 0)
			//	opt := &github.ListOptions{Page: 1, PerPage: 100}
			//	for opt.Page > 0 {
			//		list, resp, err := gitCli.PullRequests.ListCommits(context.Background(), build.RepoOwner, build.RepoName, build.PR, opt)
			//		if err != nil {
			//			fmt.Printf("ListCommits error: %v\n", err)
			//			return
			//		}
			//		prCommits = append(prCommits, list...)
			//		opt.Page = resp.NextPage
			//	}
			//
			//	if len(prCommits) > 0 {
			//		commit := prCommits[len(prCommits)-1]
			//		build.CommitID = *commit.SHA
			//	}
			//}
		}

	}

}

// 根据传入的build arg设置build参数
func setBuildFromArg(build, buildArg *types.Repository) {
	// 单pr编译
	if buildArg.PR > 0 && len(buildArg.Branch) == 0 {
		build.PR = buildArg.PR
		build.Branch = ""
	}
	//pr rebase branch编译
	if buildArg.PR > 0 && len(buildArg.Branch) > 0 {
		build.PR = buildArg.PR
		build.Branch = buildArg.Branch
	}
	// 单branch编译
	if buildArg.PR == 0 && len(buildArg.Branch) > 0 {
		build.PR = 0
		build.Branch = buildArg.Branch
	}

	if buildArg.Tag != "" {
		build.Tag = buildArg.Tag
	}

	if buildArg.CommitID != "" {
		build.CommitID = buildArg.CommitID
	}

	if buildArg.CommitMessage != "" {
		build.CommitMessage = buildArg.CommitMessage
	}

	if buildArg.IsPrimary == true {
		build.IsPrimary = true
	}
}

// 外部触发任务设置build参数
func setManunalBuilds(builds []*types.Repository, buildArgs []*types.Repository) error {
	var wg sync.WaitGroup
	for _, build := range builds {
		// 根据用户提交的工作流任务参数，更新task中source code repo相关参数
		// 并发获取commit信息
		wg.Add(1)
		go func(build *types.Repository) {
			defer func() {
				wg.Done()
			}()
			for _, buildArg := range buildArgs {
				if buildArg.RepoOwner == build.RepoOwner && buildArg.RepoName == build.RepoName && buildArg.CheckoutPath == build.CheckoutPath {
					setBuildFromArg(build, buildArg)
					break
				}
			}
			setBuildInfo(build)
		}(build)
	}
	wg.Wait()
	return nil
}

func ensurePipelineTask(pt *task.Task, log *xlog.Logger) error {
	var (
		buildEnvs []*commonmodels.KeyVal
	)

	// 验证 Subtask payload
	err := validateSubTaskSetting(pt.PipelineName, pt.SubTasks)
	if err != nil {
		log.Errorf("Validate subtask setting failed: %+v", err)
		return err
	}

	//设置执行任务时参数
	for i, subTask := range pt.SubTasks {

		pre, err := commonservice.ToPreview(subTask)
		if err != nil {
			return errors.New(e.InterfaceToTaskErrMsg)
		}

		switch pre.TaskType {

		case config.TaskBuild:
			t, err := commonservice.ToBuildTask(subTask)
			fmtBuildsTask(t, log)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				//for _, arg := range pt.TaskArgs.BuildArgs {
				//	for _, k := range t.JobCtx.EnvVars {
				//		if arg.Key == k.Key {
				//			if !(arg.Value == env.MaskValue && arg.IsCredential) {
				//				k.Value = arg.Value
				//			}
				//
				//			break
				//		}
				//	}
				//}

				// 设置Pipeline对应的服务名称
				if t.ServiceName != "" {
					pt.ServiceName = t.ServiceName
				}

				// 设置 build 安装脚本
				t.InstallCtx, err = buildInstallCtx(t.InstallItems)
				if err != nil {
					log.Error(err)
					return err
				}

				// 外部触发的pipeline
				if pt.TaskCreator == setting.WebhookTaskCreator || pt.TaskCreator == setting.CronTaskCreator {
					setTriggerBuilds(t.JobCtx.Builds, pt.TaskArgs.Builds)
				} else {
					setManunalBuilds(t.JobCtx.Builds, pt.TaskArgs.Builds)
				}

				// 生成默认镜像tag后缀
				pt.TaskArgs.Deploy.Tag = releaseCandidateTag(t, pt.TaskID)

				// 设置镜像名称
				// 编译任务使用 t.JobCtx.Image
				// 注意: 其他任务从 pt.TaskArgs.Deploy.Image 获取, 必须要有编译任务
				reg, err := commonservice.FindDefaultRegistry(log)
				if err != nil {
					log.Errorf("can't find default candidate registry: %v", err)
					return e.ErrFindRegistry.AddDesc(err.Error())
				}

				t.JobCtx.Image = GetImage(reg, t.ServiceName, pt.TaskArgs.Deploy.Tag)
				pt.TaskArgs.Deploy.Image = t.JobCtx.Image

				if pt.ConfigPayload != nil {
					pt.ConfigPayload.Registry.Addr = reg.RegAddr
					pt.ConfigPayload.Registry.AccessKey = reg.AccessKey
					pt.ConfigPayload.Registry.SecretKey = reg.SecretyKey
				}

				// 二进制文件名称
				// 编译任务使用 t.JobCtx.PackageFile
				// 注意: 其他任务从 pt.TaskArgs.Deploy.PackageFile 获取, 必须要有编译任务
				t.JobCtx.PackageFile = GetPackageFile(t.ServiceName, pt.TaskArgs.Deploy.Tag)
				pt.TaskArgs.Deploy.PackageFile = t.JobCtx.PackageFile

				// 注入编译模块中用户定义环境变量
				// 注意: 需要在pt.TaskArgs.Deploy设置完之后再设置环境变量
				t.JobCtx.EnvVars = append(t.JobCtx.EnvVars, prepareTaskEnvs(pt, log)...)
				// 如果其他模块需要使用编译模块的环境变量进行渲染，需要设置buildEnvs
				buildEnvs = t.JobCtx.EnvVars

				if t.JobCtx.DockerBuildCtx != nil {
					t.JobCtx.DockerBuildCtx.ImageName = t.JobCtx.Image
				}

				if t.JobCtx.FileArchiveCtx != nil {
					//t.JobCtx.FileArchiveCtx.FileName = t.ServiceName
					t.JobCtx.FileArchiveCtx.FileName = GetPackageFile(t.ServiceName, pt.TaskArgs.Deploy.Tag)
				}

				// TODO: generic
				//if t.JobCtx.StorageUri == "" && pt.Type == pipe.SingleType {
				//	if store, err := s.GetDefaultS3Storage(log); err != nil {
				//		return e.ErrFindDefaultS3Storage.AddDesc("default s3 storage is required by package building")
				//	} else {
				//		if t.JobCtx.StorageUri, err = store.GetEncryptedUrl(types.S3STORAGE_AES_KEY); err != nil {
				//			log.Errorf("failed encrypt storage uri %v", err)
				//			return err
				//		}
				//	}
				//}

				t.DockerBuildStatus = &task.DockerBuildStatus{
					ImageName:    t.JobCtx.Image,
					RegistryRepo: reg.RegAddr + "/" + reg.Namespace,
				}
				t.UTStatus = &task.UTStatus{}
				t.StaticCheckStatus = &task.StaticCheckStatus{}
				t.BuildStatus = &task.BuildStatus{}

				pt.SubTasks[i], err = t.ToSubTask()

				if err != nil {
					return err
				}
			}
		case config.TaskJenkinsBuild:
			t, err := commonservice.ToJenkinsBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if t.Enabled {
				// 分析镜像名称
				image := ""
				//imageRegEx := regexp.MustCompile(setting.ImageRegexString)
			loop:
				for _, buildParam := range t.JenkinsBuildArgs.JenkinsBuildParams {
					switch buildParam.Value.(type) {
					case string:
						if strings.Contains(buildParam.Value.(string), ":") {
							image = buildParam.Value.(string)
							break loop
						}
					}
				}
				pt.TaskArgs.Deploy.Image = image
				t.Image = image
				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskArtifact:
			t, err := commonservice.ToArtifactTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if t.Enabled {
				if pt.TaskArgs == nil {
					pt.TaskArgs = &commonmodels.TaskArgs{PipelineName: pt.WorkflowArgs.WorkflowName, TaskCreator: pt.WorkflowArgs.WorklowTaskCreator}
				}
				registry := pt.ConfigPayload.RepoConfigs[pt.WorkflowArgs.RegistryID]
				t.Image = fmt.Sprintf("%s/%s/%s", util.GetURLHostName(registry.RegAddr), registry.Namespace, t.Image)
				pt.TaskArgs.Deploy.Image = t.Image
				t.RegistryID = pt.WorkflowArgs.RegistryID

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskDockerBuild:
			t, err := commonservice.ToDockerBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if t.Enabled {
				t.Image = pt.TaskArgs.Deploy.Image

				// 使用环境变量KEY渲染 docker build的各个参数
				if buildEnvs != nil {
					for _, env := range buildEnvs {
						if !env.IsCredential {
							t.DockerFile = strings.Replace(t.DockerFile, fmt.Sprintf("$%s", env.Key), env.Value, -1)
							t.WorkDir = strings.Replace(t.WorkDir, fmt.Sprintf("$%s", env.Key), env.Value, -1)
							t.BuildArgs = strings.Replace(t.BuildArgs, fmt.Sprintf("$%s", env.Key), env.Value, -1)
						}
					}
				}

				err = SetCandidateRegistry(pt.ConfigPayload, log)
				if err != nil {
					return err
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskTestingV2:
			t, err := commonservice.ToTestingTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				FmtBuilds(t.JobCtx.Builds, log)
				// 获取 pipeline keystores 信息
				envs := make([]*commonmodels.KeyVal, 0)

				// Iterate test jobctx builds, and replace it if params specified from task.
				// 外部触发的pipeline
				if pt.TaskCreator == setting.WebhookTaskCreator || pt.TaskCreator == setting.CronTaskCreator {
					setTriggerBuilds(t.JobCtx.Builds, pt.TaskArgs.Test.Builds)
				} else {
					setManunalBuilds(t.JobCtx.Builds, pt.TaskArgs.Test.Builds)
				}

				err = SetCandidateRegistry(pt.ConfigPayload, log)
				if err != nil {
					return err
				}

				//use log path
				if pt.ServiceName == "" {
					pt.ServiceName = t.TestName
				}

				// 设置敏感信息
				t.JobCtx.EnvVars = append(t.JobCtx.EnvVars, envs...)

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskResetImage:
			t, err := commonservice.ToDeployTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				t.SetNamespace(pt.TaskArgs.Deploy.Namespace)
				image, err := validateServiceContainer(t.EnvName, t.ProductName, t.ServiceName, t.ContainerName)
				if err != nil {
					log.Error(err)
					return err
				}

				t.SetImage(image)

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskDeploy:
			t, err := commonservice.ToDeployTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				// 从创建任务payload设置容器部署
				t.SetImage(pt.TaskArgs.Deploy.Image)
				t.SetNamespace(pt.TaskArgs.Deploy.Namespace)

				_, err := validateServiceContainer(t.EnvName, t.ProductName, t.ServiceName, t.ContainerName)
				if err != nil {
					log.Error(err)
					return err
				}

				// Task creator can be webhook trigger or cronjob trigger or validated user
				// Validated user includes both that user is granted write permission or user is the owner of this product
				if pt.TaskCreator == setting.WebhookTaskCreator ||
					pt.TaskCreator == setting.CronTaskCreator ||
					IsProductAuthed(pt.TaskCreator, t.Namespace, pt.ProductName, config.ProductWritePermission, log) {
					log.Infof("Validating permission passed. product:%s, owner:%s, task executed by: %s", pt.ProductName, t.Namespace, pt.TaskCreator)
				} else {
					log.Errorf("permission denied. product:%s, owner:%s, task executed by: %s", pt.ProductName, t.Namespace, pt.TaskCreator)
					return errors.New(e.ProductAccessDeniedErrMsg)
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskDistributeToS3:
			task, err := commonservice.ToDistributeToS3Task(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			if task.Enabled {
				task.SetPackageFile(pt.TaskArgs.Deploy.PackageFile)

				if pt.TeamName == "" {
					task.ProductName = pt.ProductName
				} else {
					task.ProductName = pt.TeamName
				}

				task.ServiceName = pt.ServiceName

				if task.DestStorageUrl == "" {
					var storage *commonmodels.S3Storage
					// 在pipeline中配置了对象存储
					if task.S3StorageId != "" {
						if storage, err = commonrepo.NewS3StorageColl().Find(task.S3StorageId); err != nil {
							return e.ErrFindS3Storage.AddDesc(fmt.Sprintf("id:%s, %v", task.S3StorageId, err))
						}
					} else if storage, err = GetS3RelStorage(log); err != nil { // adapt for qbox
						return e.ErrFindS3Storage
					}

					defaultS3 := s3.S3{
						S3Storage: storage,
					}

					task.DestStorageUrl, err = defaultS3.GetEncryptedUrl()
					if err != nil {
						return err
					}
				}
				pt.SubTasks[i], err = task.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskReleaseImage:
			t, err := commonservice.ToReleaseImageTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				// 从创建任务payload设置线上镜像分发任务
				t.SetImage(pt.TaskArgs.Deploy.Image)
				// 兼容老的任务配置
				if len(t.Releases) == 0 {
					found := false
					for id, v := range pt.ConfigPayload.RepoConfigs {
						if v.Namespace == t.ImageRepo {
							t.Releases = append(t.Releases, commonmodels.RepoImage{RepoId: id})
							found = true
						}
					}

					if !found {
						return fmt.Errorf("没有找到命名空间是 [%s] 的镜像仓库", t.ImageRepo)
					}
				}

				var repos []commonmodels.RepoImage
				for _, repoImage := range t.Releases {
					if v, ok := pt.ConfigPayload.RepoConfigs[repoImage.RepoId]; ok {
						repoImage.Name = util.ReplaceRepo(pt.TaskArgs.Deploy.Image, v.RegAddr, v.Namespace)
						repoImage.Host = util.GetURLHostName(v.RegAddr)
						repoImage.Namespace = v.Namespace
						repos = append(repos, repoImage)
					}
				}

				t.Releases = repos
				if len(t.Releases) == 0 {
					t.Enabled = false
				} else {
					t.ImageRelease = t.Releases[0].Name
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}

		case config.TaskJira:
			t, err := commonservice.ToJiraTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}
			FmtBuilds(t.Builds, log)
			if t.Enabled {
				// 外部触发的pipeline
				if pt.TaskCreator == setting.WebhookTaskCreator || pt.TaskCreator == setting.CronTaskCreator {
					setTriggerBuilds(t.Builds, pt.TaskArgs.Builds)
				} else {
					setManunalBuilds(t.Builds, pt.TaskArgs.Builds)
				}

				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskSecurity:
			t, err := commonservice.ToSecurityTask(subTask)
			if err != nil {
				log.Error(err)
				return err
			}

			if t.Enabled {
				t.SetImageName(pt.TaskArgs.Deploy.Image)
				pt.SubTasks[i], err = t.ToSubTask()
				if err != nil {
					return err
				}
			}
		case config.TaskDistribute:
			// 为了兼容历史数据类型，目前什么都不用做，避免出错
		default:
			return e.NewErrInvalidTaskType(string(pre.TaskType))
		}
	}

	return nil
}

// releaseCandidateTag 根据 TaskID 生成编译镜像Tag或者二进制包后缀
// TODO: max length of a tag is 128
func releaseCandidateTag(b *task.Build, taskID int64) string {
	timeStamp := time.Now().Format("20060102150405")

	if len(b.JobCtx.Builds) > 0 {
		first := b.JobCtx.Builds[0]
		for index, build := range b.JobCtx.Builds {
			if build.IsPrimary {
				first = b.JobCtx.Builds[index]
			}
		}

		// 替换 Tag 和 Branch 中的非法字符为 "-", 避免 docker build 失败
		var (
			reg    = regexp.MustCompile(`[^\w.-]`)
			tag    = string(reg.ReplaceAll([]byte(first.Tag), []byte("-")))
			branch = string(reg.ReplaceAll([]byte(first.Branch), []byte("-")))
		)

		if tag != "" {
			return fmt.Sprintf("%s-%s", timeStamp, tag)
		}
		if branch != "" && first.PR != 0 {
			return fmt.Sprintf("%s-%d-%s-pr-%d", timeStamp, taskID, branch, first.PR)
		}
		if branch == "" && first.PR != 0 {
			return fmt.Sprintf("%s-%d-pr-%d", timeStamp, taskID, first.PR)
		}
		if branch != "" && first.PR == 0 {
			return fmt.Sprintf("%s-%d-%s", timeStamp, taskID, branch)
		}
	}
	return timeStamp
}

// GetImage suffix 可以是 branch name 或者 pr number
func GetImage(registry *commonmodels.RegistryNamespace, serviceName, suffix string) string {
	image := fmt.Sprintf("%s/%s/%s:%s", util.GetURLHostName(registry.RegAddr), registry.Namespace, serviceName, suffix)
	return image
}

// GetPackageFile suffix 可以是 branch name 或者 pr number
func GetPackageFile(serviceName, suffix string) string {
	packageFile := fmt.Sprintf("%s-%s.tar.gz", serviceName, suffix)
	return packageFile
}

// prepareTaskEnvs 注入运行pipeline task所需要环境变量
func prepareTaskEnvs(pt *task.Task, log *xlog.Logger) []*commonmodels.KeyVal {
	envs := make([]*commonmodels.KeyVal, 0)

	// 设置系统信息
	envs = append(envs,
		&commonmodels.KeyVal{Key: "SERVICE", Value: pt.ServiceName},
		&commonmodels.KeyVal{Key: "BUILD_URL", Value: GetLink(pt, config.ENVAslanURL(), config.WorkflowType)},
		&commonmodels.KeyVal{Key: "DIST_DIR", Value: fmt.Sprintf("%s/%s/dist/%d", pt.ConfigPayload.S3Storage.Path, pt.PipelineName, pt.TaskID)},
		&commonmodels.KeyVal{Key: "IMAGE", Value: pt.TaskArgs.Deploy.Image},
		&commonmodels.KeyVal{Key: "PKG_FILE", Value: pt.TaskArgs.Deploy.PackageFile},
		&commonmodels.KeyVal{Key: "LOG_FILE", Value: "/tmp/user_script.log"},
	)

	// 设置编译模块参数化配置信息
	if pt.BuildModuleVer != "" {
		opt := &commonrepo.BuildFindOption{
			Version:     pt.BuildModuleVer,
			Targets:     []string{pt.Target},
			ProductName: pt.ProductName,
		}
		bm, err := commonrepo.NewBuildColl().Find(opt)
		if err != nil {
			log.Errorf("[BuildModule.Find] error: %v", err)
		}

		if bm != nil && bm.PreBuild != nil {
			// 注入用户定义环境信息
			if len(bm.PreBuild.Envs) > 0 {
				envs = append(envs, bm.PreBuild.Envs...)
			}

			for _, p := range bm.PreBuild.Parameters {
				// 默认注入KEY默认值
				kv := &commonmodels.KeyVal{Key: p.Name, Value: p.DefaultValue}
				for _, pv := range p.ParamVal {
					// 匹配到对应服务的值
					if pv.Target == pt.Target {
						kv = &commonmodels.KeyVal{Key: p.Name, Value: pv.Value}
					}
				}
				envs = append(envs, kv)
			}
		}
	}
	return envs
}

func GetLink(p *task.Task, baseUri string, taskType config.PipelineType) string {
	url := ""
	switch taskType {
	case config.WorkflowType:
		url = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/%s/%s/%d", baseUri, p.ProductName, UiType(p.Type), p.PipelineName, p.TaskID)
	case config.TestType:
		url = fmt.Sprintf("%s/v1/projects/detail/%s/test/detail/function/%s/%d", baseUri, p.ProductName, p.PipelineName, p.TaskID)
	}
	return url
}

func UiType(pipelineType config.PipelineType) string {
	if pipelineType == config.SingleType {
		return "single"
	} else {
		return "multi"
	}
}

func SetCandidateRegistry(payload *commonmodels.ConfigPayload, log *xlog.Logger) error {
	if payload == nil {
		return nil
	}

	reg, err := commonservice.FindDefaultRegistry(log)
	if err != nil {
		log.Errorf("can't find default candidate registry: %v", err)
		return e.ErrFindRegistry.AddDesc(err.Error())
	}

	payload.Registry.Addr = reg.RegAddr
	payload.Registry.AccessKey = reg.AccessKey
	payload.Registry.SecretKey = reg.SecretyKey
	return nil
}

// validateServiceContainer validate container with envName like dev
func validateServiceContainer(envName, productName, serviceName, container string) (string, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return "", err
	}

	kubeClient, err := kube.GetKubeClient(product.ClusterId)
	if err != nil {
		return "", err
	}

	return validateServiceContainer2(
		product.Namespace, envName, productName, serviceName, container, product.Source, kubeClient,
	)
}

type ContainerNotFound struct {
	ServiceName string
	Container   string
	EnvName     string
	ProductName string
}

func (c *ContainerNotFound) Error() string {
	return fmt.Sprintf("serviceName:%s,container:%s", c.ServiceName, c.Container)
}

// validateServiceContainer2 validate container with raw namespace like dev-product
func validateServiceContainer2(namespace, envName, productName, serviceName, container, source string, kubeClient client.Client) (string, error) {
	var selector labels.Selector

	pods, err := getter.ListPods(namespace, selector, kubeClient)
	if err != nil {
		return "", fmt.Errorf("[%s] ListPods %s/%s error: %v", namespace, productName, serviceName, err)
	}

	for _, p := range pods {
		pod := wrapper.Pod(p).Resource()
		for _, c := range pod.ContainerStatuses {
			if c.Name == container || strings.Contains(c.Image, container) {
				return c.Image, nil
			}
		}
	}
	log.Errorf("[%s]container %s not found", namespace, container)

	return "", &ContainerNotFound{
		ServiceName: serviceName,
		Container:   container,
		EnvName:     envName,
		ProductName: productName,
	}
}

// IsProductAuthed 查询指定产品是否授权给用户, 或者用户所在的组
// TODO: REVERT Auth is diabled
func IsProductAuthed(username, productOwner, productName string, perm config.ProductPermission, log *xlog.Logger) bool {

	//// 获取目标产品信息
	//opt := &collection.ProductFindOptions{Name: productName, EnvName: productOwner}
	//
	//prod, err := s.Collections.Product.Find(opt)
	//if err != nil {
	//	log.Errorf("[%s][%s] error: %v", productOwner, productName, err)
	//	return false
	//}
	//
	//userTeams := make([]string, 0)
	////userTeams, err := s.FindUserTeams(UserIDStr, log)
	////if err != nil {
	////	log.Errorf("FindUserTeams error: %v", err)
	////	return false
	////}
	//
	//// 如果是当前用户创建的产品或者当前用户已经被产品授权过，返回true
	//if prod.EnvName == UserIDStr || prod.IsUserAuthed(UserIDStr, userTeams, perm) {
	//	return true
	//}

	return true
}

// GetS3RelStorage find the default s3storage
func GetS3RelStorage(logger *xlog.Logger) (*commonmodels.S3Storage, error) {
	return commonrepo.NewS3StorageColl().GetS3Storage()
}
