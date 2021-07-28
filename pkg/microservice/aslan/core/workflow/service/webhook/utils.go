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
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/codehub"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
	githubtool "github.com/koderover/zadig/pkg/tool/git/github"
	gitlabtool "github.com/koderover/zadig/pkg/tool/git/gitlab"
	"github.com/koderover/zadig/pkg/tool/kube/serializer"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util"
)

func syncContent(args *commonmodels.Service, logger *zap.SugaredLogger) error {
	address, _, repo, branch, path, pathType, err := GetOwnerRepoBranchPath(args.SrcPath)
	if err != nil {
		logger.Errorf("Failed to parse url %s, err: %s", args.SrcPath, err)
		return fmt.Errorf("url parse failure, err: %s", err)
	}

	var yamls []string
	switch args.Source {
	case setting.SourceFromCodeHub:
		client, err := getCodehubClientByAddress(address)
		if err != nil {
			logger.Errorf("Failed to get codehub client, error: %s", err)
			return err
		}
		repoUUID, err := client.GetRepoUUID(repo)
		if err != nil {
			logger.Errorf("Failed to get repoUUID, error: %s", err)
			return err
		}
		yamls, err = client.GetYAMLContents(repoUUID, branch, path, pathType == "tree", true)
		if err != nil {
			logger.Errorf("Failed to get yamls, error: %s", err)
			return err
		}
	}

	args.KubeYamls = yamls
	args.Yaml = util.CombineManifests(yamls)

	return nil
}

// fillServiceTmpl 更新服务模板参数
func fillServiceTmpl(userName string, args *commonmodels.Service, log *zap.SugaredLogger) error {
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
			if err := syncContentFromGitlab(userName, args); err != nil {
				log.Errorf("Sync content from gitlab failed, error: %v", err)
				return err
			}
		} else if args.Source == setting.SourceFromGithub {
			err := syncContentFromGithub(args, log)
			if err != nil {
				log.Errorf("Sync content from github failed, error: %v", err)
				return err
			}
		} else if args.Source == setting.SourceFromCodeHub {
			err := syncContent(args, log)
			if err != nil {
				log.Errorf("Sync content from codehub failed, error: %v", err)
				return err
			}
		} else {
			// 拆分 all-in-one yaml文件
			// 替换分隔符
			args.Yaml = util.ReplaceWrapLine(args.Yaml)
			// 分隔符为\n---\n
			args.KubeYamls = SplitYaml(args.Yaml)
		}

		// 遍历args.KubeYamls，获取 Deployment 或者 StatefulSet 里面所有containers 镜像和名称
		if err := setCurrentContainerImages(args); err != nil {
			return err
		}

		log.Infof("find %d containers in service %s", len(args.Containers), args.ServiceName)
	}

	// 设置新的版本号
	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, args.ServiceName, args.Type)
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

func syncCodehubLatestCommit(service *commonmodels.Service) error {
	if service.SrcPath == "" {
		return fmt.Errorf("url不能是空的")
	}

	address, owner, repo, branch, _, _, err := GetOwnerRepoBranchPath(service.SrcPath)
	if err != nil {
		return fmt.Errorf("url 必须包含 owner/repo/tree/branch/path，具体请参考 Placeholder 提示")
	}

	client, err := getCodehubClientByAddress(address)
	if err != nil {
		return err
	}

	id, message, err := CodehubGetLatestCommit(client, owner, repo, branch)
	if err != nil {
		return err
	}
	service.Commit = &commonmodels.Commit{
		SHA:     id,
		Message: message,
	}
	return nil
}

func getCodehubClientByAddress(address string) (*codehub.Client, error) {
	opt := &codehost.Option{
		Address:      address,
		CodeHostType: codehost.CodeHubProvider,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc("git client is nil")
	}
	client := codehub.NewClient(codehost.AccessKey, codehost.SecretKey, codehost.Region)

	return client, nil
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
	commit, err := client.GetLatestCommit(owner, repo, ref, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get lastest commit with project %s/%s, ref: %s, path:%s, error: %v",
			owner, repo, ref, path, err)
	}
	return commit, nil
}

func CodehubGetLatestCommit(client *codehub.Client, owner, repo string, branch string) (string, string, error) {
	commit, err := client.GetLatestRepositoryCommit(owner, repo, branch)
	if err != nil {
		return "", "", fmt.Errorf("failed to get lastest commit with project %s/%s, ref: %s, error: %s",
			owner, repo, CodehubGetLatestCommit, err)
	}
	return commit.ID, commit.Message, nil
}

