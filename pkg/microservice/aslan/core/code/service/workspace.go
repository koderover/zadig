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

package service

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	githubservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/github"
	gitlabservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/gitlab"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/shared/poetry"
	"github.com/koderover/zadig/pkg/tool/codehub"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
)

func GetRepoTree(codeHostID int, owner, repo, path, branch string, log *zap.SugaredLogger) ([]*git.TreeNode, error) {

	ch, err := codehost.GetCodeHostInfoByID(codeHostID)
	if err != nil {
		log.Errorf("Failed to get codeHost by id %d, err: %s", codeHostID, err)
		return nil, e.ErrListWorkspace.AddDesc(err.Error())
	}
	getter, err := getTreeGetter(ch)
	if err != nil {
		log.Errorf("Failed to get tree getter, err: %s", err)
		return nil, e.ErrListWorkspace.AddDesc(err.Error())
	}

	fileInfos, err := getter.GetTree(owner, repo, path, branch)
	if err != nil {
		return nil, e.ErrListWorkspace.AddDesc(err.Error())
	}

	return fileInfos, nil
}

type treeGetter interface {
	GetTree(owner, repo, path, branch string) ([]*git.TreeNode, error)
}

func getTreeGetter(ch *poetry.CodeHost) (treeGetter, error) {
	switch ch.Type {
	case setting.SourceFromGithub:
		return githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr()), nil
	case setting.SourceFromGitlab:
		return gitlabservice.NewClient(ch.Address, ch.AccessToken)
	default:
		// should not have happened here
		log.DPanicf("invalid source: %s", ch.Type)
		return nil, fmt.Errorf("invalid source: %s", ch.Type)
	}
}

func CleanWorkspace(username, pipelineName string, log *zap.SugaredLogger) error {
	wsPath, err := getWorkspaceBasePath(pipelineName)
	if err != nil {
		return e.ErrCleanWorkspace.AddErr(err)
	}

	log.Infof("user %s requests delete workspace %s", username, wsPath)

	// 清理工作目录
	if err := os.RemoveAll(wsPath); err != nil {
		return e.ErrCleanWorkspace.AddErr(err)
	}

	// 创建工作目录
	if err := os.MkdirAll(wsPath, os.ModePerm); err != nil {
		return e.ErrCleanWorkspace.AddErr(err)
	}

	return nil
}

func getWorkspaceBasePath(pipelineName string) (string, error) {
	pipe, err := commonrepo.NewPipelineColl().Find(&commonrepo.PipelineFindOption{Name: pipelineName})
	if err != nil {
		return "", err
	}

	//base := path.Join(s.Config.NFS.Path, pipe.Name)
	base := path.Join(config.S3StoragePath(), pipe.Name)

	if _, err := os.Stat(base); os.IsNotExist(err) {
		return "", err
	}

	return base, nil
}

func GetWorkspaceFilePath(username, pipelineName, file string, log *zap.SugaredLogger) (string, error) {
	base, err := getWorkspaceBasePath(pipelineName)
	if err != nil {
		return "", e.ErrListWorkspace.AddDesc(err.Error())
	}

	filePath := path.Join(base, file)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", err
	}

	return filePath, nil
}

type FileInfo struct {
	// parent path of the file
	Parent string `json:"parent"`
	// base name of the file
	Name string `json:"name"`
	// length in bytes for regular files; system-dependent for others
	Size int64 `json:"size"`
	// file mode bits
	Mode os.FileMode `json:"mode"`
	// modification time
	ModTime int64 `json:"mod_time"`
	// abbreviation for Mode().IsDir()
	IsDir bool `json:"is_dir"`
}

