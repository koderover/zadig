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
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow/controller/job"
	"regexp"
	"sort"
	"sync"

	"github.com/google/go-github/v35/github"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/base"
	git "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	kubeclient "github.com/koderover/zadig/v2/pkg/shared/kube/client"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/tool/kube/getter"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	NameSpaceRegexString   = "[^a-z0-9.-]"
	defaultNameRegexString = "^[a-zA-Z0-9-_]{1,50}$"
	InterceptCommitID      = 8
)

var (
	NameSpaceRegex   = regexp.MustCompile(NameSpaceRegexString)
	defaultNameRegex = regexp.MustCompile(defaultNameRegexString)
)

func validatePipelineHookNames(p *commonmodels.Pipeline) error {
	if p == nil || p.Hook == nil {
		return nil
	}

	var names []string
	for _, hook := range p.Hook.GitHooks {
		names = append(names, hook.Name)
	}

	return validateHookNames(names)
}

func ensurePipeline(args *commonmodels.Pipeline, log *zap.SugaredLogger) error {
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
		pre, err := base.ToPreview(subTask)
		if err != nil {
			return errors.New(e.InterfaceToTaskErrMsg)
		}

		if !pre.Enabled {
			continue
		}

		switch pre.TaskType {

		case config.TaskBuild:
			t, err := base.ToBuildTask(subTask)
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
			t, err := base.ToTestingTask(subTask)
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
			t.InstallCtx, err = BuildInstallCtx(t.InstallItems)
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
	resp := make(map[string]string)
	pipe, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: pipelineName})
	if err != nil {
		log.Errorf("find pipeline %s ERR: %v", pipelineName, err)
		return resp
	}

	for _, subTask := range pipe.SubTasks {

		pre, err := base.ToPreview(subTask)
		if err != nil {
			log.Error(err)
			return resp
		}

		switch pre.TaskType {

		case config.TaskBuild:
			t, err := base.ToBuildTask(subTask)
			if err != nil {
				log.Error(err)
				return resp
			}

			for _, v := range t.JobCtx.EnvVars {
				key := fmt.Sprintf("%s/%s", config.TaskBuild, v.Key)
				resp[key] = v.Value
			}

		case config.TaskTestingV2:
			t, err := base.ToTestingTask(subTask)
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

// 根据用户的配置和BuildStep中步骤的依赖，从系统配置的InstallItems中获取配置项，构建Install Context
func BuildInstallCtx(installItems []*commonmodels.Item) ([]*commonmodels.Install, error) {
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

func fmtBuildsTask(build *task.Build, log *zap.SugaredLogger) {
	//detail,err := s.Codehost.Detail.GetCodehostDetail(build.JobCtx.Builds)
	FmtBuilds(build.JobCtx.Builds, log)
}

// replace gitInfo with codehostID
func FmtBuilds(builds []*types.Repository, log *zap.SugaredLogger) {
	for _, repo := range builds {
		cID := repo.CodehostID
		if cID == 0 {
			log.Error("codehostID can't be empty")
			return
		}
		detail, err := systemconfig.New().GetCodeHost(cID)
		if err != nil {
			log.Error(err)
			return
		}
		repo.Source = detail.Type
		repo.OauthToken = detail.AccessToken
		repo.Address = detail.Address
		repo.Username = detail.Username
		repo.Password = detail.Password
	}
}

// 外部触发任务设置build参数
func SetTriggerBuilds(builds []*types.Repository, buildArgs []*types.Repository, log *zap.SugaredLogger) error {
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
			defer wg.Done()

			setBuildInfo(build, buildArgs, log)
		}(build)
	}
	wg.Wait()
	return nil
}

// SetRepoInfo set the information of the build param into the buildArgs, meanwhile add commit ID information into it
func SetRepoInfo(build *types.Repository, buildArgs []*types.Repository, log *zap.SugaredLogger) {
	setBuildInfo(build, buildArgs, log)
}

func setBuildInfo(build *types.Repository, buildArgs []*types.Repository, log *zap.SugaredLogger) {
	codeHostInfo, err := systemconfig.New().GetCodeHost(build.CodehostID)
	if err != nil {
		log.Errorf("failed to get codehost detail %d %v", build.CodehostID, err)
		return
	}
	if build.PR > 0 && len(build.PRs) == 0 {
		build.PRs = []int{build.PR}
	}
	build.Address = codeHostInfo.Address
	if codeHostInfo.Type == systemconfig.GitLabProvider || codeHostInfo.Type == systemconfig.GerritProvider {
		if build.CommitID == "" {
			var commit *job.RepoCommit
			var pr *job.PRCommit
			var err error
			if build.Tag != "" {
				commit, err = job.QueryByTag(build.CodehostID, build.GetRepoNamespace(), build.RepoName, build.Tag, log)
			} else if build.Branch != "" && len(build.PRs) == 0 {
				commit, err = job.QueryByBranch(build.CodehostID, build.GetRepoNamespace(), build.RepoName, build.Branch, log)
			} else if len(build.PRs) > 0 {
				pr, err = job.GetLatestPrCommit(build.CodehostID, getlatestPrNum(build), build.GetRepoNamespace(), build.RepoName, log)
				if err == nil && pr != nil {
					commit = &job.RepoCommit{
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
		// get gerrit submission_id
		if codeHostInfo.Type == systemconfig.GerritProvider {
			build.SubmissionID, err = getSubmissionID(build.CodehostID, build.CommitID)
			log.Infof("get gerrit submissionID %s by commitID %s", build.SubmissionID, build.CommitID)
			if err != nil {
				log.Errorf("getSubmissionID failed, use build %+v %s", build, err)
			}
		}
	} else if codeHostInfo.Type == systemconfig.GiteeProvider || codeHostInfo.Type == systemconfig.GiteeEEProvider {
		gitCli := gitee.NewClient(codeHostInfo.ID, codeHostInfo.Address, codeHostInfo.AccessToken, config.ProxyHTTPSAddr(), codeHostInfo.EnableProxy)
		if build.CommitID == "" {
			if build.Tag != "" && len(build.PRs) == 0 {
				tags, err := gitCli.ListTags(context.Background(), codeHostInfo.Address, codeHostInfo.AccessToken, build.RepoOwner, build.RepoName)
				if err != nil {
					log.Errorf("failed to gitee ListTags err:%s", err)
					return
				}

				for _, tag := range tags {
					if tag.Name == build.Tag {
						build.CommitID = tag.Commit.Sha
						commitInfo, err := gitCli.GetSingleCommitOfProject(context.Background(), codeHostInfo.Address, codeHostInfo.AccessToken, build.RepoOwner, build.RepoName, build.CommitID)
						if err != nil {
							log.Errorf("failed to gitee GetCommit %s err:%s", tag.Commit.Sha, err)
							return
						}
						build.CommitMessage = commitInfo.Commit.Message
						build.AuthorName = commitInfo.Commit.Author.Name
						return
					}
				}
			} else if build.Branch != "" && len(build.PRs) == 0 {
				branch, err := gitCli.GetSingleBranch(codeHostInfo.Address, codeHostInfo.AccessToken, build.RepoOwner, build.RepoName, build.Branch)
				if err != nil {
					log.Errorf("failed to gitee GetSingleBranch  repoOwner:%s,repoName:%s,repoBranch:%s err:%s", build.RepoOwner, build.RepoName, build.Branch, err)
					return
				}
				build.CommitID = branch.Commit.Sha
				build.CommitMessage = branch.Commit.Commit.Message
				build.AuthorName = branch.Commit.Commit.Author.Name
			} else if len(build.PRs) > 0 {
				prCommits, err := gitCli.ListCommitsForPR(context.Background(), build.RepoOwner, build.RepoName, getlatestPrNum(build), nil)
				sort.SliceStable(prCommits, func(i, j int) bool {
					return prCommits[i].Commit.Committer.Date.Unix() > prCommits[j].Commit.Committer.Date.Unix()
				})
				if err == nil && len(prCommits) > 0 {
					for _, commit := range prCommits {
						build.CommitID = commit.Sha
						build.CommitMessage = commit.Commit.Message
						build.AuthorName = commit.Commit.Author.Name
						return
					}
				}
			}
		}
	} else if codeHostInfo.Type == systemconfig.GitHubProvider {
		gitCli := git.NewClient(codeHostInfo.AccessToken, config.ProxyHTTPSAddr(), codeHostInfo.EnableProxy)
		if build.CommitID == "" {
			if build.Tag != "" && len(build.PRs) == 0 {
				opt := &github.ListOptions{Page: 1, PerPage: 100}
				tags, _, err := gitCli.Repositories.ListTags(context.Background(), build.RepoOwner, build.RepoName, opt)
				if err != nil {
					log.Errorf("failed to github ListTags err:%s", err)
					return
				}
				for _, tag := range tags {
					if *tag.Name == build.Tag {
						build.CommitID = tag.Commit.GetSHA()
						if tag.Commit.Message == nil {
							commitInfo, _, err := gitCli.Repositories.GetCommit(context.Background(), build.RepoOwner, build.RepoName, build.CommitID)
							if err != nil {
								log.Errorf("failed to github GetCommit %s err:%s", tag.Commit.GetURL(), err)
								return
							}
							build.CommitMessage = commitInfo.GetCommit().GetMessage()
						}
						build.AuthorName = tag.Commit.GetAuthor().GetName()
						return
					}
				}
			} else if build.Branch != "" && len(build.PRs) == 0 {
				branch, _, err := gitCli.Repositories.GetBranch(context.Background(), build.RepoOwner, build.RepoName, build.Branch)
				if err == nil {
					build.CommitID = *branch.Commit.SHA
					build.CommitMessage = *branch.Commit.Commit.Message
					build.AuthorName = *branch.Commit.Commit.Author.Name
				}
			} else if len(build.PRs) > 0 {
				opt := &github.ListOptions{Page: 1, PerPage: 100}
				prCommits, _, err := gitCli.PullRequests.ListCommits(context.Background(), build.RepoOwner, build.RepoName, getlatestPrNum(build), opt)
				sort.SliceStable(prCommits, func(i, j int) bool {
					return prCommits[i].Commit.Committer.Date.Unix() > prCommits[j].Commit.Committer.Date.Unix()
				})
				if err == nil && len(prCommits) > 0 {
					for _, commit := range prCommits {
						build.CommitID = *commit.SHA
						build.CommitMessage = *commit.Commit.Message
						build.AuthorName = *commit.Commit.Author.Name
						return
					}
				}
			} else {
				log.Warnf("github setBuildInfo failed, use build %+v", build)
				return
			}
		}
	} else if codeHostInfo.Type == systemconfig.OtherProvider {
		build.SSHKey = codeHostInfo.SSHKey
		build.PrivateAccessToken = codeHostInfo.PrivateAccessToken
		for _, buildArg := range buildArgs {
			if buildArg.RepoOwner == build.RepoOwner && buildArg.RepoName == build.RepoName {
				setBuildFromArg(build, buildArg)
				break
			}
		}
		return
	}
}

func getSubmissionID(codehostID int, commitID string) (string, error) {
	ch, err := systemconfig.New().GetCodeHost(codehostID)
	if err != nil {
		return "", err
	}

	cli := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
	changeInfo, err := cli.GetChangeDetail(commitID)
	if err != nil {
		return "", err
	}
	return changeInfo.SubmissionID, nil
}

func getlatestPrNum(build *types.Repository) int {
	var prNum int
	if len(build.PRs) > 0 {
		prNum = build.PRs[len(build.PRs)-1]
	}
	return prNum
}

// 根据传入的build arg设置build参数
func setBuildFromArg(build, buildArg *types.Repository) {
	if build.Source == types.ProviderPerforce {
		// single pr build
		if buildArg.PR > 0 && len(buildArg.Branch) == 0 {
			build.PRs = []int{buildArg.PR}
			build.Branch = ""
		}
		//single pr rebase branch build
		if buildArg.PR > 0 && len(buildArg.Branch) > 0 {
			build.PRs = []int{buildArg.PR}
			build.Branch = buildArg.Branch
		}
		// multi prs build
		if len(buildArg.PRs) > 0 && len(buildArg.Branch) == 0 {
			build.PRs = buildArg.PRs
			build.Branch = ""
		}
		//multi prs rebase branch build
		if len(buildArg.PRs) > 0 && len(buildArg.Branch) > 0 {
			build.PRs = buildArg.PRs
			build.Branch = buildArg.Branch
		}
		// single branch build
		if buildArg.PR == 0 && len(buildArg.PRs) == 0 && len(buildArg.Branch) > 0 {
			build.PRs = []int{}
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
	} else {
		build.Stream = buildArg.Stream
		build.ViewMapping = buildArg.ViewMapping
		build.ChangeListID = buildArg.ChangeListID
		build.ShelveID = buildArg.ShelveID
	}

	if buildArg.IsPrimary {
		build.IsPrimary = true
	}
}

// 外部触发任务设置build参数
func setManunalBuilds(builds []*types.Repository, buildArgs []*types.Repository, log *zap.SugaredLogger) error {
	var wg sync.WaitGroup
	for _, build := range builds {
		// 根据用户提交的工作流任务参数，更新task中source code repo相关参数
		// 并发获取commit信息
		wg.Add(1)
		go func(build *types.Repository) {
			defer wg.Done()

			for _, buildArg := range buildArgs {
				if buildArg.Source == build.Source && buildArg.RepoOwner == build.RepoOwner && buildArg.RepoName == build.RepoName && buildArg.CheckoutPath == build.CheckoutPath {
					setBuildFromArg(build, buildArg)
					break
				}
			}
			setBuildInfo(build, buildArgs, log)
		}(build)
	}
	wg.Wait()
	return nil
}

// GetImage suffix 可以是 branch name 或者 pr number
func GetImage(registry *commonmodels.RegistryNamespace, suffix string) string {
	if registry.RegProvider == config.RegistryTypeAWS {
		return fmt.Sprintf("%s/%s", util.TrimURLScheme(registry.RegAddr), suffix)
	}
	return fmt.Sprintf("%s/%s/%s", util.TrimURLScheme(registry.RegAddr), registry.Namespace, suffix)
}

// GetPackageFile suffix 可以是 branch name 或者 pr number
func GetPackageFile(suffix string) string {
	return fmt.Sprintf("%s.tar.gz", suffix)
}

// prepareTaskEnvs 注入运行pipeline task所需要环境变量
func prepareTaskEnvs(pt *task.Task, log *zap.SugaredLogger) []*commonmodels.KeyVal {
	envs := make([]*commonmodels.KeyVal, 0)

	// 设置系统信息
	envs = append(envs,
		&commonmodels.KeyVal{Key: "SERVICE", Value: pt.ServiceName},
		&commonmodels.KeyVal{Key: "BUILD_URL", Value: GetLink(pt, configbase.SystemAddress(), config.WorkflowType)},
		&commonmodels.KeyVal{Key: "DIST_DIR", Value: fmt.Sprintf("%s/%s/dist/%d", pt.ConfigPayload.S3Storage.Path, pt.PipelineName, pt.TaskID)},
		&commonmodels.KeyVal{Key: "IMAGE", Value: pt.TaskArgs.Deploy.Image},
		&commonmodels.KeyVal{Key: "PKG_FILE", Value: pt.TaskArgs.Deploy.PackageFile},
		&commonmodels.KeyVal{Key: "LOG_FILE", Value: "/tmp/user_script.log"},
	)

	// 设置编译模块参数化配置信息
	if pt.BuildModuleVer != "" {
		opt := &commonrepo.BuildFindOption{
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

func GetLink(p *task.Task, baseURI string, taskType config.PipelineType) string {
	url := ""
	switch taskType {
	case config.WorkflowType:
		url = fmt.Sprintf("%s/v1/projects/detail/%s/pipelines/%s/%s/%d", baseURI, p.ProductName, UIType(p.Type), p.PipelineName, p.TaskID)
	case config.TestType:
		url = fmt.Sprintf("%s/v1/projects/detail/%s/test/detail/function/%s/%d", baseURI, p.ProductName, p.PipelineName, p.TaskID)
	}
	return url
}

func UIType(pipelineType config.PipelineType) string {
	if pipelineType == config.SingleType {
		return "single"
	}
	return "multi"
}

func SetCandidateRegistry(payload *commonmodels.ConfigPayload, log *zap.SugaredLogger) error {
	if payload == nil {
		return nil
	}

	reg, err := commonservice.FindDefaultRegistry(true, log)
	if err != nil {
		log.Errorf("can't find default candidate registry: %s", err)
		return e.ErrFindRegistry.AddDesc(err.Error())
	}

	payload.Registry.Addr = reg.RegAddr
	payload.Registry.AccessKey = reg.AccessKey
	payload.Registry.SecretKey = reg.SecretKey
	payload.Registry.Namespace = reg.Namespace
	return nil
}

// getImageInfoFromWorkload find the current image info from the cluster
func getImageInfoFromWorkload(envName, productName, serviceName, container string) (string, error) {
	product, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:    productName,
		EnvName: envName,
	})
	if err != nil {
		return "", err
	}

	serviceInfo, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		ProductName: productName,
	})
	if err != nil {
		return "", err
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(product.ClusterID)
	if err != nil {
		return "", err
	}

	if product.Source == setting.SourceFromHelm {
		return findCurrentlyUsingImage(product, serviceName, container)
	}

	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(product.ClusterID)
	if err != nil {
		log.Errorf("get client set error: %v", err)
		return "", err
	}
	versionInfo, err := clientset.Discovery().ServerVersion()
	if err != nil {
		log.Errorf("get server version error: %v", err)
		return "", err
	}

	switch serviceInfo.WorkloadType {
	case setting.StatefulSet:
		var statefulSet *appsv1.StatefulSet
		statefulSet, _, err = getter.GetStatefulSet(product.Namespace, serviceName, kubeClient)
		if err != nil {
			return "", err
		}
		for _, c := range statefulSet.Spec.Template.Spec.Containers {
			if c.Name == container {
				return c.Image, nil
			}
		}
		return "", errors.New("no container in statefulset found")
	case setting.Deployment:
		var deployment *appsv1.Deployment
		deployment, _, err = getter.GetDeployment(product.Namespace, serviceName, kubeClient)
		if err != nil {
			return "", err
		}
		for _, c := range deployment.Spec.Template.Spec.Containers {
			if c.Name == container {
				return c.Image, nil
			}
		}
		return "", errors.New("no container in deployment found")
	case setting.CronJob:
		cronJob, cronJobBeta, _, err := getter.GetCronJob(product.Namespace, serviceName, kubeClient, kubeclient.VersionLessThan121(versionInfo))
		if err != nil {
			return "", err
		}
		if cronJob != nil {
			for _, c := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
				if c.Name == container {
					return c.Image, nil
				}
			}
		}
		if cronJobBeta != nil {
			for _, c := range cronJobBeta.Spec.JobTemplate.Spec.Template.Spec.Containers {
				if c.Name == container {
					return c.Image, nil
				}
			}
		}
		return "", errors.New("no container in cronJob found")
	default:
		// since in some version of the service, there are no workload type, we need to do something with this
		selector := labels.Set{setting.ProductLabel: product.ProductName, setting.ServiceLabel: serviceName}.AsSelector()
		var deployments []*appsv1.Deployment
		deployments, err = getter.ListDeployments(product.Namespace, selector, kubeClient)
		if err != nil {
			return "", err
		}
		for _, deploy := range deployments {
			for _, c := range deploy.Spec.Template.Spec.Containers {
				if c.Name == container {
					return c.Image, nil
				}
			}
		}
		var statefulSets []*appsv1.StatefulSet
		statefulSets, err = getter.ListStatefulSets(product.Namespace, selector, kubeClient)
		if err != nil {
			return "", err
		}
		for _, sts := range statefulSets {
			for _, c := range sts.Spec.Template.Spec.Containers {
				if c.Name == container {
					return c.Image, nil
				}
			}
		}

		log.Errorf("No workload found with container: %s", container)
		return "", errors.New("no workload found with container")
	}
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

type ImageIllegal struct {
}

func (c *ImageIllegal) Error() string {
	return ""
}

// find currently using image for services deployed by helm
func findCurrentlyUsingImage(productInfo *commonmodels.Product, serviceName, containerName string) (string, error) {
	for _, service := range productInfo.GetServiceMap() {
		if service.ServiceName == serviceName {
			for _, container := range service.Containers {
				if container.Name == containerName {
					return container.Image, nil
				}
			}
		}
	}
	return "", fmt.Errorf("failed to find image url")
}

// GetS3RelStorage find the default s3storage
func GetS3RelStorage(logger *zap.SugaredLogger) (*commonmodels.S3Storage, error) {
	return commonrepo.NewS3StorageColl().GetS3Storage()
}
