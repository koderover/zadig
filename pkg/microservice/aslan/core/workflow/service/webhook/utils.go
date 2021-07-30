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

package webhook

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	githubservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	e "github.com/koderover/zadig/pkg/tool/errors"
	gitlabtool "github.com/koderover/zadig/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
)

type yamlGetter interface {
	GetYAMLContents(owner, repo, branch, path string, isDir, split bool) ([]string, error)
}

func newGetter(source, address, owner string) (yamlGetter, error) {
	ch, err := codehost.GetCodeHostInfo(
		&codehost.Option{CodeHostType: source, Address: address, Namespace: owner})
	if err != nil {
		return nil, err
	}
	switch source {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr()), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.Address, ch.AccessToken)
	default:
		// should not have happened here
		log.DPanicf("invalid source: %s", source)
		return nil, fmt.Errorf("invalid source: %s", source)
	}
}

func syncContent(args *commonmodels.Service, logger *zap.SugaredLogger) error {
	address, owner, repo, branch, path, pathType, err := GetOwnerRepoBranchPath(args.SrcPath)
	if err != nil {
		logger.Errorf("Failed to parse url %s, err: %s", args.SrcPath, err)
		return fmt.Errorf("url parse failure, err: %s", err)
	}

	getter, err := newGetter(args.Source, address, owner)
	if err != nil {
		logger.Errorf("Failed to create new getter, error: %s", err)
		return err
	}

	yamls, err := getter.GetYAMLContents(owner, repo, branch, path, pathType != "blob", true)
	if err != nil {
		logger.Errorf("Failed to get yamls, error: %s", err)
		return err
	}
	args.KubeYamls = yamls
	args.Yaml = util.CombineManifests(yamls)

	return nil
}

// fillServiceTmpl 更新服务模板参数
func fillServiceTmpl(args *commonmodels.Service, log *zap.SugaredLogger) error {
	if args == nil {
		return errors.New("service template arg is null")
	}
	if len(args.ServiceName) == 0 {
		return errors.New("service name is empty")
	}
	if !config.ServiceNameRegex.MatchString(args.ServiceName) {
		return fmt.Errorf("service name must match %s", config.ServiceNameRegexString)
	}
	if args.Type == setting.K8SDeployType {
		if args.Containers == nil {
			args.Containers = make([]*commonmodels.Container, 0)
		}
		// 配置来源为Gitlab，需要从Gitlab同步配置，并设置KubeYamls.
		if args.Source == setting.SourceFromGitlab {
			// Set args.Commit
			if err := syncLatestCommit(args); err != nil {
				log.Errorf("Sync change log from gitlab failed, error: %v", err)
				return err
			}
			// 从Gitlab同步args指定的Commit下，指定目录中对应的Yaml文件
			// Set args.Yaml & args.KubeYamls
			if err := syncContent(args, log); err != nil {
				log.Errorf("Sync content from gitlab failed, error: %v", err)
				return err
			}
		} else if args.Source == setting.SourceFromGithub {
			err := syncContent(args, log)
			if err != nil {
				log.Errorf("Sync content from github failed, error: %v", err)
				return err
			}
		} else {
			// 拆分 all-in-one yaml文件
			// 替换分隔符
			args.Yaml = util.ReplaceWrapLine(args.Yaml)
			// 分隔符为\n---\n
			args.KubeYamls = util.SplitManifests(args.Yaml)
		}

		// 遍历args.KubeYamls，获取 Deployment 或者 StatefulSet 里面所有containers 镜像和名称
		if err := setCurrentContainerImages(args); err != nil {
			return err
		}

		log.Infof("find %d containers in service %s", len(args.Containers), args.ServiceName)
	}

	// 设置新的版本号
	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, args.ServiceName, args.ProductName)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		return fmt.Errorf("get next service template revision error: %v", err)
	}

	args.Revision = rev

	return nil
}

func syncLatestCommit(service *commonmodels.Service) error {
	if service.SrcPath == "" {
		return fmt.Errorf("url不能是空的")
	}

	address, owner, repo, branch, path, _, err := GetOwnerRepoBranchPath(service.SrcPath)
	if err != nil {
		return fmt.Errorf("url 必须包含 owner/repo/tree/branch/path，具体请参考 Placeholder 提示")
	}

	client, err := getGitlabClientByAddress(address)
	if err != nil {
		return err
	}

	commit, err := GitlabGetLatestCommit(client, owner, repo, branch, path)
	if err != nil {
		return err
	}
	service.Commit = &commonmodels.Commit{
		SHA:     commit.ID,
		Message: commit.Message,
	}
	return nil
}

