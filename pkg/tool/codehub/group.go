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
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
)

type RepoListInfo struct {
	Result RepoListInfoResult `json:"result"`
	Status string             `json:"status"`
}

type RepoListInfoResult struct {
	Total        int            `json:"total"`
	Repositories []Repositories `json:"repositories"`
}

type Repositories struct {
	Star             bool        `json:"star"`
	Status           int         `json:"status"`
	UserRole         interface{} `json:"userRole"`
	RepositoryUUID   string      `json:"repository_uuid"`
	RepositoryID     int         `json:"repository_id"`
	RepositoryName   string      `json:"repository_name"`
	SSHURL           string      `json:"ssh_url"`
	HTTPSURL         string      `json:"https_url"`
	GroupName        string      `json:"group_name"`
	WebURL           string      `json:"web_url"`
	VisibilityLevel  int         `json:"visibility_level"`
	CreatedAt        string      `json:"created_at"`
	UpdatedAt        string      `json:"updated_at"`
	RepositorySize   string      `json:"repository_size"`
	LfsSize          string      `json:"lfs_size"`
	CreatorName      string      `json:"creator_name"`
	DomainName       string      `json:"domain_name"`
	IsOwner          int         `json:"is_owner"`
	IamUserUUID      string      `json:"iam_user_uuid"`
	ProjectUUID      string      `json:"project_uuid"`
	ProjectIsDeleted string      `json:"project_is_deleted"`
}

type Namespace struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	ProjectUUID string `json:"project_uuid,omitempty"`
	RepoUUID    string `json:"repo_uuid,omitempty"`
	RepoName    string `json:"repo_name,omitempty"`
}

func (c *CodeHubClient) NamespaceList() ([]*Namespace, error) {
	groupInfos := make([]*Namespace, 0)
	repoListInfo := new(RepoListInfo)
	body, err := c.sendRequest("GET", "/v2/projects/repositories?page_size=100", []byte{})
	if err != nil {
		return groupInfos, err
	}
	defer body.Close()

	if err = json.NewDecoder(body).Decode(repoListInfo); err != nil {
		return groupInfos, err
	}
	if repoListInfo.Result.Total == 0 {
		return groupInfos, nil
	}
	groupNames := sets.NewString()
	for _, repository := range repoListInfo.Result.Repositories {
		if repository.ProjectIsDeleted == "true" {
			continue
		}
		if groupNames.Has(repository.GroupName) {
			continue
		}
		groupNames.Insert(repository.GroupName)
		groupInfos = append(groupInfos, &Namespace{
			Name:        repository.CreatorName,
			Path:        repository.GroupName,
			ProjectUUID: repository.ProjectUUID,
			RepoUUID:    repository.RepositoryUUID,
			RepoName:    repository.RepositoryName,
			Kind:        "user",
		})
	}
	return groupInfos, nil
}

func (c *CodeHubClient) GetRepoUUID(repoName string) (string, error) {
	repos, err := c.NamespaceList()
	if err != nil {
		return "", err
	}
	for _, repo := range repos {
		if repo.RepoName == repoName {
			return repo.RepoUUID, nil
		}
	}
	return "", fmt.Errorf("not found repoUUID by repoName:%s", repoName)
}
