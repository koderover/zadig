package codehub

import (
	"encoding/json"
	"fmt"
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
	body, err := c.sendRequest("GET", fmt.Sprintf("/v1/projects/%s/repositories?search=%s&page_size=%d", projectUUID, search, pageSize), "")
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