func getGitlabClientByAddress(address string) (*gitlabtool.Client, error) {
	opt := &codehost.Option{
		Address:      address,
		CodeHostType: codehost.GitLabProvider,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc("git client is nil")
	}
	client, err := gitlabtool.NewClient(codehost.Address, codehost.AccessToken)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
	}

	return client, nil
}

func GitlabGetLatestCommit(client *gitlabtool.Client, owner, repo string, ref, path string) (*gitlab.Commit, error) {
	commit, err := client.GetLatestRepositoryCommit(owner, repo, ref, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get lastest commit with project %s/%s, ref: %s, path:%s, error: %v",
			owner, repo, ref, path, err)
	}
	return commit, nil
}

// 从 kube yaml 中获取所有当前 containers 镜像和名称
// 支持 Deployment StatefulSet Job
func setCurrentContainerImages(args *commonmodels.Service) error {
	srvContainers := make([]*commonmodels.Container, 0)
	for _, data := range args.KubeYamls {
		manifests := releaseutil.SplitManifests(data)
		for _, item := range manifests {
			//在Unmarshal之前填充渲染变量{{.}}
			item = config.RenderTemplateAlias.ReplaceAllLiteralString(item, "ssssssss")
			// replace $Service$ with service name
			item = config.ServiceNameAlias.ReplaceAllLiteralString(item, args.ServiceName)

			u, err := serializer.NewDecoder().YamlToUnstructured([]byte(item))
			if err != nil {
				return fmt.Errorf("unmarshal ResourceKind error: %v", err)
			}

			switch u.GetKind() {
			case setting.Deployment, setting.StatefulSet, setting.Job:
				cs, err := getContainers(u)
				if err != nil {
					return fmt.Errorf("GetContainers error: %v", err)
				}
				srvContainers = append(srvContainers, cs...)
			}
		}
	}

	args.Containers = srvContainers

	return nil
}

// 从kube yaml中查找所有containers 镜像和名称
func getContainers(u *unstructured.Unstructured) ([]*commonmodels.Container, error) {
	var containers []*commonmodels.Container
	cs, _, _ := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "containers")
	for _, c := range cs {
		val, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		nameStr, ok := val["name"].(string)
		if !ok {
			return containers, errors.New("error name value")
		}

		imageStr, ok := val["image"].(string)
		if !ok {
			return containers, errors.New("error image value")
		}

		containers = append(containers, &commonmodels.Container{
			Name:  nameStr,
			Image: imageStr,
		})
	}

	return containers, nil
}

type MatchFolders []string

// ContainsFile  "/" 代表全部文件
func ContainsFile(h *commonmodels.GitHook, file string) bool {
	return MatchFolders(h.MatchFolders).ContainsFile(file)
}

func (m MatchFolders) ContainsFile(file string) bool {
	var excludes []string
	var matches []string

	for _, match := range m {
		if strings.HasPrefix(match, "!") {
			excludes = append(excludes, match)
		} else {
			matches = append(matches, match)
		}
	}

	for _, match := range matches {
		if match == "/" || strings.HasPrefix(file, match) {
			// 以!开头的目录或者后缀名为不运行pipeline的过滤条件
			for _, exclude := range excludes {
				// 如果！后面不跟任何目录或者文件，忽略
				if len(exclude) <= 2 {
					return false
				}
				eCheck := exclude[1:]
				if eCheck == "/" || path.Ext(file) == eCheck || strings.HasPrefix(file, eCheck) || strings.HasSuffix(file, eCheck) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func MatchChanges(m commonmodels.MainHookRepo, files []string) bool {
	mf := MatchFolders(m.MatchFolders)
	for _, file := range files {
		if matches := mf.ContainsFile(file); matches {
			return true
		}
	}
	return false
}

func EventConfigured(m commonmodels.MainHookRepo, event config.HookEventType) bool {
	for _, ev := range m.Events {
		if ev == event {
			return true
		}
	}

	return false
}