// GitlabGetRawFiles ...
// projectID: identity of project, can be retrieved from s.GitlabGetProjectID(owner, repo)
// ref: branch (e.g. master) or commit (commit id) or tag
// path: file path of raw files, only retrieve leaf node(blob type == file), no recursive get
func GitlabGetRawFiles(client *gitlabtool.Client, owner, repo, ref, path, pathType string) (files []string, err error) {
	files = make([]string, 0)
	var errs *multierror.Error
	if pathType == "tree" {
		nodes, err := client.ListTree(owner, repo, ref, path)
		if err != nil {
			return files, err
		}
		for _, node := range nodes {
			// if node type is "tree", it is a directory, skip it for now
			if node.Type == "tree" {
				continue
			}
			fileName := strings.ToLower(node.Name)
			if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
				continue
			}
			// if node type is "blob", it is a file
			// Path is filepath of a node
			content, err := client.GetRawFile(owner, repo, ref, node.Path)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
			contentStr := string(content)
			contentStr = util.ReplaceWrapLine(contentStr)
			files = append(files, contentStr)
		}
		return files, errs.ErrorOrNil()
	}
	content, err := client.GetFileContent(owner, repo, ref, path)
	if err != nil {
		return files, err
	}
	files = append(files, string(content))
	return files, errs.ErrorOrNil()
}

// syncContentFromGitlab ...
// sync content with commit, args.Commit should not be nil
func syncContentFromGitlab(userName string, args *commonmodels.Service) error {
	if args.Commit == nil {
		return nil
	}

	address, owner, repo, branch, path, pathType, err := GetOwnerRepoBranchPath(args.SrcPath)
	if err != nil {
		return fmt.Errorf("url format failed")
	}

	client, err := getGitlabClientByAddress(address)
	if err != nil {
		return err
	}

	files, err := GitlabGetRawFiles(client, owner, repo, branch, path, pathType)
	if err != nil {
		return err
	}
	if userName != setting.WebhookTaskCreator {
		if len(files) == 0 {
			return fmt.Errorf("没有检索到yml,yaml类型文件，请检查目录是否正确")
		}
	}
	// KubeYamls field is dynamicly synced.
	// 根据gitlab sync的内容来设置args.KubeYamls
	args.KubeYamls = files
	// 拼装并设置args.Yaml
	args.Yaml = joinYamls(files)
	return nil
}

func joinYamls(files []string) string {
	return strings.Join(files, setting.YamlFileSeperator)
}

func syncContentFromGithub(args *commonmodels.Service, log *zap.SugaredLogger) error {
	// 根据pipeline中的filepath获取文件内容
	address, owner, repo, branch, path, _, err := GetOwnerRepoBranchPath(args.SrcPath)
	if err != nil {
		log.Errorf("GetOwnerRepoBranchPath failed, srcPath:%s, err:%v", args.SrcPath, err)
		return errors.New("invalid url " + args.SrcPath)
	}

	ch, err := codehost.GetCodeHostInfo(
		&codehost.Option{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
	if err != nil {
		log.Errorf("GetCodeHostInfo failed, srcPath:%s, err:%v", args.SrcPath, err)
		return err
	}

	gc := githubtool.NewClient(&githubtool.Config{AccessToken: ch.AccessToken, Proxy: config.ProxyHTTPSAddr()})
	fileContent, directoryContent, err := gc.GetContents(context.TODO(), owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if fileContent != nil {
		svcContent, _ := fileContent.GetContent()
		splitYaml := SplitYaml(svcContent)
		args.KubeYamls = splitYaml
		args.Yaml = svcContent
	} else {
		var files []string
		for _, f := range directoryContent {
			// 排除目录
			if *f.Type != "file" {
				continue
			}
			fileName := strings.ToLower(*f.Path)
			if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
				continue
			}

			file, err := syncSingleFileFromGithub(owner, repo, branch, *f.Path, ch.AccessToken)
			if err != nil {
				log.Errorf("syncSingleFileFromGithub failed, path: %s, err: %v", *f.Path, err)
				continue
			}
			files = append(files, file)
		}

		args.KubeYamls = files
		args.Yaml = joinYamls(files)
	}

	return nil
}

func SplitYaml(yaml string) []string {
	return strings.Split(yaml, setting.YamlFileSeperator)
}

type githubFileContent struct {
	SHA      string `json:"sha"`
	NodeID   string `json:"node_id"`
	Size     int    `json:"size"`
	URL      string `json:"url"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func syncSingleFileFromGithub(owner, repo, branch, path, token string) (string, error) {
	gc := githubtool.NewClient(&githubtool.Config{AccessToken: token, Proxy: config.ProxyHTTPSAddr()})
	fileContent, _, err := gc.GetContents(context.TODO(), owner, repo, path, &github.RepositoryContentGetOptions{Ref: branch})
	if fileContent != nil {
		return fileContent.GetContent()
	}

	return "", err
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
