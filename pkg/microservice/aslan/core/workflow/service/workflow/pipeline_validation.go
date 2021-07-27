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
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/task"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/codehub"
	git "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/shared/kube/wrapper"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/kube/getter"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
	"github.com/koderover/zadig/pkg/util"
)

const (
	NameSpaceRegexString   = "[^a-z0-9.-]"
	defaultNameRegexString = "^[a-zA-Z0-9-_]{1,50}$"
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

func fmtBuildsTask(build *task.Build, log *zap.SugaredLogger) {
	//detail,err := s.Codehost.Detail.GetCodehostDetail(build.JobCtx.Builds)
	FmtBuilds(build.JobCtx.Builds, log)
}

//replace gitInfo with codehostID
func FmtBuilds(builds []*types.Repository, log *zap.SugaredLogger) {
	for _, repo := range builds {
		cID := repo.CodehostID
		if cID == 0 {
			log.Error("codehostID can't be empty")
			return
		}
		opt := &codehost.Option{
			CodeHostID: cID,
		}
		detail, err := codehost.GetCodeHostInfo(opt)
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
func SetTriggerBuilds(builds []*types.Repository, buildArgs []*types.Repository) error {
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

			setBuildInfo(build)
		}(build)
	}
	wg.Wait()
	return nil
}

func setBuildInfo(build *types.Repository) {
	opt := &codehost.Option{
		CodeHostID: build.CodehostID,
	}
	codeHostInfo, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Errorf("failed to get codehost detail %d %v", build.CodehostID, err)
		return
	}
	if codeHostInfo.Type == codehost.GitLabProvider || codeHostInfo.Type == codehost.GerritProvider {
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
	} else if codeHostInfo.Type == codehost.CodeHubProvider {
		codeHubClient := codehub.NewClient(codeHostInfo.AccessKey, codeHostInfo.SecretKey, codeHostInfo.Region)
		if build.CommitID == "" && build.Branch != "" {
			branchList, _ := codeHubClient.BranchList(build.RepoUUID)
			for _, branchInfo := range branchList {
				if branchInfo.Name == build.Branch {
					build.CommitID = branchInfo.Commit.ID
					build.CommitMessage = branchInfo.Commit.Message
					build.AuthorName = branchInfo.Commit.AuthorName
					return
				}
			}
		}
	} else {
		gitCli := git.NewClient(codeHostInfo.AccessToken, config.ProxyHTTPSAddr())
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

	if buildArg.IsPrimary {
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
			defer wg.Done()

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

	reg, err := commonservice.FindDefaultRegistry(log)
	if err != nil {
		log.Errorf("can't find default candidate registry: %v", err)
		return e.ErrFindRegistry.AddDesc(err.Error())
	}

	payload.Registry.Addr = reg.RegAddr
	payload.Registry.AccessKey = reg.AccessKey
	payload.Registry.SecretKey = reg.SecretKey
	payload.Registry.Namespace = reg.Namespace
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

	kubeClient, err := kube.GetKubeClient(product.ClusterID)
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

type ImageIllegal struct {
}

func (c *ImageIllegal) Error() string {
	return ""
}

// validateServiceContainer2 validate container with raw namespace like dev-product
func validateServiceContainer2(namespace, envName, productName, serviceName, container, source string, kubeClient client.Client) (string, error) {
	var selector labels.Selector

	//helm类型的服务查询所有标签的pod
	if source != setting.SourceFromHelm {
		selector = labels.Set{setting.ProductLabel: productName, setting.ServiceLabel: serviceName}.AsSelector()
		//builder := &SelectorBuilder{ProductName: productName, ServiceName: serviceName}
		//selector = builder.BuildSelector()
	}

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
func IsProductAuthed(username, productOwner, productName string, perm config.ProductPermission, log *zap.SugaredLogger) bool {

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
func GetS3RelStorage(logger *zap.SugaredLogger) (*commonmodels.S3Storage, error) {
	return commonrepo.NewS3StorageColl().GetS3Storage()
}