func GetGitRepoInfo(codehostID int, repoOwner, repoName, branchName, remoteName, dir string, log *zap.SugaredLogger) ([]*FileInfo, error) {
	fis := make([]*FileInfo, 0)
	if dir == "" {
		dir = "/"
	}

	base := path.Join(config.S3StoragePath(), repoName)
	if err := os.RemoveAll(base); err != nil {
		log.Errorf("dir remove err:%v", err)
	}
	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		log.Errorf("GetGitRepoInfo GetCodehostDetail err:%v", err)
		return fis, e.ErrListRepoDir.AddDesc(err.Error())
	}
	err = command.RunGitCmds(detail, repoOwner, repoName, branchName, remoteName)
	if err != nil {
		log.Errorf("GetGitRepoInfo runGitCmds err:%v", err)
		return fis, e.ErrListRepoDir.AddDesc(err.Error())
	}
	files, err := ioutil.ReadDir(path.Join(base, dir))
	if err != nil {
		return fis, e.ErrListRepoDir.AddDesc(err.Error())
	}

	for _, file := range files {
		if file.Name() == ".git" && file.IsDir() {
			continue
		}
		fi := &FileInfo{
			Parent:  dir,
			Name:    file.Name(),
			Size:    file.Size(),
			Mode:    file.Mode(),
			ModTime: file.ModTime().Unix(),
			IsDir:   file.IsDir(),
		}

		fis = append(fis, fi)
	}
	return fis, nil
}

func GetPublicGitRepoInfo(urlPath, dir string, log *zap.SugaredLogger) ([]*FileInfo, error) {
	fis := make([]*FileInfo, 0)

	if dir == "" {
		dir = "/"
	}
	if !strings.Contains(urlPath, "https") && !strings.Contains(urlPath, "http") {
		return fis, e.ErrListRepoDir.AddDesc("url is illegal")
	}
	uri, err := url.Parse(urlPath)
	if err != nil {
		return fis, e.ErrListRepoDir.AddDesc("url parse failed")
	}
	host := uri.Host
	if host != "github.com" {
		return fis, e.ErrListRepoDir.AddDesc("only support github")
	}
	uriPath := uri.Path
	repoNameArr := strings.Split(uriPath, "/")
	repoName := ""
	if len(repoNameArr) == 3 {
		repoName = repoNameArr[2]
	}
	if repoName == "" {
		return fis, e.ErrListRepoDir.AddDesc("repoName not found")
	}

	base := path.Join(config.S3StoragePath(), repoName)
	if err := os.RemoveAll(base); err != nil {
		log.Errorf("dir remove err:%v", err)
	}
	err = command.RunGitCmds(&codehost.Detail{Address: urlPath, Source: "github"}, "", repoName, "main", "origin")
	if err != nil {
		log.Errorf("GetPublicGitRepoInfo runGitCmds err:%v", err)
		return fis, e.ErrListRepoDir.AddDesc(err.Error())
	}

	files, err := ioutil.ReadDir(path.Join(base, dir))
	if err != nil {
		return fis, e.ErrListRepoDir.AddDesc(err.Error())
	}

	for _, file := range files {
		if file.Name() == ".git" && file.IsDir() {
			continue
		}
		fi := &FileInfo{
			Parent:  dir,
			Name:    file.Name(),
			Size:    file.Size(),
			Mode:    file.Mode(),
			ModTime: file.ModTime().Unix(),
			IsDir:   file.IsDir(),
		}

		fis = append(fis, fi)
	}

	return fis, nil
}

type CodehostFileInfo struct {
	Name     string `json:"name"`
	Size     int    `json:"size"`
	IsDir    bool   `json:"is_dir"`
	FullPath string `json:"full_path"`
}

// 获取codehub的目录内容接口
func GetCodehubRepoInfo(codehostID int, repoUUID, branchName, path string, log *zap.SugaredLogger) ([]*CodehostFileInfo, error) {
	fileInfos := make([]*CodehostFileInfo, 0)

	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		log.Errorf("GetCodehubRepoInfo GetCodehostDetail err:%s", err)
		return fileInfos, e.ErrListWorkspace.AddDesc(err.Error())
	}

	codeHubClient := codehub.NewCodeHubClient(detail.AccessKey, detail.SecretKey, detail.Region)
	treeNodes, err := codeHubClient.FileTree(repoUUID, branchName, path)
	if err != nil {
		log.Errorf("Failed to list tree from codehub err:%s", err)
		return nil, err
	}
	for _, treeInfo := range treeNodes {
		fileInfos = append(fileInfos, &CodehostFileInfo{
			Name:     treeInfo.Name,
			Size:     0,
			IsDir:    treeInfo.Type == "tree",
			FullPath: treeInfo.Path,
		})
	}
	return fileInfos, nil
}
