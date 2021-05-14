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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/hashicorp/go-multierror"
	"github.com/qiniu/x/log.v7"
	"golang.org/x/oauth2"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/codehost"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/service/poetry"
	"github.com/koderover/zadig/lib/setting"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/gitlab"
	"github.com/koderover/zadig/lib/tool/kube/serializer"
	"github.com/koderover/zadig/lib/tool/xlog"
	"github.com/koderover/zadig/lib/util"
)

// fillServiceTmpl 更新服务模板参数
func fillServiceTmpl(userName string, args *commonmodels.Service, log *xlog.Logger) error {
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

	projectID, err := GitlabGetProjectID(client, owner, repo)
	if err != nil {
		return err
	}
	commit, err := GitlabGetLatestCommit(client, projectID, branch, path)
	if err != nil {
		return err
	}
	service.Commit = &commonmodels.Commit{
		SHA:     commit.ID,
		Message: commit.Message,
	}
	return nil
}

func getGitlabClientByAddress(address string) (*gitlab.Client, error) {
	opt := &codehost.CodeHostOption{
		Address:      address,
		CodeHostType: codehost.GitLabProvider,
	}
	codehost, err := codehost.GetCodeHostInfo(opt)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc("git client is nil")
	}
	client, err := gitlab.NewGitlabClient(codehost.Address, codehost.AccessToken)
	if err != nil {
		log.Error(err)
		return nil, e.ErrCodehostListProjects.AddDesc(err.Error())
	}

	return client, nil
}

func GitlabGetProjectID(client *gitlab.Client, owner, repo string) (projectID int, err error) {
	project, err := client.GetProject(owner, repo)
	if err != nil {
		return -1, fmt.Errorf("Failed to get project with %s/%s, error: %v", owner, repo, err)
	}
	return project.ID, nil
}

func GitlabGetLatestCommit(client *gitlab.Client, projectID int, ref, path string) (*gitlab.RepoCommit, error) {
	commit, err := client.GetLatestCommit(projectID, ref, path)
	if err != nil {
		return nil, fmt.Errorf("Failed to get lastest commit with project id: %d, ref: %s, path:%s, error: %v",
			projectID, ref, path, err)
	}
	return commit, nil
}

// GitlabGetRawFiles ...
// projectID: identity of project, can be retrieved from s.GitlabGetProjectID(owner, repo)
// ref: branch (e.g. master) or commit (commit id) or tag
// path: file path of raw files, only retrieve leaf node(blob type == file), no recursive get
func GitlabGetRawFiles(client *gitlab.Client, projectID int, ref, path, pathType string) (files []string, err error) {
	files = make([]string, 0)
	var errs *multierror.Error
	if pathType == "tree" {
		nodes, err := client.ListTree(projectID, ref, path)
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
			content, err := client.GetRawFile(projectID, ref, node.Path)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
			contentStr := string(content)
			contentStr = util.ReplaceWrapLine(contentStr)
			files = append(files, contentStr)
		}
		return files, errs.ErrorOrNil()
	}
	content, err := client.GetFileContent(projectID, ref, path)
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

	projectID, err := GitlabGetProjectID(client, owner, repo)
	if owner == "" {
		return fmt.Errorf("url format failed")
	}

	files, err := GitlabGetRawFiles(client, projectID, branch, path, pathType)
	if err != nil {
		return err
	}
	if userName != setting.WebhookTaskCreator {
		if len(files) == 0 {
			return fmt.Errorf("没有检索到yml,yaml类型文件，请检查目录是否正确!")
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

func syncContentFromGithub(args *commonmodels.Service, log *xlog.Logger) error {
	// 根据pipeline中的filepath获取文件内容
	address, owner, repo, branch, path, pathType, err := GetOwnerRepoBranchPath(args.SrcPath)
	if err != nil {
		log.Errorf("GetOwnerRepoBranchPath failed, srcPath:%s, err:%v", args.SrcPath, err)
		return errors.New("invalid url " + args.SrcPath)
	}

	codehost, err := codehost.GetCodeHostInfo(
		&codehost.CodeHostOption{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
	if err != nil {
		log.Errorf("GetCodeHostInfo failed, srcPath:%s, err:%v", args.SrcPath, err)
		return err
	}

	if pathType == "blob" {
		ctx := context.Background()
		tokenSource := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: codehost.AccessToken},
		)
		tokenclient := oauth2.NewClient(ctx, tokenSource)
		githubClient := github.NewClient(tokenclient)

		fileContent, _, _, err := githubClient.Repositories.GetContents(ctx, owner, repo, args.LoadPath, &github.RepositoryContentGetOptions{Ref: branch})
		if err != nil {
			log.Errorf("cannot get file content for loadPath: %s", args.SrcPath)
			return err
		}
		svcContent, _ := fileContent.GetContent()
		splittedYaml := SplitYaml(svcContent)
		args.KubeYamls = splittedYaml
		args.Yaml = svcContent
		return nil
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("NewRequest failed, url:%s, err:%v", url, err)
		return err
	}
	request.Header.Set("Authorization", fmt.Sprintf("token %s", codehost.AccessToken))
	var ret *http.Response
	client := &http.Client{}
	ret, err = client.Do(request)
	if err != nil {
		log.Errorf("client.Do failed, url:%s, err:%v", url, err)
		return err
	}

	defer func() { _ = ret.Body.Close() }()
	var body []byte
	body, err = ioutil.ReadAll(ret.Body)
	if err != nil {
		log.Errorf("ioutil.ReadAll failed, url:%s, err:%v", url, err)
		return err
	}
	repositoryContent := make([]github.RepositoryContent, 0)
	fileUnmarshalError := json.Unmarshal(body, &repositoryContent)
	if fileUnmarshalError != nil {
		log.Errorf("Unmarshal failed, url:%s, err:%v", url, fileUnmarshalError)
		return err
	}

	files := make([]string, 0)
	for _, fileContent := range repositoryContent {
		// 排除目录
		if *fileContent.Type != "file" {
			continue
		}
		fileName := strings.ToLower(*fileContent.Path)
		if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
			continue
		}

		url := *fileContent.URL
		file, err := syncSingleFileFromGithub(url, codehost.AccessToken, log)
		if err != nil {
			log.Errorf("syncSingleFileFromGithub failed, url:%s, err:%v", url, err)
			continue
		}
		files = append(files, file)
	}

	args.KubeYamls = files
	args.Yaml = joinYamls(files)
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

func syncSingleFileFromGithub(url, token string, log *xlog.Logger) (string, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("NewRequest failed, url:%s,err:%v", url, err)
		return "", err
	}

	request.Header.Set("Authorization", fmt.Sprintf("token %s", token))
	var ret *http.Response
	client := &http.Client{}
	ret, err = client.Do(request)
	if err != nil {
		log.Errorf("client.Do failed, url:%s, err:%v", url, err)
		return "", err
	}

	defer func() { _ = ret.Body.Close() }()
	var body []byte
	body, err = ioutil.ReadAll(ret.Body)
	if err != nil {
		return "", err
	}

	fileContent := new(githubFileContent)
	fileUnmarshalError := json.Unmarshal(body, &fileContent)
	if fileUnmarshalError != nil {
		log.Errorf("Unmarshal failed, url:%s, err:%v", url, fileUnmarshalError)
		return "", err
	}

	keyBytes, err := base64.StdEncoding.DecodeString(fileContent.Content)
	if err != nil {
		fmt.Printf("decode fileContent error: %v", err)
		return "", err
	}

	return string(keyBytes), nil
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
