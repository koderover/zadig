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
	"context"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/google/go-github/v35/github"
	"github.com/xanzy/go-gitlab"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/tool/codehub"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

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
	err = command.RunGitCmds(&codehost.Detail{Address: urlPath, Source: "github"}, "", repoName, "master", "origin")
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

func GetGithubRepoInfo(codehostID int, repoName, branchName, path string, log *zap.SugaredLogger) ([]*CodehostFileInfo, error) {
	fileInfo := make([]*CodehostFileInfo, 0)

	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		log.Errorf("GetGithubRepoInfo GetCodehostDetail err:%v", err)
		return fileInfo, e.ErrListWorkspace.AddDesc(err.Error())
	}
	ctx := context.Background()
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: detail.OauthToken},
	)
	tokenClient := oauth2.NewClient(ctx, tokenSource)
	githubClient := github.NewClient(tokenClient)

	_, dirContent, _, err := githubClient.Repositories.GetContents(ctx, detail.Owner, repoName, path, &github.RepositoryContentGetOptions{Ref: branchName})
	if err != nil {
		return nil, e.ErrListWorkspace.AddDesc(err.Error())
	}
	for _, file := range dirContent {
		fileInfo = append(fileInfo, &CodehostFileInfo{
			Name:     file.GetName(),
			Size:     file.GetSize(),
			IsDir:    file.GetType() == "dir",
			FullPath: file.GetPath(),
		})
	}
	return fileInfo, nil
}

// 获取gitlab的目录内容接口
func GetGitlabRepoInfo(codehostID int, repoName, branchName, path string, log *zap.SugaredLogger) ([]*CodehostFileInfo, error) {
	fileInfos := make([]*CodehostFileInfo, 0)

	detail, err := codehost.GetCodehostDetail(codehostID)
	if err != nil {
		log.Errorf("GetGitlabRepoInfo GetCodehostDetail err:%v", err)
		return fileInfos, e.ErrListWorkspace.AddDesc(err.Error())
	}
	gitlabClient, err := gitlab.NewOAuthClient(detail.OauthToken, gitlab.WithBaseURL(detail.Address))
	if err != nil {
		log.Errorf("GetGitlabRepoInfo Prepare gitlab client err:%v", err)
		return fileInfos, e.ErrListWorkspace.AddDesc(err.Error())
	}
	nextPage := "1"
	for nextPage != "" {
		pagenum, err := strconv.Atoi(nextPage)
		if err != nil {
			log.Errorf("Failed to get the amount of entries from gitlab, err: %v", err)
			return nil, err
		}
		opt := &gitlab.ListTreeOptions{
			ListOptions: gitlab.ListOptions{Page: pagenum},
			Path:        gitlab.String(path),
			Ref:         gitlab.String(branchName),
			Recursive:   gitlab.Bool(false),
		}
		treeNodes, resp, err := gitlabClient.Repositories.ListTree(repoName, opt)
		if err != nil {
			log.Errorf("Failed to list tree from gitlab")
			return nil, err
		}
		for _, entry := range treeNodes {
			fileInfos = append(fileInfos, &CodehostFileInfo{
				Name:     entry.Name,
				Size:     0,
				IsDir:    entry.Type == "tree",
				FullPath: entry.Path,
			})
		}
		nextPage = resp.Header.Get("x-next-page")
	}
	return fileInfos, nil
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
