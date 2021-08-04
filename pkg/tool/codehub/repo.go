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

package codehub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/koderover/zadig/pkg/util"
)

type RepoInfo struct {
	Result RepoInfoResult `json:"result"`
	Status string         `json:"status"`
}

type RepoInfoResult struct {
	Total       int           `json:"total"`
	Repositorys []Repositorys `json:"repositorys"`
}

type Repositorys struct {
	ID              string `json:"id"`
	RepoID          string `json:"repoId"`
	Name            string `json:"name"`
	SSHURL          string `json:"sshUrl"`
	HTTPURL         string `json:"httpUrl"`
	GroupName       string `json:"groupName"`
	WebURL          string `json:"webUrl"`
	VisibilityLevel int    `json:"visibilityLevel"`
	CreateAt        string `json:"createAt"`
	ProjectID       string `json:"projectId"`
	ProjectIsDelete string `json:"projectIsDelete"`
}

type Project struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	DefaultBranch string `json:"defaultBranch"`
	Namespace     string `json:"namespace"`
	RepoUUID      string `json:"repo_uuid"`
	RepoID        string `json:"repo_id"`
}

func (c *CodeHubClient) RepoList(projectUUID, search string, pageSize int) ([]*Project, error) {
	repoInfos := make([]*Project, 0)

	repoInfo := new(RepoInfo)
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/projects/%s/repositories?search=%s&page_size=%d", projectUUID, search, pageSize), []byte{})
	if err != nil {
		return repoInfos, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(repoInfo); err != nil {
		return repoInfos, err
	}

	if repoInfo.Result.Total == 0 {
		return repoInfos, nil
	}

	for _, repository := range repoInfo.Result.Repositorys {
		repoInfos = append(repoInfos, &Project{
			RepoUUID:      repository.ID,
			Name:          repository.Name,
			DefaultBranch: "master",
			Namespace:     repository.GroupName,
			RepoID:        repository.RepoID,
		})
	}

	return repoInfos, nil
}

func (c *CodeHubClient) GetYAMLContents(repoUUID, branchName, path string, isDir, split bool) ([]string, error) {
	var res []string
	if !isDir {
		if !(strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")) {
			return nil, nil
		}

		fileContent, err := c.FileContent(repoUUID, branchName, path)
		if err != nil {
			return nil, err
		}

		contentByte, err := base64.StdEncoding.DecodeString(fileContent.Content)
		content := string(contentByte)
		if split {
			res = util.SplitManifests(content)
		} else {
			res = []string{content}
		}

		return res, nil
	}

	treeNodes, err := c.FileTree(repoUUID, branchName, path)
	if err != nil {
		return nil, err
	}

	for _, tn := range treeNodes {
		r, err := c.GetYAMLContents(repoUUID, branchName, tn.Path, tn.Type == "tree", split)
		if err != nil {
			return nil, err
		}

		res = append(res, r...)
	}

	return res, nil
}
